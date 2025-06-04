package framework

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	resourcehelper "k8s.io/kubectl/pkg/util/resource"

	"github.com/ops-tool/pkg/util"
)

var (
	ResourceNames      = []string{"cpu", "memory", "ephemeral-storage", "hugepages-1Gi", "hugepages-2Mi", "cloudbed.abcstack.com/mlnx_numa0_netdevice", "cloudbed.abcstack.com/mlnx_numa1_netdevice", "cloudbed.abcstack.com/hdd-passthrough", "cloudbed.abcstack.com/ssd-passthrough"}
	printResourceNames = []string{"cpu", "memory", "ephemeral-storage", "hugepages-1Gi", "hugepages-2Mi", "mlnx_numa0_netdevice", "mlnx_numa1_netdevice", "hdd-passthrough", "ssd-passthrough"}
)

type Resource struct {
	Name             string  `json:"name"`
	Requests         int64   `json:"requests"`
	RequestsFraction float64 `json:"requestsFraction"`
	Limits           int64   `json:"limits"`
	LimitsFraction   float64 `json:"limitsFraction"`
	Capacity         int64   `json:"capacity"`
	Left             int64   `json:"left"`
}

func (r *Resource) String() string {

	resourceName := r.Name
	if resourceName == "cpu" {
		return fmt.Sprintf("%dm", r.Requests)
	} else if resourceName == "cloudbed.abcstack.com/mlnx_numa0_netdevice" || resourceName == "cloudbed.abcstack.com/mlnx_numa1_netdevice" || resourceName == "cloudbed.abcstack.com/hdd-passthrough" || resourceName == "cloudbed.abcstack.com/ssd-passthrough" {
		return fmt.Sprintf("%d", int64(r.Requests)/1000)
	} else if resourceName == "ephemeral-storage" {
		return fmt.Sprintf("%.1fGi", float64(r.Requests)/(1000*1024*1024*1024))
	}
	return fmt.Sprintf("%.1fGi", float64(r.Requests)/(1000*1024*1024*1024))
}

type ResourceList map[string]*Resource

type Node struct {
	Name                 string
	AllocatedResourceMap ResourceList
	v1.Node

	Pods             []*PodInfo
	PodsWithAffinity []*PodInfo

	// The subset of pods with required anti-affinity.
	PodsWithRequiredAntiAffinity []*PodInfo
}

func (n *Node) AddPodInfo(podInfo *PodInfo) {
	n.Pods = append(n.Pods, podInfo)
	if PodWithAffinity(podInfo.Pod) {
		n.PodsWithAffinity = append(n.PodsWithAffinity, podInfo)
	}
	if PodWithRequiredAntiAffinity(podInfo.Pod) {
		n.PodsWithRequiredAntiAffinity = append(n.PodsWithRequiredAntiAffinity, podInfo)
	}
}
func (n *Node) AddPod(pod *v1.Pod) {
	// ignore this err since apiserver doesn't properly validate affinity terms
	// and we can't fix the validation for backwards compatibility.
	podInfo, _ := NewPodInfo(pod)
	n.AddPodInfo(podInfo)
}

func (nr *Node) String() []string {

	res := []string{strings.Split(nr.Name, "-")[0]}
	for _, resourceName := range ResourceNames {
		if cur, ok := nr.AllocatedResourceMap[resourceName]; ok {
			//res = append(res, nr.AllocatedResourceMap[resourceName].String())
			if cur.Capacity == 0 {
				res = append(res, "-")
				continue
			}
			capacity := &Resource{Name: cur.Name, Requests: cur.Capacity}
			res = append(res, fmt.Sprintf("%s/%s(%d%%)", nr.AllocatedResourceMap[resourceName].String(), capacity.String(), int64(cur.RequestsFraction)))
		} else {
			res = append(res, "-")
		}

	}

	return res
}

func (n *Node) SetNode(node *v1.Node) {
	n.Node = *node
}

func NewNodeInfo(pods ...*v1.Pod) *Node {
	ni := &Node{}
	for _, pod := range pods {
		ni.AddPod(pod)
	}
	return ni
}

type PodInfo struct {
	Pod                        *v1.Pod
	RequiredAffinityTerms      []AffinityTerm
	RequiredAntiAffinityTerms  []AffinityTerm
	PreferredAffinityTerms     []WeightedAffinityTerm
	PreferredAntiAffinityTerms []WeightedAffinityTerm
}

// DeepCopy returns a deep copy of the PodInfo object.
func (pi *PodInfo) DeepCopy() *PodInfo {
	return &PodInfo{
		Pod:                        pi.Pod.DeepCopy(),
		RequiredAffinityTerms:      pi.RequiredAffinityTerms,
		RequiredAntiAffinityTerms:  pi.RequiredAntiAffinityTerms,
		PreferredAffinityTerms:     pi.PreferredAffinityTerms,
		PreferredAntiAffinityTerms: pi.PreferredAntiAffinityTerms,
	}
}

// Update creates a full new PodInfo by default. And only updates the pod when the PodInfo
// has been instantiated and the passed pod is the exact same one as the original pod.
func (pi *PodInfo) Update(pod *v1.Pod) error {
	if pod != nil && pi.Pod != nil && pi.Pod.UID == pod.UID {
		// PodInfo includes immutable information, and so it is safe to update the pod in place if it is
		// the exact same pod
		pi.Pod = pod
		return nil
	}
	var preferredAffinityTerms []v1.WeightedPodAffinityTerm
	var preferredAntiAffinityTerms []v1.WeightedPodAffinityTerm
	if affinity := pod.Spec.Affinity; affinity != nil {
		if a := affinity.PodAffinity; a != nil {
			preferredAffinityTerms = a.PreferredDuringSchedulingIgnoredDuringExecution
		}
		if a := affinity.PodAntiAffinity; a != nil {
			preferredAntiAffinityTerms = a.PreferredDuringSchedulingIgnoredDuringExecution
		}
	}

	// Attempt to parse the affinity terms
	var parseErrs []error
	requiredAffinityTerms, err := GetAffinityTerms(pod, GetPodAffinityTerms(pod.Spec.Affinity))
	if err != nil {
		parseErrs = append(parseErrs, fmt.Errorf("requiredAffinityTerms: %w", err))
	}
	requiredAntiAffinityTerms, err := GetAffinityTerms(pod,
		GetPodAntiAffinityTerms(pod.Spec.Affinity))
	if err != nil {
		parseErrs = append(parseErrs, fmt.Errorf("requiredAntiAffinityTerms: %w", err))
	}
	weightedAffinityTerms, err := getWeightedAffinityTerms(pod, preferredAffinityTerms)
	if err != nil {
		parseErrs = append(parseErrs, fmt.Errorf("preferredAffinityTerms: %w", err))
	}
	weightedAntiAffinityTerms, err := getWeightedAffinityTerms(pod, preferredAntiAffinityTerms)
	if err != nil {
		parseErrs = append(parseErrs, fmt.Errorf("preferredAntiAffinityTerms: %w", err))
	}

	pi.Pod = pod
	pi.RequiredAffinityTerms = requiredAffinityTerms
	pi.RequiredAntiAffinityTerms = requiredAntiAffinityTerms
	pi.PreferredAffinityTerms = weightedAffinityTerms
	pi.PreferredAntiAffinityTerms = weightedAntiAffinityTerms

	return err
}

func NewPodInfo(pod *v1.Pod) (*PodInfo, error) {
	pInfo := &PodInfo{}
	err := pInfo.Update(pod)
	return pInfo, err
}

type AffinityTerm struct {
	Namespaces        sets.Set[string]
	Selector          labels.Selector
	TopologyKey       string
	NamespaceSelector labels.Selector
}

func (at *AffinityTerm) Matches(pod *v1.Pod, nsLabels labels.Set) bool {
	if at.Namespaces.Has(pod.Namespace) || at.NamespaceSelector.Matches(nsLabels) {
		return at.Selector.Matches(labels.Set(pod.Labels))
	}
	return false
}

type WeightedAffinityTerm struct {
	AffinityTerm
	Weight int32
}

func newAffinityTerm(pod *v1.Pod, term *v1.PodAffinityTerm) (*AffinityTerm, error) {
	selector, err := metav1.LabelSelectorAsSelector(term.LabelSelector)
	if err != nil {
		return nil, err
	}

	namespaces := getNamespacesFromPodAffinityTerm(pod, term)
	nsSelector, err := metav1.LabelSelectorAsSelector(term.NamespaceSelector)
	if err != nil {
		return nil, err
	}

	return &AffinityTerm{Namespaces: namespaces, Selector: selector, TopologyKey: term.TopologyKey, NamespaceSelector: nsSelector}, nil
}

func getNamespacesFromPodAffinityTerm(pod *v1.Pod, podAffinityTerm *v1.PodAffinityTerm) sets.Set[string] {
	names := sets.Set[string]{}
	if len(podAffinityTerm.Namespaces) == 0 && podAffinityTerm.NamespaceSelector == nil {
		names.Insert(pod.Namespace)
	} else {
		names.Insert(podAffinityTerm.Namespaces...)
	}
	return names
}

func GetAffinityTerms(pod *v1.Pod, v1Terms []v1.PodAffinityTerm) ([]AffinityTerm, error) {
	if v1Terms == nil {
		return nil, nil
	}

	var terms []AffinityTerm
	for i := range v1Terms {
		t, err := newAffinityTerm(pod, &v1Terms[i])
		if err != nil {
			// We get here if the label selector failed to process
			return nil, err
		}
		terms = append(terms, *t)
	}
	return terms, nil
}

func getWeightedAffinityTerms(pod *v1.Pod, v1Terms []v1.WeightedPodAffinityTerm) ([]WeightedAffinityTerm, error) {
	if v1Terms == nil {
		return nil, nil
	}

	var terms []WeightedAffinityTerm
	for i := range v1Terms {
		t, err := newAffinityTerm(pod, &v1Terms[i].PodAffinityTerm)
		if err != nil {
			// We get here if the label selector failed to process
			return nil, err
		}
		terms = append(terms, WeightedAffinityTerm{AffinityTerm: *t, Weight: v1Terms[i].Weight})
	}
	return terms, nil
}

func GetPodAffinityTerms(affinity *v1.Affinity) (terms []v1.PodAffinityTerm) {
	if affinity != nil && affinity.PodAffinity != nil {
		if len(affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution) != 0 {
			terms = affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution
		}
		// TODO: Uncomment this block when implement RequiredDuringSchedulingRequiredDuringExecution.
		// if len(affinity.PodAffinity.RequiredDuringSchedulingRequiredDuringExecution) != 0 {
		//	terms = append(terms, affinity.PodAffinity.RequiredDuringSchedulingRequiredDuringExecution...)
		// }
	}
	return terms
}

func GetPodAntiAffinityTerms(affinity *v1.Affinity) (terms []v1.PodAffinityTerm) {
	if affinity != nil && affinity.PodAntiAffinity != nil {
		if len(affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution) != 0 {
			terms = affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution
		}
		// TODO: Uncomment this block when implement RequiredDuringSchedulingRequiredDuringExecution.
		// if len(affinity.PodAntiAffinity.RequiredDuringSchedulingRequiredDuringExecution) != 0 {
		//	terms = append(terms, affinity.PodAntiAffinity.RequiredDuringSchedulingRequiredDuringExecution...)
		// }
	}
	return terms
}

func PodWithAffinity(p *v1.Pod) bool {
	affinity := p.Spec.Affinity
	return affinity != nil && (affinity.PodAffinity != nil || affinity.PodAntiAffinity != nil)
}

func PodWithRequiredAntiAffinity(p *v1.Pod) bool {
	affinity := p.Spec.Affinity
	return affinity != nil && affinity.PodAntiAffinity != nil &&
		len(affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution) != 0
}

func BuildNodeList(clientset *kubernetes.Clientset) ([]*Node, error) {
	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})

	if err != nil {
		return nil, err
	}

	var nodeList []*Node

	for _, node := range nodes.Items {
		allocatedResourceMap, err := BuildAllocatedResourceMap(clientset, &node)
		if err != nil {
			fmt.Printf("error fetching node resource of node %s\n", node.Name)
			continue
		}

		fieldSelector := fmt.Sprintf("spec.nodeName=%s", node.Name)
		pods, err := clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{
			FieldSelector: fieldSelector,
		})

		if err != nil {
			fmt.Printf("error fetching pods on node  %s\n", node.Name)
			continue
		}

		nodeInfo := &Node{
			Name:                 node.Name,
			AllocatedResourceMap: allocatedResourceMap,
			Node:                 node,
		}

		for _, pod := range pods.Items {
			nodeInfo.AddPod(&pod)
		}

		nodeList = append(nodeList, nodeInfo)
	}

	return nodeList, nil
}

func BuildAllocatedResourceMap(clientset *kubernetes.Clientset, node *v1.Node) (ResourceList, error) {

	fieldSelector, err := fields.ParseSelector("spec.nodeName=" + node.Name +
		",status.phase!=" + string(v1.PodSucceeded) +
		",status.phase!=" + string(v1.PodFailed))

	if err != nil {
		return nil, err
	}

	pods, err := clientset.CoreV1().Pods(v1.NamespaceAll).List(context.TODO(), metav1.ListOptions{
		FieldSelector: fieldSelector.String(),
	})

	allocatables := node.Status.Capacity
	if len(node.Status.Allocatable) > 0 {
		allocatables = node.Status.Allocatable
	}

	reqs, limits := GetPodsTotalRequestsAndLimits(pods)

	return resourceListToAllocatedResource(reqs, limits, allocatables), nil
}

func resourceListToAllocatedResource(reqs, limits, allocatables map[v1.ResourceName]resource.Quantity) ResourceList {
	result := make(ResourceList)
	for name, allocatable := range allocatables {
		request, limit := reqs[name], limits[name]
		requestFraction := float64(request.Value()) / float64(allocatable.Value()) * 100
		limitFraction := float64(limit.Value()) / float64(allocatable.Value()) * 100
		result[name.String()] = &Resource{
			Name:             name.String(),
			Requests:         request.MilliValue(),
			RequestsFraction: requestFraction,
			Limits:           limit.MilliValue(),
			LimitsFraction:   limitFraction,
			Capacity:         allocatable.MilliValue(),
			Left:             allocatable.MilliValue() - request.MilliValue(),
		}
	}
	return result
}
func GetPodsTotalRequestsAndLimits(podList *v1.PodList) (reqs map[v1.ResourceName]resource.Quantity, limits map[v1.ResourceName]resource.Quantity) {
	reqs, limits = map[v1.ResourceName]resource.Quantity{}, map[v1.ResourceName]resource.Quantity{}
	for _, pod := range podList.Items {
		podReqs, podLimits := resourcehelper.PodRequestsAndLimits(&pod)
		for podReqName, podReqValue := range podReqs {
			if value, ok := reqs[podReqName]; !ok {
				reqs[podReqName] = podReqValue.DeepCopy()
			} else {
				value.Add(podReqValue)
				reqs[podReqName] = value
			}
		}
		for podLimitName, podLimitValue := range podLimits {
			if value, ok := limits[podLimitName]; !ok {
				limits[podLimitName] = podLimitValue.DeepCopy()
			} else {
				value.Add(podLimitValue)
				limits[podLimitName] = value
			}
		}
	}
	return
}

func PrintNodeList(nodeList []*Node) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)

	header := append([]string{"nodeName"}, printResourceNames...)
	t.AppendHeader(util.ListToRow(header))
	for _, n := range nodeList {
		t.AppendRow(util.ListToRow(n.String()))
	}

	columnConfigs := make([]table.ColumnConfig, len(header))
	for i := range header {
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

func BuildPodResourceList(pod *v1.Pod) ResourceList {

	reqs, limits := GetPodsTotalRequestsAndLimits(&v1.PodList{
		Items: []v1.Pod{*pod},
	})

	result := make(ResourceList)
	for name, req := range reqs {
		limit := limits[name]
		result[name.String()] = &Resource{
			Name:     name.String(),
			Requests: req.MilliValue(),
			Limits:   limit.MilliValue(),
		}

	}
	return result
}

type PVCStatus struct {
	Name             string
	PVName           string
	PVVolumeAffinity *v1.VolumeNodeAffinity
	PVError          string
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
		// should show the reason if pv is not exist.
		if pvc.Spec.VolumeName == "" {
			pvAffinity = append(pvAffinity, &PVCStatus{
				Name:             pvcName,
				PVName:           "",
				PVVolumeAffinity: &v1.VolumeNodeAffinity{},
				PVError:          fmt.Sprintf("pvc %s's pv not found", pvcName),
			})
			continue
		}
		pv, err := clientset.CoreV1().PersistentVolumes().Get(context.Background(), pvc.Spec.VolumeName, metav1.GetOptions{})
		if err != nil {
			pvAffinity = append(pvAffinity, &PVCStatus{
				Name:             pvcName,
				PVName:           "",
				PVVolumeAffinity: &v1.VolumeNodeAffinity{},
				PVError:          fmt.Sprintf("pvc %s's pv not found", pvcName),
			})
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
