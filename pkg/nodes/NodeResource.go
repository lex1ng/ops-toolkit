package nodes

import (
	"github.com/ops-tool/pkg/scheduler/framework"
	"k8s.io/client-go/kubernetes"
)

var (
	printResourceNames = []string{"cpu", "memory", "ephemeral-storage", "hugepages-1Gi", "hugepages-2Mi", "mlnx_numa0_netdevice", "mlnx_numa1_netdevice", "hdd-passthrough", "ssd-passthrough"}
)

type NodeResourceReporter struct {
	ClientSet *kubernetes.Clientset
}

func (n *NodeResourceReporter) GetNodeResource() error {

	nodeResourceList, err := framework.BuildNodeList(n.ClientSet)

	if err != nil {
		return err
	}

	framework.PrintNodeList(nodeResourceList)
	return nil
}
