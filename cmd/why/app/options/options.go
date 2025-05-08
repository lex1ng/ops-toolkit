package options

import (
	"github.com/ops-tool/pkg/nodes"
	"github.com/ops-tool/pkg/scheduler"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type WhyFailedOptions struct {
	Kubeconfig string
	Namespace  string
	PodName    string
}

func NewWhyFailedOptions() *WhyFailedOptions {
	return &WhyFailedOptions{}
}

func (o *WhyFailedOptions) NewAnalyzer() (*scheduler.Analyzer, error) {

	config, err := clientcmd.BuildConfigFromFlags("", o.Kubeconfig)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &scheduler.Analyzer{
		ClientSet: clientset,
		Namespace: o.Namespace,
		PodName:   o.PodName,
		NodeResourceReporter: nodes.NodeResourceReporter{
			ClientSet: clientset,
		},
	}, nil

}
