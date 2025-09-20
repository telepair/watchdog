package system

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
)

// SystemInfo represents basic system information
type SystemInfo struct {
	Hostname      string    `json:"hostname"`
	OS            string    `json:"os"`
	Architecture  string    `json:"architecture"`
	KernelVersion string    `json:"kernel_version"`
	CPUCount      int       `json:"cpu_count"`
	TotalMemory   uint64    `json:"total_memory"`
	CollectedAt   time.Time `json:"collected_at"`
}

// CollectSystemInfo collects basic system information
func CollectSystemInfo(ctx context.Context) (*SystemInfo, error) {
	// Check if context is already cancelled
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	hostInfo, err := host.InfoWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get host info: %w", err)
	}

	// Check context again between potentially slow operations
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	vmStat, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get memory info: %w", err)
	}

	collectedAt := time.Now()
	return &SystemInfo{
		Hostname:      hostInfo.Hostname,
		OS:            hostInfo.OS,
		Architecture:  hostInfo.KernelArch,
		KernelVersion: hostInfo.KernelVersion,
		CPUCount:      runtime.NumCPU(),
		TotalMemory:   vmStat.Total,
		CollectedAt:   collectedAt,
	}, nil
}

// CPUMetrics represents CPU usage metrics
type CPUMetrics struct {
	UsagePercent []float64 `json:"usage_percent"`
	LoadAverage  []float64 `json:"load_average"`
	CollectedAt  time.Time `json:"collected_at"`
}

// CollectCPU collects CPU usage metrics
func CollectCPU(ctx context.Context) (*CPUMetrics, error) {
	// Get CPU usage percentages for each core (non-blocking)
	percentages, err := cpu.PercentWithContext(ctx, 0, true)
	if err != nil {
		return nil, fmt.Errorf("failed to get CPU percentages: %w", err)
	}

	// Get load average (if available on platform)
	var loadAvg []float64
	if loadInfo, err := load.AvgWithContext(ctx); err == nil {
		loadAvg = []float64{loadInfo.Load1, loadInfo.Load5, loadInfo.Load15}
	}

	collectedAt := time.Now()
	return &CPUMetrics{
		UsagePercent: percentages,
		LoadAverage:  loadAvg,
		CollectedAt:  collectedAt,
	}, nil
}

// MemoryMetrics represents memory usage metrics
type MemoryMetrics struct {
	TotalBytes     uint64    `json:"total_bytes"`
	AvailableBytes uint64    `json:"available_bytes"`
	UsedBytes      uint64    `json:"used_bytes"`
	FreeBytes      uint64    `json:"free_bytes"`
	UsagePercent   float64   `json:"usage_percent"`
	CollectedAt    time.Time `json:"collected_at"`
}

// CollectMemory collects memory usage metrics
func CollectMemory(ctx context.Context) (*MemoryMetrics, error) {
	vmStat, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get memory stats: %w", err)
	}

	collectedAt := time.Now()
	return &MemoryMetrics{
		TotalBytes:     vmStat.Total,
		AvailableBytes: vmStat.Available,
		UsedBytes:      vmStat.Used,
		FreeBytes:      vmStat.Free,
		UsagePercent:   vmStat.UsedPercent,
		CollectedAt:    collectedAt,
	}, nil
}

// DiskMetrics represents disk usage metrics
type DiskMetrics struct {
	MountPoint   string    `json:"mount_point"`
	TotalBytes   uint64    `json:"total_bytes"`
	UsedBytes    uint64    `json:"used_bytes"`
	FreeBytes    uint64    `json:"free_bytes"`
	UsagePercent float64   `json:"usage_percent"`
	CollectedAt  time.Time `json:"collected_at"`
}

// CollectDisk collects disk usage metrics for all mounted filesystems
func CollectDisk(ctx context.Context) ([]DiskMetrics, error) {
	partitions, err := disk.PartitionsWithContext(ctx, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get disk partitions: %w", err)
	}

	// Pre-allocate slice to avoid repeated allocations
	metrics := make([]DiskMetrics, 0, len(partitions))
	collectedAt := time.Now() // Single timestamp for all metrics

	for _, partition := range partitions {
		usage, err := disk.UsageWithContext(ctx, partition.Mountpoint)
		if err != nil {
			// Skip partitions we can't read
			continue
		}

		metrics = append(metrics, DiskMetrics{
			MountPoint:   partition.Mountpoint,
			TotalBytes:   usage.Total,
			UsedBytes:    usage.Used,
			FreeBytes:    usage.Free,
			UsagePercent: usage.UsedPercent,
			CollectedAt:  collectedAt, // Reuse timestamp
		})
	}

	return metrics, nil
}

// NetworkMetrics represents network usage metrics
type NetworkMetrics struct {
	Interface   string    `json:"interface"`
	BytesSent   uint64    `json:"bytes_sent"`
	BytesRecv   uint64    `json:"bytes_recv"`
	PacketsSent uint64    `json:"packets_sent"`
	PacketsRecv uint64    `json:"packets_recv"`
	ErrorsIn    uint64    `json:"errors_in"`
	ErrorsOut   uint64    `json:"errors_out"`
	CollectedAt time.Time `json:"collected_at"`
}

// CollectNetwork collects network interface metrics
func CollectNetwork(ctx context.Context) ([]NetworkMetrics, error) {
	interfaces, err := net.IOCountersWithContext(ctx, true)
	if err != nil {
		return nil, fmt.Errorf("failed to get network stats: %w", err)
	}

	// Pre-allocate slice to avoid repeated allocations
	metrics := make([]NetworkMetrics, 0, len(interfaces))
	collectedAt := time.Now() // Single timestamp for all metrics

	for _, iface := range interfaces {
		metrics = append(metrics, NetworkMetrics{
			Interface:   iface.Name,
			BytesSent:   iface.BytesSent,
			BytesRecv:   iface.BytesRecv,
			PacketsSent: iface.PacketsSent,
			PacketsRecv: iface.PacketsRecv,
			ErrorsIn:    iface.Errin,
			ErrorsOut:   iface.Errout,
			CollectedAt: collectedAt, // Reuse timestamp
		})
	}

	return metrics, nil
}

// LoadMetrics represents system load metrics
type LoadMetrics struct {
	Load1       float64   `json:"load_1"`
	Load5       float64   `json:"load_5"`
	Load15      float64   `json:"load_15"`
	CollectedAt time.Time `json:"collected_at"`
}

// CollectLoad collects system load metrics
func CollectLoad(ctx context.Context) (*LoadMetrics, error) {
	loadInfo, err := load.AvgWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get load average: %w", err)
	}

	collectedAt := time.Now()
	return &LoadMetrics{
		Load1:       loadInfo.Load1,
		Load5:       loadInfo.Load5,
		Load15:      loadInfo.Load15,
		CollectedAt: collectedAt,
	}, nil
}

// UptimeMetrics represents system uptime
type UptimeMetrics struct {
	UptimeSeconds uint64    `json:"uptime_seconds"`
	BootTime      time.Time `json:"boot_time"`
	CollectedAt   time.Time `json:"collected_at"`
}

// CollectUptime collects system uptime metrics
func CollectUptime(ctx context.Context) (*UptimeMetrics, error) {
	bootTime, err := host.BootTimeWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get boot time: %w", err)
	}

	uptime, err := host.UptimeWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get uptime: %w", err)
	}

	// Safely convert uint64 to int64, handling potential overflow
	var bootTimeInt64 int64
	if bootTime > 1<<63-1 {
		// If bootTime exceeds int64 max, use current time as fallback
		bootTimeInt64 = time.Now().Unix()
	} else {
		bootTimeInt64 = int64(bootTime)
	}

	collectedAt := time.Now()
	return &UptimeMetrics{
		UptimeSeconds: uptime,
		BootTime:      time.Unix(bootTimeInt64, 0),
		CollectedAt:   collectedAt,
	}, nil
}
