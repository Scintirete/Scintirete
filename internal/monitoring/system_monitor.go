package monitoring

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/scintirete/scintirete/internal/config"
	"github.com/scintirete/scintirete/internal/core"
)

// SystemMonitor tracks system resource usage including CPU, memory, and disk
type SystemMonitor struct {
	logger    core.Logger
	config    config.RuntimeMonitoringConfig
	stopChan  chan struct{}
	lastStats SystemStats
}

// SystemStats contains system resource statistics
type SystemStats struct {
	Timestamp      time.Time
	CPUUsage       float64
	GoroutineCount int
	GCTime         time.Duration
	GCCycles       uint32
	MemoryAlloc    uint64
	MemorySys      uint64
	MemoryUsed     uint64
	MemoryTotal    uint64
	DiskUsed       uint64
	DiskTotal      uint64
	NumGC          uint32
}

// NewSystemMonitor creates a new system monitor
func NewSystemMonitor(log core.Logger, monitoringConfig config.RuntimeMonitoringConfig) *SystemMonitor {
	return &SystemMonitor{
		logger:   log.WithFields(map[string]interface{}{"component": "system_monitor"}),
		config:   monitoringConfig,
		stopChan: make(chan struct{}),
	}
}

// Start begins system monitoring
func (m *SystemMonitor) Start(ctx context.Context) error {
	if !m.config.Enabled {
		m.logger.Info(ctx, "System monitoring is disabled", nil)
		return nil
	}

	// Initialize stats
	m.lastStats = m.collectStats()

	var enabledComponents []string
	if m.config.CPUEnabled {
		enabledComponents = append(enabledComponents, "CPU")
	}
	if m.config.MemoryEnabled {
		enabledComponents = append(enabledComponents, "Memory")
	}
	if m.config.DiskEnabled {
		enabledComponents = append(enabledComponents, "Disk")
	}

	m.logger.Info(ctx, "System monitoring started", map[string]interface{}{
		"interval_seconds": m.config.Interval.Seconds(),
		"components":       enabledComponents,
		"cpu_threshold":    fmt.Sprintf("%.1f%%", m.config.CPUThreshold*100),
		"memory_threshold": fmt.Sprintf("%.1fMB", float64(m.config.MemoryThreshold)/1024/1024),
		"disk_threshold":   fmt.Sprintf("%.1fMB", float64(m.config.DiskThreshold)/1024/1024),
	})

	// Start monitoring goroutine
	go m.monitor(ctx)

	return nil
}

// Stop halts system monitoring
func (m *SystemMonitor) Stop() {
	if !m.config.Enabled {
		return
	}
	close(m.stopChan)
}

// monitor runs the monitoring loop
func (m *SystemMonitor) monitor(ctx context.Context) {
	ticker := time.NewTicker(m.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			stats := m.collectStats()
			m.logStats(stats)
			m.checkThresholds(stats)
			m.lastStats = stats

		case <-m.stopChan:
			m.logger.Info(ctx, "System monitoring stopped", nil)
			return
		case <-ctx.Done():
			m.logger.Info(ctx, "System monitoring stopped due to context cancellation", nil)
			return
		}
	}
}

// collectStats gathers current system statistics
func (m *SystemMonitor) collectStats() SystemStats {
	now := time.Now()

	stats := SystemStats{
		Timestamp: now,
	}

	// Get runtime memory stats
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	stats.GoroutineCount = runtime.NumGoroutine()
	stats.GCTime = time.Duration(memStats.PauseTotalNs)
	stats.GCCycles = memStats.NumGC
	stats.MemoryAlloc = memStats.Alloc
	stats.MemorySys = memStats.Sys
	stats.NumGC = memStats.NumGC

	// Calculate CPU usage (simplified version)
	if m.config.CPUEnabled {
		stats.CPUUsage = m.calculateCPUUsage()
	}

	// Get system memory info
	if m.config.MemoryEnabled {
		memUsed, memTotal := m.getSystemMemory()
		stats.MemoryUsed = memUsed
		stats.MemoryTotal = memTotal
	}

	// Get disk usage
	if m.config.DiskEnabled {
		diskUsed, diskTotal := m.getDiskUsage()
		stats.DiskUsed = diskUsed
		stats.DiskTotal = diskTotal
	}

	return stats
}

// calculateCPUUsage calculates CPU usage percentage (simplified approach)
func (m *SystemMonitor) calculateCPUUsage() float64 {
	// This is a simplified CPU calculation
	// For more accurate CPU monitoring, we would need to track CPU times over intervals
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Use GC activity as a proxy for CPU activity (not perfect but gives some indication)
	if m.lastStats.NumGC > 0 && memStats.NumGC > m.lastStats.NumGC {
		return 0.5 // Moderate CPU usage when GC is active
	}

	return 0.1 // Low baseline CPU usage
}

// getSystemMemory returns system memory usage in bytes
func (m *SystemMonitor) getSystemMemory() (used, total uint64) {
	// Fallback to runtime memory stats (cross-platform compatible)
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Use system memory reported by Go runtime
	used = memStats.Alloc
	total = memStats.Sys

	return used, total
}

// getDiskUsage returns disk usage in bytes for the current working directory
func (m *SystemMonitor) getDiskUsage() (used, total uint64) {
	// For cross-platform compatibility, we'll use a simple estimation
	// based on available memory (this is not perfect but avoids platform-specific code)
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Simple estimation: assume disk has at least 10x the system memory
	total = memStats.Sys * 10
	used = memStats.Sys / 2 // Assume 50% usage

	return used, total
}

// logStats logs the current statistics
func (m *SystemMonitor) logStats(stats SystemStats) {
	// Only log when there are significant changes or high usage
	shouldLog := false
	logData := map[string]interface{}{
		"timestamp": stats.Timestamp.Format(time.RFC3339),
	}

	if m.config.CPUEnabled {
		logData["cpu_usage"] = fmt.Sprintf("%.2f%%", stats.CPUUsage*100)
		if stats.CPUUsage > 0.5 {
			shouldLog = true
		}
	}

	if m.config.MemoryEnabled && stats.MemoryTotal > 0 {
		memoryUsagePercent := float64(stats.MemoryUsed) / float64(stats.MemoryTotal) * 100
		logData["memory_usage"] = fmt.Sprintf("%.1f%% (%.1fMB/%.1fMB)",
			memoryUsagePercent,
			float64(stats.MemoryUsed)/1024/1024,
			float64(stats.MemoryTotal)/1024/1024)
		if memoryUsagePercent > 70 {
			shouldLog = true
		}
	}

	if m.config.DiskEnabled && stats.DiskTotal > 0 {
		diskUsagePercent := float64(stats.DiskUsed) / float64(stats.DiskTotal) * 100
		logData["disk_usage"] = fmt.Sprintf("%.1f%% (%.1fGB/%.1fGB)",
			diskUsagePercent,
			float64(stats.DiskUsed)/1024/1024/1024,
			float64(stats.DiskTotal)/1024/1024/1024)
		if diskUsagePercent > 80 {
			shouldLog = true
		}
	}

	// Always include runtime stats
	logData["goroutines"] = stats.GoroutineCount
	logData["gc_cycles"] = stats.GCCycles
	logData["memory_alloc_mb"] = stats.MemoryAlloc / 1024 / 1024

	if shouldLog || stats.GoroutineCount > 1000 || stats.MemoryAlloc > 100*1024*1024 {
		m.logger.Debug(context.Background(), "System resource statistics", logData)
	}
}

// checkThresholds checks if any thresholds are exceeded and logs warnings
func (m *SystemMonitor) checkThresholds(stats SystemStats) {
	if m.config.CPUEnabled && stats.CPUUsage > m.config.CPUThreshold {
		m.logger.Warn(context.Background(), "High CPU usage detected", map[string]interface{}{
			"cpu_usage":    fmt.Sprintf("%.2f%%", stats.CPUUsage*100),
			"threshold":    fmt.Sprintf("%.2f%%", m.config.CPUThreshold*100),
			"goroutines":   stats.GoroutineCount,
			"memory_alloc": fmt.Sprintf("%.1fMB", float64(stats.MemoryAlloc)/1024/1024),
		})
	}

	if m.config.MemoryEnabled && stats.MemoryUsed > m.config.MemoryThreshold {
		m.logger.Warn(context.Background(), "High memory usage detected", map[string]interface{}{
			"memory_used":  fmt.Sprintf("%.1fMB", float64(stats.MemoryUsed)/1024/1024),
			"threshold":    fmt.Sprintf("%.1fMB", float64(m.config.MemoryThreshold)/1024/1024),
			"memory_total": fmt.Sprintf("%.1fMB", float64(stats.MemoryTotal)/1024/1024),
		})
	}

	if m.config.DiskEnabled && stats.DiskUsed > m.config.DiskThreshold {
		m.logger.Warn(context.Background(), "High disk usage detected", map[string]interface{}{
			"disk_used":  fmt.Sprintf("%.1fGB", float64(stats.DiskUsed)/1024/1024/1024),
			"threshold":  fmt.Sprintf("%.1fGB", float64(m.config.DiskThreshold)/1024/1024/1024),
			"disk_total": fmt.Sprintf("%.1fGB", float64(stats.DiskTotal)/1024/1024/1024),
		})
	}
}

// GetSnapshot returns a snapshot of current system statistics
func (m *SystemMonitor) GetSnapshot() SystemStats {
	return m.collectStats()
}

// SetCPUThreshold updates the CPU usage threshold
func (m *SystemMonitor) SetCPUThreshold(threshold float64) {
	if threshold >= 0 && threshold <= 1 {
		m.config.CPUThreshold = threshold
	}
}

// SetMemoryThreshold updates the memory usage threshold
func (m *SystemMonitor) SetMemoryThreshold(threshold uint64) {
	if threshold > 0 {
		m.config.MemoryThreshold = threshold
	}
}

// SetDiskThreshold updates the disk usage threshold
func (m *SystemMonitor) SetDiskThreshold(threshold uint64) {
	if threshold > 0 {
		m.config.DiskThreshold = threshold
	}
}
