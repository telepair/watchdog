package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/telepair/watchdog/internal/config"
	"github.com/telepair/watchdog/pkg/utils"
)

func newConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Configuration management",
		Long:  "Manage watchdog configuration files",
	}

	cmd.AddCommand(newConfigShowCommand())
	cmd.AddCommand(newConfigValidateCommand())
	cmd.AddCommand(newConfigInitCommand())

	return cmd
}

func newConfigShowCommand() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		Long:  "Display the current configuration with all resolved values",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			switch format {
			case "yaml":
				data, err := yaml.Marshal(cfg)
				if err != nil {
					return fmt.Errorf("failed to marshal config to YAML: %w", err)
				}
				fmt.Printf("# Configuration from: %s\n", ConfigFile)
				fmt.Print(string(data))
			case "summary":
				fmt.Printf("Configuration file: %s\n", ConfigFile)
				fmt.Printf("Embedded NATS: %t\n", cfg.Server.EnableEmbedNATS)
				fmt.Printf("Agent ID: %s\n", cfg.Agent.ID)
				fmt.Printf("NATS URLs: %v\n", cfg.NATS.URLs)
				fmt.Printf("Console Log Level: %s\n", cfg.Logger.Console.Level)
				fmt.Printf("File Log Level: %s\n", cfg.Logger.File.Level)
				fmt.Printf("Health Check Address: %s\n", cfg.HealthAddr)
			default:
				return fmt.Errorf("unsupported format: %s", format)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "yaml", "Output format (yaml, summary)")

	return cmd
}

func newConfigValidateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate configuration file",
		Long:  "Check if the configuration file is valid and properly formatted",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := loadConfig()
			if err != nil {
				return fmt.Errorf("invalid configuration: %w", err)
			}

			fmt.Printf("Configuration file %s is valid\n", ConfigFile)
			return nil
		},
	}
}

func newConfigInitCommand() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize configuration file",
		Long:  "Create a new configuration file with default values",
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath, err := utils.ExpandPath(ConfigFile)
			if err != nil {
				return fmt.Errorf("failed to expand config file path: %w", err)
			}

			// Check if file already exists
			if _, err := os.Stat(configPath); err == nil && !force {
				return fmt.Errorf("configuration file %s already exists, use --force to overwrite", configPath)
			}

			// Generate default config for server
			cfg := config.DefaultConfig()
			if err := cfg.Parse(); err != nil {
				return fmt.Errorf("failed to parse config: %w", err)
			}

			if err := processConfig(cfg); err != nil {
				return fmt.Errorf("failed to process config: %w", err)
			}

			// Marshal to YAML
			data, err := yaml.Marshal(cfg)
			if err != nil {
				return fmt.Errorf("failed to marshal config: %w", err)
			}

			// Write to file
			if err := os.WriteFile(configPath, data, 0600); err != nil {
				return fmt.Errorf("failed to write config file: %w", err)
			}

			fmt.Printf("Configuration file created: %s\n", configPath)
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing configuration file")

	return cmd
}
