package scheduler

import (
	"context"
	"fmt"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/ops-tool/pkg/scheduler/framework"
	"github.com/ops-tool/pkg/scheduler/framework/interpodaffinity"
	"github.com/ops-tool/pkg/util"
	"golang.org/x/term"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"os"
	"runtime"
	"strings"
	"sync"
)

var ReportHeader = []string{"nodeName", "Unschedulable", "nodeSelector", "nodeAffinity", "podAffinity", "Toleration", "resource", "PV"}

type Analyzer struct {
	ClientSet              *kubernetes.Clientset
	targetPod              *v1.Pod
	Namespace              string
	PodName                string
	TargetConditions       *Conditions
	NodeReport             NodeReport
	allNodes               []v1.Node
	interPodAffinityPlugin *interpodaffinity.InterPodAffinity
}

func NewAnalyzer(clientSet *kubernetes.Clientset, podNamespace, podName string) (*Analyzer, error) {

	pod, err := clientSet.CoreV1().Pods(podNamespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get pod %s/%s: %w", podNamespace, podName, err)
	}

	cond := &Conditions{
		NodeSelector:             pod.Spec.NodeSelector,
		Affinity:                 pod.Spec.Affinity,
		ResourceRequirement:      BuildResourceList(pod),
		Toleration:               pod.Spec.Tolerations,
		PersistentVolumeAffinity: BuildPVAffinity(clientSet, pod),
	}

	allPods, err := clientSet.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	allNodes, err := clientSet.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	interPodAffinityPlugin := interpodaffinity.NewInterPodAffinityFilter(clientSet, allPods.Items, allNodes.Items)

	return &Analyzer{
		ClientSet:              clientSet,
		targetPod:              pod,
		Namespace:              podNamespace,
		PodName:                podName,
		TargetConditions:       cond,
		allNodes:               allNodes.Items,
		interPodAffinityPlugin: interPodAffinityPlugin,
	}, nil

}

type NodeReport []*Report

func (nr *NodeReport) Print() {

	fmt.Print("hello")
}

type Conditions struct {
	NodeSelector             map[string]string
	Affinity                 *corev1.Affinity
	ResourceRequirement      framework.ResourceList
	Toleration               []v1.Toleration
	PersistentVolumeAffinity []*PVCStatus
}

type PVCStatus struct {
	Name             string
	PVName           string
	PVVolumeAffinity *v1.VolumeNodeAffinity
}

func (a *Analyzer) Why() error {

	var nodeReports []*Report
	for _, node := range a.allNodes {
		fmt.Printf("start dianose node %s ****************\n", node.Name)
		report := a.DiagnoseNodeMulti(&node)
		nodeReports = append(nodeReports, report)
		fmt.Printf("dianose node %s done  ****************\n", node.Name)
	}
	printReport(nodeReports)
	return nil

}

func getTerminalWidth() int {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return 120 // 默认 fallback 宽度
	}
	return width
}
func printReport(report []*Report) {

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(util.ListToRow(ReportHeader))

	for _, r := range report {
		t.AppendRow(util.ListToRow(r.ToStringList()))
	}
	t.SetStyle(table.Style{
		Name: "MyStyle",
		Box:  table.StyleBoxRounded, // 圆角边框
		Options: table.Options{
			DrawBorder:      true, // 启用外边框
			SeparateColumns: true, // 列分隔线
			SeparateRows:    true, // 行分隔线（核心配置）
			SeparateFooter:  true,
			SeparateHeader:  true,
		},
		Color: table.ColorOptions{
			Separator: text.Colors{text.FgHiCyan}, // 行线颜色
			Border:    text.Colors{text.FgHiCyan},
		},
	})

	termWidth := getTerminalWidth()
	numCols := len(ReportHeader)

	margin := numCols + 1 // 估计边框 + padding + 总体留白
	availableWidth := termWidth - margin
	if availableWidth <= 0 {
		availableWidth = 40 // fallback 宽度
	}

	// 3. 每列最大宽度 = 总宽度 / 列数
	widthPerCol := availableWidth / numCols
	if widthPerCol > 40 {
		widthPerCol = 40 // 限制最大单列宽度
	}
	fmt.Printf("calculated widthPerCol: %d", widthPerCol)

	columnConfigs := make([]table.ColumnConfig, len(ReportHeader))
	for i := range ReportHeader {
		columnConfigs[i] = table.ColumnConfig{
			Number: i + 1, // 列号从 1 开始
			//Align:       text.AlignLeft, // 设置居中对齐
			//AlignHeader: text.AlignLeft, // 设置居中对齐
			WidthMax: widthPerCol,
		}

	}

	t.SetColumnConfigs(columnConfigs)
	//style := table.StyleDefault
	//style.Format.Header = 0
	//t.SetStyle(style)
	fmt.Println()
	t.Render()
}

func (a *Analyzer) DiagnoseNode(node *v1.Node) *Report {

	return &Report{
		NodeName:               strings.Split(node.Name, "-")[0],
		NodeUnschedulable:      a.checkUnSchedulableNode(node),
		NodeSelectorReason:     a.checkNodeSelector(node.Labels),
		TolerationReason:       a.checkTaints(node.Spec.Taints),
		PersistentVolumeReason: a.checkVolumeNodeAffinity(node.Labels),
		ResourceReason:         a.checkResource(node),
		PodAffinityReason:      a.checkPodAffinity(node),
		NodeAffinityReason:     a.checkNodeAffinity(node),
	}
}

type CheckTask struct {
	checkFunc func() util.ColorTextList // 检查函数
	result    *util.ColorTextList       // 结果存储指针
}

func (a *Analyzer) DiagnoseNodeMulti(node *v1.Node) *Report {
	report := &Report{NodeName: strings.Split(node.Name, "-")[0]}

	// 定义所有检查任务
	tasks := []CheckTask{
		{checkFunc: func() util.ColorTextList { return a.checkUnSchedulableNode(node) }, result: &report.NodeUnschedulable},
		{checkFunc: func() util.ColorTextList { return a.checkNodeSelector(node.Labels) }, result: &report.NodeSelectorReason},
		{checkFunc: func() util.ColorTextList { return a.checkTaints(node.Spec.Taints) }, result: &report.TolerationReason},
		{checkFunc: func() util.ColorTextList { return a.checkVolumeNodeAffinity(node.Labels) }, result: &report.PersistentVolumeReason},
		{checkFunc: func() util.ColorTextList { return a.checkResource(node) }, result: &report.ResourceReason},
		{checkFunc: func() util.ColorTextList { return a.checkPodAffinity(node) }, result: &report.PodAffinityReason},
		{checkFunc: func() util.ColorTextList { return a.checkNodeAffinity(node) }, result: &report.NodeAffinityReason},
	}
	// 创建带缓冲的任务通道（避免阻塞）
	taskChan := make(chan CheckTask, len(tasks))
	var wg sync.WaitGroup

	// 启动 Worker 协程（建议按 CPU 核数限制并发）
	for i := 0; i < runtime.NumCPU(); i++ {
		go func() {
			for task := range taskChan {
				*task.result = task.checkFunc()
				wg.Done()
			}
		}()
	}

	// 分发任务
	wg.Add(len(tasks))
	for _, task := range tasks {
		taskChan <- task
	}
	close(taskChan)
	wg.Wait()

	return report

}

func BuildPVAffinity(clientset *kubernetes.Clientset, pod *v1.Pod) []*PVCStatus {

	var pvAffinity []*PVCStatus
	for _, volume := range pod.Spec.Volumes {
		if volume.PersistentVolumeClaim == nil || volume.PersistentVolumeClaim.ClaimName == "" {
			continue
		}

		pvcName := volume.PersistentVolumeClaim.ClaimName

		pvc, err := clientset.CoreV1().PersistentVolumeClaims(pod.Namespace).Get(context.Background(), pvcName, metav1.GetOptions{})
		if err != nil {
			continue
		}

		pv, err := clientset.CoreV1().PersistentVolumes().Get(context.Background(), pvc.Spec.VolumeName, metav1.GetOptions{})
		if err != nil {
			continue
		}

		pvAffinity = append(pvAffinity, &PVCStatus{
			Name:             pvcName,
			PVName:           pv.Name,
			PVVolumeAffinity: pv.Spec.NodeAffinity,
		})
	}

	return pvAffinity
}

func BuildResourceList(pod *v1.Pod) framework.ResourceList {

	reqs, limits := framework.GetPodsTotalRequestsAndLimits(&v1.PodList{
		Items: []v1.Pod{*pod},
	})

	result := make(framework.ResourceList)
	for name, req := range reqs {
		limit := limits[name]
		result[name.String()] = &framework.Resource{
			Name:     name.String(),
			Requests: req.Value(),
			Limits:   limit.Value(),
		}

	}
	return result
}
