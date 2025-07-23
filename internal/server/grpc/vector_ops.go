// Package grpc provides vector operations for the gRPC server.
package grpc

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"

	pb "github.com/scintirete/scintirete/gen/go/scintirete/v1"
	"github.com/scintirete/scintirete/internal/utils"
	"github.com/scintirete/scintirete/pkg/types"
)

// InsertVectors adds vectors to a collection
func (s *Server) InsertVectors(ctx context.Context, req *pb.InsertVectorsRequest) (*pb.InsertVectorsResponse, error) {
	// Authenticate
	if err := s.authenticate(req.Auth); err != nil {
		return nil, err
	}

	// Validate input
	if req.DbName == "" {
		return nil, status.Error(codes.InvalidArgument, "database name cannot be empty")
	}
	if req.CollectionName == "" {
		return nil, status.Error(codes.InvalidArgument, "collection name cannot be empty")
	}
	if len(req.Vectors) == 0 {
		return nil, status.Error(codes.InvalidArgument, "no vectors provided")
	}

	// Convert protobuf vectors to internal format
	vectors := make([]types.Vector, len(req.Vectors))
	insertedIds := make([]uint64, len(req.Vectors))
	for i, pbVector := range req.Vectors {
		if len(pbVector.Elements) == 0 {
			return nil, status.Error(codes.InvalidArgument, "vector elements cannot be empty")
		}

		// Convert metadata
		metadata := make(map[string]interface{})
		if pbVector.Metadata != nil {
			metadata = pbVector.Metadata.AsMap()
		}

		vectors[i] = types.Vector{
			ID:       pbVector.Id, // This will be auto-generated in the collection
			Elements: pbVector.Elements,
			Metadata: metadata,
		}
		insertedIds[i] = pbVector.Id
	}

	// Get database
	db, err := s.engine.GetDatabase(ctx, req.DbName)
	if err != nil {
		if utils.IsScintireteError(err) {
			return nil, s.convertError(err)
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Get collection
	collection, err := db.GetCollection(ctx, req.CollectionName)
	if err != nil {
		if utils.IsScintireteError(err) {
			return nil, s.convertError(err)
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Insert vectors
	if err := collection.Insert(ctx, vectors); err != nil {
		if utils.IsScintireteError(err) {
			return nil, s.convertError(err)
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Log to persistence
	if err := s.persistence.LogInsertVectors(ctx, req.DbName, req.CollectionName, vectors); err != nil {
		return nil, status.Error(codes.Internal, "failed to log insert vectors operation")
	}

	// Log to audit
	s.logAuditOperation(ctx, "InsertVectors", req.DbName, req.CollectionName, req.Auth, map[string]interface{}{
		"operation_type": "vector_data",
		"vector_count":   len(vectors),
	})

	s.updateRequestStats()
	return &pb.InsertVectorsResponse{
		InsertedIds:   insertedIds,
		InsertedCount: int32(len(vectors)),
	}, nil
}

// DeleteVectors marks vectors as deleted by their IDs
func (s *Server) DeleteVectors(ctx context.Context, req *pb.DeleteVectorsRequest) (*pb.DeleteVectorsResponse, error) {
	// Authenticate
	if err := s.authenticate(req.Auth); err != nil {
		return nil, err
	}

	// Validate input
	if req.DbName == "" {
		return nil, status.Error(codes.InvalidArgument, "database name cannot be empty")
	}
	if req.CollectionName == "" {
		return nil, status.Error(codes.InvalidArgument, "collection name cannot be empty")
	}
	if len(req.Ids) == 0 {
		return nil, status.Error(codes.InvalidArgument, "no IDs provided")
	}

	// Get database
	db, err := s.engine.GetDatabase(ctx, req.DbName)
	if err != nil {
		if utils.IsScintireteError(err) {
			return nil, s.convertError(err)
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Get collection
	collection, err := db.GetCollection(ctx, req.CollectionName)
	if err != nil {
		if utils.IsScintireteError(err) {
			return nil, s.convertError(err)
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Convert uint64 IDs to strings for compatibility with existing collection interface
	stringIds := make([]string, len(req.Ids))
	for i, id := range req.Ids {
		stringIds[i] = fmt.Sprintf("%d", id)
	}

	// Delete vectors
	deletedCount, err := collection.Delete(ctx, stringIds)
	if err != nil {
		if utils.IsScintireteError(err) {
			return nil, s.convertError(err)
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Log to persistence
	if err := s.persistence.LogDeleteVectors(ctx, req.DbName, req.CollectionName, stringIds); err != nil {
		return nil, status.Error(codes.Internal, "failed to log delete vectors operation")
	}

	// Log to audit
	s.logAuditOperation(ctx, "DeleteVectors", req.DbName, req.CollectionName, req.Auth, map[string]interface{}{
		"operation_type":  "vector_data",
		"requested_count": len(req.Ids),
		"actual_deleted":  deletedCount,
	})

	s.updateRequestStats()
	return &pb.DeleteVectorsResponse{DeletedCount: int32(deletedCount)}, nil
}

// Search performs vector similarity search
func (s *Server) Search(ctx context.Context, req *pb.SearchRequest) (*pb.SearchResponse, error) {
	// Authenticate
	if err := s.authenticate(req.Auth); err != nil {
		return nil, err
	}

	// Validate input
	if req.DbName == "" {
		return nil, status.Error(codes.InvalidArgument, "database name cannot be empty")
	}
	if req.CollectionName == "" {
		return nil, status.Error(codes.InvalidArgument, "collection name cannot be empty")
	}
	if len(req.QueryVector) == 0 {
		return nil, status.Error(codes.InvalidArgument, "query vector cannot be empty")
	}
	if req.TopK <= 0 {
		return nil, status.Error(codes.InvalidArgument, "top_k must be positive")
	}

	// Convert search parameters
	params := types.SearchParams{
		TopK: int(req.TopK),
	}
	if req.EfSearch != nil {
		efSearch := int(*req.EfSearch)
		params.EfSearch = &efSearch
	}

	// Get database
	db, err := s.engine.GetDatabase(ctx, req.DbName)
	if err != nil {
		if utils.IsScintireteError(err) {
			return nil, s.convertError(err)
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Get collection
	collection, err := db.GetCollection(ctx, req.CollectionName)
	if err != nil {
		if utils.IsScintireteError(err) {
			return nil, s.convertError(err)
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Perform search
	results, err := collection.Search(ctx, req.QueryVector, params)
	if err != nil {
		if utils.IsScintireteError(err) {
			return nil, s.convertError(err)
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Check if we should include vector data (default: false for performance)
	includeVector := false
	if req.IncludeVector != nil {
		includeVector = *req.IncludeVector
	}

	// Convert results to protobuf
	pbResults := make([]*pb.SearchResultItem, len(results))
	for i, result := range results {
		// Convert metadata back to protobuf Struct
		metadata, err := structpb.NewStruct(result.Vector.Metadata)
		if err != nil {
			return nil, status.Error(codes.Internal, "failed to convert metadata")
		}

		// Build result item - Vector object is always included with id and metadata
		item := &pb.SearchResultItem{
			Distance: result.Distance,
			Id:       result.Vector.ID,
			Metadata: metadata,
			Vector: &pb.Vector{
				Id:       result.Vector.ID,
				Metadata: metadata,
			},
		}

		// Only include vector elements if explicitly requested
		if includeVector {
			item.Vector.Elements = result.Vector.Elements
		}
		pbResults[i] = item
	}

	s.updateRequestStats()
	return &pb.SearchResponse{Results: pbResults}, nil
}

// EmbedAndInsert processes text through embedding API and inserts the resulting vectors
func (s *Server) EmbedAndInsert(ctx context.Context, req *pb.EmbedAndInsertRequest) (*pb.EmbedAndInsertResponse, error) {
	// Authenticate
	if err := s.authenticate(req.Auth); err != nil {
		return nil, err
	}

	// Validate input
	if req.DbName == "" {
		return nil, status.Error(codes.InvalidArgument, "database name cannot be empty")
	}
	if req.CollectionName == "" {
		return nil, status.Error(codes.InvalidArgument, "collection name cannot be empty")
	}
	if len(req.Texts) == 0 {
		return nil, status.Error(codes.InvalidArgument, "no texts provided")
	}

	// Convert protobuf texts to types.TextWithMetadata
	texts := make([]types.TextWithMetadata, len(req.Texts))
	for i, text := range req.Texts {
		metadata := make(map[string]interface{})
		if text.Metadata != nil {
			metadata = text.Metadata.AsMap()
		}
		texts[i] = types.TextWithMetadata{
			ID:       text.Id, // Can be nil for auto-generation
			Text:     text.Text,
			Metadata: metadata,
		}
	}

	// Get embedding model (use default if not specified)
	model := "text-embedding-ada-002" // Default model
	if req.EmbeddingModel != nil {
		model = *req.EmbeddingModel
	}

	// Convert texts to vectors using embedding API
	vectors, err := s.embedding.ConvertTextsToVectors(ctx, texts, model)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get embeddings: %v", err)
	}

	// Get database
	db, err := s.engine.GetDatabase(ctx, req.DbName)
	if err != nil {
		if isNotFoundError(err) {
			return nil, status.Error(codes.NotFound, "database not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get database: %v", err)
	}

	// Get collection
	coll, err := db.GetCollection(ctx, req.CollectionName)
	if err != nil {
		if isNotFoundError(err) {
			return nil, status.Error(codes.NotFound, "collection not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get collection: %v", err)
	}

	// Insert vectors
	if err := coll.Insert(ctx, vectors); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to insert vectors: %v", err)
	}

	// Log the actual data operation (INSERT_VECTORS) to AOF - this is what actually happened at data level
	if err := s.persistence.LogInsertVectors(ctx, req.DbName, req.CollectionName, vectors); err != nil {
		// Log error but don't fail the operation - AOF write failure shouldn't block operation
		if s.logger != nil {
			s.logger.Error(ctx, "AOF write failed for EmbedAndInsert operation", err, map[string]interface{}{
				"operation":    "EmbedAndInsert",
				"database":     req.DbName,
				"collection":   req.CollectionName,
				"vector_count": len(vectors),
			})
		}
		// Note: Operation continues successfully even if AOF write fails
		// This ensures user operations aren't blocked by persistence issues
	}

	// Log the auxiliary operation (EmbedAndInsert) to audit log for tracking purposes
	s.logAuditOperation(ctx, "EmbedAndInsert", req.DbName, req.CollectionName, req.Auth, map[string]interface{}{
		"operation_type":  "auxiliary",
		"embedding_model": model,
		"text_count":      len(texts),
		"vector_count":    len(vectors),
	})

	// Extract inserted IDs from vectors
	insertedIds := make([]uint64, len(vectors))
	for i, vector := range vectors {
		insertedIds[i] = vector.ID
	}

	s.updateRequestStats()
	return &pb.EmbedAndInsertResponse{
		InsertedIds:   insertedIds,
		InsertedCount: int32(len(vectors)),
	}, nil
}

// EmbedAndSearch processes query text through embedding API and performs search
func (s *Server) EmbedAndSearch(ctx context.Context, req *pb.EmbedAndSearchRequest) (*pb.SearchResponse, error) {
	// Authenticate
	if err := s.authenticate(req.Auth); err != nil {
		return nil, err
	}

	// Validate input
	if req.DbName == "" {
		return nil, status.Error(codes.InvalidArgument, "database name cannot be empty")
	}
	if req.CollectionName == "" {
		return nil, status.Error(codes.InvalidArgument, "collection name cannot be empty")
	}
	if req.QueryText == "" {
		return nil, status.Error(codes.InvalidArgument, "query text cannot be empty")
	}
	if req.TopK <= 0 {
		return nil, status.Error(codes.InvalidArgument, "top_k must be positive")
	}

	// Get embedding model (use default if not specified)
	model := "text-embedding-ada-002" // Default model
	if req.EmbeddingModel != nil {
		model = *req.EmbeddingModel
	}

	// Get embedding for query text
	queryEmbedding, err := s.embedding.GetSingleEmbedding(ctx, req.QueryText, model)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get query embedding: %v", err)
	}

	// Get database
	db, err := s.engine.GetDatabase(ctx, req.DbName)
	if err != nil {
		if isNotFoundError(err) {
			return nil, status.Error(codes.NotFound, "database not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get database: %v", err)
	}

	// Get collection
	coll, err := db.GetCollection(ctx, req.CollectionName)
	if err != nil {
		if isNotFoundError(err) {
			return nil, status.Error(codes.NotFound, "collection not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get collection: %v", err)
	}

	// Prepare search parameters
	searchParams := types.SearchParams{
		TopK: int(req.TopK),
	}
	if req.EfSearch != nil {
		efSearch := int(*req.EfSearch)
		searchParams.EfSearch = &efSearch
	}

	// Perform search
	results, err := coll.Search(ctx, queryEmbedding, searchParams)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to perform search: %v", err)
	}

	// Check if we should include vector data (default: false for performance)
	includeVector := false
	if req.IncludeVector != nil {
		includeVector = *req.IncludeVector
	}

	// Convert results to protobuf format
	pbResults := make([]*pb.SearchResultItem, len(results))
	for i, result := range results {
		// Build result item - Vector object is always included with id and metadata
		item := &pb.SearchResultItem{
			Distance: result.Distance,
			Id:       result.Vector.ID,
			Metadata: mapToStruct(result.Vector.Metadata),
			Vector: &pb.Vector{
				Id:       result.Vector.ID,
				Metadata: mapToStruct(result.Vector.Metadata),
			},
		}

		// Only include vector elements if explicitly requested
		if includeVector {
			item.Vector.Elements = result.Vector.Elements
		}

		pbResults[i] = item
	}

	return &pb.SearchResponse{
		Results: pbResults,
	}, nil
}

// EmbedText converts text to embeddings without inserting into collections
func (s *Server) EmbedText(ctx context.Context, req *pb.EmbedTextRequest) (*pb.EmbedTextResponse, error) {
	// Authenticate
	if err := s.authenticate(req.Auth); err != nil {
		return nil, err
	}

	// Validate input
	if len(req.Texts) == 0 {
		return nil, status.Error(codes.InvalidArgument, "no texts provided")
	}

	// Get embedding model (use default if not specified)
	model := "text-embedding-ada-002" // Default model
	if req.EmbeddingModel != nil {
		model = *req.EmbeddingModel
	}

	// Get embeddings for all texts
	embeddingResp, err := s.embedding.GetEmbeddings(ctx, req.Texts, model)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get embeddings: %v", err)
	}

	// Convert to response format
	results := make([]*pb.EmbedTextResult, len(req.Texts))
	for i, text := range req.Texts {
		if i < len(embeddingResp.Data) {
			results[i] = &pb.EmbedTextResult{
				Text:      text,
				Embedding: embeddingResp.Data[i].Embedding,
				Index:     int32(i),
			}
		}
	}

	// Log to audit
	s.logAuditOperation(ctx, "EmbedText", "", "", req.Auth, map[string]interface{}{
		"operation_type":  "embedding",
		"embedding_model": model,
		"text_count":      len(req.Texts),
	})

	s.updateRequestStats()
	return &pb.EmbedTextResponse{Results: results}, nil
}

// ListEmbeddingModels returns available embedding models
func (s *Server) ListEmbeddingModels(ctx context.Context, req *pb.ListEmbeddingModelsRequest) (*pb.ListEmbeddingModelsResponse, error) {
	// Authenticate
	if err := s.authenticate(req.Auth); err != nil {
		return nil, err
	}

	// Return commonly used embedding models (hardcoded for now)
	models := []*pb.EmbeddingModel{
		{
			Id:          "text-embedding-ada-002",
			Name:        "OpenAI Ada v2",
			Dimension:   1536,
			Available:   true,
			Description: "OpenAI's most capable embedding model",
		},
		{
			Id:          "text-embedding-3-small",
			Name:        "OpenAI Text Embedding 3 Small",
			Dimension:   1536,
			Available:   true,
			Description: "Smaller, faster embedding model",
		},
		{
			Id:          "text-embedding-3-large",
			Name:        "OpenAI Text Embedding 3 Large",
			Dimension:   3072,
			Available:   true,
			Description: "Larger, more capable embedding model",
		},
	}
	defaultModel := "text-embedding-ada-002"

	s.updateRequestStats()
	return &pb.ListEmbeddingModelsResponse{
		Models:       models,
		DefaultModel: defaultModel,
	}, nil
}
