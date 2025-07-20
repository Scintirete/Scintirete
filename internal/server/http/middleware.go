// Package http provides middleware for the HTTP server.
package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// setupMiddleware adds middleware to the HTTP server
func (h *Server) setupMiddleware() {
	// Add middleware
	h.engine.Use(gin.Logger())
	h.engine.Use(gin.Recovery())
	h.engine.Use(corsMiddleware())
}

// corsMiddleware adds CORS headers
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusOK)
			return
		}

		c.Next()
	}
}
