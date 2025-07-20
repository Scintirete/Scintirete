// Package grpc provides database operations for the gRPC server.
package grpc

import (
	"context"

	pb "github.com/scintirete/scintirete/gen/go/scintirete/v1"
	"github.com/scintirete/scintirete/internal/utils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// CreateDatabase creates a new database
func (s *Server) CreateDatabase(ctx context.Context, req *pb.CreateDatabaseRequest) (*emptypb.Empty, error) {
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
func (s *Server) DropDatabase(ctx context.Context, req *pb.DropDatabaseRequest) (*emptypb.Empty, error) {
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
func (s *Server) ListDatabases(ctx context.Context, req *pb.ListDatabasesRequest) (*pb.ListDatabasesResponse, error) {
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
