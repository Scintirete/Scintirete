package monitoring

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/scintirete/scintirete/internal/observability/logger"
)

// CPUMonitorCLI provides a command-line interface for CPU monitoring
type CPUMonitorCLI struct {
	monitor *CPUMonitor
}

// NewCPUMonitorCLI creates a new CPU monitor CLI
func NewCPUMonitorCLI() (*CPUMonitorCLI, error) {
	// Create logger
	log, err := logger.NewFromConfigString("info", "text")
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	// Create CPU monitor with 5-second interval and 70% threshold
	monitor := NewCPUMonitor(log, 5*time.Second, 0.7)

	return &CPUMonitorCLI{
		monitor: monitor,
	}, nil
}

// Run starts the CPU monitoring
func (cli *CPUMonitorCLI) Run(ctx context.Context) error {
	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start monitoring
	if err := cli.monitor.Start(ctx); err != nil {
		return fmt.Errorf("failed to start CPU monitor: %w", err)
	}

	fmt.Println("🚀 CPU monitoring started...")
	fmt.Println("Press Ctrl+C to stop")
	fmt.Println("=====================================")

	// Wait for signal or context cancellation
	select {
	case <-sigChan:
		fmt.Println("\n🛑 Received stop signal...")
	case <-ctx.Done():
		fmt.Println("\n🛑 Context cancelled...")
	}

	// Stop monitoring
	cli.monitor.Stop()

	// Print final stats
	finalStats := cli.monitor.GetCPUSnapshot()
	fmt.Println("=====================================")
	fmt.Println("📊 Final CPU Statistics:")
	fmt.Printf("   CPU Usage: %.2f%%\n", finalStats.CPUUsage*100)
	fmt.Printf("   Goroutines: %d\n", finalStats.GoroutineCount)
	fmt.Printf("   Memory Alloc: %.2f MB\n", float64(finalStats.MemoryAlloc)/1024/1024)
	fmt.Printf("   Memory Sys: %.2f MB\n", float64(finalStats.MemorySys)/1024/1024)
	fmt.Printf("   GC Cycles: %d\n", finalStats.GCCycles)
	fmt.Println("=====================================")

	return nil
}

// MonitorServerCPU monitors CPU usage for a running server
func MonitorServerCPU(ctx context.Context, duration time.Duration) error {
	cli, err := NewCPUMonitorCLI()
	if err != nil {
		return err
	}

	fmt.Printf("🔍 Monitoring CPU usage for %v...\n", duration)

	// Create context with timeout
	monitorCtx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	return cli.Run(monitorCtx)
}
