// Package grpc provides collection operations for the gRPC server.
package grpc

import (
	"context"

	pb "github.com/scintirete/scintirete/gen/go/scintirete/v1"
	"github.com/scintirete/scintirete/internal/utils"
	"github.com/scintirete/scintirete/pkg/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// CreateCollection creates a new collection in a database
func (s *Server) CreateCollection(ctx context.Context, req *pb.CreateCollectionRequest) (*pb.CreateCollectionResponse, error) {
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

	// Log to audit
	s.logAuditOperation(ctx, "CreateCollection", req.DbName, req.CollectionName, req.Auth, map[string]interface{}{
		"operation_type": "collection_management",
		"metric_type":    config.Metric.String(),
		"hnsw_params":    config.HNSWParams,
	})

	// Get collection info for response
	collection, err := db.GetCollection(ctx, req.CollectionName)
	var collectionInfo *pb.CollectionInfo
	if err == nil && collection != nil {
		info := collection.Info()
		collectionInfo = info.ToProto()
	}

	s.updateRequestStats()
	return &pb.CreateCollectionResponse{
		DbName:         req.DbName,
		CollectionName: req.CollectionName,
		Success:        true,
		Message:        "Collection created successfully",
		Info:           collectionInfo,
	}, nil
}

// DropCollection removes a collection from a database
func (s *Server) DropCollection(ctx context.Context, req *pb.DropCollectionRequest) (*pb.DropCollectionResponse, error) {
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

	// Get collection info before dropping to record vector count
	var droppedVectors int64
	collectionInfo, err := db.GetCollectionInfo(ctx, req.CollectionName)
	if err == nil {
		droppedVectors = collectionInfo.VectorCount
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

	// Log to audit
	s.logAuditOperation(ctx, "DropCollection", req.DbName, req.CollectionName, req.Auth, map[string]interface{}{
		"operation_type": "collection_management",
	})

	s.updateRequestStats()
	return &pb.DropCollectionResponse{
		DbName:         req.DbName,
		CollectionName: req.CollectionName,
		Success:        true,
		Message:        "Collection dropped successfully",
		DroppedVectors: droppedVectors,
	}, nil
}

// GetCollectionInfo returns metadata about a collection
func (s *Server) GetCollectionInfo(ctx context.Context, req *pb.GetCollectionInfoRequest) (*pb.CollectionInfo, error) {
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
func (s *Server) ListCollections(ctx context.Context, req *pb.ListCollectionsRequest) (*pb.ListCollectionsResponse, error) {
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
