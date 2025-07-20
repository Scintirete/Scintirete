// Package grpc provides unit tests for the gRPC server.
package grpc

import (
	"testing"
	"time"

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
