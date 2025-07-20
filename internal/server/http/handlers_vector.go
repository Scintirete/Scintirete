// Package http provides vector operation handlers for the HTTP server.
package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	pb "github.com/scintirete/scintirete/gen/go/scintirete/v1"
)

// handleInsertVectors handles vector insertion requests
func (h *Server) handleInsertVectors(c *gin.Context) {
	dbName := c.Param("db_name")
	collName := c.Param("coll_name")

	var req pb.InsertVectorsRequest
	if err := h.bindJSON(c, &req); err != nil {
		h.respondError(c, http.StatusBadRequest, "Invalid JSON", err)
		return
	}

	// Set path parameters
	req.DbName = dbName
	req.CollectionName = collName

	// Validate required fields
	if len(req.Vectors) == 0 {
		h.respondError(c, http.StatusBadRequest, "Vectors are required", nil)
		return
	}

	_, err := h.grpcServer.InsertVectors(c.Request.Context(), &req)
	if err != nil {
		h.handleGRPCError(c, err)
		return
	}

	h.respondSuccess(c, http.StatusCreated, "Vectors inserted successfully")
}

// handleDeleteVectors handles vector deletion requests
func (h *Server) handleDeleteVectors(c *gin.Context) {
	dbName := c.Param("db_name")
	collName := c.Param("coll_name")

	var req pb.DeleteVectorsRequest
	if err := h.bindJSON(c, &req); err != nil {
		h.respondError(c, http.StatusBadRequest, "Invalid JSON", err)
		return
	}

	// Set path parameters
	req.DbName = dbName
	req.CollectionName = collName

	// Validate required fields
	if len(req.Ids) == 0 {
		h.respondError(c, http.StatusBadRequest, "Vector IDs are required", nil)
		return
	}

	resp, err := h.grpcServer.DeleteVectors(c.Request.Context(), &req)
	if err != nil {
		h.handleGRPCError(c, err)
		return
	}

	h.respondJSON(c, http.StatusOK, resp)
}

// handleSearch handles vector search requests
func (h *Server) handleSearch(c *gin.Context) {
	dbName := c.Param("db_name")
	collName := c.Param("coll_name")

	var req pb.SearchRequest
	if err := h.bindJSON(c, &req); err != nil {
		h.respondError(c, http.StatusBadRequest, "Invalid JSON", err)
		return
	}

	// Set path parameters
	req.DbName = dbName
	req.CollectionName = collName

	// Validate required fields
	if len(req.QueryVector) == 0 {
		h.respondError(c, http.StatusBadRequest, "Query vector is required", nil)
		return
	}
	if req.TopK <= 0 {
		h.respondError(c, http.StatusBadRequest, "TopK must be greater than 0", nil)
		return
	}

	resp, err := h.grpcServer.Search(c.Request.Context(), &req)
	if err != nil {
		h.handleGRPCError(c, err)
		return
	}

	h.respondJSON(c, http.StatusOK, resp)
}
