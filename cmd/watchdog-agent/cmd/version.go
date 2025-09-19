package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/telepair/watchdog/pkg/version"
)

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Long:  "Display version, build, and runtime information for watchdog-agent",
		Run: func(cmd *cobra.Command, args []string) {
			info := version.Get()
			fmt.Println(info.String())
		},
	}
}
