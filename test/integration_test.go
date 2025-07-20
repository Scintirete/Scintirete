// Package test provides integration tests for Scintirete.
package test

import (
	"context"
	"testing"
	"time"

	"github.com/scintirete/scintirete/internal/embedding"
	"github.com/scintirete/scintirete/internal/persistence"
	"github.com/scintirete/scintirete/internal/server"
	grpcserver "github.com/scintirete/scintirete/internal/server/grpc"
	httpserver "github.com/scintirete/scintirete/internal/server/http"
)

// TestServerIntegration tests that we can create and start both gRPC and HTTP servers
func TestServerIntegration(t *testing.T) {
	// Create temporary directory for test data
	tempDir := t.TempDir()

	// Create server configuration
	serverConfig := server.ServerConfig{
		Passwords: []string{"test-password"},
		PersistenceConfig: persistence.Config{
			DataDir:         tempDir,
			RDBFilename:     "test.rdb",
			AOFFilename:     "test.aof",
			AOFSyncStrategy: "always",
			RDBInterval:     time.Minute,
			AOFRewriteSize:  1024 * 1024,
			BackupRetention: 3,
		},
		EmbeddingConfig: embedding.Config{
			BaseURL: "https://api.test.com/v1/embeddings", // Use fake URL for testing
			APIKey:  "",
			Timeout: 30 * time.Second,
		},
		EnableMetrics:  false,
		EnableAuditLog: false,
	}

	// Test gRPC server creation and lifecycle
	t.Run("grpc_server_lifecycle", func(t *testing.T) {
		// Create gRPC server
		grpcServer, err := grpcserver.NewServer(serverConfig)
		if err != nil {
			t.Fatalf("Failed to create gRPC server: %v", err)
		}

		// Start server
		ctx := context.Background()
		if err := grpcServer.Start(ctx); err != nil {
			t.Fatalf("Failed to start gRPC server: %v", err)
		}

		// Check server stats
		stats := grpcServer.GetStats()
		if stats.RequestCount < 0 {
			t.Errorf("Invalid request count: %d", stats.RequestCount)
		}

		if time.Since(stats.StartTime) > time.Second {
			t.Error("Start time should be recent")
		}

		// Stop server
		if err := grpcServer.Stop(ctx); err != nil {
			t.Fatalf("Failed to stop gRPC server: %v", err)
		}
	})

	// Test HTTP server creation
	t.Run("http_server_creation", func(t *testing.T) {
		// Create gRPC server first
		grpcServer, err := grpcserver.NewServer(serverConfig)
		if err != nil {
			t.Fatalf("Failed to create gRPC server: %v", err)
		}

		// Create HTTP server
		httpServer := httpserver.NewServer(grpcServer)
		if httpServer == nil {
			t.Fatal("HTTP server is nil")
		}
	})

	// Test server components
	t.Run("server_components", func(t *testing.T) {
		// Create gRPC server
		grpcServer, err := grpcserver.NewServer(serverConfig)
		if err != nil {
			t.Fatalf("Failed to create gRPC server: %v", err)
		}

		// Verify server has all required components by testing that it can start/stop
		ctx := context.Background()

		// Start should succeed
		if err := grpcServer.Start(ctx); err != nil {
			t.Fatalf("Server should start successfully: %v", err)
		}

		// Stop should succeed
		if err := grpcServer.Stop(ctx); err != nil {
			t.Fatalf("Server should stop successfully: %v", err)
		}
	})
}
