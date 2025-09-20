package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/telepair/watchdog/internal/server"
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
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
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
