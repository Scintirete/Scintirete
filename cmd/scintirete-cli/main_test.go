// Package main provides unit tests for the CLI.
package main

import (
	"context"
	"testing"

	pb "github.com/scintirete/scintirete/gen/go/scintirete/v1"
	"google.golang.org/grpc"
)

// MockScintireteServiceClient implements pb.ScintireteServiceClient for testing
type MockScintireteServiceClient struct {
	pb.ScintireteServiceClient
	saveResponse   *pb.SaveResponse
	saveError      error
	bgsaveResponse *pb.BgSaveResponse
	bgsaveError    error
}

func (m *MockScintireteServiceClient) Save(ctx context.Context, req *pb.SaveRequest, opts ...grpc.CallOption) (*pb.SaveResponse, error) {
	return m.saveResponse, m.saveError
}

func (m *MockScintireteServiceClient) BgSave(ctx context.Context, req *pb.BgSaveRequest, opts ...grpc.CallOption) (*pb.BgSaveResponse, error) {
	return m.bgsaveResponse, m.bgsaveError
}

func TestCLI_SaveCommand(t *testing.T) {
	// Test successful save
	mockClient := &MockScintireteServiceClient{
		saveResponse: &pb.SaveResponse{
			Success:         true,
			Message:         "RDB snapshot saved successfully",
			SnapshotSize:    1024000, // 1MB
			DurationSeconds: 2.5,
		},
		saveError: nil,
	}

	cli := &CLI{
		client:   mockClient,
		password: "test-password",
	}

	// Test with no arguments (correct usage)
	err := cli.saveCommand([]string{})
	if err != nil {
		t.Errorf("Save command should succeed, got error: %v", err)
	}

	// Test with arguments (incorrect usage)
	err = cli.saveCommand([]string{"extra", "arguments"})
	if err == nil {
		t.Error("Save command with arguments should fail")
	}

	// Test with server error
	mockClient.saveResponse = &pb.SaveResponse{
		Success: false,
		Message: "Failed to save snapshot",
	}

	err = cli.saveCommand([]string{})
	if err != nil {
		t.Errorf("Save command should not return error even if server fails, got: %v", err)
	}

	// Test with connection error
	mockClient.saveError = grpc.ErrClientConnClosing
	err = cli.saveCommand([]string{})
	if err == nil {
		t.Error("Save command should fail when client connection is closed")
	}
}

func TestCLI_BgSaveCommand(t *testing.T) {
	// Test successful bgsave
	mockClient := &MockScintireteServiceClient{
		bgsaveResponse: &pb.BgSaveResponse{
			Success: true,
			Message: "Background save started successfully",
			JobId:   "bgsave_123456789",
		},
		bgsaveError: nil,
	}

	cli := &CLI{
		client:   mockClient,
		password: "test-password",
	}

	// Test with no arguments (correct usage)
	err := cli.bgsaveCommand([]string{})
	if err != nil {
		t.Errorf("BgSave command should succeed, got error: %v", err)
	}

	// Test with arguments (incorrect usage)
	err = cli.bgsaveCommand([]string{"extra", "arguments"})
	if err == nil {
		t.Error("BgSave command with arguments should fail")
	}

	// Test with server error
	mockClient.bgsaveResponse = &pb.BgSaveResponse{
		Success: false,
		Message: "Failed to start background save",
	}

	err = cli.bgsaveCommand([]string{})
	if err != nil {
		t.Errorf("BgSave command should not return error even if server fails, got: %v", err)
	}

	// Test with connection error
	mockClient.bgsaveError = grpc.ErrClientConnClosing
	err = cli.bgsaveCommand([]string{})
	if err == nil {
		t.Error("BgSave command should fail when client connection is closed")
	}
}
