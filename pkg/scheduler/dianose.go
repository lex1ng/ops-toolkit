package scheduler

import (
	"context"
	"fmt"
	"github.com/ops-tool/pkg/nodes"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	componenthelpers "k8s.io/component-helpers/scheduling/corev1"

	"strings"
)

func checkNodeSelector(selector, nodeLabels map[string]string) []string {
	// noSelector: meet
	var notMeetSelector []string
	if selector == nil {
		return notMeetSelector
	}

	for k, v := range selector {
		if nodeLabels[k] != v {
			notMeetSelector = append(notMeetSelector, strings.Join([]string{k, v}, ":"))
		}
	}
	return notMeetSelector
}

func checkTaints(tolerations []corev1.Toleration, taints []corev1.Taint) []corev1.Taint {

	var untolerableTaints []corev1.Taint

	if taints == nil || len(taints) == 0 {
		return untolerableTaints
	}

	for _, taint := range taints {
		var tolerate bool
		for _, toleration := range tolerations {
			if toleration.ToleratesTaint(&taint) {
				tolerate = true
				break
			}
		}
		if !tolerate {
			untolerableTaints = append(untolerableTaints, taint)
		}
	}

	return untolerableTaints

}

func checkAffinity(affinity *corev1.Affinity, node nodes.Node) {

	if affinity == nil {
		return
	}
	podAffinity := affinity.PodAffinity
	podAntiAffinity := affinity.PodAntiAffinity

	nodeAffinityMatch, err := checkNodeAffinity(affinity.NodeAffinity, node)
	if err != nil {
		return
	}

}

func (a *Analyzer) checkPodAffinity(podAffinity *corev1.PodAffinity, node nodes.Node, namespace string) (bool, error) {
	//if podAffinity == nil {
	//	return true, nil
	//}
	//if podAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil || len(podAffinity.RequiredDuringSchedulingIgnoredDuringExecution) == 0 {
	//	return true, nil
	//}

	//topologyValue := node.Node.Labels[podAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0].TopologyKey]
	//
	//pods, err := a.ClientSet.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
	//	FieldSelector: "spec.Name=" + node.Name,
	//})
	//
	//if err != nil {
	//	return false, fmt.Errorf("get pod list when check pod affinity of node %s error: %v", node.Name, err)
	//}
	//
	//for _, term := range podAffinity.RequiredDuringSchedulingIgnoredDuringExecution {
	//	for _, pod := range pods.Items {
	//		if metav1.A(&pod, term.LabelSelector) {
	//			return true
	//		}
	//	}
	//}

	return false, nil
}

func checkNodeAffinity(nodeAffinity *corev1.NodeAffinity, node nodes.Node) (bool, error) {
	if nodeAffinity == nil {
		return true, nil
	}
	nodeAffinityRequired := nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution

	if nodeAffinityRequired == nil {
		return true, nil
	}

	tmpNode := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Labels: node.Labels}}
	NodeAffinityMatches := true
	if matches, err := componenthelpers.MatchNodeSelectorTerms(tmpNode, nodeAffinityRequired); err != nil {
		fmt.Printf("check match node selector terms on node %s error: %v", nodeAffinityRequired)
		return false, err
	} else if !matches {
		NodeAffinityMatches = false
		fmt.Printf("node %s not match node selector terms %v", node.Name, nodeAffinityRequired)
	}

	return NodeAffinityMatches, nil

}
func CheckVolumeNodeAffinity(volumeNodeAffinities []*corev1.VolumeNodeAffinity, nodeLabels map[string]string) []*corev1.VolumeNodeAffinity {
	var notMatchNodeAffinity []*corev1.VolumeNodeAffinity
	for _, volumeNodeAffinity := range volumeNodeAffinities {
		if volumeNodeAffinity == nil {
			continue
		}

		if volumeNodeAffinity.Required != nil {
			node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Labels: nodeLabels}}
			terms := volumeNodeAffinity.Required
			if matches, err := componenthelpers.MatchNodeSelectorTerms(node, terms); err != nil {
				fmt.Printf("check match node selector terms on node %s error: %v", volumeNodeAffinity.Required)
			} else if !matches {
				notMatchNodeAffinity = append(notMatchNodeAffinity, volumeNodeAffinity)
			}
		}
	}

	return notMatchNodeAffinity
}

func checkResource(have, want nodes.ResourceList) []string {

	var notMeetResource []string
	for k, v := range want {
		reason := ""
		if h, ok := have[k]; !ok {
			reason = fmt.Sprintf("want %v: %s, have 0", v.Requests, k)
		} else {
			if h.Requests+v.Requests > h.Capacity {
				reason = fmt.Sprintf("want %v: %s, have %v", v.Requests, k, h.Capacity-h.Requests)
			}
		}
		if reason != "" {
			notMeetResource = append(notMeetResource, reason)
		}
	}

	return notMeetResource

}
