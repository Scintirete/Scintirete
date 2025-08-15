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
	"github.com/scintirete/scintirete/internal/monitoring"
	"github.com/scintirete/scintirete/internal/observability/audit"
	"github.com/scintirete/scintirete/internal/observability/logger"
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
	auditLogger *audit.Logger
	auth        server.Authenticator
	cpuMonitor  *monitoring.CPUMonitor

	// Statistics
	startTime    time.Time
	requestCount int64
}

// NewServer creates a new gRPC server instance
func NewServer(config server.ServerConfig) (*Server, error) {
	// Create database engine
	engine := database.NewEngine()

	// Create persistence manager with database engine connection
	persistenceManager, err := persistence.NewManagerWithEngine(config.PersistenceConfig, engine)
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
	// Create a default logger for the server component
	defaultLogger, err := logger.NewFromConfigString("info", "json")
	if err != nil {
		return nil, fmt.Errorf("failed to create default logger: %w", err)
	}
	serverLogger = defaultLogger.WithFields(map[string]interface{}{
		"component": "grpc_server",
	})

	// Create audit logger
	var auditLogger *audit.Logger
	if config.EnableAuditLog {
		auditConfig := audit.Config{
			Enabled:    true,
			OutputPath: config.PersistenceConfig.DataDir + "/audit.log",
			MaxSize:    10 * 1024 * 1024, // 10MB
			MaxFiles:   5,                // Keep 5 rotated files
		}
		auditLogger, err = audit.NewLogger(auditConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create audit logger: %w", err)
		}
	} else {
		// Create disabled audit logger
		auditLogger, _ = audit.NewLogger(audit.Config{Enabled: false})
	}

	// Create CPU monitor for performance monitoring
	cpuMonitor := monitoring.NewCPUMonitor(serverLogger, 10*time.Second, 0.8)

	return &Server{
		engine:      engine,
		persistence: persistenceManager,
		embedding:   embeddingClient,
		config:      config,
		logger:      serverLogger,
		auditLogger: auditLogger,
		auth:        auth,
		cpuMonitor:  cpuMonitor,
		startTime:   time.Now(),
	}, nil
}

// Start starts the server and recovery process
func (s *Server) Start(ctx context.Context) error {
	// Start CPU monitoring
	if err := s.cpuMonitor.Start(ctx); err != nil {
		s.logger.Error(ctx, "Failed to start CPU monitoring", err, nil)
		// Don't fail server startup for CPU monitoring issues
	}

	// Start persistence manager background tasks
	if err := s.persistence.StartBackgroundTasks(ctx); err != nil {
		return fmt.Errorf("failed to start persistence background tasks: %w", err)
	}

	// Recover from persistent data
	if err := s.persistence.Recover(ctx); err != nil {
		return fmt.Errorf("failed to recover from persistent data: %w", err)
	}

	s.logger.Info(ctx, "Server started successfully", map[string]interface{}{
		"cpu_monitoring_enabled": true,
		"monitoring_interval":    "10s",
		"cpu_threshold":          "80%",
	})

	return nil
}

// Stop gracefully stops the server
func (s *Server) Stop(ctx context.Context) error {
	// Stop CPU monitoring
	s.cpuMonitor.Stop()

	// Stop persistence manager
	if err := s.persistence.Stop(ctx); err != nil {
		return fmt.Errorf("failed to stop persistence manager: %w", err)
	}

	// Close database engine
	if err := s.engine.Close(ctx); err != nil {
		return fmt.Errorf("failed to close database engine: %w", err)
	}

	// Close audit logger
	if s.auditLogger != nil {
		if err := s.auditLogger.Close(); err != nil {
			return fmt.Errorf("failed to close audit logger: %w", err)
		}
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

// Save performs a synchronous RDB snapshot save
func (s *Server) Save(ctx context.Context, req *pb.SaveRequest) (*pb.SaveResponse, error) {
	defer s.updateRequestStats()

	// Authenticate the request
	if err := s.authenticate(req.Auth); err != nil {
		return nil, err
	}

	// Log the operation
	s.logAuditOperation(ctx, "SAVE", "", "", req.Auth, map[string]interface{}{
		"operation": "sync_save",
	})

	startTime := time.Now()

	// Get current database state
	databases, err := s.engine.GetDatabaseState(ctx)
	if err != nil {
		s.logger.Error(ctx, "Failed to get database state for RDB save", err, map[string]interface{}{
			"operation": "save_rdb",
		})
		return &pb.SaveResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to get database state: %v", err),
		}, nil
	}

	// Perform synchronous save
	err = s.persistence.SaveSnapshot(ctx, databases)
	if err != nil {
		s.logger.Error(ctx, "Failed to save RDB snapshot", err, map[string]interface{}{
			"operation": "save_rdb",
		})
		return &pb.SaveResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to save RDB snapshot: %v", err),
		}, nil
	}

	duration := time.Since(startTime)

	// Get snapshot file size (if available)
	var snapshotSize int64
	// Note: We would need to expose RDB manager through persistence interface to get file info
	// For now, we'll leave this as 0

	s.logger.Info(ctx, "RDB snapshot saved successfully", map[string]interface{}{
		"operation": "save_rdb",
		"duration":  duration,
		"size":      snapshotSize,
	})

	return &pb.SaveResponse{
		Success:         true,
		Message:         "RDB snapshot saved successfully",
		SnapshotSize:    snapshotSize,
		DurationSeconds: duration.Seconds(),
	}, nil
}

// BgSave performs an asynchronous RDB snapshot save
func (s *Server) BgSave(ctx context.Context, req *pb.BgSaveRequest) (*pb.BgSaveResponse, error) {
	defer s.updateRequestStats()

	// Authenticate the request
	if err := s.authenticate(req.Auth); err != nil {
		return nil, err
	}

	// Generate a job ID for tracking
	jobID := fmt.Sprintf("bgsave_%d", time.Now().UnixNano())

	// Log the operation
	s.logAuditOperation(ctx, "BGSAVE", "", "", req.Auth, map[string]interface{}{
		"operation": "async_save",
		"job_id":    jobID,
	})

	// Start background save operation
	go func() {
		saveCtx := context.Background() // Use background context for long-running operation

		s.logger.Info(saveCtx, "Starting background RDB save", map[string]interface{}{
			"operation": "bgsave",
			"job_id":    jobID,
		})
		startTime := time.Now()

		// Get current database state
		databases, err := s.engine.GetDatabaseState(saveCtx)
		if err != nil {
			s.logger.Error(saveCtx, "Background RDB save failed - could not get database state", err, map[string]interface{}{
				"operation": "bgsave",
				"job_id":    jobID,
				"duration":  time.Since(startTime),
			})
			return
		}

		// Perform save
		err = s.persistence.SaveSnapshot(saveCtx, databases)
		duration := time.Since(startTime)

		if err != nil {
			s.logger.Error(saveCtx, "Background RDB save failed", err, map[string]interface{}{
				"operation": "bgsave",
				"job_id":    jobID,
				"duration":  duration,
			})
		} else {
			s.logger.Info(saveCtx, "Background RDB save completed successfully", map[string]interface{}{
				"operation": "bgsave",
				"job_id":    jobID,
				"duration":  duration,
			})
		}
	}()

	return &pb.BgSaveResponse{
		Success: true,
		Message: "Background save started successfully",
		JobId:   jobID,
	}, nil
}
