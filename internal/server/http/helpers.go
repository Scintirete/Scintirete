// Package http provides helper methods for the HTTP server.
package http

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"google.golang.org/protobuf/proto"
)

// bindJSON binds JSON request body to a proto message using protojson
func (h *Server) bindJSON(c *gin.Context, msg proto.Message) error {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return err
	}

	// Restore the body for potential future reading
	c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

	return h.unmarshaler.Unmarshal(body, msg)
}

// respondJSON responds with a proto message as JSON using protojson
func (h *Server) respondJSON(c *gin.Context, status int, msg proto.Message) {
	data, err := h.marshaler.Marshal(msg)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, "Failed to marshal response", err)
		return
	}

	c.Header("Content-Type", "application/json")
	c.Data(status, "application/json", data)
}

// respondSuccess responds with a success message
func (h *Server) respondSuccess(c *gin.Context, status int, message string) {
	response := map[string]interface{}{
		"success": true,
		"message": message,
	}
	c.JSON(status, response)
}

// respondError responds with an error message
func (h *Server) respondError(c *gin.Context, status int, message string, err error) {
	response := map[string]interface{}{
		"success": false,
		"error":   message,
	}

	if err != nil {
		response["details"] = err.Error()
	}

	c.JSON(status, response)
}

// handleGRPCError converts gRPC errors to HTTP errors
func (h *Server) handleGRPCError(c *gin.Context, err error) {
	errMsg := err.Error()

	// Simple error conversion - could be made more sophisticated
	if strings.Contains(errMsg, "NotFound") {
		h.respondError(c, http.StatusNotFound, errMsg, nil)
	} else if strings.Contains(errMsg, "InvalidArgument") {
		h.respondError(c, http.StatusBadRequest, errMsg, nil)
	} else if strings.Contains(errMsg, "Unauthenticated") || strings.Contains(errMsg, "unauthorized") {
		h.respondError(c, http.StatusUnauthorized, errMsg, nil)
	} else if strings.Contains(errMsg, "AlreadyExists") {
		h.respondError(c, http.StatusConflict, errMsg, nil)
	} else {
		h.respondError(c, http.StatusInternalServerError, errMsg, nil)
	}
}
