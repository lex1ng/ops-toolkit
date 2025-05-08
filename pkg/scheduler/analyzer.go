package scheduler

import (
	"context"
	"fmt"
	"github.com/ops-tool/pkg/nodes"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Analyzer struct {
	ClientSet            *kubernetes.Clientset
	Namespace            string
	PodName              string
	TargetConditions     *Conditions
	NodeResourceReporter nodes.NodeResourceReporter
	NodeReport           NodeReport
}

type NodeReport []*Report

func (nr *NodeReport) Print() {

	fmt.Print("hello")
}

type Conditions struct {
	NodeSelector             map[string]string
	Affinity                 *corev1.Affinity
	ResourceRequirement      nodes.ResourceList
	Toleration               []v1.Toleration
	PersistentVolumeAffinity []*v1.VolumeNodeAffinity
}

func (a *Analyzer) Why() error {

	pod, err := a.ClientSet.CoreV1().Pods(a.Namespace).Get(context.Background(), a.PodName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get unschedulable pod, err: %v", err)
	}

	a.TargetConditions = &Conditions{
		NodeSelector:             pod.Spec.NodeSelector,
		Affinity:                 pod.Spec.Affinity,
		ResourceRequirement:      BuildResourceList(pod),
		Toleration:               pod.Spec.Tolerations,
		PersistentVolumeAffinity: a.GetRelatedPVAffinity(pod),
	}

	nodeList, err := a.NodeResourceReporter.BuildNodeList()

	if err != nil {
		return err
	}

	var nodeReport NodeReport
	for _, node := range nodeList {
		report := a.DiagnoseNode(node)
		nodeReport = append(nodeReport, report)
	}
	nodeReport.Print()
	return nil

}

func (a *Analyzer) DiagnoseNode(node nodes.Node) *Report {

	notMeetSelector := checkNodeSelector(a.TargetConditions.NodeSelector, node.Node.Labels)

	untolerateTaints := checkTaints(a.TargetConditions.Toleration, node.Node.Spec.Taints)

	unMatchVolumeAffinity := CheckVolumeNodeAffinity(a.TargetConditions.PersistentVolumeAffinity, node.Labels)
}

func (a *Analyzer) GetRelatedPVAffinity(pod *v1.Pod) []*v1.VolumeNodeAffinity {

	var pvcNames []string
	var pvAffinity []*v1.VolumeNodeAffinity
	for _, volume := range pod.Spec.Volumes {
		if volume.PersistentVolumeClaim.ClaimName != "" {
			pvcNames = append(pvcNames, volume.PersistentVolumeClaim.ClaimName)
		}
	}
	for _, pvcName := range pvcNames {

		pvc, err := a.ClientSet.CoreV1().PersistentVolumeClaims(pod.Namespace).Get(context.Background(), pvcName, metav1.GetOptions{})
		if err != nil {
			continue
		}

		pv, err := a.ClientSet.CoreV1().PersistentVolumes().Get(context.Background(), pvc.Spec.VolumeName, metav1.GetOptions{})
		if err != nil {
			continue
		}

		pvAffinity = append(pvAffinity, pv.Spec.NodeAffinity)

	}
	return pvAffinity
}

func BuildResourceList(pod *v1.Pod) nodes.ResourceList {

	reqs, limits := nodes.GetPodsTotalRequestsAndLimits(&v1.PodList{
		Items: []v1.Pod{*pod},
	})

	result := make(nodes.ResourceList)
	for name, req := range reqs {
		limit := limits[name]
		result[name.String()] = &nodes.Resource{
			Name:     name.String(),
			Requests: req.Value(),
			Limits:   limit.Value(),
		}

	}
	return result
}
