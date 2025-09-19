package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/telepair/watchdog/internal/agent/collector"
	"github.com/telepair/watchdog/pkg/version"
)

// AgentInfo represents basic agent information
type AgentInfo struct {
	AgentID    string               `json:"agent_id"`
	Version    version.Info         `json:"version"`
	StartedAt  time.Time            `json:"started_at"`
	UpdatedAt  time.Time            `json:"updated_at"`
	SystemInfo collector.SystemInfo `json:"system_info"`
}

// initializeInfo initializes agent info
func (a *Agent) initializeInfo() {
	sysInfo, err := collector.CollectSystemInfo(context.Background())
	if err != nil {
		a.logger.Error("failed to collect system info", "error", err)
		sysInfo = &collector.SystemInfo{}
	}
	a.info = &AgentInfo{
		AgentID:    a.config.ID,
		Version:    version.Get(),
		StartedAt:  a.startedAt,
		UpdatedAt:  time.Now(),
		SystemInfo: *sysInfo,
	}
}

// GetInfo returns current agent info
func (a *Agent) GetInfo() *AgentInfo {
	info := *a.info
	info.Version = version.Get()
	info.UpdatedAt = time.Now()
	sysInfo, err := collector.CollectSystemInfo(context.Background())
	if err != nil {
		a.logger.Error("failed to collect system info", "error", err)
	} else {
		info.SystemInfo = *sysInfo
	}
	return &info
}

// updateInfo updates agent info in KV store
func (a *Agent) updateInfo() error {
	info := a.GetInfo()

	data, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("failed to marshal agent info: %w", err)
	}

	// Use KV store instead of regular publish
	key := a.bucketCfg.InfoKey(a.config.ID)
	if err := a.natsClient.PutKV(context.Background(), a.bucketCfg.BucketName(), key, data); err != nil {
		return fmt.Errorf("failed to put agent info to KV store: %w", err)
	}

	return nil
}

// runInfoReportTimer runs the info reporting timer
func (a *Agent) runInfoReportTimer() {
	ticker := time.NewTicker(time.Duration(a.config.InfoReportInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			a.logger.Debug("info report timer stopped")
			return
		case <-ticker.C:
			if err := a.updateInfo(); err != nil {
				a.logger.Error("failed to update agent info", "error", err)
			} else {
				a.logger.Debug("agent info reported successfully")
			}
		}
	}
}

// AgentStatus represents current agent status
type AgentStatus struct {
	AgentID          string    `json:"agent_id"`
	Status           string    `json:"status"` // running, stopping, stopped
	CollectorRunning bool      `json:"collector_running"`
	ExecutorRunning  bool      `json:"executor_running"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// initializeStatus initializes agent status
func (a *Agent) initializeStatus() {
	a.status = &AgentStatus{
		AgentID:          a.config.ID,
		Status:           "stopped",
		CollectorRunning: false,
		ExecutorRunning:  false,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
}

// GetStatus returns current agent status
func (a *Agent) GetStatus() *AgentStatus {
	status := *a.status
	status.UpdatedAt = time.Now()
	status.CollectorRunning = a.collector.IsRunning()
	status.ExecutorRunning = a.executor.IsRunning()
	return &status
}

// updateStatus updates agent status in KV store
func (a *Agent) updateStatus() error {
	status := a.GetStatus()

	data, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("failed to marshal agent status: %w", err)
	}

	// Use KV store instead of regular publish
	key := a.bucketCfg.StatusKey(a.config.ID)
	if err := a.natsClient.PutKV(context.Background(), a.bucketCfg.BucketName(), key, data); err != nil {
		return fmt.Errorf("failed to put agent status to KV store: %w", err)
	}

	return nil
}

// runStatusReportTimer runs the status reporting timer
func (a *Agent) runStatusReportTimer() {
	ticker := time.NewTicker(time.Duration(a.config.HeartbeatInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			a.logger.Debug("status report timer stopped")
			return
		case <-ticker.C:
			if err := a.updateStatus(); err != nil {
				a.logger.Error("failed to update agent status", "error", err)
			} else {
				a.logger.Debug("agent status reported successfully")
			}
		}
	}
}

// publishEvent publishes an agent lifecycle event
func (a *Agent) publishEvent(eventType string) error {
	subject := a.streamCfg.EventSubject(a.config.ID, eventType)

	event := map[string]any{
		"agent_id":   a.config.ID,
		"event_type": eventType,
		"timestamp":  time.Now(),
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	if err := a.natsClient.Publish(subject, data); err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	return nil
}

// startReportTimers starts periodic info and status reporting timers
func (a *Agent) startReportTimers() {
	// Start info reporting timer
	a.wg.Go(a.runInfoReportTimer)

	// Start status reporting timer
	a.wg.Go(a.runStatusReportTimer)

	a.logger.Debug("started periodic reporting timers",
		"info_interval", a.config.InfoReportInterval,
		"heartbeat_interval", a.config.HeartbeatInterval)
}
