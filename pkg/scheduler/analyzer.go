package scheduler

import (
	"context"
	"fmt"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/ops-tool/pkg/scheduler/framework"
	"github.com/ops-tool/pkg/scheduler/framework/interpodaffinity"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"os"
	"strings"
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
	PersistentVolumeAffinity []*v1.VolumeNodeAffinity
}

func (a *Analyzer) Why() error {

	var nodeReports []*Report
	for _, node := range a.allNodes {
		report := a.DiagnoseNode(&node)
		nodeReports = append(nodeReports, report)
	}

	printReport(nodeReports)
	return nil

}

func printReport(report []*Report) {

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(framework.ListToRow(ReportHeader))

	for _, r := range report {
		t.AppendRow(framework.ListToRow(r.ToStringList()))
	}

	columnConfigs := make([]table.ColumnConfig, len(ReportHeader))
	for i := range ReportHeader {
		columnConfigs[i] = table.ColumnConfig{
			Number:      i + 1,            // 列号从 1 开始
			Align:       text.AlignCenter, // 设置居中对齐
			AlignHeader: text.AlignCenter, // 设置居中对齐
		}
	}
	t.SetColumnConfigs(columnConfigs)
	style := table.StyleDefault
	style.Format.Header = 0
	t.SetStyle(style)
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

func BuildPVAffinity(clientset *kubernetes.Clientset, pod *v1.Pod) []*v1.VolumeNodeAffinity {

	var pvcNames []string
	var pvAffinity []*v1.VolumeNodeAffinity
	for _, volume := range pod.Spec.Volumes {
		if volume.PersistentVolumeClaim != nil && volume.PersistentVolumeClaim.ClaimName != "" {
			pvcNames = append(pvcNames, volume.PersistentVolumeClaim.ClaimName)
		}
	}
	for _, pvcName := range pvcNames {

		pvc, err := clientset.CoreV1().PersistentVolumeClaims(pod.Namespace).Get(context.Background(), pvcName, metav1.GetOptions{})
		if err != nil {
			continue
		}

		pv, err := clientset.CoreV1().PersistentVolumes().Get(context.Background(), pvc.Spec.VolumeName, metav1.GetOptions{})
		if err != nil {
			continue
		}

		pvAffinity = append(pvAffinity, pv.Spec.NodeAffinity)

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
