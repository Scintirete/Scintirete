// Package grpc provides unit tests for the gRPC server.
package grpc

import (
	"context"
	"strings"
	"testing"
	"time"

	pb "github.com/scintirete/scintirete/gen/go/scintirete/v1"
	"github.com/scintirete/scintirete/internal/embedding"
	"github.com/scintirete/scintirete/internal/persistence"
	"github.com/scintirete/scintirete/internal/server"
)

func TestNewServer(t *testing.T) {
	// Create test config
	config := server.ServerConfig{
		Passwords: []string{"test-password"},
		PersistenceConfig: persistence.Config{
			DataDir:     "/tmp/test",
			RDBFilename: "test.rdb",
			AOFFilename: "test.aof",
		},
		EmbeddingConfig: embedding.Config{
			BaseURL: "http://localhost:8080",
			APIKey:  "test-key",
		},
		EnableMetrics:  false,
		EnableAuditLog: false,
	}

	// Create server
	srv, err := NewServer(config)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	if srv == nil {
		t.Fatal("Server is nil")
	}

	// Check that server has required components
	if srv.engine == nil {
		t.Error("Engine is nil")
	}
	if srv.persistence == nil {
		t.Error("Persistence is nil")
	}
	if srv.embedding == nil {
		t.Error("Embedding is nil")
	}
	if srv.auth == nil {
		t.Error("Auth is nil")
	}
}

func TestServerStats(t *testing.T) {
	// Create test config
	config := server.ServerConfig{
		Passwords: []string{"test-password"},
		PersistenceConfig: persistence.Config{
			DataDir:     "/tmp/test",
			RDBFilename: "test.rdb",
			AOFFilename: "test.aof",
		},
		EmbeddingConfig: embedding.Config{
			BaseURL: "http://localhost:8080",
			APIKey:  "test-key",
		},
	}

	// Create server
	srv, err := NewServer(config)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Get initial stats
	stats := srv.GetStats()

	// Check initial values
	if stats.RequestCount != 0 {
		t.Errorf("Expected initial request count to be 0, got %d", stats.RequestCount)
	}

	if time.Since(stats.StartTime) > time.Second {
		t.Error("Start time should be recent")
	}

	// Update stats
	srv.updateRequestStats()
	srv.updateRequestStats()

	// Get updated stats
	stats = srv.GetStats()
	if stats.RequestCount != 2 {
		t.Errorf("Expected request count to be 2, got %d", stats.RequestCount)
	}
}

func TestServerAuthentication(t *testing.T) {
	// Create test config
	config := server.ServerConfig{
		Passwords: []string{"valid-password", "another-valid-password"},
		PersistenceConfig: persistence.Config{
			DataDir:     "/tmp/test",
			RDBFilename: "test.rdb",
			AOFFilename: "test.aof",
		},
		EmbeddingConfig: embedding.Config{
			BaseURL: "http://localhost:8080",
			APIKey:  "test-key",
		},
	}

	// Create server
	srv, err := NewServer(config)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Test valid authentication
	err = srv.auth.Authenticate("valid-password")
	if err != nil {
		t.Errorf("Valid password should authenticate successfully: %v", err)
	}

	err = srv.auth.Authenticate("another-valid-password")
	if err != nil {
		t.Errorf("Another valid password should authenticate successfully: %v", err)
	}

	// Test invalid authentication
	err = srv.auth.Authenticate("invalid-password")
	if err == nil {
		t.Error("Invalid password should fail authentication")
	}

	err = srv.auth.Authenticate("")
	if err == nil {
		t.Error("Empty password should fail authentication")
	}
}

func TestServer_Save(t *testing.T) {
	// Create test config
	config := server.ServerConfig{
		Passwords: []string{"test-password"},
		PersistenceConfig: persistence.Config{
			DataDir:     t.TempDir(),
			RDBFilename: "test.rdb",
			AOFFilename: "test.aof",
		},
		EmbeddingConfig: embedding.Config{
			BaseURL: "http://localhost:8080",
			APIKey:  "test-key",
		},
		EnableMetrics:  false,
		EnableAuditLog: false,
	}

	// Create server
	srv, err := NewServer(config)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Test valid save request
	req := &pb.SaveRequest{
		Auth: &pb.AuthInfo{
			Password: "test-password",
		},
	}

	resp, err := srv.Save(context.Background(), req)
	if err != nil {
		t.Errorf("Save failed: %v", err)
	}

	if resp == nil {
		t.Fatal("Response is nil")
	}

	if !resp.Success {
		t.Errorf("Save should succeed, got: %s", resp.Message)
	}

	if resp.DurationSeconds < 0 {
		t.Error("Duration should be non-negative")
	}

	// Test invalid authentication
	invalidReq := &pb.SaveRequest{
		Auth: &pb.AuthInfo{
			Password: "invalid-password",
		},
	}

	_, err = srv.Save(context.Background(), invalidReq)
	if err == nil {
		t.Error("Save with invalid password should fail")
	}

	// Test missing authentication
	nilAuthReq := &pb.SaveRequest{
		Auth: nil,
	}

	_, err = srv.Save(context.Background(), nilAuthReq)
	if err == nil {
		t.Error("Save with nil auth should fail")
	}
}

func TestServer_BgSave(t *testing.T) {
	// Create test config
	config := server.ServerConfig{
		Passwords: []string{"test-password"},
		PersistenceConfig: persistence.Config{
			DataDir:     t.TempDir(),
			RDBFilename: "test.rdb",
			AOFFilename: "test.aof",
		},
		EmbeddingConfig: embedding.Config{
			BaseURL: "http://localhost:8080",
			APIKey:  "test-key",
		},
		EnableMetrics:  false,
		EnableAuditLog: false,
	}

	// Create server
	srv, err := NewServer(config)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Test valid bgsave request
	req := &pb.BgSaveRequest{
		Auth: &pb.AuthInfo{
			Password: "test-password",
		},
	}

	resp, err := srv.BgSave(context.Background(), req)
	if err != nil {
		t.Errorf("BgSave failed: %v", err)
	}

	if resp == nil {
		t.Fatal("Response is nil")
	}

	if !resp.Success {
		t.Errorf("BgSave should succeed, got: %s", resp.Message)
	}

	if resp.JobId == "" {
		t.Error("Job ID should not be empty")
	}

	// Verify job ID format
	if !strings.HasPrefix(resp.JobId, "bgsave_") {
		t.Errorf("Job ID should start with 'bgsave_', got: %s", resp.JobId)
	}

	// Test invalid authentication
	invalidReq := &pb.BgSaveRequest{
		Auth: &pb.AuthInfo{
			Password: "invalid-password",
		},
	}

	_, err = srv.BgSave(context.Background(), invalidReq)
	if err == nil {
		t.Error("BgSave with invalid password should fail")
	}

	// Test missing authentication
	nilAuthReq := &pb.BgSaveRequest{
		Auth: nil,
	}

	_, err = srv.BgSave(context.Background(), nilAuthReq)
	if err == nil {
		t.Error("BgSave with nil auth should fail")
	}

	// Give background save some time to complete
	time.Sleep(100 * time.Millisecond)
}
