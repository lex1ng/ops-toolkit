package options

import (
	"github.com/ops-tool/pkg/nodes"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type NodeResourceOptions struct {
	Kubeconfig string
}

func NewNodeResourceOptions() *NodeResourceOptions {
	return &NodeResourceOptions{}
}
func (n *NodeResourceOptions) NodeResourceReporter() (*nodes.NodeResourceReporter, error) {

	config, err := clientcmd.BuildConfigFromFlags("", n.Kubeconfig)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &nodes.NodeResourceReporter{
		ClientSet: clientset,
	}, nil

}
