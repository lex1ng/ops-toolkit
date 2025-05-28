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

func (a *Analyzer) checkNodeSelector(nodeLabels map[string]string) util.ColorTextList {
	//fmt.Printf("checking node selector...\n")
	selector := a.TargetConditions.NodeSelector
	// noSelector: meet
	var notMeetSelector, meetSelector []string
	if selector == nil {
		return nil
	}

	for k, v := range selector {
		nodeV, ok := nodeLabels[k]
		if !ok || nodeV != v {
			notMeetSelector = append(notMeetSelector, strings.Join([]string{k, v}, ":"))
		} else {
			meetSelector = append(meetSelector, strings.Join([]string{k, v}, ":"))
		}
	}
	//fmt.Printf("not meet selector: %s\n", strings.Join(notMeetSelector, "\n"))
	return util.ColorTextList{
		util.NewGreenText(strings.Join(meetSelector, ",")),
		util.NewRedText(strings.Join(notMeetSelector, ",")),
	}
}

func (a *Analyzer) checkTaints(taints []corev1.Taint) util.ColorTextList {
	//fmt.Printf("checking taints...\n")
	tolerations := a.TargetConditions.Toleration
	var untolerableTaints, tolerableTaints []string

	if taints == nil || len(taints) == 0 {
		return nil
	}

	for _, taint := range taints {
		var tolerate bool
		for _, toleration := range tolerations {
			if toleration.ToleratesTaint(&taint) {
				tolerate = true
				break
			}
		}
		toSave := fmt.Sprintf("%s,%s,%s", taint.Key, taint.Value, taint.Effect)
		if !tolerate {
			//fmt.Printf("untolerate taints: %s\n", util.ToJSONIndent(taint))
			untolerableTaints = append(untolerableTaints, toSave)
		} else {
			tolerableTaints = append(tolerableTaints, toSave)
		}
	}

	//fmt.Printf("untolerable taints: %s\n", strings.Join(untolerableTaints, "\n"))
	return util.ColorTextList{
		util.NewGreenText(strings.Join(tolerableTaints, ",")),
		util.NewRedText(strings.Join(untolerableTaints, ",")),
	}

}

func (a *Analyzer) checkUnSchedulableNode(node *corev1.Node) util.ColorTextList {
	//fmt.Printf("checking unschedulable node...\n")
	toleration := a.TargetConditions.Toleration
	if !node.Spec.Unschedulable {
		return util.ColorTextList{
			util.NewGreenText(fmt.Sprintf("node schedulable")),
		}
	}

	// If pod tolerate unschedulable taint, it's also tolerate `node.Spec.Unschedulable`.
	podToleratesUnschedulable := componenthelpers.TolerationsTolerateTaint(toleration, &corev1.Taint{
		Key:    corev1.TaintNodeUnschedulable,
		Effect: corev1.TaintEffectNoSchedule,
	})
	if !podToleratesUnschedulable {
		return util.ColorTextList{
			util.NewRedText(fmt.Sprintf("pod not tolerate unschedulable")),
		}
	}

	return util.ColorTextList{
		util.NewGreenText(fmt.Sprintf("pod tolerates unschedulable")),
	}

}

func (a *Analyzer) checkNodeAffinity(node *corev1.Node) util.ColorTextList {
	//fmt.Printf("checking node affinity...\n")
	if a.TargetConditions.Affinity == nil || a.TargetConditions.Affinity.NodeAffinity == nil {
		return nil
	}
	nodeAffinity := a.TargetConditions.Affinity.NodeAffinity
	if nodeAffinity == nil {
		return nil
	}
	nodeAffinityRequired := nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution

	if nodeAffinityRequired == nil {
		return nil
	}

	tmpNode := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Labels: node.Labels}}

	matches, err := componenthelpers.MatchNodeSelectorTerms(tmpNode, nodeAffinityRequired)
	if err != nil {
		return util.ColorTextList{
			util.NewRedText(err.Error()),
		}
	}
	target := nodeAffinityRequired.NodeSelectorTerms[0].MatchExpressions[0]
	toSave := fmt.Sprintf("pod have node affinity: %s %s %s", target.Key, target.Operator, target.Values[0])
	if matches {
		return util.ColorTextList{
			util.NewGreenText(toSave),
		}
	}

	// TODO show which one is not match
	//fmt.Printf("don't match node affinity: %s\n", util.ToJSONIndent(nodeAffinityRequired))
	return util.ColorTextList{
		util.NewRedText(toSave),
	}

}

func findPVNodeName(inputs []corev1.NodeSelectorRequirement) string {
	if inputs == nil || len(inputs) == 0 {
		return ""
	}
	for _, input := range inputs {
		if input.Key == "directpv.min.io/node" || input.Key == "kubernetes.io/hostname" {
			return input.Values[0]
		}
	}

	return ""
}
func (a *Analyzer) checkVolumeNodeAffinity(nodeLabels map[string]string) util.ColorTextList {
	//fmt.Printf("checking volume node affinity...\n")
	volumeNodeAffinities := a.TargetConditions.PersistentVolumeAffinity
	var notMatchNodeAffinity, matchNodeAffinity []string
	for _, pvcStatus := range volumeNodeAffinities {
		if pvcStatus == nil {
			continue
		}
		volumeNodeAffinity := pvcStatus.PVVolumeAffinity
		if volumeNodeAffinity.Required != nil {
			node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Labels: nodeLabels}}
			terms := volumeNodeAffinity.Required
			//toSave := fmt.Sprintf("pvc %s's pv %s in %s", pvcStatus.Name, pvcStatus.PVName, terms.NodeSelectorTerms[0].MatchExpressions[0].Values[0])
			//toSave := fmt.Sprintf("pv %s in %s", pvcStatus.PVName, terms.NodeSelectorTerms[0].MatchExpressions[0].Values[0])
			toSave := fmt.Sprintf("pv %s in %s", pvcStatus.PVName, findPVNodeName(terms.NodeSelectorTerms[0].MatchExpressions))
			if matches, err := componenthelpers.MatchNodeSelectorTerms(node, terms); err != nil {
				matchNodeAffinity = append(matchNodeAffinity, toSave)
				//fmt.Printf("check match node selector terms on node %s error: %s", node.Name, util.ToJSONIndent(volumeNodeAffinity.Required))
			} else if !matches {
				//fmt.Printf("not match volumeNodeAffinity: %s\n", util.ToJSONIndent(volumeNodeAffinity.Required))
				notMatchNodeAffinity = append(notMatchNodeAffinity, toSave)
			}
		}
	}

	//fmt.Printf("not match volumeNodeAffinity: %s\n", strings.Join(notMatchNodeAffinity, "\n"))
	return util.ColorTextList{
		util.NewGreenText(strings.Join(matchNodeAffinity, ",")),
		util.NewRedText(strings.Join(notMatchNodeAffinity, ",")),
	}
}
func (a *Analyzer) doCheckResource(want, have framework.ResourceList) util.ColorTextList {
	//fmt.Printf("checking resource...\n")
	var notMeetResource, meetResource []string

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
		} else {
			meetResource = append(meetResource, fmt.Sprintf("%s: have %d", k, v.Requests))
		}
	}
	//fmt.Printf("not meet resource: %s\n", strings.Join(notMeetResource, "\n"))
	//return strings.Join(notMeetResource, "\n")
	return util.ColorTextList{
		util.NewGreenText(strings.Join(meetResource, ",")),
		util.NewRedText(strings.Join(notMeetResource, ",")),
	}

}
func (a *Analyzer) checkResource(node *corev1.Node) util.ColorTextList {
	want := a.TargetConditions.ResourceRequirement
	have, err := framework.BuildAllocatedResourceMap(a.ClientSet, node)

	if err != nil {
		return util.ColorTextList{
			util.NewRedText(fmt.Sprintf("cannot build node allocated resource")),
		}
	}

	return a.doCheckResource(want, have)

}

func (a *Analyzer) checkPodAffinity(node *corev1.Node) util.ColorTextList {
	return a.interPodAffinityPlugin.Filter(a.targetPod, node)
}
