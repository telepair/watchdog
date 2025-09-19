package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/telepair/watchdog/internal/config"
	"github.com/telepair/watchdog/internal/server"
	"github.com/telepair/watchdog/pkg/utils"
)

func newStartCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the monitoring server",
		Long:  "Start the watchdog server to manage agents and collect system metrics",
		RunE:  runServer,
	}
}

func runServer(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.LoadConfig(ConfigFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Override config with command line flags
	if err := cfg.Logger.SetLevel(LogLevel); err != nil {
		return fmt.Errorf("failed to set log level: %w", err)
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

	// Create and start unified server in server mode
	srv, err := server.NewServer(cfg)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	if err := srv.Start(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	// Wait for shutdown signal
	if err := srv.Wait(); err != nil {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}
