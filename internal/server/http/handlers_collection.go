// Package http provides collection operation handlers for the HTTP server.
package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	pb "github.com/scintirete/scintirete/gen/go/scintirete/v1"
)

// handleCreateCollection handles collection creation requests
func (h *Server) handleCreateCollection(c *gin.Context) {
	dbName := c.Param("db_name")

	var req pb.CreateCollectionRequest
	if err := h.bindJSON(c, &req); err != nil {
		h.respondError(c, http.StatusBadRequest, "Invalid JSON", err)
		return
	}

	// Set database name from URL path
	req.DbName = dbName

	// Validate required fields
	if req.CollectionName == "" {
		h.respondError(c, http.StatusBadRequest, "Collection name is required", nil)
		return
	}
	if req.MetricType == pb.DistanceMetric_DISTANCE_METRIC_UNSPECIFIED {
		h.respondError(c, http.StatusBadRequest, "Metric type is required", nil)
		return
	}

	_, err := h.grpcServer.CreateCollection(c.Request.Context(), &req)
	if err != nil {
		h.handleGRPCError(c, err)
		return
	}

	h.respondSuccess(c, http.StatusCreated, "Collection created successfully")
}

// handleDropCollection handles collection deletion requests
func (h *Server) handleDropCollection(c *gin.Context) {
	dbName := c.Param("db_name")
	collName := c.Param("coll_name")
	password := c.GetHeader("Authorization")

	req := &pb.DropCollectionRequest{
		Auth:           &pb.AuthInfo{Password: password},
		DbName:         dbName,
		CollectionName: collName,
	}

	_, err := h.grpcServer.DropCollection(c.Request.Context(), req)
	if err != nil {
		h.handleGRPCError(c, err)
		return
	}

	h.respondSuccess(c, http.StatusOK, "Collection dropped successfully")
}

// handleGetCollectionInfo handles collection info requests
func (h *Server) handleGetCollectionInfo(c *gin.Context) {
	dbName := c.Param("db_name")
	collName := c.Param("coll_name")
	password := c.GetHeader("Authorization")

	req := &pb.GetCollectionInfoRequest{
		Auth:           &pb.AuthInfo{Password: password},
		DbName:         dbName,
		CollectionName: collName,
	}

	resp, err := h.grpcServer.GetCollectionInfo(c.Request.Context(), req)
	if err != nil {
		h.handleGRPCError(c, err)
		return
	}

	h.respondJSON(c, http.StatusOK, resp)
}

// handleListCollections handles collection list requests
func (h *Server) handleListCollections(c *gin.Context) {
	dbName := c.Param("db_name")
	password := c.GetHeader("Authorization")

	req := &pb.ListCollectionsRequest{
		Auth:   &pb.AuthInfo{Password: password},
		DbName: dbName,
	}

	resp, err := h.grpcServer.ListCollections(c.Request.Context(), req)
	if err != nil {
		h.handleGRPCError(c, err)
		return
	}

	h.respondJSON(c, http.StatusOK, resp)
}
