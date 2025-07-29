// Package http provides middleware for the HTTP server.
package http

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	pb "github.com/scintirete/scintirete/gen/go/scintirete/v1"
)

// setupMiddleware adds middleware to the HTTP server
func (h *Server) setupMiddleware() {
	// Add middleware
	h.engine.Use(gin.Logger())
	h.engine.Use(gin.Recovery())
	h.engine.Use(corsMiddleware())
}

// authMiddleware extracts and validates Bearer token from Authorization header
func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")

		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "Authorization header required",
			})
			c.Abort()
			return
		}

		// Parse Bearer token format: "Bearer {token}"
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "Invalid authorization format. Expected: Bearer {token}",
			})
			c.Abort()
			return
		}

		token := parts[1]
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "Token cannot be empty",
			})
			c.Abort()
			return
		}

		// Store auth info in context for handlers to use
		authInfo := &pb.AuthInfo{Password: token}
		c.Set("auth", authInfo)

		c.Next()
	}
}

// getAuthFromContext retrieves auth info from Gin context
func getAuthFromContext(c *gin.Context) *pb.AuthInfo {
	if auth, exists := c.Get("auth"); exists {
		return auth.(*pb.AuthInfo)
	}
	return nil
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
