// Package http provides HTTP REST API gateway for Scintirete gRPC service.
package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	grpcserver "github.com/scintirete/scintirete/internal/server/grpc"
	"google.golang.org/protobuf/encoding/protojson"
)

// Server provides HTTP REST API gateway for gRPC service
type Server struct {
	grpcServer  *grpcserver.Server
	engine      *gin.Engine
	marshaler   protojson.MarshalOptions
	unmarshaler protojson.UnmarshalOptions
}

// NewServer creates a new HTTP server
func NewServer(grpcServer *grpcserver.Server) *Server {
	gin.SetMode(gin.ReleaseMode) // Set to release mode for production
	engine := gin.New()

	server := &Server{
		grpcServer: grpcServer,
		engine:     engine,
		marshaler: protojson.MarshalOptions{
			UseProtoNames:     true,
			EmitUnpopulated:   false,
			EmitDefaultValues: true,
			Indent:            "  ",
			UseEnumNumbers:    true,
		},
		unmarshaler: protojson.UnmarshalOptions{
			DiscardUnknown: true,
		},
	}

	// Setup middleware
	server.setupMiddleware()

	// Setup routes
	server.setupRoutes()

	return server
}

// ServeHTTP implements http.Handler interface
func (h *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.engine.ServeHTTP(w, r)
}
