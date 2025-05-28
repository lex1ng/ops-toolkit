package statefulsets

import (
	"context"
	"fmt"
	"os"

	"github.com/360EntSecGroup-Skylar/excelize/v2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var headers = []string{"Namespace", "StatefulSet", "Container", "CPU request", "CPU limit", "Mem request", "Mem limit", "CPU Policy", "Mem Policy"}

func getPodResourceByNamespace(clientset *kubernetes.Clientset, namespace string) error {

	return nil
}
func PrintAllPodResource(clientset *kubernetes.Clientset) error {
	namespaces, err := clientset.CoreV1().Namespaces().List(context.TODO(), metaV1.ListOptions{})
	if err != nil {
		fmt.Printf("Error list namespaces: %v", err)
		return err
	}
	f := excelize.NewFile()
	style, err := f.NewStyle(`{"alignment":{"horizontal":"center","vertical":"center"}, "font": {"size":14}, "autowrap":true}`)
	colsWidths := []float64{20, 50, 45, 15, 15, 15, 15, 15, 15}
	for idx, value := range headers {
		_ = f.SetCellValue("Sheet1", fmt.Sprintf("%s%d", string(rune(idx+65)), 1), value)
		_ = f.SetCellStyle("Sheet1", fmt.Sprintf("%s%d", string(rune(idx+65)), 1), fmt.Sprintf("%s%d", string(rune(idx+65)), 1), style)
		err := f.SetColWidth("Sheet1", fmt.Sprintf("%c", 'A'+idx), fmt.Sprintf("%c", 'A'+idx), colsWidths[idx])
		if err != nil {
			fmt.Printf("Error set col width: %v", err)
			os.Exit(1)
		}
	}

	namespaceRowIndx := 2
	statefulsetRowIndx := 2
	for _, namespace := range namespaces.Items {
		allStatefulSets, err := clientset.AppsV1().StatefulSets(namespace.Name).List(context.TODO(), metaV1.ListOptions{})
		if err != nil {
			fmt.Printf("Error list statefulsets: %v", err)
			return err
		}
		//numStateFulSets := len(allStatefulSets.Items)
		//_ = f.MergeCell("Sheet1", fmt.Sprintf("A%d:A%d", namespaceRowIndx, namespaceRowIndx+numStateFulSets), namespace.Name)

		numContainerOfStatefulset := 0
		for _, statefulSet := range allStatefulSets.Items {
			allContainer := []v1.Container{}
			allContainer = append(allContainer, statefulSet.Spec.Template.Spec.Containers...)
			allContainer = append(allContainer, statefulSet.Spec.Template.Spec.InitContainers...)

			numContainer := len(allContainer)
			numContainerOfStatefulset += numContainer
			_ = f.MergeCell("Sheet1", fmt.Sprintf("B%d", statefulsetRowIndx), fmt.Sprintf("B%d", statefulsetRowIndx+numContainer-1))
			_ = f.SetCellValue("Sheet1", fmt.Sprintf("B%d", statefulsetRowIndx), statefulSet.Name)
			_ = f.SetCellStyle("Sheet1", fmt.Sprintf("B%d", statefulsetRowIndx), fmt.Sprintf("B%d", statefulsetRowIndx), style)

			maxCPURequest, maxCPULimit := resource.Quantity{}, resource.Quantity{}
			maxMemRequest, maxMemLimit := resource.Quantity{}, resource.Quantity{}
			for col, container := range allContainer {
				resourceRequests, resourceLimits := container.Resources.Requests, container.Resources.Limits
				cpuRequest, cpuLimit, memRequest, memLimit := resourceRequests[v1.ResourceCPU], resourceLimits[v1.ResourceCPU], resourceRequests[v1.ResourceMemory], resourceLimits[v1.ResourceMemory]
				if cpuRequest.Cmp(maxCPURequest) == 1 {
					maxCPURequest = cpuRequest
				}
				if cpuLimit.Cmp(maxCPULimit) == 1 {
					maxCPULimit = cpuLimit
				}
				if memRequest.Cmp(maxMemRequest) == 1 {
					maxMemRequest = memRequest
				}
				if memLimit.Cmp(maxMemLimit) == 1 {
					maxMemLimit = memLimit
				}

				//fmt.Printf("containercol: %s, cpurequest col:%s, cpulimitcol:%s, memrequestcol:%s, memlimitcol:%s\n", string(rune(65+2+1+0)), string(rune(65+2+1+1)), string(rune(65+2+1+2)), string(rune(65+2+1+3)), string(rune(65+2+1+4)))
				cell1 := fmt.Sprintf("%s%d", string(rune(65+1+1+0)), statefulsetRowIndx+col)
				_ = f.SetCellValue("Sheet1", cell1, container.Name)
				_ = f.SetCellStyle("Sheet1", cell1, cell1, style)

				cell2 := fmt.Sprintf("%s%d", string(rune(65+1+1+1)), statefulsetRowIndx+col)
				_ = f.SetCellValue("Sheet1", cell2, cpuRequest.String())
				_ = f.SetCellStyle("Sheet1", cell2, cell2, style)

				cell3 := fmt.Sprintf("%s%d", string(rune(65+1+1+2)), statefulsetRowIndx+col)
				_ = f.SetCellValue("Sheet1", cell3, cpuLimit.String())
				_ = f.SetCellStyle("Sheet1", cell3, cell3, style)

				cell4 := fmt.Sprintf("%s%d", string(rune(65+1+1+3)), statefulsetRowIndx+col)
				_ = f.SetCellValue("Sheet1", cell4, memRequest.String())
				_ = f.SetCellStyle("Sheet1", cell4, cell4, style)

				cell5 := fmt.Sprintf("%s%d", string(rune(65+1+1+4)), statefulsetRowIndx+col)
				_ = f.SetCellValue("Sheet1", cell5, memLimit.String())
				_ = f.SetCellStyle("Sheet1", cell5, cell5, style)

				fmt.Printf("Namespace: %s StatefulSet: %s, Container: %s, Cpu request: %s, CPU limit: %s, mem request: %s, mem limit: %s\n", namespace.Name, statefulSet.Name, container.Name, cpuRequest.String(), cpuLimit.String(), memRequest.String(), memLimit.String())
			}
			cpuPolicy := "Guaranteed"
			memPolicy := "Guaranteed"
			if maxCPURequest.MilliValue() < maxCPULimit.MilliValue() {
				cpuPolicy = "Burstable"
			}

			if maxMemRequest.Value() < maxMemLimit.Value() {
				memPolicy = "Burstable"
			}

			_ = f.MergeCell("Sheet1", fmt.Sprintf("H%d", statefulsetRowIndx), fmt.Sprintf("H%d", statefulsetRowIndx+numContainer-1))
			_ = f.SetCellValue("Sheet1", fmt.Sprintf("H%d", statefulsetRowIndx), cpuPolicy)
			_ = f.SetCellStyle("Sheet1", fmt.Sprintf("H%d", statefulsetRowIndx), fmt.Sprintf("H%d", statefulsetRowIndx), style)

			_ = f.MergeCell("Sheet1", fmt.Sprintf("I%d", statefulsetRowIndx), fmt.Sprintf("I%d", statefulsetRowIndx+numContainer-1))
			_ = f.SetCellValue("Sheet1", fmt.Sprintf("I%d", statefulsetRowIndx), memPolicy)
			_ = f.SetCellStyle("Sheet1", fmt.Sprintf("I%d", statefulsetRowIndx), fmt.Sprintf("I%d", statefulsetRowIndx), style)

			statefulsetRowIndx += numContainer
		}
		_ = f.MergeCell("Sheet1", fmt.Sprintf("A%d", namespaceRowIndx), fmt.Sprintf("A%d", namespaceRowIndx+numContainerOfStatefulset-1))
		_ = f.SetCellValue("Sheet1", fmt.Sprintf("A%d", namespaceRowIndx), namespace.Name)
		_ = f.SetCellStyle("Sheet1", fmt.Sprintf("A%d", namespaceRowIndx), fmt.Sprintf("A%d", namespaceRowIndx), style)
		namespaceRowIndx += numContainerOfStatefulset
		//fmt.Printf("namespace: %s, numStatefulset: %d, numContainerOfStatefulset: %d\n", namespace.Name, len(allStatefulSets.Items), numContainerOfStatefulset)
	}

	if err := f.SaveAs("example.xlsx"); err != nil {
		fmt.Println(err)
		os.Exit(1) // 如果保存失败则退出程序
	}
	return nil
}
