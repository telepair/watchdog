package collector

import (
	"fmt"
	"strings"

	"github.com/telepair/watchdog/internal/collector/system"
	"github.com/telepair/watchdog/internal/collector/types"
)

type Manager struct {
	cfg        *Config
	reporter   types.Publisher
	collectors []types.Collector
}

func NewManager(agentID string, cfg *Config, reporter types.Publisher) (*Manager, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}
	if reporter == nil {
		return nil, fmt.Errorf("reporter is required")
	}

	m := &Manager{
		cfg:        cfg,
		reporter:   reporter,
		collectors: make([]types.Collector, 0, 1),
	}

	prefix := strings.TrimRight(cfg.AgentSubjectPrefix, ".>")
	prefix = strings.TrimRight(prefix, ".") + "." + agentID + "."

	collector, err := system.NewCollector(&cfg.System, prefix, reporter)
	if err != nil {
		return nil, err
	}
	m.collectors = append(m.collectors, collector)

	return m, nil
}

func (m *Manager) Start() error {
	for _, collector := range m.collectors {
		if err := collector.Start(); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) Stop() error {
	for _, collector := range m.collectors {
		if err := collector.Stop(); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) Health() error {
	for _, collector := range m.collectors {
		if err := collector.Health(); err != nil {
			return err
		}
	}
	return nil
}
