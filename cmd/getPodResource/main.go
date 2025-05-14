package getPodResource

import (
	"github.com/ops-tool/cmd/getPodResource/app/options"
	"github.com/spf13/cobra"
)

func NewGetPodResourceCommand() *cobra.Command {
	opts := options.NewPodResourceOptions()
	cmd := &cobra.Command{
		Use:          "getPodResource",
		Short:        "get Pod Resource",
		Long:         `get Pod Resource`,
		SilenceUsage: true,

		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Kubeconfig = cmd.Root().PersistentFlags().Lookup("kubeconfig").Value.String()
			return run(opts)
		},

		Args: cobra.NoArgs,
	}

	cmd.Flags().StringVarP(&opts.Namespace, "namespace", "n", "", "get pod resource in specific namespace")
	cmd.Flags().StringVar(&opts.Node, "node", "", "get pod resource in specific node")
	cmd.Flags().StringVarP(&opts.Workload, "target", "t", "sts", "get pod resource of specific workload")
	cmd.Flags().StringVarP(&opts.Sort, "sort", "s", "", "sort pod resource using key desc. (e.g. cpurequest,desc)")

	return cmd
}

func run(opts *options.PodResourceOptions) error {

	podResourceReporter, err := opts.NewPodResourceReporter()
	if err != nil {
		return err
	}

	return podResourceReporter.GetPodResource()

}
