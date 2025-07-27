// Package http provides route setup for the HTTP server.
package http

// setupRoutes sets up all HTTP routes
func (h *Server) setupRoutes() {
	api := h.engine.Group("/api/v1")

	// Public routes (no authentication required)
	// Health check - only endpoint that doesn't require auth
	api.GET("/health", h.handleHealth)

	// Protected routes (authentication required)
	// All other endpoints require authentication
	protected := api.Group("")
	protected.Use(authMiddleware())
	{
		// Database operations requiring auth
		protected.POST("/databases", h.handleCreateDatabase)
		protected.DELETE("/databases/:db_name", h.handleDropDatabase)
		protected.GET("/databases", h.handleListDatabases)

		// Collection operations requiring auth
		protected.POST("/databases/:db_name/collections", h.handleCreateCollection)
		protected.DELETE("/databases/:db_name/collections/:coll_name", h.handleDropCollection)
		protected.GET("/databases/:db_name/collections/:coll_name", h.handleGetCollectionInfo)
		protected.GET("/databases/:db_name/collections", h.handleListCollections)

		// Vector operations requiring auth
		protected.POST("/databases/:db_name/collections/:coll_name/vectors", h.handleInsertVectors)
		protected.DELETE("/databases/:db_name/collections/:coll_name/vectors", h.handleDeleteVectors)
		protected.POST("/databases/:db_name/collections/:coll_name/search", h.handleSearch)

		// Text embedding operations requiring auth
		protected.POST("/databases/:db_name/collections/:coll_name/embed", h.handleEmbedAndInsert)
		protected.POST("/databases/:db_name/collections/:coll_name/embed/search", h.handleEmbedAndSearch)
		protected.POST("/embed", h.handleEmbedText)
		protected.GET("/embed/models", h.handleListEmbeddingModels)
	}
}
