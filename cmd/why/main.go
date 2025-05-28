package why

import (
	"fmt"
	"github.com/ops-tool/cmd/why/app/options"
	"github.com/spf13/cobra"
)

func NewWhyCommand() *cobra.Command {
	opts := options.NewWhyFailedOptions()
	cmd := &cobra.Command{
		Use:          "schedule-detect podname -n namespace",
		Short:        "show why pod cannot be scheduled",
		Long:         `show why pod cannot be scheduled`,
		SilenceUsage: true,
		Args: func(cmd *cobra.Command, args []string) error {
			// 无参数时打印帮助信息
			if len(args) == 0 {
				cmd.Help() // 触发帮助信息输出
				return fmt.Errorf("pod name is required")
			}
			// 参数数量校验
			return cobra.ExactArgs(1)(cmd, args) // 强制要求 1 个参数
		},

		RunE: func(cmd *cobra.Command, args []string) error {
			opts.PodName = args[0]
			opts.Kubeconfig = cmd.Root().PersistentFlags().Lookup("kubeconfig").Value.String()
			return run(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Namespace, "namespace", "n", "default", "get pod resource in specific namespace")

	return cmd
}

func run(opts *options.WhyFailedOptions) error {

	err := opts.Validate()
	if err != nil {
		return err
	}
	analyzer, err := opts.NewAnalyzer()
	if err != nil {
		return err
	}

	return analyzer.Why()

}
