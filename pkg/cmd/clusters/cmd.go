package clusters

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/skeeey/xcm-cli/pkg/configs"
	"github.com/skeeey/xcm-cli/pkg/genericflags"
	"github.com/skeeey/xcm-cli/pkg/printer"
	"github.com/skeeey/xcm-cli/pkg/rest"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clusters",
		Short: "List clusters from xCM",
		Long: "List clusters from xCM\n" +
			"`xcm clusters` list all clusters\n" +
			"`xcm clusters <cluster-id>` list a specified cluster with its id\n",
		Args: cobra.MaximumNArgs(1),
		RunE: run,
	}

	genericflags.AddFlag(cmd.Flags())

	return cmd
}

func run(cmd *cobra.Command, argv []string) error {
	xcmConfig, err := configs.LoadAPIConfig()
	if err != nil {
		return err
	}

	if xcmConfig.AccessToken == "" || xcmConfig.RefreshToken == "" || xcmConfig.URL == "" {
		return fmt.Errorf("login required")
	}

	if len(argv) == 0 {
		clusters, err := rest.GetAllClusters(xcmConfig.URL)
		if err != nil {
			return err
		}

		printer.PrintClustersTable(clusters...)
		return nil

	}

	cluster, err := rest.GetCluster(xcmConfig.URL, argv[0])
	if err != nil {
		return err
	}
	printer.PrintClustersTable(*cluster)
	return nil
}
