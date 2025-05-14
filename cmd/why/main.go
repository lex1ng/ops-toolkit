package why

import (
	"github.com/ops-tool/cmd/why/app/options"
	"github.com/spf13/cobra"
)

func NewWhyCommand() *cobra.Command {
	opts := options.NewWhyFailedOptions()
	cmd := &cobra.Command{
		Use:          "why podname -n namespace",
		Short:        "show why pod cannot be scheduled",
		Long:         `show why pod cannot be scheduled`,
		SilenceUsage: true,

		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Kubeconfig = cmd.Root().PersistentFlags().Lookup("kubeconfig").Value.String()
			return run(opts)
		},

		Args: cobra.NoArgs,
	}

	cmd.Flags().StringVarP(&opts.Namespace, "namespace", "n", "default", "get pod resource in specific namespace")
	cmd.Flags().StringVar(&opts.PodName, "pod", "", "pod that unable to scheduler ")

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
