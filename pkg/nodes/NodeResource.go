package nodes

import (
	"context"
	"fmt"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	resourcehelper "k8s.io/kubectl/pkg/util/resource"
	"os"
	"strings"
)

var (
	ResourceNames      = []string{"cpu", "memory", "ephemeral-storage", "hugepages-1Gi", "hugepages-2Mi", "cloudbed.abcstack.com/mlnx_numa0_netdevice", "cloudbed.abcstack.com/mlnx_numa1_netdevice", "cloudbed.abcstack.com/hdd-passthrough", "cloudbed.abcstack.com/ssd-passthrough"}
	printResourceNames = []string{"cpu", "memory", "ephemeral-storage", "hugepages-1Gi", "hugepages-2Mi", "mlnx_numa0_netdevice", "mlnx_numa1_netdevice", "hdd-passthrough", "ssd-passthrough"}
)

type NodeResourceReporter struct {
	ClientSet *kubernetes.Clientset
}

type Resource struct {
	Name             string  `json:"name"`
	Requests         int64   `json:"requests"`
	RequestsFraction float64 `json:"requestsFraction"`
	Limits           int64   `json:"limits"`
	LimitsFraction   float64 `json:"limitsFraction"`
	Capacity         int64   `json:"capacity"`
}

func (r *Resource) String() string {
	if r.Capacity == 0 {
		return "-"
	}
	resourceName := r.Name
	if resourceName == "cpu" {
		return fmt.Sprintf("%d/%d(%d%%)", r.Requests, r.Capacity, int64(r.RequestsFraction))
	} else if resourceName == "cloudbed.abcstack.com/mlnx_numa0_netdevice" || resourceName == "cloudbed.abcstack.com/mlnx_numa1_netdevice" || resourceName == "cloudbed.abcstack.com/hdd-passthrough" || resourceName == "cloudbed.abcstack.com/ssd-passthrough" {
		return fmt.Sprintf("%d/%d", int64(r.Requests), r.Capacity)
	} else if resourceName == "ephemeral-storage" {
		return fmt.Sprintf("%dMi/%dGi(%d%%)", r.Requests/(1024*1024), r.Capacity/(1024*1024*1024), int64(r.RequestsFraction))
	}
	return fmt.Sprintf("%dGi/%dGi(%d%%)", r.Requests/(1024*1024*1024), r.Capacity/(1024*1024*1024), int64(r.RequestsFraction))
}

type ResourceList map[string]*Resource

type Node struct {
	Name                 string
	AllocatedResourceMap ResourceList
	v1.Node
}

func (nr *Node) String() []string {

	res := []string{strings.Split(nr.Name, "-")[0]}
	for _, resourceName := range ResourceNames {
		if _, ok := nr.AllocatedResourceMap[resourceName]; ok {
			res = append(res, nr.AllocatedResourceMap[resourceName].String())
		} else {
			res = append(res, "-")
		}

	}

	return res
}

type NodeList []Node

func listToRow(toTransfer []string) table.Row {

	var row table.Row

	for _, v := range toTransfer {
		row = append(row, v)
	}

	return row
}
func (nl NodeList) Print() {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)

	header := append([]string{"nodeName"}, printResourceNames...)
	t.AppendHeader(listToRow(header))
	for _, n := range nl {
		t.AppendRow(listToRow(n.String()))
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

func (n *NodeResourceReporter) GetNodeResource() error {

	nodeResourceList, err := n.BuildNodeList()

	if err != nil {
		return err
	}

	nodeResourceList.Print()

	return nil
}

func (n *NodeResourceReporter) BuildNodeList() (NodeList, error) {
	nodes, err := n.ClientSet.CoreV1().Nodes().List(context.TODO(), metaV1.ListOptions{})

	if err != nil {
		return nil, err
	}

	var nodeResourceList NodeList

	for _, node := range nodes.Items {
		allocatedResourceMap, err := n.BuildAllocatedResourceMap(node)
		if err != nil {
			fmt.Printf("error fetching node resource of node %s\n", node.Name)
			continue
		}

		nodeResourceList = append(nodeResourceList, Node{
			Name:                 node.Name,
			AllocatedResourceMap: allocatedResourceMap,
			Node:                 node,
		})
	}

	return nodeResourceList, nil
}

func (n *NodeResourceReporter) BuildAllocatedResourceMap(node v1.Node) (ResourceList, error) {

	fieldSelector, err := fields.ParseSelector("spec.nodeName=" + node.Name +
		",status.phase!=" + string(v1.PodSucceeded) +
		",status.phase!=" + string(v1.PodFailed))

	if err != nil {
		return nil, err
	}

	pods, err := n.ClientSet.CoreV1().Pods(v1.NamespaceAll).List(context.TODO(), metaV1.ListOptions{
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
			Requests:         request.Value(),
			RequestsFraction: requestFraction,
			Limits:           limit.Value(),
			LimitsFraction:   limitFraction,
			Capacity:         allocatable.Value(),
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
