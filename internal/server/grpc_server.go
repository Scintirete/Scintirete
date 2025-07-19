// Package server provides the gRPC server implementation for Scintirete.
package server

import (
	"context"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/scintirete/scintirete/gen/go/scintirete/v1"
	"github.com/scintirete/scintirete/internal/core"
	"github.com/scintirete/scintirete/internal/core/database"
	"github.com/scintirete/scintirete/internal/embedding"
	"github.com/scintirete/scintirete/internal/persistence"
	"github.com/scintirete/scintirete/internal/utils"
	"github.com/scintirete/scintirete/pkg/types"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

// GRPCServer implements the ScintireteService gRPC interface
type GRPCServer struct {
	pb.UnimplementedScintireteServiceServer

	mu          sync.RWMutex
	engine      *database.Engine
	persistence *persistence.Manager
	embedding   *embedding.Client
	config      Config
	logger      core.Logger // Add logger field

	// Authentication
	validPasswords map[string]bool

	// Statistics
	startTime    time.Time
	requestCount int64
}

// Config contains server configuration
type Config struct {
	// Authentication
	Passwords []string `toml:"passwords"`

	// Persistence
	PersistenceConfig persistence.Config `toml:"persistence"`

	// Embedding
	EmbeddingConfig embedding.Config `toml:"embedding"`

	// Features
	EnableMetrics  bool `toml:"enable_metrics"`
	EnableAuditLog bool `toml:"enable_audit_log"`
}

// NewGRPCServer creates a new gRPC server instance
func NewGRPCServer(config Config) (*GRPCServer, error) {
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

	// Create password map for O(1) lookup
	passwords := make(map[string]bool)
	for _, pwd := range config.Passwords {
		passwords[pwd] = true
	}

	// Create logger for server component
	var serverLogger core.Logger
	// For now, create a default logger - this could be passed from config later
	// serverLogger = defaultLogger.WithFields(map[string]interface{}{
	//     "component": "grpc_server",
	// })

	return &GRPCServer{
		engine:         engine,
		persistence:    persistenceManager,
		embedding:      embeddingClient,
		config:         config,
		logger:         serverLogger,
		validPasswords: passwords,
		startTime:      time.Now(),
	}, nil
}

// Start starts the server and recovery process
func (s *GRPCServer) Start(ctx context.Context) error {
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
func (s *GRPCServer) Stop(ctx context.Context) error {
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

// Database management operations

// CreateDatabase creates a new database
func (s *GRPCServer) CreateDatabase(ctx context.Context, req *pb.CreateDatabaseRequest) (*emptypb.Empty, error) {
	// Authenticate
	if err := s.authenticate(req.Auth); err != nil {
		return nil, err
	}

	// Validate input
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "database name cannot be empty")
	}

	// Create database
	if err := s.engine.CreateDatabase(ctx, req.Name); err != nil {
		if utils.IsScintireteError(err) {
			return nil, s.convertError(err)
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Log to persistence
	if err := s.persistence.LogCreateDatabase(ctx, req.Name); err != nil {
		// Database was created but logging failed - this is serious
		return nil, status.Error(codes.Internal, "failed to log create database operation")
	}

	s.updateRequestStats()
	return &emptypb.Empty{}, nil
}

// DropDatabase removes a database and all its collections
func (s *GRPCServer) DropDatabase(ctx context.Context, req *pb.DropDatabaseRequest) (*emptypb.Empty, error) {
	// Authenticate
	if err := s.authenticate(req.Auth); err != nil {
		return nil, err
	}

	// Validate input
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "database name cannot be empty")
	}

	// Drop database
	if err := s.engine.DropDatabase(ctx, req.Name); err != nil {
		if utils.IsScintireteError(err) {
			return nil, s.convertError(err)
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Log to persistence
	if err := s.persistence.LogDropDatabase(ctx, req.Name); err != nil {
		return nil, status.Error(codes.Internal, "failed to log drop database operation")
	}

	s.updateRequestStats()
	return &emptypb.Empty{}, nil
}

// ListDatabases returns a list of all database names
func (s *GRPCServer) ListDatabases(ctx context.Context, req *pb.ListDatabasesRequest) (*pb.ListDatabasesResponse, error) {
	// Authenticate
	if err := s.authenticate(req.Auth); err != nil {
		return nil, err
	}

	// Get database list
	names, err := s.engine.ListDatabases(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	s.updateRequestStats()
	return &pb.ListDatabasesResponse{Names: names}, nil
}

// Collection management operations

// CreateCollection creates a new collection in a database
func (s *GRPCServer) CreateCollection(ctx context.Context, req *pb.CreateCollectionRequest) (*emptypb.Empty, error) {
	// Authenticate
	if err := s.authenticate(req.Auth); err != nil {
		return nil, err
	}

	// Validate input
	if req.DbName == "" {
		return nil, status.Error(codes.InvalidArgument, "database name cannot be empty")
	}
	if req.CollectionName == "" {
		return nil, status.Error(codes.InvalidArgument, "collection name cannot be empty")
	}
	if req.MetricType == pb.DistanceMetric_DISTANCE_METRIC_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "distance metric must be specified")
	}

	// Convert protobuf config to internal config
	config := types.CollectionConfig{
		Name:   req.CollectionName,
		Metric: types.DistanceMetricFromProto(req.MetricType),
	}

	// Set HNSW parameters
	if req.HnswConfig != nil {
		config.HNSWParams = types.HNSWParamsFromProto(req.HnswConfig)
	} else {
		config.HNSWParams = types.DefaultHNSWParams()
	}

	// Get database
	db, err := s.engine.GetDatabase(ctx, req.DbName)
	if err != nil {
		if utils.IsScintireteError(err) {
			return nil, s.convertError(err)
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Create collection
	if err := db.CreateCollection(ctx, config); err != nil {
		if utils.IsScintireteError(err) {
			return nil, s.convertError(err)
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Log to persistence
	if err := s.persistence.LogCreateCollection(ctx, req.DbName, req.CollectionName, config); err != nil {
		return nil, status.Error(codes.Internal, "failed to log create collection operation")
	}

	s.updateRequestStats()
	return &emptypb.Empty{}, nil
}

// DropCollection removes a collection from a database
func (s *GRPCServer) DropCollection(ctx context.Context, req *pb.DropCollectionRequest) (*emptypb.Empty, error) {
	// Authenticate
	if err := s.authenticate(req.Auth); err != nil {
		return nil, err
	}

	// Validate input
	if req.DbName == "" {
		return nil, status.Error(codes.InvalidArgument, "database name cannot be empty")
	}
	if req.CollectionName == "" {
		return nil, status.Error(codes.InvalidArgument, "collection name cannot be empty")
	}

	// Get database
	db, err := s.engine.GetDatabase(ctx, req.DbName)
	if err != nil {
		if utils.IsScintireteError(err) {
			return nil, s.convertError(err)
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Drop collection
	if err := db.DropCollection(ctx, req.CollectionName); err != nil {
		if utils.IsScintireteError(err) {
			return nil, s.convertError(err)
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Log to persistence
	if err := s.persistence.LogDropCollection(ctx, req.DbName, req.CollectionName); err != nil {
		return nil, status.Error(codes.Internal, "failed to log drop collection operation")
	}

	s.updateRequestStats()
	return &emptypb.Empty{}, nil
}

// GetCollectionInfo returns metadata about a collection
func (s *GRPCServer) GetCollectionInfo(ctx context.Context, req *pb.GetCollectionInfoRequest) (*pb.CollectionInfo, error) {
	// Authenticate
	if err := s.authenticate(req.Auth); err != nil {
		return nil, err
	}

	// Validate input
	if req.DbName == "" {
		return nil, status.Error(codes.InvalidArgument, "database name cannot be empty")
	}
	if req.CollectionName == "" {
		return nil, status.Error(codes.InvalidArgument, "collection name cannot be empty")
	}

	// Get database
	db, err := s.engine.GetDatabase(ctx, req.DbName)
	if err != nil {
		if utils.IsScintireteError(err) {
			return nil, s.convertError(err)
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Get collection info
	info, err := db.GetCollectionInfo(ctx, req.CollectionName)
	if err != nil {
		if utils.IsScintireteError(err) {
			return nil, s.convertError(err)
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	s.updateRequestStats()
	return info.ToProto(), nil
}

// ListCollections returns all collections in a database
func (s *GRPCServer) ListCollections(ctx context.Context, req *pb.ListCollectionsRequest) (*pb.ListCollectionsResponse, error) {
	// Authenticate
	if err := s.authenticate(req.Auth); err != nil {
		return nil, err
	}

	// Validate input
	if req.DbName == "" {
		return nil, status.Error(codes.InvalidArgument, "database name cannot be empty")
	}

	// Get database
	db, err := s.engine.GetDatabase(ctx, req.DbName)
	if err != nil {
		if utils.IsScintireteError(err) {
			return nil, s.convertError(err)
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	// List collections
	collections, err := db.ListCollections(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Convert to protobuf
	pbCollections := make([]*pb.CollectionInfo, len(collections))
	for i, collection := range collections {
		pbCollections[i] = collection.ToProto()
	}

	s.updateRequestStats()
	return &pb.ListCollectionsResponse{Collections: pbCollections}, nil
}

// Helper methods

// authenticate checks if the provided authentication is valid
func (s *GRPCServer) authenticate(auth *pb.AuthInfo) error {
	if auth == nil || auth.Password == "" {
		return status.Error(codes.Unauthenticated, "authentication required")
	}

	if !s.validPasswords[auth.Password] {
		return status.Error(codes.Unauthenticated, "invalid credentials")
	}

	return nil
}

// convertError converts a ScintireteError to a gRPC status error
func (s *GRPCServer) convertError(err error) error {
	if scintErr, ok := err.(*utils.ScintireteError); ok {
		switch scintErr.Code {
		case utils.ErrorCodeDatabaseNotFound, utils.ErrorCodeCollectionNotFound, utils.ErrorCodeVectorNotFound:
			return status.Error(codes.NotFound, scintErr.Message)
		case utils.ErrorCodeDatabaseAlreadyExists, utils.ErrorCodeCollectionAlreadyExists:
			return status.Error(codes.AlreadyExists, scintErr.Message)
		case utils.ErrorCodeInvalidParameters, utils.ErrorCodeDimensionMismatch:
			return status.Error(codes.InvalidArgument, scintErr.Message)
		case utils.ErrorCodeUnauthorized:
			return status.Error(codes.Unauthenticated, scintErr.Message)
		case utils.ErrorCodeForbidden:
			return status.Error(codes.PermissionDenied, scintErr.Message)
		case utils.ErrorCodeRateLimited:
			return status.Error(codes.ResourceExhausted, scintErr.Message)
		default:
			return status.Error(codes.Internal, scintErr.Message)
		}
	}

	return status.Error(codes.Internal, err.Error())
}

// updateRequestStats updates internal request statistics
func (s *GRPCServer) updateRequestStats() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.requestCount++
}

// GetStats returns server statistics
func (s *GRPCServer) GetStats() ServerStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	uptime := time.Since(s.startTime)

	return ServerStats{
		StartTime:    s.startTime,
		Uptime:       uptime,
		RequestCount: s.requestCount,
	}
}

// ServerStats contains server statistics
type ServerStats struct {
	StartTime    time.Time     `json:"start_time"`
	Uptime       time.Duration `json:"uptime"`
	RequestCount int64         `json:"request_count"`
}
