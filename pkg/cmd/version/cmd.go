package version

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/skeeey/xcm-cli/pkg/info"
)

func NewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Prints the version",
		Long:  "Prints the version number of the client.",
		Args:  cobra.NoArgs,
		RunE:  run,
	}
}

func run(cmd *cobra.Command, argv []string) error {
	// Print the version:
	fmt.Fprintf(os.Stdout, "%s\n", info.Version)

	return nil
}
