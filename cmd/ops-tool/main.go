package main

import (
	"github.com/ops-tool/cmd/getNodeResource"
	"github.com/ops-tool/cmd/getPodResource"
	"github.com/ops-tool/cmd/why"
	"github.com/spf13/cobra"
	"k8s.io/client-go/util/homedir"
	"os"
	"path/filepath"
)

var kubeconfig string

func main() {

	rootCmd := &cobra.Command{
		Use:   "ops",
		Short: "Kubernetes operations tool-kit",
		Long:  "maintained by cloudbed team",
		// 不定义 Run 函数，强制用户必须使用子命令
	}

	rootCmd.PersistentFlags().StringVarP(&kubeconfig, "kubeconfig", "k", filepath.Join(homedir.HomeDir(), ".kube", "config"), "Kubeconfig 文件路径")

	rootCmd.AddCommand(getNodeResource.NewGetNodeResourceCommand())
	rootCmd.AddCommand(getPodResource.NewGetPodResourceCommand())
	rootCmd.AddCommand(why.NewWhyCommand())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}

}
