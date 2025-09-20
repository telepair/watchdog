package agent

import (
	"context"
	"time"

	"github.com/telepair/watchdog/internal/collector/system"
	"github.com/telepair/watchdog/pkg/version"
)

// AgentInfo represents basic agent information
type AgentInfo struct {
	AgentID    string            `json:"agent_id"`
	Version    version.Info      `json:"version"`
	StartedAt  time.Time         `json:"started_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
	SystemInfo system.SystemInfo `json:"system_info"`
}

// GetInfo returns current agent info
func (a *Agent) GetInfo() *AgentInfo {
	info := AgentInfo{
		AgentID:   a.config.ID,
		Version:   version.Get(),
		StartedAt: a.startedAt,
		UpdatedAt: time.Now(),
	}
	sysInfo, err := system.CollectSystemInfo(context.Background())
	if err != nil {
		a.logger.Error("failed to collect system info", "error", err)
	} else {
		info.SystemInfo = *sysInfo
	}
	return &info
}

// updateInfo updates agent info in KV store
func (a *Agent) updateInfo() {
	// Use KV store instead of regular publish
	key := "info" + "." + a.config.ID
	if err := a.bucket.Put(context.Background(), key, a.GetInfo()); err != nil {
		a.logger.Error("failed to put agent info to KV store", "error", err)
		return
	}
	a.logger.Debug("agent info reported successfully")
}

// runInfoReport runs the info reporting timer
func (a *Agent) runInfoReport() {
	a.updateInfo()

	ticker := time.NewTicker(time.Duration(a.config.ReportInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			a.updateInfo()
			a.logger.Debug("info report timer stopped")
			return
		case <-ticker.C:
			a.updateInfo()
		}
	}
}

// AgentStatus represents current agent status
type AgentStatus struct {
	AgentID          string    `json:"agent_id"`
	Running          bool      `json:"running"`
	CollectorHealthy bool      `json:"collector_healthy"`
	StartedAt        time.Time `json:"started_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// GetStatus returns current agent status
func (a *Agent) GetStatus() *AgentStatus {
	status := AgentStatus{
		AgentID:   a.config.ID,
		Running:   a.running.Load(),
		StartedAt: a.startedAt,
		UpdatedAt: time.Now(),
	}
	if err := a.collector.Health(); err != nil {
		status.CollectorHealthy = false
	} else {
		status.CollectorHealthy = true
	}
	return &status
}

// updateStatus updates agent status in KV store
func (a *Agent) updateStatus() {
	// Use KV store instead of regular publish
	key := "status" + "." + a.config.ID
	if err := a.bucket.Put(context.Background(), key, a.GetStatus()); err != nil {
		a.logger.Error("failed to put agent status to KV store", "error", err)
		return
	}
	a.logger.Debug("agent status reported successfully")
}

// runStatusReport runs the status reporting timer
func (a *Agent) runStatusReport() {
	a.updateStatus()

	ticker := time.NewTicker(time.Duration(a.config.HeartbeatInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			a.updateStatus()
			a.logger.Debug("status report timer stopped")
			return
		case <-ticker.C:
			a.updateStatus()
		}
	}
}

// startReport starts periodic reporting
func (a *Agent) startReport() {
	a.wg.Go(a.runInfoReport)

	// Start status reporting timer
	a.wg.Go(a.runStatusReport)

	a.logger.Debug("started periodic reporting timers",
		"info_interval", a.config.ReportInterval,
		"heartbeat_interval", a.config.HeartbeatInterval)
}
