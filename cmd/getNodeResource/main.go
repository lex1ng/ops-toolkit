package getNodeResource

import (
	"github.com/ops-tool/cmd/getNodeResource/app/options"
	"github.com/spf13/cobra"
)

func NewGetNodeResourceCommand() *cobra.Command {
	opts := options.NewNodeResourceOptions()
	cmd := &cobra.Command{
		Use:          "getNodeResource",
		Short:        "get all node Resource in the cluster",
		Long:         `get all node Resource in the cluster`,
		SilenceUsage: true,

		RunE: func(cmd *cobra.Command, args []string) error {

			opts.Kubeconfig = cmd.Root().PersistentFlags().Lookup("kubeconfig").Value.String()

			return run(opts)
		},

		Args: cobra.NoArgs,
	}

	return cmd
}

func run(opts *options.NodeResourceOptions) error {

	nodeResourceReporter, err := opts.NodeResourceReporter()
	if err != nil {
		return err
	}

	return nodeResourceReporter.GetNodeResource()

}
