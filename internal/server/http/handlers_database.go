// Package http provides database operation handlers for the HTTP server.
package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	pb "github.com/scintirete/scintirete/gen/go/scintirete/v1"
)

// handleCreateDatabase handles database creation requests
func (h *Server) handleCreateDatabase(c *gin.Context) {
	var req pb.CreateDatabaseRequest

	if err := h.bindJSON(c, &req); err != nil {
		h.respondError(c, http.StatusBadRequest, "Invalid JSON", err)
		return
	}

	// Validate required fields
	if req.Name == "" {
		h.respondError(c, http.StatusBadRequest, "Database name is required", nil)
		return
	}

	resp, err := h.grpcServer.CreateDatabase(c.Request.Context(), &req)
	if err != nil {
		h.handleGRPCError(c, err)
		return
	}

	h.respondJSON(c, http.StatusCreated, resp)
}

// handleDropDatabase handles database deletion requests
func (h *Server) handleDropDatabase(c *gin.Context) {
	dbName := c.Param("db_name")
	auth := getAuthFromContext(c)

	req := &pb.DropDatabaseRequest{
		Auth: auth,
		Name: dbName,
	}

	resp, err := h.grpcServer.DropDatabase(c.Request.Context(), req)
	if err != nil {
		h.handleGRPCError(c, err)
		return
	}

	h.respondJSON(c, http.StatusOK, resp)
}

// handleListDatabases handles database list requests
func (h *Server) handleListDatabases(c *gin.Context) {
	auth := getAuthFromContext(c)

	req := &pb.ListDatabasesRequest{
		Auth: auth,
	}

	resp, err := h.grpcServer.ListDatabases(c.Request.Context(), req)
	if err != nil {
		h.handleGRPCError(c, err)
		return
	}

	h.respondJSON(c, http.StatusOK, resp)
}
