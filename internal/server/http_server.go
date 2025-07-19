// Package server provides HTTP gateway for Scintirete gRPC service.
package server

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	pb "github.com/scintirete/scintirete/gen/go/scintirete/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// HTTPServer provides HTTP REST API gateway for gRPC service
type HTTPServer struct {
	grpcServer  *GRPCServer
	engine      *gin.Engine
	marshaler   protojson.MarshalOptions
	unmarshaler protojson.UnmarshalOptions
}

// NewHTTPServer creates a new HTTP server
func NewHTTPServer(grpcServer *GRPCServer) *HTTPServer {
	gin.SetMode(gin.ReleaseMode) // Set to release mode for production
	engine := gin.New()

	// Add middleware
	engine.Use(gin.Logger())
	engine.Use(gin.Recovery())
	engine.Use(corsMiddleware())

	server := &HTTPServer{
		grpcServer: grpcServer,
		engine:     engine,
		marshaler: protojson.MarshalOptions{
			UseProtoNames:   true,
			EmitUnpopulated: false,
			Indent:          "  ",
		},
		unmarshaler: protojson.UnmarshalOptions{
			DiscardUnknown: true,
		},
	}

	server.setupRoutes()
	return server
}

// ServeHTTP implements http.Handler interface
func (h *HTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.engine.ServeHTTP(w, r)
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

// setupRoutes sets up all HTTP routes
func (h *HTTPServer) setupRoutes() {
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

	// Health check
	api.GET("/health", h.handleHealth)
}

// Database handlers

func (h *HTTPServer) handleCreateDatabase(c *gin.Context) {
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

	_, err := h.grpcServer.CreateDatabase(c.Request.Context(), &req)
	if err != nil {
		h.handleGRPCError(c, err)
		return
	}

	h.respondSuccess(c, http.StatusCreated, "Database created successfully")
}

func (h *HTTPServer) handleDropDatabase(c *gin.Context) {
	dbName := c.Param("db_name")
	password := c.GetHeader("Authorization")

	req := &pb.DropDatabaseRequest{
		Auth: &pb.AuthInfo{Password: password},
		Name: dbName,
	}

	_, err := h.grpcServer.DropDatabase(c.Request.Context(), req)
	if err != nil {
		h.handleGRPCError(c, err)
		return
	}

	h.respondSuccess(c, http.StatusOK, "Database dropped successfully")
}

func (h *HTTPServer) handleListDatabases(c *gin.Context) {
	password := c.GetHeader("Authorization")

	req := &pb.ListDatabasesRequest{
		Auth: &pb.AuthInfo{Password: password},
	}

	resp, err := h.grpcServer.ListDatabases(c.Request.Context(), req)
	if err != nil {
		h.handleGRPCError(c, err)
		return
	}

	h.respondJSON(c, http.StatusOK, resp)
}

// Collection handlers

func (h *HTTPServer) handleCreateCollection(c *gin.Context) {
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

func (h *HTTPServer) handleDropCollection(c *gin.Context) {
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

func (h *HTTPServer) handleGetCollectionInfo(c *gin.Context) {
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

func (h *HTTPServer) handleListCollections(c *gin.Context) {
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

// Vector operation handlers

func (h *HTTPServer) handleInsertVectors(c *gin.Context) {
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

func (h *HTTPServer) handleDeleteVectors(c *gin.Context) {
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

func (h *HTTPServer) handleSearch(c *gin.Context) {
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

// Text embedding handlers

func (h *HTTPServer) handleEmbedAndInsert(c *gin.Context) {
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

func (h *HTTPServer) handleEmbedAndSearch(c *gin.Context) {
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

// Health check handler
func (h *HTTPServer) handleHealth(c *gin.Context) {
	healthResp := map[string]interface{}{
		"status":  "healthy",
		"service": "scintirete",
		"version": "1.0.0",
	}

	c.Header("Content-Type", "application/json")
	c.JSON(http.StatusOK, healthResp)
}

// Helper functions

// bindJSON binds JSON request body to a proto message using protojson
func (h *HTTPServer) bindJSON(c *gin.Context, msg proto.Message) error {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return err
	}

	// Restore the body for potential future reading
	c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

	return h.unmarshaler.Unmarshal(body, msg)
}

// respondJSON responds with a proto message as JSON using protojson
func (h *HTTPServer) respondJSON(c *gin.Context, status int, msg proto.Message) {
	data, err := h.marshaler.Marshal(msg)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, "Failed to marshal response", err)
		return
	}

	c.Header("Content-Type", "application/json")
	c.Data(status, "application/json", data)
}

// respondSuccess responds with a success message
func (h *HTTPServer) respondSuccess(c *gin.Context, status int, message string) {
	response := map[string]interface{}{
		"success": true,
		"message": message,
	}
	c.JSON(status, response)
}

// respondError responds with an error message
func (h *HTTPServer) respondError(c *gin.Context, status int, message string, err error) {
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
func (h *HTTPServer) handleGRPCError(c *gin.Context, err error) {
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
