// Package http provides route setup for the HTTP server.
package http

// setupRoutes sets up all HTTP routes
func (h *Server) setupRoutes() {
	api := h.engine.Group("/api/v1")

	// Database operations
	api.POST("/databases", h.handleCreateDatabase)
	api.DELETE("/databases/:db_name", h.handleDropDatabase)
	api.GET("/databases", h.handleListDatabases)

	// Collection operations
	api.POST("/databases/:db_name/collections", h.handleCreateCollection)
	api.DELETE("/databases/:db_name/collections/:coll_name", h.handleDropCollection)
	api.GET("/databases/:db_name/collections/:coll_name", h.handleGetCollectionInfo)
	api.GET("/databases/:db_name/collections", h.handleListCollections)

	// Vector operations
	api.POST("/databases/:db_name/collections/:coll_name/vectors", h.handleInsertVectors)
	api.DELETE("/databases/:db_name/collections/:coll_name/vectors", h.handleDeleteVectors)
	api.POST("/databases/:db_name/collections/:coll_name/search", h.handleSearch)

	// Text embedding operations
	api.POST("/databases/:db_name/collections/:coll_name/embed", h.handleEmbedAndInsert)
	api.POST("/databases/:db_name/collections/:coll_name/embed/search", h.handleEmbedAndSearch)
	api.POST("/embed", h.handleEmbedText)
	api.GET("/embed/models", h.handleListEmbeddingModels)

	// Health check
	api.GET("/health", h.handleHealth)
}
