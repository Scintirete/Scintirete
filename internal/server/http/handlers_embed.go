// Package http provides embedding operation handlers for the HTTP server.
package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	pb "github.com/scintirete/scintirete/gen/go/scintirete/v1"
)

// handleEmbedAndInsert handles text embedding and insertion requests
func (h *Server) handleEmbedAndInsert(c *gin.Context) {
	dbName := c.Param("db_name")
	collName := c.Param("coll_name")

	var req pb.EmbedAndInsertRequest
	if err := h.bindJSON(c, &req); err != nil {
		h.respondError(c, http.StatusBadRequest, "Invalid JSON", err)
		return
	}

	// Set path parameters
	req.DbName = dbName
	req.CollectionName = collName

	// Validate required fields
	if len(req.Texts) == 0 {
		h.respondError(c, http.StatusBadRequest, "Texts are required", nil)
		return
	}

	_, err := h.grpcServer.EmbedAndInsert(c.Request.Context(), &req)
	if err != nil {
		h.handleGRPCError(c, err)
		return
	}

	h.respondSuccess(c, http.StatusCreated, "Texts embedded and inserted successfully")
}

// handleEmbedAndSearch handles text embedding and search requests
func (h *Server) handleEmbedAndSearch(c *gin.Context) {
	dbName := c.Param("db_name")
	collName := c.Param("coll_name")

	var req pb.EmbedAndSearchRequest
	if err := h.bindJSON(c, &req); err != nil {
		h.respondError(c, http.StatusBadRequest, "Invalid JSON", err)
		return
	}

	// Set path parameters
	req.DbName = dbName
	req.CollectionName = collName

	// Validate required fields
	if req.QueryText == "" {
		h.respondError(c, http.StatusBadRequest, "Query text is required", nil)
		return
	}
	if req.TopK <= 0 {
		h.respondError(c, http.StatusBadRequest, "TopK must be greater than 0", nil)
		return
	}

	resp, err := h.grpcServer.EmbedAndSearch(c.Request.Context(), &req)
	if err != nil {
		h.handleGRPCError(c, err)
		return
	}

	h.respondJSON(c, http.StatusOK, resp)
}

// handleHealth handles health check requests
func (h *Server) handleHealth(c *gin.Context) {
	healthResp := map[string]interface{}{
		"status":  "healthy",
		"service": "scintirete",
		"version": "1.0.0",
	}

	c.Header("Content-Type", "application/json")
	c.JSON(http.StatusOK, healthResp)
}
