package connect

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/skeeey/xcm-cli/pkg/clustermanagement"
	"github.com/skeeey/xcm-cli/pkg/configs"
	"github.com/skeeey/xcm-cli/pkg/constants"
	"github.com/skeeey/xcm-cli/pkg/genericflags"
)

var args struct {
	kubeconfig  string
	displayName string
}

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "connect",
		Short: "Connect a specified cluster to xCM",
		Long:  "Connect a specified cluster to xCM\n",
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
		"The kubeconfig of your cluster. The xCM connector will deploy on this cluster.",
	)

	flags.StringVar(
		&args.displayName,
		"display-name",
		"",
		"A display name for the current cluster. The default value is cluster ID.",
	)

}

func run(cmd *cobra.Command, argv []string) error {
	apiConfig, err := configs.LoadAPIConfig()
	if err != nil {
		return err
	}

	if apiConfig.AccessToken == "" || apiConfig.RefreshToken == "" || apiConfig.URL == "" {
		return fmt.Errorf("login required")
	}

	// TODO configure the namespace with cli
	eksDeployer, err := clustermanagement.BuildEKSDeployer(
		args.kubeconfig, constants.DefaultControlPlaneNamespace, apiConfig.URL)
	if err != nil {
		return fmt.Errorf("failed to build eks deployer with %q: %v", args.kubeconfig, err)
	}

	if err := eksDeployer.Connect(context.Background()); err != nil {
		return err
	}

	fmt.Fprintln(os.Stdout, "The cluster is connected to xCM with id", eksDeployer.GetControlPlaneID())
	return nil
}
