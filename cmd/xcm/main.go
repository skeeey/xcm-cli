package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/skeeey/xcm-cli/pkg/cmd/clusters"
	"github.com/skeeey/xcm-cli/pkg/cmd/connect"
	"github.com/skeeey/xcm-cli/pkg/cmd/login"
	"github.com/skeeey/xcm-cli/pkg/cmd/logout"
	"github.com/skeeey/xcm-cli/pkg/cmd/relay"
	"github.com/skeeey/xcm-cli/pkg/cmd/version"
)

var root = &cobra.Command{
	Use:           "xcm",
	Long:          "Command line tool for xCM.",
	Run:           help,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	// Send logs to the standard error stream by default:
	err := flag.Set("logtostderr", "true")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Can't set default error stream: %v\n", err)
		os.Exit(1)
	}

	// Register the options that are managed by the 'flag' package, so that they will also be parsed
	// by the 'pflag' package:
	//pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

	// Register the subcommands:
	root.AddCommand(login.NewCmd())
	root.AddCommand(logout.NewCmd())
	root.AddCommand(connect.NewCmd())
	root.AddCommand(relay.NewCmd())
	root.AddCommand(clusters.NewCmd())
	root.AddCommand(version.NewCmd())
}

func main() {
	// This is needed to make `glog` believe that the flags have already been parsed, otherwise
	// every log messages is prefixed by an error message stating the the flags haven't been
	// parsed.
	err := flag.CommandLine.Parse([]string{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Can't parse empty command line to satisfy 'glog': %v\n", err)
		os.Exit(1)
	}

	// // Execute the root command and exit inmediately if there was no error:
	root.SetArgs(os.Args[1:])
	err = root.Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
		os.Exit(1)
	}
}

func help(cmd *cobra.Command, argv []string) {
	_ = cmd.Help()
}
