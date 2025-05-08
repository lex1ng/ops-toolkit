package interpodaffinity

import (
	"context"
	"fmt"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sync/atomic"
)

type InterPodAffinity struct {
	ClientSet *kubernetes.Clientset
}

func NewInterPodAffinityFilter(clientset *kubernetes.Clientset) *InterPodAffinity {

	return &InterPodAffinity{
		ClientSet: clientset,
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
	terms []AffinityTerm, pod *v1.Pod, node *v1.Node, value int64) {
	if podMatchesAllAffinityTerms(terms, pod) {
		for _, t := range terms {
			m.update(node, t.TopologyKey, value)
		}
	}
}

// updates the topologyToMatchedTermCount map with the specified value
// for each anti-affinity term matched the target pod.
func (m topologyToMatchedTermCount) updateWithAntiAffinityTerms(terms []AffinityTerm, pod *v1.Pod, nsLabels labels.Set, node *v1.Node, value int64) {
	// Check anti-affinity terms.
	for _, t := range terms {
		if t.Matches(pod, nsLabels) {
			m.update(node, t.TopologyKey, value)
		}
	}
}
func (pl *InterPodAffinity) mergeAffinityTermNamespacesIfNotEmpty(at *AffinityTerm) error {
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
	terms []AffinityTerm, pod *v1.Pod, node *v1.Node, value int64) {
	if podMatchesAllAffinityTerms(terms, pod) {
		for _, t := range terms {
			m.append(node, t.TopologyKey, value)
		}
	}
}

// appends the specified value to the topologyToMatchedTermCountList
// for each anti-affinity term matched the target pod.
func (m *topologyToMatchedTermCountList) appendWithAntiAffinityTerms(terms []AffinityTerm, pod *v1.Pod, nsLabels labels.Set, node *v1.Node, value int64) {
	// Check anti-affinity terms.
	for _, t := range terms {
		if t.Matches(pod, nsLabels) {
			m.append(node, t.TopologyKey, value)
		}
	}
}

// returns true IFF the given pod matches all the given terms.
func podMatchesAllAffinityTerms(terms []AffinityTerm, pod *v1.Pod) bool {
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
func (pl *InterPodAffinity) getExistingAntiAffinityCounts(ctx context.Context, pod *v1.Pod, nsLabels labels.Set, nodes []*framework.NodeInfo) topologyToMatchedTermCount {
	antiAffinityCountsList := make([]topologyToMatchedTermCountList, len(nodes))
	index := int32(-1)
	processNode := func(i int) {
		nodeInfo := nodes[i]
		node := nodeInfo.Node()

		antiAffinityCounts := make(topologyToMatchedTermCountList, 0)
		for _, existingPod := range nodeInfo.PodsWithRequiredAntiAffinity {
			antiAffinityCounts.appendWithAntiAffinityTerms(existingPod.RequiredAntiAffinityTerms, pod, nsLabels, node, 1)
		}
		if len(antiAffinityCounts) != 0 {
			antiAffinityCountsList[atomic.AddInt32(&index, 1)] = antiAffinityCounts
		}
	}
	pl.parallelizer.Until(ctx, len(nodes), processNode, pl.Name())

	result := make(topologyToMatchedTermCount)
	// Traditional for loop is slightly faster in this case than its "for range" equivalent.
	for i := 0; i <= int(index); i++ {
		result.mergeWithList(antiAffinityCountsList[i])
	}

	return result
}

func (pl *InterPodAffinity) getIncomingAffinityAntiAffinityCounts(ctx context.Context, podInfo *PodInfo, allNodes []*framework.NodeInfo) (topologyToMatchedTermCount, topologyToMatchedTermCount) {
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
		node := nodeInfo.Node()

		affinity := make(topologyToMatchedTermCountList, 0)
		antiAffinity := make(topologyToMatchedTermCountList, 0)
		for _, existingPod := range nodeInfo.Pods {
			affinity.appendWithAffinityTerms(podInfo.RequiredAffinityTerms, existingPod.Pod, node, 1)
			// The incoming pod's terms have the namespaceSelector merged into the namespaces, and so
			// here we don't lookup the existing pod's namespace labels, hence passing nil for nsLabels.
			antiAffinity.appendWithAntiAffinityTerms(podInfo.RequiredAntiAffinityTerms, existingPod.Pod, nil, node, 1)
		}

		if len(affinity) > 0 || len(antiAffinity) > 0 {
			k := atomic.AddInt32(&index, 1)
			affinityCountsList[k] = affinity
			antiAffinityCountsList[k] = antiAffinity
		}
	}
	pl.parallelizer.Until(ctx, len(allNodes), processNode, pl.Name())

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
	return
}
func (pl *InterPodAffinity) Filter(pod *v1.Pod) error {

	podInfo, err := NewPodInfo(pod)
	if err != nil {
		return fmt.Errorf("parsing pod: %+v", err)
	}

	for i := range podInfo.RequiredAffinityTerms {
		if err := pl.mergeAffinityTermNamespacesIfNotEmpty(&podInfo.RequiredAffinityTerms[i]); err != nil {
			return err
		}
	}
	for i := range podInfo.RequiredAntiAffinityTerms {
		if err := pl.mergeAffinityTermNamespacesIfNotEmpty(&podInfo.RequiredAntiAffinityTerms[i]); err != nil {
			return err
		}
	}
	namespaceLabels := pl.GetNamespaceLabelsSnapshot(pod.Namespace)

	existingAntiAffinityCounts = pl.getExistingAntiAffinityCounts(ctx, pod, namespaceLabels, nodesWithRequiredAntiAffinityPods)
	affinityCounts, antiAffinityCounts = pl.getIncomingAffinityAntiAffinityCounts(ctx, podInfo, allNodes)

	if len(existingAntiAffinityCounts) == 0 && len(podInfo.RequiredAffinityTerms) == 0 && len(podInfo.RequiredAntiAffinityTerms) == 0 {
		return nil, framework.NewStatus(framework.Skip)
	}

	return nil
}
