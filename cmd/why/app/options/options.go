package options

import (
	"fmt"
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

	analyzer, err := scheduler.NewAnalyzer(clientset, o.Namespace, o.PodName)
	if err != nil {
		return nil, err
	}

	return analyzer, nil

}

func (o *WhyFailedOptions) Validate() error {

	if o.PodName == "" {
		return fmt.Errorf("pod name is required")
	}

	if o.Namespace == "" {
		return fmt.Errorf("namespace is required")
	}

	return nil
}
