package cli

import (
	"context"
	"fmt"

	pb "github.com/scintirete/scintirete/gen/go/scintirete/v1"
)

// saveCommand handles the save command
func (c *CLI) saveCommand(args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: save")
	}

	// Create the request
	req := &pb.SaveRequest{
		Auth: &pb.AuthInfo{
			Password: c.password,
		},
	}

	// Make the request
	resp, err := c.client.Save(context.Background(), req)
	if err != nil {
		return fmt.Errorf("failed to save RDB snapshot: %v", err)
	}

	if resp.Success {
		fmt.Printf("RDB snapshot saved successfully\n")
		fmt.Printf("Duration: %.3f seconds\n", resp.DurationSeconds)
		if resp.SnapshotSize > 0 {
			fmt.Printf("Snapshot size: %d bytes (%.2f MB)\n", resp.SnapshotSize, float64(resp.SnapshotSize)/(1024*1024))
		}
	} else {
		fmt.Printf("Failed to save RDB snapshot: %s\n", resp.Message)
	}

	return nil
}

// bgsaveCommand handles the bgsave command
func (c *CLI) bgsaveCommand(args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: bgsave")
	}

	// Create the request
	req := &pb.BgSaveRequest{
		Auth: &pb.AuthInfo{
			Password: c.password,
		},
	}

	// Make the request
	resp, err := c.client.BgSave(context.Background(), req)
	if err != nil {
		return fmt.Errorf("failed to start background RDB save: %v", err)
	}

	if resp.Success {
		fmt.Printf("Background RDB save started successfully\n")
		fmt.Printf("Job ID: %s\n", resp.JobId)
		fmt.Printf("Note: Check server logs for completion status\n")
	} else {
		fmt.Printf("Failed to start background RDB save: %s\n", resp.Message)
	}

	return nil
}
