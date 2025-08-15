package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/scintirete/scintirete/internal/monitoring"
)

func main() {
	// Parse command line flags
	duration := flag.Duration("duration", 30*time.Second, "Monitoring duration")
	threshold := flag.Float64("threshold", 0.7, "CPU usage threshold (0.0-1.0)")
	interval := flag.Duration("interval", 5*time.Second, "Monitoring interval")

	flag.Parse()

	// Validate threshold
	if *threshold < 0 || *threshold > 1 {
		fmt.Printf("Error: threshold must be between 0.0 and 1.0, got %.2f\n", *threshold)
		os.Exit(1)
	}

	// Create context
	ctx := context.Background()

	fmt.Println("🔍 Scintirete CPU Monitor")
	fmt.Println("========================")
	fmt.Printf("Duration: %v\n", *duration)
	fmt.Printf("Interval: %v\n", *interval)
	fmt.Printf("Threshold: %.1f%%\n", *threshold*100)
	fmt.Println("========================")

	// Run CPU monitoring
	err := monitoring.MonitorServerCPU(ctx, *duration)
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ CPU monitoring completed successfully")
}
