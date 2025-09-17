package health

import (
	"errors"
	"fmt"
	"net"
	"strings"
)

// Default values for health configuration.
const (
	DefaultAddr        = ":9091"
	DefaultLivezPath   = "/livez"
	DefaultReadyzPath  = "/readyz"
	DefaultMetricsPath = "/metrics"
)

// Config holds configuration for health components.
type Config struct {
	Addr             string `json:"addr"              yaml:"addr"`
	LivezPath        string `json:"livez_path"        yaml:"livez_path"`
	ReadyzPath       string `json:"readyz_path"       yaml:"readyz_path"`
	MetricsPath      string `json:"metrics_path"      yaml:"metrics_path"`
	MetricsNamespace string `json:"metrics_namespace" yaml:"metrics_namespace"`
}

// DefaultConfig returns a configuration with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Addr:        DefaultAddr,
		LivezPath:   DefaultLivezPath,
		ReadyzPath:  DefaultReadyzPath,
		MetricsPath: DefaultMetricsPath,
	}
}

// Parse validates and normalizes the configuration.
func (c *Config) Parse() error {
	c.applyDefaults()
	c.normalizeServerPaths()

	// Validate server address
	if err := validateAddr(c.Addr); err != nil {
		return fmt.Errorf("invalid addr: %w", err)
	}

	// Validate unique paths
	return validateUniquePaths(c.LivezPath, c.ReadyzPath, c.MetricsPath)
}

// applyDefaults sets default values for empty fields.
func (c *Config) applyDefaults() {
	if c.Addr == "" {
		c.Addr = DefaultAddr
	}
	if c.LivezPath == "" {
		c.LivezPath = DefaultLivezPath
	}
	if c.ReadyzPath == "" {
		c.ReadyzPath = DefaultReadyzPath
	}
	if c.MetricsPath == "" {
		c.MetricsPath = DefaultMetricsPath
	}
}

// normalizeServerPaths ensures all HTTP paths start with "/".
func (c *Config) normalizeServerPaths() {
	c.LivezPath = normalizePath(c.LivezPath)
	c.ReadyzPath = normalizePath(c.ReadyzPath)
	c.MetricsPath = normalizePath(c.MetricsPath)
}

// validateAddr validates the TCP address format without binding to the port.
func validateAddr(addr string) error {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return errors.New("addr is required")
	}
	if _, err := net.ResolveTCPAddr("tcp", addr); err != nil {
		return fmt.Errorf("resolve tcp addr %q: %w", addr, err)
	}
	return nil
}

// normalizePath ensures a path starts with "/".
func normalizePath(path string) string {
	if !strings.HasPrefix(path, "/") {
		return "/" + path
	}
	return path
}

// validateUniquePaths checks that all provided paths are unique after normalization.
func validateUniquePaths(livezPath, readyzPath, metricsPath string) error {
	// Normalize paths before comparison
	normalizedLivez := normalizePath(livezPath)
	normalizedReadyz := normalizePath(readyzPath)
	normalizedMetrics := normalizePath(metricsPath)

	if normalizedLivez == normalizedReadyz ||
		normalizedLivez == normalizedMetrics ||
		normalizedReadyz == normalizedMetrics {
		return errors.New("livez, readyz, and metrics paths must be unique")
	}
	return nil
}
