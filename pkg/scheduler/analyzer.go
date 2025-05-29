package scheduler

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"

	"github.com/ops-tool/pkg/util"
	"github.com/schollz/progressbar/v3"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/ops-tool/pkg/scheduler/framework"
	"github.com/ops-tool/pkg/scheduler/framework/interpodaffinity"
)

var ReportHeader = []string{"nodeName", "Unschedulable", "nodeSelector", "nodeAffinity", "podAffinity", "Toleration", "resource", "PV"}

type Analyzer struct {
	ClientSet              *kubernetes.Clientset
	targetPod              *v1.Pod
	Namespace              string
	PodName                string
	TargetConditions       *Conditions
	allNodes               []v1.Node
	interPodAffinityPlugin *interpodaffinity.InterPodAffinity
}

func filterOutNode(nodeList *v1.NodeList) *v1.NodeList {
	roles := []string{"node-role.kubernetes.io/controller", "node-role.kubernetes.io/gw", "node-role.kubernetes.io/master"}
	result := &v1.NodeList{}
	for _, node := range nodeList.Items {

		want := false
		for _, role := range roles {
			if _, ok := node.Labels[role]; ok {
				want = true
				break
			}
		}
		if want {
			result.Items = append(result.Items, node)
		} else {
			fmt.Printf("filter out node %s\n", node.Name)
		}
	}
	return result
}
func NewAnalyzer(clientSet *kubernetes.Clientset, podNamespace, podName string) (*Analyzer, error) {

	pod, err := clientSet.CoreV1().Pods(podNamespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get pod %s/%s: %w", podNamespace, podName, err)
	}

	cond := &Conditions{
		NodeSelector:             pod.Spec.NodeSelector,
		Affinity:                 pod.Spec.Affinity,
		ResourceRequirement:      framework.BuildPodResourceList(pod),
		Toleration:               pod.Spec.Tolerations,
		PersistentVolumeAffinity: framework.BuildPVAffinity(clientSet, pod),
	}

	allPods, err := clientSet.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	allNodes, err := clientSet.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	//allNodes = filterOutNode(allNodes)

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

func (a *Analyzer) Why() error {

	var nodeReports []*Report

	bar := progressbar.NewOptions(len(a.allNodes),
		progressbar.OptionSetDescription("Diagnosing nodes"),
		progressbar.OptionShowCount(),
		progressbar.OptionSetWidth(40),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "#",
			SaucerHead:    ">",
			SaucerPadding: "-",
			BarStart:      "[",
			BarEnd:        "]",
		}),
	)

	for _, node := range a.allNodes {
		//fmt.Printf("start diagnose node %s ****************\n", node.Name)
		report := a.DiagnoseNodeMulti(&node)
		nodeReports = append(nodeReports, report)
		//fmt.Printf("done ****************\n")
		bar.Add(1)
	}
	printReport(nodeReports)
	return nil
}

type Conditions struct {
	NodeSelector             map[string]string
	Affinity                 *corev1.Affinity
	ResourceRequirement      framework.ResourceList
	Toleration               []v1.Toleration
	PersistentVolumeAffinity []*framework.PVCStatus
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
