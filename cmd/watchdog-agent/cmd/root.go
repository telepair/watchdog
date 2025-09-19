package cmd

import (
	"github.com/spf13/cobra"
)

var (
	ConfigFile string
	LogLevel   string
	NatsURL    string
)

func Execute() error {
	return newRootCommand().Execute()
}

func newRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watchdog-agent",
		Short: "Watchdog monitoring agent",
		Long:  "A monitoring agent that collects system metrics and reports to the watchdog server",
	}

	cmd.PersistentFlags().StringVarP(&ConfigFile, "config", "c", "~/.watchdog.yaml", "Configuration file path")
	cmd.PersistentFlags().StringVarP(&LogLevel, "log-level", "l", "info", "Log level (debug, info, warn, error)")
	cmd.PersistentFlags().StringVarP(&NatsURL, "nats-url", "n", "", "NATS server URL")

	// Add subcommands
	cmd.AddCommand(newStartCommand())
	cmd.AddCommand(newVersionCommand())
	cmd.AddCommand(newConfigCommand())

	return cmd
}
