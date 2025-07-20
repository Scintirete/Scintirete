// Package grpc provides authentication methods for the gRPC server.
package grpc

import (
	pb "github.com/scintirete/scintirete/gen/go/scintirete/v1"
	"github.com/scintirete/scintirete/internal/server"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// authenticate checks if the provided authentication is valid
func (s *Server) authenticate(auth *pb.AuthInfo) error {
	if auth == nil || auth.Password == "" {
		return status.Error(codes.Unauthenticated, "authentication required")
	}

	if err := s.auth.Authenticate(auth.Password); err != nil {
		if err == server.ErrInvalidCredentials {
			return status.Error(codes.Unauthenticated, "invalid credentials")
		}
		return status.Error(codes.Internal, "authentication failed")
	}

	return nil
}
