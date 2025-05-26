package interpodaffinity

import (
	"context"
	"fmt"
	"github.com/ops-tool/pkg/scheduler/framework"
	"github.com/ops-tool/pkg/util"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/workqueue"
	"strings"
	"sync/atomic"
)

type InterPodAffinity struct {
	ClientSet                                    *kubernetes.Clientset
	AllNodes                                     []*framework.Node
	havePodsWithAffinityNodeInfoList             []*framework.Node
	havePodsWithRequiredAntiAffinityNodeInfoList []*framework.Node
}

func createNodeInfoMap(pods []v1.Pod, allnodes []v1.Node) map[string]*framework.Node {
	nodeNameToInfo := make(map[string]*framework.Node)
	for _, pod := range pods {
		nodeName := pod.Spec.NodeName
		if _, ok := nodeNameToInfo[nodeName]; !ok {
			nodeNameToInfo[nodeName] = framework.NewNodeInfo()
		}
		nodeNameToInfo[nodeName].AddPod(&pod)
	}

	for _, node := range allnodes {
		if _, ok := nodeNameToInfo[node.Name]; !ok {
			nodeNameToInfo[node.Name] = framework.NewNodeInfo()
		}
		nodeInfo := nodeNameToInfo[node.Name]
		nodeInfo.SetNode(&node)
	}
	return nodeNameToInfo
}

func NewInterPodAffinityFilter(clientset *kubernetes.Clientset, allPods []v1.Pod, allNodes []v1.Node) *InterPodAffinity {

	nodeInfoMap := createNodeInfoMap(allPods, allNodes)
	nodeInfoList := make([]*framework.Node, 0, len(nodeInfoMap))
	havePodsWithAffinityNodeInfoList := make([]*framework.Node, 0, len(nodeInfoMap))
	havePodsWithRequiredAntiAffinityNodeInfoList := make([]*framework.Node, 0, len(nodeInfoMap))
	for _, v := range nodeInfoMap {
		nodeInfoList = append(nodeInfoList, v)
		if len(v.PodsWithAffinity) > 0 {
			havePodsWithAffinityNodeInfoList = append(havePodsWithAffinityNodeInfoList, v)
		}
		if len(v.PodsWithRequiredAntiAffinity) > 0 {
			havePodsWithRequiredAntiAffinityNodeInfoList = append(havePodsWithRequiredAntiAffinityNodeInfoList, v)
		}
	}

	return &InterPodAffinity{
		ClientSet: clientset,
		AllNodes:  nodeInfoList,
		havePodsWithRequiredAntiAffinityNodeInfoList: havePodsWithRequiredAntiAffinityNodeInfoList,
	}
}

type topologyPair struct {
	key   string
	value string
}
type topologyToMatchedTermCount map[topologyPair]int64

func (m topologyToMatchedTermCount) merge(toMerge topologyToMatchedTermCount) {
	for pair, count := range toMerge {
		m[pair] += count
	}
}

func (m topologyToMatchedTermCount) mergeWithList(toMerge topologyToMatchedTermCountList) {
	for _, tmtc := range toMerge {
		m[tmtc.topologyPair] += tmtc.count
	}
}

func (m topologyToMatchedTermCount) clone() topologyToMatchedTermCount {
	copy := make(topologyToMatchedTermCount, len(m))
	copy.merge(m)
	return copy
}

func (m topologyToMatchedTermCount) update(node *v1.Node, tk string, value int64) {
	if tv, ok := node.Labels[tk]; ok {
		pair := topologyPair{key: tk, value: tv}
		m[pair] += value
		// value could be negative, hence we delete the entry if it is down to zero.
		if m[pair] == 0 {
			delete(m, pair)
		}
	}
}

// updates the topologyToMatchedTermCount map with the specified value
// for each affinity term if "targetPod" matches ALL terms.
func (m topologyToMatchedTermCount) updateWithAffinityTerms(
	terms []framework.AffinityTerm, pod *v1.Pod, node *v1.Node, value int64) {
	if podMatchesAllAffinityTerms(terms, pod) {
		for _, t := range terms {
			m.update(node, t.TopologyKey, value)
		}
	}
}

// updates the topologyToMatchedTermCount map with the specified value
// for each anti-affinity term matched the target pod.
func (m topologyToMatchedTermCount) updateWithAntiAffinityTerms(terms []framework.AffinityTerm, pod *v1.Pod, nsLabels labels.Set, node *v1.Node, value int64) {
	// Check anti-affinity terms.
	for _, t := range terms {
		if t.Matches(pod, nsLabels) {
			m.update(node, t.TopologyKey, value)
		}
	}
}
func (pl *InterPodAffinity) mergeAffinityTermNamespacesIfNotEmpty(at *framework.AffinityTerm) error {
	if at.NamespaceSelector.Empty() {
		return nil
	}
	ns, err := pl.ClientSet.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{
		LabelSelector: at.NamespaceSelector.String(),
	})
	if err != nil {
		return err
	}
	for _, n := range ns.Items {
		at.Namespaces.Insert(n.Name)
	}
	at.NamespaceSelector = labels.Nothing()
	return nil
}

type topologyToMatchedTermCountList []topologyPairCount

type topologyPairCount struct {
	topologyPair topologyPair
	count        int64
}

func (m *topologyToMatchedTermCountList) append(node *v1.Node, tk string, value int64) {
	if tv, ok := node.Labels[tk]; ok {
		pair := topologyPair{key: tk, value: tv}
		*m = append(*m, topologyPairCount{
			topologyPair: pair,
			count:        value,
		})
	}
}

// appends the specified value to the topologyToMatchedTermCountList
// for each affinity term if "targetPod" matches ALL terms.
func (m *topologyToMatchedTermCountList) appendWithAffinityTerms(
	terms []framework.AffinityTerm, pod *v1.Pod, node *v1.Node, value int64) {
	if podMatchesAllAffinityTerms(terms, pod) {
		for _, t := range terms {
			m.append(node, t.TopologyKey, value)
		}
	}
}

// appends the specified value to the topologyToMatchedTermCountList
// for each anti-affinity term matched the target pod.
func (m *topologyToMatchedTermCountList) appendWithAntiAffinityTerms(terms []framework.AffinityTerm, pod *v1.Pod, nsLabels labels.Set, node *v1.Node, value int64) {
	// Check anti-affinity terms.
	for _, t := range terms {
		if t.Matches(pod, nsLabels) {
			m.append(node, t.TopologyKey, value)
		}
	}
}

// returns true IFF the given pod matches all the given terms.
func podMatchesAllAffinityTerms(terms []framework.AffinityTerm, pod *v1.Pod) bool {
	if len(terms) == 0 {
		return false
	}
	for _, t := range terms {
		// The incoming pod NamespaceSelector was merged into the Namespaces set, and so
		// we are not explicitly passing in namespace labels.
		if !t.Matches(pod, nil) {
			return false
		}
	}
	return true
}

func (pl *InterPodAffinity) getExistingAntiAffinityCounts(ctx context.Context, pod *v1.Pod, nsLabels labels.Set, nodes []*framework.Node) topologyToMatchedTermCount {
	antiAffinityCountsList := make([]topologyToMatchedTermCountList, len(nodes))
	index := int32(-1)
	processNode := func(i int) {
		nodeInfo := nodes[i]
		node := nodeInfo.Node

		antiAffinityCounts := make(topologyToMatchedTermCountList, 0)
		for _, existingPod := range nodeInfo.PodsWithRequiredAntiAffinity {
			antiAffinityCounts.appendWithAntiAffinityTerms(existingPod.RequiredAntiAffinityTerms, pod, nsLabels, &node, 1)
		}
		if len(antiAffinityCounts) != 0 {
			antiAffinityCountsList[atomic.AddInt32(&index, 1)] = antiAffinityCounts
		}
	}
	workqueue.ParallelizeUntil(ctx, 16, len(nodes), processNode)

	result := make(topologyToMatchedTermCount)
	// Traditional for loop is slightly faster in this case than its "for range" equivalent.
	for i := 0; i <= int(index); i++ {
		result.mergeWithList(antiAffinityCountsList[i])
	}

	return result
}

func (pl *InterPodAffinity) getIncomingAffinityAntiAffinityCounts(ctx context.Context, podInfo *framework.PodInfo, allNodes []*framework.Node) (topologyToMatchedTermCount, topologyToMatchedTermCount) {
	affinityCounts := make(topologyToMatchedTermCount)
	antiAffinityCounts := make(topologyToMatchedTermCount)
	if len(podInfo.RequiredAffinityTerms) == 0 && len(podInfo.RequiredAntiAffinityTerms) == 0 {
		return affinityCounts, antiAffinityCounts
	}

	affinityCountsList := make([]topologyToMatchedTermCountList, len(allNodes))
	antiAffinityCountsList := make([]topologyToMatchedTermCountList, len(allNodes))
	index := int32(-1)
	processNode := func(i int) {
		nodeInfo := allNodes[i]
		node := nodeInfo.Node

		affinity := make(topologyToMatchedTermCountList, 0)
		antiAffinity := make(topologyToMatchedTermCountList, 0)
		for _, existingPod := range nodeInfo.Pods {
			affinity.appendWithAffinityTerms(podInfo.RequiredAffinityTerms, existingPod.Pod, &node, 1)
			// The incoming pod's terms have the namespaceSelector merged into the namespaces, and so
			// here we don't lookup the existing pod's namespace labels, hence passing nil for nsLabels.
			antiAffinity.appendWithAntiAffinityTerms(podInfo.RequiredAntiAffinityTerms, existingPod.Pod, nil, &node, 1)
		}

		if len(affinity) > 0 || len(antiAffinity) > 0 {
			k := atomic.AddInt32(&index, 1)
			affinityCountsList[k] = affinity
			antiAffinityCountsList[k] = antiAffinity
		}
	}
	workqueue.ParallelizeUntil(ctx, 16, len(allNodes), processNode)

	for i := 0; i <= int(index); i++ {
		affinityCounts.mergeWithList(affinityCountsList[i])
		antiAffinityCounts.mergeWithList(antiAffinityCountsList[i])
	}

	return affinityCounts, antiAffinityCounts
}

func (pl *InterPodAffinity) GetNamespaceLabelsSnapshot(ns string) (nsLabels labels.Set) {
	podNS, err := pl.ClientSet.CoreV1().Namespaces().Get(context.Background(), ns, metav1.GetOptions{})
	if err == nil {
		// Create and return snapshot of the labels.
		return labels.Merge(podNS.Labels, nil)
	}
	fmt.Printf("error fetching namespace %s labels\n", ns)

	return
}

type preFilterState struct {
	// A map of topology pairs to the number of existing pods that has anti-affinity terms that match the "pod".
	existingAntiAffinityCounts topologyToMatchedTermCount
	// A map of topology pairs to the number of existing pods that match the affinity terms of the "pod".
	affinityCounts topologyToMatchedTermCount
	// A map of topology pairs to the number of existing pods that match the anti-affinity terms of the "pod".
	antiAffinityCounts topologyToMatchedTermCount
	// podInfo of the incoming pod.
	podInfo *framework.PodInfo
	// A copy of the incoming pod's namespace labels.
	namespaceLabels labels.Set
}

func (pl *InterPodAffinity) preFilter(pod *v1.Pod) (*preFilterState, error) {
	//

	podInfo, err := framework.NewPodInfo(pod)
	if err != nil {
		return nil, fmt.Errorf("parsing pod: %+v", err)
	}

	for i := range podInfo.RequiredAffinityTerms {
		if err := pl.mergeAffinityTermNamespacesIfNotEmpty(&podInfo.RequiredAffinityTerms[i]); err != nil {
			return nil, err
		}
	}
	for i := range podInfo.RequiredAntiAffinityTerms {
		if err := pl.mergeAffinityTermNamespacesIfNotEmpty(&podInfo.RequiredAntiAffinityTerms[i]); err != nil {
			return nil, err
		}
	}

	s := &preFilterState{}
	if s.podInfo, err = framework.NewPodInfo(pod); err != nil {
		return nil, err
	}

	for i := range s.podInfo.RequiredAffinityTerms {
		if err := pl.mergeAffinityTermNamespacesIfNotEmpty(&s.podInfo.RequiredAffinityTerms[i]); err != nil {
			return nil, err
		}
	}

	for i := range s.podInfo.RequiredAntiAffinityTerms {
		if err := pl.mergeAffinityTermNamespacesIfNotEmpty(&s.podInfo.RequiredAntiAffinityTerms[i]); err != nil {
			return nil, err
		}
	}

	s.namespaceLabels = pl.GetNamespaceLabelsSnapshot(pod.Namespace)
	s.existingAntiAffinityCounts = pl.getExistingAntiAffinityCounts(context.Background(), pod, s.namespaceLabels, pl.havePodsWithRequiredAntiAffinityNodeInfoList)
	s.affinityCounts, s.antiAffinityCounts = pl.getIncomingAffinityAntiAffinityCounts(context.Background(), s.podInfo, pl.AllNodes)

	if len(s.existingAntiAffinityCounts) == 0 && len(s.podInfo.RequiredAffinityTerms) == 0 && len(s.podInfo.RequiredAntiAffinityTerms) == 0 {
		return s, nil
	}

	return s, nil
}

func (pl *InterPodAffinity) Filter(pod *v1.Pod, node *v1.Node) util.ColorTextList {
	//fmt.Printf("checking pod affinity...\n")
	state, err := pl.preFilter(pod)
	var result util.ColorTextList
	if err != nil {
		return util.ColorTextList{
			util.NewRedText(fmt.Sprintf("pre-filtering pod in interpodAffinity Failed %s/%s: %+v", pod.Namespace, pod.Name, err)),
		}
	}
	if podAffinityReason := satisfyPodAffinity(state, node); podAffinityReason != nil {
		result = append(result, podAffinityReason...)
	}

	if podAntiAffinityReason := satisfyPodAntiAffinity(state, node); podAntiAffinityReason != nil {
		result = append(result, podAntiAffinityReason...)
	}

	if existingPodAntiAffinityReason := satisfyExistingPodsAntiAffinity(state, node); existingPodAntiAffinityReason != nil {
		result = append(result, existingPodAntiAffinityReason...)
	}

	//fmt.Printf("check pod affinity result: %s\n", strings.Join(result, "\n"))
	return result
}

func satisfyPodAffinity(state *preFilterState, nodeInfo *v1.Node) util.ColorTextList {
	podsExist := true
	var notInNodeTopologyKey, inNodeTopologyKey []string
	if state.podInfo.RequiredAffinityTerms == nil || len(state.podInfo.RequiredAffinityTerms) == 0 {
		return nil
	}
	for _, term := range state.podInfo.RequiredAffinityTerms {
		if topologyValue, ok := nodeInfo.Labels[term.TopologyKey]; ok {
			tp := topologyPair{key: term.TopologyKey, value: topologyValue}
			if state.affinityCounts[tp] <= 0 {
				podsExist = false
			} else {
				inNodeTopologyKey = append(inNodeTopologyKey, fmt.Sprintf("%s:%s", term.TopologyKey, topologyValue))
			}
		} else {
			// All topology labels must exist on the node.
			notInNodeTopologyKey = append(notInNodeTopologyKey, fmt.Sprintf("%s", term.TopologyKey))
			//return false
		}
	}
	if len(notInNodeTopologyKey) > 0 {
		return util.ColorTextList{
			//util.NewGreenText(strings.Join([]string{"Satisfied Pod Affinity:", strings.Join(inNodeTopologyKey, ",")}, ",")),
			util.NewRedText(strings.Join([]string{"Not Satisfied Pod Affinity:\t", strings.Join(notInNodeTopologyKey, ",")}, ",")),
		}
	}
	if !podsExist {
		// This pod may be the first pod in a series that have affinity to themselves. In order
		// to not leave such pods in pending state forever, we check that if no other pod
		// in the cluster matches the namespace and selector of this pod, the pod matches
		// its own terms, and the node has all the requested topologies, then we allow the pod
		// to pass the affinity check.
		if len(state.affinityCounts) == 0 && podMatchesAllAffinityTerms(state.podInfo.RequiredAffinityTerms, state.podInfo.Pod) {
			return nil
		}
		return util.ColorTextList{
			util.NewRedText("pod dont's match self affinity."),
		}
	}
	return nil
}

func satisfyPodAntiAffinity(state *preFilterState, nodeInfo *v1.Node) util.ColorTextList {
	var notPassAntiAffinity, passAntiAffinity []string
	if len(state.antiAffinityCounts) > 0 {
		for _, term := range state.podInfo.RequiredAntiAffinityTerms {
			if topologyValue, ok := nodeInfo.Labels[term.TopologyKey]; ok {
				tp := topologyPair{key: term.TopologyKey, value: topologyValue}
				if state.antiAffinityCounts[tp] > 0 {
					notPassAntiAffinity = append(notPassAntiAffinity, fmt.Sprintf("%s:%s,", term.TopologyKey, topologyValue))
				} else {
					passAntiAffinity = append(passAntiAffinity, fmt.Sprintf("%s:%s,", term.TopologyKey, topologyValue))
				}
			}
		}
	}
	if len(notPassAntiAffinity) == 0 {
		return nil
	}
	return util.ColorTextList{
		//util.NewGreenText(strings.Join([]string{"Satisfied Pod Anti-Affinity:,", strings.Join(passAntiAffinity, "")}, "")),
		util.NewRedText(strings.Join([]string{"Not Satisfied Pod Anti-Affinity:\t", strings.Join(notPassAntiAffinity, "")}, "")),
	}

}

func satisfyExistingPodsAntiAffinity(state *preFilterState, nodeInfo *v1.Node) util.ColorTextList {
	var notPassExistingAntiAffinity, passExistingAntiAffinity []string
	if len(state.existingAntiAffinityCounts) > 0 {
		// Iterate over topology pairs to get any of the pods being affected by
		// the scheduled pod anti-affinity terms
		for topologyKey, topologyValue := range nodeInfo.Labels {
			tp := topologyPair{key: topologyKey, value: topologyValue}
			if state.existingAntiAffinityCounts[tp] > 0 {
				notPassExistingAntiAffinity = append(notPassExistingAntiAffinity, fmt.Sprintf("%s:%s,", topologyKey, topologyValue))
			} else {
				passExistingAntiAffinity = append(passExistingAntiAffinity, fmt.Sprintf("%s:%s,", topologyKey, topologyValue))
			}
		}
	}
	if len(notPassExistingAntiAffinity) == 0 {
		return nil
	}
	//return util.StringListToColorTextList(append([]string{"Not Satisfied existing Pod Anti-Affinity:\n"}, notPassExistingAntiAffinity...), "red")
	return util.ColorTextList{
		//util.NewGreenText(strings.Join([]string{"Satisfied existing Pod Anti-Affinity:", strings.Join(passExistingAntiAffinity, "")}, "")),
		util.NewRedText(strings.Join([]string{"Not Satisfied existing Pod Anti-Affinity:\n", strings.Join(notPassExistingAntiAffinity, "")}, "")),
	}

}
