// Package grpc provides helper methods for the gRPC server.
package grpc

import (
	"github.com/scintirete/scintirete/internal/utils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

// convertError converts a ScintireteError to a gRPC status error
func (s *Server) convertError(err error) error {
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

// isNotFoundError checks if an error is a "not found" type error
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	// Check if it's a ScintireteError with appropriate code
	if scintireteErr, ok := err.(*utils.ScintireteError); ok {
		switch scintireteErr.Code {
		case utils.ErrorCodeDatabaseNotFound,
			utils.ErrorCodeCollectionNotFound,
			utils.ErrorCodeVectorNotFound:
			return true
		}
	}

	return false
}

// mapToStruct converts a map[string]interface{} to *structpb.Struct
func mapToStruct(m map[string]interface{}) *structpb.Struct {
	if m == nil {
		return nil
	}

	s, err := structpb.NewStruct(m)
	if err != nil {
		// Return empty struct if conversion fails
		return &structpb.Struct{}
	}

	return s
}
