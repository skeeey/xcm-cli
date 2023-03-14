package printer

import (
	"fmt"
	"os"

	"github.com/skeeey/xcm-cli/pkg/rest"
)

// ID       string `json:"id"`
// 	Status   string `json:"status"`
// 	Type     string `json:"type"`
// 	Version  string `json:"version"`
// 	Platform string `json:"platform"`
// 	Region   string `json:"region"`

func PrintClustersTable(clusters ...rest.Cluster) {
	fmt.Fprintln(os.Stdout, "ID\t\t\t\t\t Status\t\t Type\t Version\t\t Platform")

	for _, cluster := range clusters {
		platform := fmt.Sprintf("%s (%s)", cluster.Platform, cluster.Region)
		fmt.Fprintln(os.Stdout, cluster.ID, "\t", cluster.Status, "\t", cluster.Type, "\t", cluster.Version, "\t", platform)
	}
}
