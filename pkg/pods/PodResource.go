package pods

import (
	"context"
	"fmt"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metrics "k8s.io/metrics/pkg/client/clientset/versioned"
	"sort"
	"strings"
)

type PodResourceReporter struct {
	ClientSet    *kubernetes.Clientset
	MetricClient *metrics.Clientset

	Namespace string
	Node      string
	Workload  string
	Sort      string
}

type PodResource struct {
	Name       string
	Namespace  string
	NodeName   string
	CPURequest resource.Quantity
	CPULimit   resource.Quantity
	CPUUsage   resource.Quantity
	MemRequest resource.Quantity
	MemLimit   resource.Quantity
	MemUsage   resource.Quantity
}

func (pr *PodResource) String() string {
	return fmt.Sprintf("%s/%s,%s,%d,%d,%d,%d,%d,%d\n",
		pr.Namespace, pr.Name, pr.NodeName,
		pr.CPURequest.MilliValue(), pr.CPULimit.MilliValue(), pr.CPUUsage.MilliValue(),
		pr.MemRequest.Value()/(1024*1024), pr.MemLimit.Value()/(1024*1024), pr.MemUsage.Value()/(1024*1024))
}

type PodResourceList []*PodResource

const (
	SortAsc  = "asc"
	SortDesc = "desc"
)

var validSortKeys = map[string]bool{
	"Name":       true,
	"Namespace":  true,
	"NodeName":   true,
	"CPURequest": true,
	"CPULimit":   true,
	"CPUUsage":   true,
	"MemRequest": true,
	"MemLimit":   true,
	"MemUsage":   true,
}

func (pl PodResourceList) Sort(key, mode string) {
	if _, valid := validSortKeys[key]; !valid {
		return // 或返回error
	}

	sort.Slice(pl, func(i, j int) bool {
		// 获取比较值
		var less bool
		switch key {
		case "Name":
			less = strings.Compare(pl[i].Name, pl[j].Name) < 0
		case "Namespace":
			less = strings.Compare(pl[i].Namespace, pl[j].Namespace) < 0
		case "NodeName":
			less = strings.Compare(pl[i].NodeName, pl[j].NodeName) < 0
		case "CPURequest":
			less = pl[i].CPURequest.MilliValue() < pl[j].CPURequest.MilliValue()
		case "CPULimit":
			less = pl[i].CPULimit.MilliValue() < pl[j].CPULimit.MilliValue()
		case "CPUUsage":
			less = pl[i].CPUUsage.MilliValue() < pl[j].CPUUsage.MilliValue()
		case "MemRequest":
			less = pl[i].MemRequest.Value() < pl[j].MemRequest.Value()
		case "MemLimit":
			less = pl[i].MemLimit.Value() < pl[j].MemLimit.Value()
		case "MemUsage":
			less = pl[i].MemUsage.Value() < pl[j].MemUsage.Value()
		}

		// 处理排序模式
		if mode == SortDesc {
			return !less
		}
		return less
	})
}

func (pl PodResourceList) Print() {
	fmt.Printf("Namespace/PodName,NodeName,CPURequest(m),CPULimit(m),CPUUsage(m),MemRequest(Mi),MemLimit(Mi),MemUsage(Mi)\n")
	for _, pr := range pl {
		fmt.Print(pr.String())
	}
}

type PodUsage struct {
	CPUUsage resource.Quantity
	MemUsage resource.Quantity
}

type NamespacePodMetric map[string]PodUsage

type PodMetricMap map[string]NamespacePodMetric

func (p *PodResourceReporter) GetPodResource() error {
	listOptions := metav1.ListOptions{}

	if p.Node != "" {
		listOptions.FieldSelector = fmt.Sprintf("spec.nodeName=%s", p.Node)
	}

	// 获取所有符合条件的pod
	pods, err := p.ClientSet.CoreV1().Pods(p.Namespace).List(context.Background(), listOptions)
	if err != nil {
		panic(err)
	}

	// 获取所有 Pod 的 Metrics 数据
	podMetricsList, err := p.MetricClient.MetricsV1beta1().PodMetricses(p.Namespace).List(context.Background(), listOptions)
	if err != nil {
		return err
	}

	podMetricMap := BuildPodMetricMap(podMetricsList)

	podResourceList := PodResourceList{}

	for _, pod := range pods.Items {
		podResource := BuildPodResource(pod)
		if nsMetrics, ok := podMetricMap[pod.Namespace]; ok {
			if usage, ok := nsMetrics[pod.Name]; ok {
				podResource.CPUUsage = usage.CPUUsage
				podResource.MemUsage = usage.MemUsage
			}
		}
		podResourceList = append(podResourceList, podResource)
	}

	// 需要对资源进行排序
	if p.Sort != "" {
		key, mode := "cpuRequest", "desc"
		sortParam := strings.Split(p.Sort, ",")
		if len(sortParam) == 1 {
			key = sortParam[0]
		} else if len(sortParam) == 2 {
			key, mode = sortParam[0], sortParam[1]
		} else {
			return fmt.Errorf("invalid sort paramter %s, using format key(,mode)", p.Sort)
		}

		podResourceList.Sort(key, mode)
	}
	podResourceList.Print()

	return nil

}

func BuildPodResource(pod v1.Pod) *PodResource {
	totalCPURequest := resource.NewQuantity(0, resource.DecimalSI)
	totalCPULimit := resource.NewQuantity(0, resource.DecimalSI)
	totalMemRequest := resource.NewQuantity(0, resource.BinarySI)
	totalMemLimit := resource.NewQuantity(0, resource.BinarySI)

	for _, container := range pod.Spec.Containers {
		req := container.Resources.Requests
		lim := container.Resources.Limits
		if cpu, ok := req["cpu"]; ok {
			totalCPURequest.Add(cpu)
		}
		if mem, ok := req["memory"]; ok {
			totalMemRequest.Add(mem)
		}
		if cpu, ok := lim["cpu"]; ok {
			totalCPULimit.Add(cpu)
		}
		if mem, ok := lim["memory"]; ok {
			totalMemLimit.Add(mem)
		}
	}

	return &PodResource{
		Name:       pod.Name,
		Namespace:  pod.Namespace,
		NodeName:   pod.Spec.NodeName,
		CPURequest: *totalCPURequest,
		CPULimit:   *totalCPULimit,
		MemRequest: *totalMemRequest,
		MemLimit:   *totalCPULimit,
	}
}
func BuildPodMetricMap(podMetricsList *metricsv1beta1.PodMetricsList) PodMetricMap {
	metricsMap := make(PodMetricMap)
	for _, podMetric := range podMetricsList.Items {
		ns := podMetric.Namespace

		if _, ok := metricsMap[ns]; !ok {
			metricsMap[ns] = make(NamespacePodMetric)
		}

		totalCPU := resource.NewQuantity(0, resource.DecimalSI)
		totalMem := resource.NewQuantity(0, resource.BinarySI)

		// 累加所有容器的资源使用情况
		for _, container := range podMetric.Containers {
			cpuQuantity := container.Usage["cpu"]
			memQuantity := container.Usage["memory"]
			totalCPU.Add(cpuQuantity)
			totalMem.Add(memQuantity)
		}
		metricsMap[ns][podMetric.Name] = PodUsage{
			CPUUsage: *totalCPU,
			MemUsage: *totalMem,
		}
	}

	return metricsMap
}
