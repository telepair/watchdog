package cmd

import (
	"github.com/spf13/cobra"
)

var (
	ConfigFile           string
	LogLevel             string
	NatsURL              string
	EmbedNatsStoragePath string
)

func Execute() error {
	return newRootCommand().Execute()
}

func newRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watchdog",
		Short: "Watchdog monitoring server",
		Long:  "A monitoring server that manages agents and collects system metrics",
	}

	cmd.PersistentFlags().StringVarP(&ConfigFile, "config", "c", "~/.watchdog.yaml", "Configuration file path")
	cmd.PersistentFlags().StringVarP(&LogLevel, "log-level", "l", "info", "Log level (debug, info, warn, error)")
	cmd.PersistentFlags().StringVarP(&NatsURL, "nats-url", "n", "", "NATS server URL")
	cmd.PersistentFlags().StringVarP(&EmbedNatsStoragePath, "embed-nats-storage-path", "", "~/.watchdog/data/nats", "Embedded NATS server storage path")

	// Add subcommands
	cmd.AddCommand(newStartCommand())
	cmd.AddCommand(newVersionCommand())
	cmd.AddCommand(newConfigCommand())

	return cmd
}
