package scheduler

import (
	"fmt"
	"github.com/ops-tool/pkg/scheduler/framework"
	"github.com/ops-tool/pkg/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	componenthelpers "k8s.io/component-helpers/scheduling/corev1"

	"strings"
)

func (a *Analyzer) checkNodeSelector(nodeLabels map[string]string) string {
	selector := a.TargetConditions.NodeSelector
	// noSelector: meet
	var notMeetSelector []string
	if selector == nil {
		return ""
	}

	for k, v := range selector {
		nodeV, ok := nodeLabels[k]
		if !ok || nodeV != v {
			notMeetSelector = append(notMeetSelector, strings.Join([]string{k, v}, ":"))
		}
	}

	return strings.Join(notMeetSelector, "\n")
}

func (a *Analyzer) checkTaints(taints []corev1.Taint) string {
	tolerations := a.TargetConditions.Toleration
	var untolerableTaints []string

	if taints == nil || len(taints) == 0 {
		return ""
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
			fmt.Printf("untolerate taints: %s\n", util.ToJSONIndent(taint))
			untolerableTaints = append(untolerableTaints, util.ToJSONIndent(taint))
		}
	}
	return strings.Join(untolerableTaints, "\n")

}

func (a *Analyzer) checkUnSchedulableNode(node *corev1.Node) string {
	toleration := a.TargetConditions.Toleration
	if !node.Spec.Unschedulable {
		return ""
	}

	// If pod tolerate unschedulable taint, it's also tolerate `node.Spec.Unschedulable`.
	podToleratesUnschedulable := componenthelpers.TolerationsTolerateTaint(toleration, &corev1.Taint{
		Key:    corev1.TaintNodeUnschedulable,
		Effect: corev1.TaintEffectNoSchedule,
	})
	if !podToleratesUnschedulable {
		return "node unSchedulable"
	}

	return ""

}

func (a *Analyzer) checkNodeAffinity(node *corev1.Node) string {
	if a.TargetConditions.Affinity == nil || a.TargetConditions.Affinity.NodeAffinity == nil {
		return ""
	}
	nodeAffinity := a.TargetConditions.Affinity.NodeAffinity
	if nodeAffinity == nil {
		return ""
	}
	nodeAffinityRequired := nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution

	if nodeAffinityRequired == nil {
		return ""
	}

	tmpNode := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Labels: node.Labels}}

	matches, err := componenthelpers.MatchNodeSelectorTerms(tmpNode, nodeAffinityRequired)
	if err != nil {
		return fmt.Sprintf("check node affinity failed: %v", err)
	}

	if matches {
		fmt.Printf("node %s match node selector terms %s", node.Name, nodeAffinityRequired.String())
		return ""
	}

	fmt.Printf("don't match node affinity: %s", util.ToJSON(nodeAffinityRequired))
	return fmt.Sprintf("don't match node affinity: %s", util.ToJSON(nodeAffinityRequired))

}
func (a *Analyzer) checkVolumeNodeAffinity(nodeLabels map[string]string) string {
	volumeNodeAffinities := a.TargetConditions.PersistentVolumeAffinity
	var notMatchNodeAffinity []string
	for _, volumeNodeAffinity := range volumeNodeAffinities {
		if volumeNodeAffinity == nil {
			continue
		}

		if volumeNodeAffinity.Required != nil {
			node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Labels: nodeLabels}}
			terms := volumeNodeAffinity.Required
			if matches, err := componenthelpers.MatchNodeSelectorTerms(node, terms); err != nil {
				fmt.Printf("check match node selector terms on node %s error: %s", node.Name, util.ToJSONIndent(volumeNodeAffinity.Required))
			} else if !matches {
				fmt.Printf("not match volumeNodeAffinity: %s\n", util.ToJSONIndent(volumeNodeAffinity.Required))
				notMatchNodeAffinity = append(notMatchNodeAffinity, util.ToJSONIndent(volumeNodeAffinity.Required))
			}
		}
	}

	return strings.Join(notMatchNodeAffinity, "\n")
}
func (a *Analyzer) doCheckResource(want, have framework.ResourceList) string {

	var notMeetResource []string
	for k, v := range want {
		reason := ""
		if h, ok := have[k]; !ok {
			reason = fmt.Sprintf("%s: want %d, have 0", k, v.Requests)
		} else {
			if h.Requests+v.Requests > h.Capacity {
				reason = fmt.Sprintf("%s: want %d, have %d left", k, v.Requests, h.Capacity-h.Requests)
			}
		}
		if reason != "" {
			notMeetResource = append(notMeetResource, reason)
		}
	}

	return strings.Join(notMeetResource, "\n")

}
func (a *Analyzer) checkResource(node *corev1.Node) string {
	want := a.TargetConditions.ResourceRequirement
	have, err := framework.BuildAllocatedResourceMap(a.ClientSet, node)

	if err != nil {
		return "cannot build node allocated resource"
	}

	return a.doCheckResource(want, have)

}

func (a *Analyzer) checkPodAffinity(node *corev1.Node) string {
	podAffinityReason, err := a.interPodAffinityPlugin.Filter(a.targetPod, node)
	if err != nil {
		return fmt.Sprintf("check affinity error %s", err.Error())
	}
	return podAffinityReason
}
