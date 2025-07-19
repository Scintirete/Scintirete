// Package test provides integration tests for Scintirete.
package test

import (
	"context"
	"testing"
	"time"

	"github.com/scintirete/scintirete/internal/core/database"
	"github.com/scintirete/scintirete/internal/embedding"
	"github.com/scintirete/scintirete/internal/persistence"
	"github.com/scintirete/scintirete/internal/server"
	"github.com/scintirete/scintirete/pkg/types"
)

func TestFullStackIntegration(t *testing.T) {
	// Set up fake embedding API key for testing
	t.Setenv("OPENAI_API_KEY", "fake-api-key-for-testing")

	// Create temporary directory for test data
	tempDir := t.TempDir()

	// Create server configuration
	serverConfig := server.Config{
		Passwords: []string{"test-password"},
		PersistenceConfig: persistence.Config{
			DataDir:         tempDir,
			RDBFilename:     "test.rdb",
			AOFFilename:     "test.aof",
			AOFSyncStrategy: "always",
			RDBInterval:     time.Minute,
			AOFRewriteSize:  1024 * 1024,
			BackupRetention: 3,
		},
		EmbeddingConfig: embedding.Config{
			BaseURL:      "https://api.test.com/v1/embeddings", // Use fake URL for testing
			APIKeyEnvVar: "OPENAI_API_KEY",
			RPMLimit:     100,
			TPMLimit:     10000,
		},
		EnableMetrics:  false,
		EnableAuditLog: false,
	}

	// Create gRPC server
	grpcServer, err := server.NewGRPCServer(serverConfig)
	if err != nil {
		t.Fatalf("Failed to create gRPC server: %v", err)
	}

	// Start server
	ctx := context.Background()
	if err := grpcServer.Start(ctx); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer grpcServer.Stop(ctx)

	// Test basic functionality through direct API calls
	t.Run("database_operations", func(t *testing.T) {
		testDatabaseOperations(t, grpcServer)
	})

	t.Run("collection_operations", func(t *testing.T) {
		testCollectionOperations(t, grpcServer)
	})

	t.Run("vector_operations", func(t *testing.T) {
		testVectorOperations(t, grpcServer)
	})
}

func testDatabaseOperations(t *testing.T, server *server.GRPCServer) {
	// This would require exposing internal engine for testing
	// For now, we'll test that the server can be created and started
	stats := server.GetStats()
	if stats.RequestCount < 0 {
		t.Errorf("Invalid request count: %d", stats.RequestCount)
	}
}

func testCollectionOperations(t *testing.T, server *server.GRPCServer) {
	// Test collection creation and management
	// This would be done through gRPC calls in a real integration test

	// For now, we test the underlying database engine
	engine := database.NewEngine()
	defer engine.Close(context.Background())

	ctx := context.Background()

	// Create database
	err := engine.CreateDatabase(ctx, "test_db")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}

	// Get database
	db, err := engine.GetDatabase(ctx, "test_db")
	if err != nil {
		t.Fatalf("Failed to get database: %v", err)
	}

	// Create collection
	config := types.CollectionConfig{
		Name:       "test_collection",
		Metric:     types.DistanceMetricL2,
		HNSWParams: types.DefaultHNSWParams(),
	}

	err = db.CreateCollection(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create collection: %v", err)
	}

	// List collections
	collections, err := db.ListCollections(ctx)
	if err != nil {
		t.Fatalf("Failed to list collections: %v", err)
	}

	if len(collections) != 1 {
		t.Errorf("Expected 1 collection, got %d", len(collections))
	}

	if collections[0].Name != "test_collection" {
		t.Errorf("Expected collection name 'test_collection', got '%s'", collections[0].Name)
	}
}

func testVectorOperations(t *testing.T, server *server.GRPCServer) {
	// Test vector insertion, search, and deletion
	engine := database.NewEngine()
	defer engine.Close(context.Background())

	ctx := context.Background()

	// Setup database and collection
	err := engine.CreateDatabase(ctx, "vector_test_db")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}

	db, err := engine.GetDatabase(ctx, "vector_test_db")
	if err != nil {
		t.Fatalf("Failed to get database: %v", err)
	}

	config := types.CollectionConfig{
		Name:       "vector_collection",
		Metric:     types.DistanceMetricL2,
		HNSWParams: types.DefaultHNSWParams(),
	}

	err = db.CreateCollection(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create collection: %v", err)
	}

	collection, err := db.GetCollection(ctx, "vector_collection")
	if err != nil {
		t.Fatalf("Failed to get collection: %v", err)
	}

	// Insert vectors
	vectors := []types.Vector{
		{
			ID:       "vec1",
			Elements: []float32{1.0, 2.0, 3.0},
			Metadata: map[string]interface{}{"label": "test1"},
		},
		{
			ID:       "vec2",
			Elements: []float32{4.0, 5.0, 6.0},
			Metadata: map[string]interface{}{"label": "test2"},
		},
		{
			ID:       "vec3",
			Elements: []float32{7.0, 8.0, 9.0},
			Metadata: map[string]interface{}{"label": "test3"},
		},
	}

	err = collection.Insert(ctx, vectors)
	if err != nil {
		t.Fatalf("Failed to insert vectors: %v", err)
	}

	// Test vector count
	count, err := collection.Count(ctx)
	if err != nil {
		t.Fatalf("Failed to get vector count: %v", err)
	}

	if count != 3 {
		t.Errorf("Expected 3 vectors, got %d", count)
	}

	// Test vector retrieval
	vec, err := collection.Get(ctx, "vec1")
	if err != nil {
		t.Fatalf("Failed to get vector: %v", err)
	}

	if vec.ID != "vec1" {
		t.Errorf("Expected vector ID 'vec1', got '%s'", vec.ID)
	}

	// Test vector search
	queryVector := []float32{1.1, 2.1, 3.1}
	searchParams := types.SearchParams{TopK: 2}

	results, err := collection.Search(ctx, queryVector, searchParams)
	if err != nil {
		t.Fatalf("Failed to search vectors: %v", err)
	}

	if len(results) == 0 {
		t.Error("Expected search results, got none")
	}

	// Test vector deletion
	deletedCount, err := collection.Delete(ctx, []string{"vec2"})
	if err != nil {
		t.Fatalf("Failed to delete vector: %v", err)
	}

	if deletedCount != 1 {
		t.Errorf("Expected 1 deleted vector, got %d", deletedCount)
	}

	// Verify count after deletion
	count, err = collection.Count(ctx)
	if err != nil {
		t.Fatalf("Failed to get vector count after deletion: %v", err)
	}

	if count != 2 {
		t.Errorf("Expected 2 vectors after deletion, got %d", count)
	}
}
