// Package grpc provides the gRPC server implementation for Scintirete.
package grpc

import (
	"context"
	"fmt"
	"sync"
	"time"

	pb "github.com/scintirete/scintirete/gen/go/scintirete/v1"
	"github.com/scintirete/scintirete/internal/core"
	"github.com/scintirete/scintirete/internal/core/database"
	"github.com/scintirete/scintirete/internal/embedding"
	"github.com/scintirete/scintirete/internal/persistence"
	"github.com/scintirete/scintirete/internal/server"
)

// Server implements the ScintireteService gRPC interface
type Server struct {
	pb.UnimplementedScintireteServiceServer

	mu          sync.RWMutex
	engine      *database.Engine
	persistence *persistence.Manager
	embedding   *embedding.Client
	config      server.ServerConfig
	logger      core.Logger
	auth        server.Authenticator

	// Statistics
	startTime    time.Time
	requestCount int64
}

// NewServer creates a new gRPC server instance
func NewServer(config server.ServerConfig) (*Server, error) {
	// Create database engine
	engine := database.NewEngine()

	// Create persistence manager
	persistenceManager, err := persistence.NewManager(config.PersistenceConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create persistence manager: %w", err)
	}

	// Create embedding client
	embeddingClient, err := embedding.NewClient(config.EmbeddingConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create embedding client: %w", err)
	}

	// Create authenticator
	auth := server.NewBasicAuthenticator(config.Passwords)

	// Create logger for server component
	var serverLogger core.Logger
	// For now, create a default logger - this could be passed from config later
	// serverLogger = defaultLogger.WithFields(map[string]interface{}{
	//     "component": "grpc_server",
	// })

	return &Server{
		engine:      engine,
		persistence: persistenceManager,
		embedding:   embeddingClient,
		config:      config,
		logger:      serverLogger,
		auth:        auth,
		startTime:   time.Now(),
	}, nil
}

// Start starts the server and recovery process
func (s *Server) Start(ctx context.Context) error {
	// Start persistence manager background tasks
	if err := s.persistence.StartBackgroundTasks(ctx); err != nil {
		return fmt.Errorf("failed to start persistence background tasks: %w", err)
	}

	// Recover from persistent data
	if err := s.persistence.Recover(ctx); err != nil {
		return fmt.Errorf("failed to recover from persistent data: %w", err)
	}

	return nil
}

// Stop gracefully stops the server
func (s *Server) Stop(ctx context.Context) error {
	// Stop persistence manager
	if err := s.persistence.Stop(ctx); err != nil {
		return fmt.Errorf("failed to stop persistence manager: %w", err)
	}

	// Close database engine
	if err := s.engine.Close(ctx); err != nil {
		return fmt.Errorf("failed to close database engine: %w", err)
	}

	return nil
}

// GetStats returns server statistics
func (s *Server) GetStats() server.Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	uptime := time.Since(s.startTime)

	return server.Stats{
		StartTime:    s.startTime,
		Uptime:       uptime,
		RequestCount: s.requestCount,
	}
}

// updateRequestStats updates internal request statistics
func (s *Server) updateRequestStats() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.requestCount++
}
