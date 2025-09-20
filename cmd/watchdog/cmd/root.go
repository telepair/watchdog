package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/telepair/watchdog/internal/config"
	"github.com/telepair/watchdog/pkg/utils"
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

func loadConfig() (*config.Config, error) {
	configPath, err := utils.ExpandPath(ConfigFile)
	if err != nil {
		return nil, fmt.Errorf("failed to expand config file path: %w", err)
	}

	// Check if file already exists
	if _, err := os.Stat(configPath); err != nil {
		return nil, fmt.Errorf("configuration file %s does not exist", configPath)
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	if err := processConfig(cfg); err != nil {
		return nil, fmt.Errorf("failed to process config: %w", err)
	}

	return cfg, nil
}

func processConfig(cfg *config.Config) error {
	if err := cfg.Parse(); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}
	if LogLevel != "" {
		if err := cfg.Logger.SetLevel(LogLevel); err != nil {
			return fmt.Errorf("failed to set log level: %w", err)
		}
	}
	if NatsURL != "" {
		cfg.NATS.URLs = []string{NatsURL}
	}
	if EmbedNatsStoragePath != "" && cfg.Server.EnableEmbedNATS && cfg.Server.EmbedNATS != nil {
		path, err := utils.EnsurePath(EmbedNatsStoragePath)
		if err != nil {
			return fmt.Errorf("failed to ensure embed NATS storage path: %w", err)
		}
		cfg.Server.EmbedNATS.StorePath = path
	}

	subjectPrefix := strings.TrimRight(cfg.Collector.AgentSubjectPrefix, ".>")
	subjectPrefix = strings.TrimRight(subjectPrefix, ".") + ".>"
	cfg.Collector.AgentStream.Subjects = []string{subjectPrefix}
	return nil
}
