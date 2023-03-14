package relay

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/skeeey/xcm-cli/pkg/clustermanagement"
	"github.com/skeeey/xcm-cli/pkg/genericflags"
)

var args struct {
	kubeconfig string
}

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "relay",
		Short: "Relay a specified cluster to xCM",
		Long:  "Relay a specified cluster to xCM\n",
		Args:  cobra.NoArgs,
		RunE:  run,
	}

	addFlags(cmd.Flags())
	genericflags.AddFlag(cmd.Flags())

	return cmd
}

func addFlags(flags *pflag.FlagSet) {
	flags.StringVar(
		&args.kubeconfig,
		"kubeconfig",
		"",
		"The kubeconfig of your cluster",
	)

}

func run(cmd *cobra.Command, argv []string) error {
	spokeDeployer, err := clustermanagement.BuildSpokeDeployer(args.kubeconfig, false)
	if err != nil {
		return fmt.Errorf("failed to build spoke deployer with %q: %v", args.kubeconfig, err)
	}

	if err := spokeDeployer.Relay(context.Background()); err != nil {
		return err
	}

	fmt.Fprintln(os.Stdout, "The cluster is connected to xCM with id", spokeDeployer.GetClusterID())
	return nil
}
