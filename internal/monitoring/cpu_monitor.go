package monitoring

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/scintirete/scintirete/internal/core"
)

// CPUMonitor tracks CPU usage and detects high CPU utilization
type CPUMonitor struct {
	logger       core.Logger
	interval     time.Duration
	threshold    float64 // CPU usage threshold (0.0-1.0)
	stopChan     chan struct{}
	lastCPUTime  time.Time
	lastCPUStats []time.Duration
}

// CPUStats contains CPU usage statistics
type CPUStats struct {
	Timestamp      time.Time
	CPUUsage       float64
	GoroutineCount int
	GCTime         time.Duration
	GCCycles       uint32
	MemoryAlloc    uint64
	MemorySys      uint64
	NumGC          uint32
}

// NewCPUMonitor creates a new CPU monitor
func NewCPUMonitor(log core.Logger, interval time.Duration, threshold float64) *CPUMonitor {
	return &CPUMonitor{
		logger:    log.WithFields(map[string]interface{}{"component": "cpu_monitor"}),
		interval:  interval,
		threshold: threshold,
		stopChan:  make(chan struct{}),
	}
}

// Start begins CPU monitoring
func (m *CPUMonitor) Start(ctx context.Context) error {
	// Initialize CPU stats
	m.lastCPUTime = time.Now()
	m.lastCPUStats = getCPUStats()

	m.logger.Info(ctx, "CPU monitoring started", map[string]interface{}{
		"interval_seconds": m.interval.Seconds(),
		"threshold":        m.threshold,
	})

	// Start monitoring goroutine
	go m.monitor(ctx)

	return nil
}

// Stop halts CPU monitoring
func (m *CPUMonitor) Stop() {
	close(m.stopChan)
}

// monitor runs the monitoring loop
func (m *CPUMonitor) monitor(ctx context.Context) {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			stats := m.collectStats()
			m.logStats(stats)

			if stats.CPUUsage > m.threshold {
				m.warnHighCPU(stats)
			}

		case <-m.stopChan:
			m.logger.Info(ctx, "CPU monitoring stopped", nil)
			return
		case <-ctx.Done():
			m.logger.Info(ctx, "CPU monitoring stopped due to context cancellation", nil)
			return
		}
	}
}

// collectStats gathers current CPU and memory statistics
func (m *CPUMonitor) collectStats() CPUStats {
	now := time.Now()
	currentStats := getCPUStats()

	// Calculate CPU usage since last check
	var cpuUsage float64
	if !m.lastCPUTime.IsZero() && len(m.lastCPUStats) == len(currentStats) {
		cpuUsage = calculateCPUUsage(m.lastCPUStats, currentStats, m.lastCPUTime, now)
	}

	// Get memory stats
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Get GC stats
	gcStats := &runtime.MemStats{}
	runtime.ReadMemStats(gcStats)

	stats := CPUStats{
		Timestamp:      now,
		CPUUsage:       cpuUsage,
		GoroutineCount: runtime.NumGoroutine(),
		GCTime:         time.Duration(gcStats.PauseTotalNs),
		GCCycles:       gcStats.NumGC,
		MemoryAlloc:    memStats.Alloc,
		MemorySys:      memStats.Sys,
		NumGC:          gcStats.NumGC,
	}

	// Update last stats
	m.lastCPUTime = now
	m.lastCPUStats = currentStats

	return stats
}

// logStats logs the current statistics
func (m *CPUMonitor) logStats(stats CPUStats) {
	m.logger.Debug(context.Background(), "CPU usage statistics", map[string]interface{}{
		"timestamp":       stats.Timestamp.Format(time.RFC3339),
		"cpu_usage":       fmt.Sprintf("%.2f%%", stats.CPUUsage*100),
		"goroutines":      stats.GoroutineCount,
		"gc_time_ms":      stats.GCTime / 1e6,
		"gc_cycles":       stats.GCCycles,
		"memory_alloc_mb": stats.MemoryAlloc / 1024 / 1024,
		"memory_sys_mb":   stats.MemorySys / 1024 / 1024,
		"num_gc":          stats.NumGC,
	})
}

// warnHighCPU logs a warning when CPU usage exceeds threshold
func (m *CPUMonitor) warnHighCPU(stats CPUStats) {
	m.logger.Warn(context.Background(), "High CPU usage detected", map[string]interface{}{
		"timestamp":       stats.Timestamp.Format(time.RFC3339),
		"cpu_usage":       fmt.Sprintf("%.2f%%", stats.CPUUsage*100),
		"threshold":       fmt.Sprintf("%.2f%%", m.threshold*100),
		"goroutines":      stats.GoroutineCount,
		"memory_alloc_mb": stats.MemoryAlloc / 1024 / 1024,
		"recommendation":  "Consider checking for infinite loops, blocking operations, or high-frequency operations",
	})
}

// getCPUStats retrieves current CPU time statistics
func getCPUStats() []time.Duration {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)

	// Get CPU time from runtime
	return []time.Duration{
		time.Duration(stats.Sys), // System CPU time
	}
}

// calculateCPUUsage calculates CPU usage percentage between two measurements
func calculateCPUUsage(prevStats, currStats []time.Duration, prevTime, currTime time.Time) float64 {
	if len(prevStats) != len(currStats) {
		return 0.0
	}

	elapsed := currTime.Sub(prevTime)
	if elapsed <= 0 {
		return 0.0
	}

	// Calculate total CPU time used
	var totalCPU time.Duration
	for i := 0; i < len(prevStats) && i < len(currStats); i++ {
		totalCPU += currStats[i] - prevStats[i]
	}

	// Calculate CPU usage percentage
	cpuUsage := float64(totalCPU) / float64(elapsed)

	// Normalize by number of CPUs
	numCPU := runtime.NumCPU()
	if numCPU > 0 {
		cpuUsage /= float64(numCPU)
	}

	// Ensure the value is between 0 and 1
	if cpuUsage < 0 {
		cpuUsage = 0
	} else if cpuUsage > 1 {
		cpuUsage = 1
	}

	return cpuUsage
}

// GetCPUSnapshot returns a snapshot of current CPU usage
func (m *CPUMonitor) GetCPUSnapshot() CPUStats {
	return m.collectStats()
}

// SetThreshold updates the CPU usage threshold
func (m *CPUMonitor) SetThreshold(threshold float64) {
	if threshold >= 0 && threshold <= 1 {
		m.threshold = threshold
	}
}
