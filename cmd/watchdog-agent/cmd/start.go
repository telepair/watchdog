package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/telepair/watchdog/internal/server"
)

func newStartCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the monitoring agent",
		Long:  "Start the agent server to collect and report system metrics",
		RunE:  runAgent,
	}
}

func runAgent(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create and start unified server in agent mode
	srv, err := server.NewAgent(cfg)
	if err != nil {
		return fmt.Errorf("failed to create agent server: %w", err)
	}

	if err := srv.Start(); err != nil {
		return fmt.Errorf("failed to start agent server: %w", err)
	}

	// Wait for shutdown signal
	if err := srv.Wait(); err != nil {
		return fmt.Errorf("agent server error: %w", err)
	}

	return nil
}
