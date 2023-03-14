package logout

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/skeeey/xcm-cli/pkg/configs"
)

func NewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Log out",
		Long:  "Log out, removing connection related variables from the config file.",
		Args:  cobra.NoArgs,
		RunE:  run,
	}
}

func run(cmd *cobra.Command, argv []string) error {
	// Load the configuration file:
	cfg, err := configs.LoadAPIConfig()
	if err != nil {
		return fmt.Errorf("cannot load configuration file: %w", err)
	}

	// Remove all the login related settings from the configuration file:
	cfg.Disarm()

	// Save the configuration file:
	err = cfg.Save()
	if err != nil {
		return fmt.Errorf("cannot save configuration file: %w", err)
	}

	fmt.Fprintln(os.Stdout, "Logout successful")
	return nil
}
