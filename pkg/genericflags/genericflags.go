package genericflags

import (
	"time"

	"github.com/spf13/pflag"
)

// AddFlag adds the debug flag to the given set of command line flags.
func AddFlag(flags *pflag.FlagSet) {
	flags.BoolVar(
		&debugEnabled,
		"debug",
		false,
		"Enable debug mode.",
	)

	flags.IntVar(
		&timeout,
		"timeout",
		30,
		"The timeout for command execution with seconds.",
	)
}

// Enabled retursn a boolean flag that indicates if the debug mode is enabled.
func DebugEnabled() bool {
	return debugEnabled
}

func TimeOut() time.Duration {
	return time.Duration(timeout) * time.Second
}

// debugEnabled is a boolean flag that indicates that the debug mode is enabled.
var debugEnabled bool
var timeout int
