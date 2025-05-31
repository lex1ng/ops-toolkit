package nodes

import (
	"github.com/ops-tool/pkg/scheduler/framework"
	"k8s.io/client-go/kubernetes"
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
