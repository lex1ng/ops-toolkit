package options

import (
	"github.com/ops-tool/pkg/pods"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	metrics "k8s.io/metrics/pkg/client/clientset/versioned"
)

type PodResourceOptions struct {
	Kubeconfig string
	Namespace  string
	Node       string
	Workload   string
	Sort       string
}

func NewPodResourceOptions() *PodResourceOptions {
	return &PodResourceOptions{}
}

func (o *PodResourceOptions) NewPodResourceReporter() (*pods.PodResourceReporter, error) {

	config, err := clientcmd.BuildConfigFromFlags("", o.Kubeconfig)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	// 创建 Metrics 客户端
	metricsClient, err := metrics.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &pods.PodResourceReporter{
		ClientSet:    clientset,
		MetricClient: metricsClient,
		Namespace:    o.Namespace,
		Node:         o.Node,
		Workload:     o.Workload,
		Sort:         o.Sort,
	}, nil

}
