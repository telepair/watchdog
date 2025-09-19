package collector

import (
	"context"
	"testing"
	"time"
)

func TestCollectSystemInfo(t *testing.T) {
	ctx := context.Background()

	sysInfo, err := CollectSystemInfo(ctx)
	if err != nil {
		t.Fatalf("CollectSystemInfo failed: %v", err)
	}

	// Basic validation
	if sysInfo.Hostname == "" {
		t.Error("Hostname should not be empty")
	}
	if sysInfo.OS == "" {
		t.Error("OS should not be empty")
	}
	if sysInfo.CPUCount <= 0 {
		t.Error("CPU count should be positive")
	}
	if sysInfo.TotalMemory == 0 {
		t.Error("Total memory should be positive")
	}
	if sysInfo.CollectedAt.IsZero() {
		t.Error("CollectedAt should be set")
	}
}

func TestCollectSystemInfoWithTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// This should still work within timeout
	_, err := CollectSystemInfo(ctx)
	if err != nil {
		t.Fatalf("CollectSystemInfo with timeout failed: %v", err)
	}
}

func TestCollectSystemInfoWithCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := CollectSystemInfo(ctx)
	if err == nil {
		t.Error("Expected error with cancelled context")
	}
}

func TestCollectCPU(t *testing.T) {
	ctx := context.Background()

	cpuMetrics, err := CollectCPU(ctx)
	if err != nil {
		t.Fatalf("CollectCPU failed: %v", err)
	}

	// Validate CPU metrics
	if len(cpuMetrics.UsagePercent) == 0 {
		t.Error("CPU usage percentages should not be empty")
	}
	for i, usage := range cpuMetrics.UsagePercent {
		if usage < 0 || usage > 100 {
			t.Errorf("CPU usage[%d] should be between 0-100, got: %f", i, usage)
		}
	}
	if cpuMetrics.CollectedAt.IsZero() {
		t.Error("CollectedAt should be set")
	}
}

func TestCollectCPUNonBlocking(t *testing.T) {
	start := time.Now()

	ctx := context.Background()
	_, err := CollectCPU(ctx)

	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("CollectCPU failed: %v", err)
	}

	// Should complete much faster than 1 second (non-blocking)
	if elapsed > 500*time.Millisecond {
		t.Errorf("CPU collection took too long: %v (should be non-blocking)", elapsed)
	}
}

func TestCollectMemory(t *testing.T) {
	ctx := context.Background()

	memMetrics, err := CollectMemory(ctx)
	if err != nil {
		t.Fatalf("CollectMemory failed: %v", err)
	}

	// Validate memory metrics
	if memMetrics.TotalBytes == 0 {
		t.Error("Total memory should be positive")
	}
	if memMetrics.UsagePercent < 0 || memMetrics.UsagePercent > 100 {
		t.Errorf("Memory usage percent should be between 0-100, got: %f", memMetrics.UsagePercent)
	}
	if memMetrics.CollectedAt.IsZero() {
		t.Error("CollectedAt should be set")
	}
}

func TestCollectDisk(t *testing.T) {
	ctx := context.Background()

	diskMetrics, err := CollectDisk(ctx)
	if err != nil {
		t.Fatalf("CollectDisk failed: %v", err)
	}

	// Should have at least one mount point
	if len(diskMetrics) == 0 {
		t.Error("Should have at least one disk metric")
	}

	for i, disk := range diskMetrics {
		if disk.MountPoint == "" {
			t.Errorf("Disk[%d] mount point should not be empty", i)
		}
		// Some virtual/special filesystems may have 0 total bytes, skip validation
		if disk.TotalBytes == 0 {
			t.Logf("Disk[%d] (%s) has zero total bytes, likely a special filesystem", i, disk.MountPoint)
		}
		if disk.UsagePercent < 0 || disk.UsagePercent > 100 {
			t.Errorf("Disk[%d] usage percent should be between 0-100, got: %f", i, disk.UsagePercent)
		}
		if disk.CollectedAt.IsZero() {
			t.Errorf("Disk[%d] CollectedAt should be set", i)
		}
	}
}

func TestCollectNetwork(t *testing.T) {
	ctx := context.Background()

	netMetrics, err := CollectNetwork(ctx)
	if err != nil {
		t.Fatalf("CollectNetwork failed: %v", err)
	}

	// Should have at least one network interface
	if len(netMetrics) == 0 {
		t.Error("Should have at least one network metric")
	}

	for i, net := range netMetrics {
		if net.Interface == "" {
			t.Errorf("Network[%d] interface should not be empty", i)
		}
		if net.CollectedAt.IsZero() {
			t.Errorf("Network[%d] CollectedAt should be set", i)
		}
	}
}

func TestCollectLoad(t *testing.T) {
	ctx := context.Background()

	loadMetrics, err := CollectLoad(ctx)
	if err != nil {
		// Load average might not be available on all platforms
		t.Logf("CollectLoad not available on this platform: %v", err)
		return
	}

	// Validate load metrics
	if loadMetrics.Load1 < 0 {
		t.Error("Load1 should not be negative")
	}
	if loadMetrics.Load5 < 0 {
		t.Error("Load5 should not be negative")
	}
	if loadMetrics.Load15 < 0 {
		t.Error("Load15 should not be negative")
	}
	if loadMetrics.CollectedAt.IsZero() {
		t.Error("CollectedAt should be set")
	}
}

func TestCollectUptime(t *testing.T) {
	ctx := context.Background()

	uptimeMetrics, err := CollectUptime(ctx)
	if err != nil {
		t.Fatalf("CollectUptime failed: %v", err)
	}

	// Validate uptime metrics
	if uptimeMetrics.UptimeSeconds == 0 {
		t.Error("Uptime should be positive")
	}
	if uptimeMetrics.BootTime.IsZero() {
		t.Error("Boot time should be set")
	}
	if uptimeMetrics.CollectedAt.IsZero() {
		t.Error("CollectedAt should be set")
	}

	// Boot time should be before current time
	if uptimeMetrics.BootTime.After(time.Now()) {
		t.Error("Boot time should be in the past")
	}
}

func TestCollectUptimeWithTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// This should work quickly
	_, err := CollectUptime(ctx)
	if err != nil {
		t.Fatalf("CollectUptime with timeout failed: %v", err)
	}
}

func BenchmarkCollectCPU(b *testing.B) {
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := CollectCPU(ctx)
		if err != nil {
			b.Fatalf("CollectCPU failed: %v", err)
		}
	}
}

func BenchmarkCollectMemory(b *testing.B) {
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := CollectMemory(ctx)
		if err != nil {
			b.Fatalf("CollectMemory failed: %v", err)
		}
	}
}

func BenchmarkCollectSystemInfo(b *testing.B) {
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := CollectSystemInfo(ctx)
		if err != nil {
			b.Fatalf("CollectSystemInfo failed: %v", err)
		}
	}
}
