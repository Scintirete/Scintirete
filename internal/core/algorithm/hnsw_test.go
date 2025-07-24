package algorithm

import (
	"context"
	"testing"

	"github.com/scintirete/scintirete/pkg/types"
)

func TestNewHNSW(t *testing.T) {
	params := types.DefaultHNSWParams()
	metric := types.DistanceMetricL2

	index, err := NewHNSW(params, metric)
	if err != nil {
		t.Fatalf("NewHNSW failed: %v", err)
	}

	if index == nil {
		t.Fatal("NewHNSW returned nil index")
	}

	hnswIndex, ok := index.(*HNSW)
	if !ok {
		t.Fatal("NewHNSW did not return HNSW type")
	}

	if hnswIndex.params.M != params.M {
		t.Errorf("HNSW params.M = %d, want %d", hnswIndex.params.M, params.M)
	}

	if hnswIndex.metric != metric {
		t.Errorf("HNSW metric = %v, want %v", hnswIndex.metric, metric)
	}
}

func TestNewHNSW_InvalidMetric(t *testing.T) {
	params := types.DefaultHNSWParams()
	metric := types.DistanceMetricUnspecified

	_, err := NewHNSW(params, metric)
	if err == nil {
		t.Fatal("NewHNSW should fail with invalid metric")
	}
}

func TestHNSW_EmptyIndex(t *testing.T) {
	index := createTestHNSW(t)

	// Test empty index properties
	if size := index.Size(); size != 0 {
		t.Errorf("Empty index size = %d, want 0", size)
	}

	if layers := index.GetLayers(); layers != 0 {
		t.Errorf("Empty index layers = %d, want 0", layers)
	}

	// Test search on empty index
	query := []float32{1.0, 2.0}
	params := types.SearchParams{TopK: 5}
	results, err := index.Search(context.Background(), query, params)
	if err != nil {
		t.Errorf("Search on empty index failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Search on empty index returned %d results, want 0", len(results))
	}

	// Test get on empty index
	_, err = index.Get(context.Background(), "nonexistent")
	if err == nil {
		t.Error("Get on empty index should return error")
	}
}

func TestHNSW_SingleVector(t *testing.T) {
	index := createTestHNSW(t)

	vector := types.Vector{
		ID:       1,
		Elements: []float32{1.0, 2.0, 3.0},
		Metadata: map[string]interface{}{"tag": "test"},
	}

	// Insert vector
	err := index.Insert(context.Background(), vector)
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	// Check size
	if size := index.Size(); size != 1 {
		t.Errorf("Index size = %d, want 1", size)
	}

	// Get vector
	retrieved, err := index.Get(context.Background(), "1")
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Get returned nil")
	}
	if retrieved.ID != 1 {
		t.Errorf("Retrieved ID = %d, want 1", retrieved.ID)
	}

	// Search for similar vector
	query := []float32{1.1, 2.1, 3.1}
	searchParams := types.SearchParams{TopK: 1}
	results, err := index.Search(context.Background(), query, searchParams)
	if err != nil {
		t.Errorf("Search failed: %v", err)
	}
	if len(results) == 0 {
		t.Error("Search returned no results")
	}
	if len(results) > 0 && results[0].Vector.ID != 1 {
		t.Errorf("Search result ID = %d, want 1", results[0].Vector.ID)
	}
}

func TestHNSW_MultipleVectors(t *testing.T) {
	index := createTestHNSW(t)

	vectors := []types.Vector{
		{ID: 1, Elements: []float32{1.0, 0.0}},
		{ID: 2, Elements: []float32{0.0, 1.0}},
		{ID: 3, Elements: []float32{1.0, 1.0}},
		{ID: 4, Elements: []float32{2.0, 2.0}},
	}

	// Build index
	err := index.Build(context.Background(), vectors)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Check size
	if size := index.Size(); size != 4 {
		t.Errorf("Index size = %d, want 4", size)
	}

	// Test search
	query := []float32{0.0, 0.0}
	params := types.SearchParams{TopK: 2}
	results, err := index.Search(context.Background(), query, params)
	if err != nil {
		t.Errorf("Search failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("Search returned %d results, want 2", len(results))
	}

	// Results should be sorted by distance
	if results[0].Distance > results[1].Distance {
		t.Error("Search results not sorted by distance")
	}
}

func TestHNSW_Delete(t *testing.T) {
	index := createTestHNSW(t)

	vectors := []types.Vector{
		{ID: 1, Elements: []float32{1.0, 0.0}},
		{ID: 2, Elements: []float32{0.0, 1.0}},
		{ID: 3, Elements: []float32{1.0, 1.0}},
	}

	// Build index
	err := index.Build(context.Background(), vectors)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Delete vector
	err = index.Delete(context.Background(), "2")
	if err != nil {
		t.Errorf("Delete failed: %v", err)
	}

	// Check size
	if size := index.Size(); size != 2 {
		t.Errorf("Index size after delete = %d, want 2", size)
	}

	// Try to get deleted vector
	_, err = index.Get(context.Background(), "2")
	if err == nil {
		t.Error("Get should fail for deleted vector")
	}

	// Search should not return deleted vector
	query := []float32{0.0, 1.0}
	params := types.SearchParams{TopK: 3}
	results, err := index.Search(context.Background(), query, params)
	if err != nil {
		t.Errorf("Search failed: %v", err)
	}

	for _, result := range results {
		if result.Vector.ID == 2 {
			t.Error("Search returned deleted vector")
		}
	}

	// Delete non-existent vector
	err = index.Delete(context.Background(), "nonexistent")
	if err == nil {
		t.Error("Delete should fail for non-existent vector")
	}

	// Delete already deleted vector (should succeed)
	err = index.Delete(context.Background(), "2")
	if err != nil {
		t.Errorf("Delete of already deleted vector failed: %v", err)
	}
}

func TestHNSW_DuplicateInsert(t *testing.T) {
	index := createTestHNSW(t)

	vector := types.Vector{
		ID:       1,
		Elements: []float32{1.0, 2.0},
	}

	// Insert vector
	err := index.Insert(context.Background(), vector)
	if err != nil {
		t.Fatalf("First insert failed: %v", err)
	}

	// Try to insert again
	err = index.Insert(context.Background(), vector)
	if err == nil {
		t.Error("Duplicate insert should fail")
	}
}

func TestHNSW_DifferentMetrics(t *testing.T) {
	metrics := []types.DistanceMetric{
		types.DistanceMetricL2,
		types.DistanceMetricCosine,
		types.DistanceMetricInnerProduct,
	}

	for _, metric := range metrics {
		t.Run(metric.String(), func(t *testing.T) {
			params := types.DefaultHNSWParams()
			index, err := NewHNSW(params, metric)
			if err != nil {
				t.Fatalf("NewHNSW failed for metric %v: %v", metric, err)
			}

			vectors := []types.Vector{
				{ID: 1, Elements: []float32{1.0, 0.0}},
				{ID: 2, Elements: []float32{0.0, 1.0}},
			}

			err = index.Build(context.Background(), vectors)
			if err != nil {
				t.Fatalf("Build failed for metric %v: %v", metric, err)
			}

			query := []float32{1.0, 0.0}
			searchParams := types.SearchParams{TopK: 1}
			results, err := index.Search(context.Background(), query, searchParams)
			if err != nil {
				t.Errorf("Search failed for metric %v: %v", metric, err)
			}
			if len(results) != 1 {
				t.Errorf("Search returned %d results for metric %v, want 1", len(results), metric)
			}
		})
	}
}

func TestHNSW_Parameters(t *testing.T) {
	params := types.HNSWParams{
		M:              32,
		EfConstruction: 400,
		EfSearch:       100,
		MaxLayers:      10,
		Seed:           12345,
	}

	index, err := NewHNSW(params, types.DistanceMetricL2)
	if err != nil {
		t.Fatalf("NewHNSW failed: %v", err)
	}

	returnedParams := index.GetParameters()
	if returnedParams.M != params.M {
		t.Errorf("GetParameters().M = %d, want %d", returnedParams.M, params.M)
	}
	if returnedParams.EfConstruction != params.EfConstruction {
		t.Errorf("GetParameters().EfConstruction = %d, want %d", returnedParams.EfConstruction, params.EfConstruction)
	}

	// Test SetEfSearch
	newEfSearch := 200
	index.SetEfSearch(newEfSearch)
	updatedParams := index.GetParameters()
	if updatedParams.EfSearch != newEfSearch {
		t.Errorf("After SetEfSearch, EfSearch = %d, want %d", updatedParams.EfSearch, newEfSearch)
	}
}

func TestHNSW_Statistics(t *testing.T) {
	index := createTestHNSW(t)

	vectors := []types.Vector{
		{ID: 1, Elements: []float32{1.0, 0.0}},
		{ID: 2, Elements: []float32{0.0, 1.0}},
		{ID: 3, Elements: []float32{1.0, 1.0}},
	}

	err := index.Build(context.Background(), vectors)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	stats := index.GetGraphStatistics()
	if stats.Nodes != 3 {
		t.Errorf("Statistics nodes = %d, want 3", stats.Nodes)
	}
	if stats.Layers < 1 {
		t.Errorf("Statistics layers = %d, want >= 1", stats.Layers)
	}
	if stats.MemoryUsage <= 0 {
		t.Errorf("Statistics memory usage = %d, want > 0", stats.MemoryUsage)
	}

	// Check that statistics interface works
	genericStats := index.GetStatistics()
	if genericStats == nil {
		t.Error("GetStatistics returned nil")
	}
}

func TestHNSW_MemoryUsage(t *testing.T) {
	index := createTestHNSW(t)

	// Empty index should have minimal memory usage
	initialMemory := index.MemoryUsage()
	if initialMemory < 0 {
		t.Errorf("Memory usage should be non-negative, got %d", initialMemory)
	}

	// Add some vectors
	vectors := []types.Vector{
		{ID: 1, Elements: make([]float32, 128)}, // Large vector
		{ID: 2, Elements: make([]float32, 128)},
	}

	err := index.Build(context.Background(), vectors)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Memory usage should increase
	finalMemory := index.MemoryUsage()
	if finalMemory <= initialMemory {
		t.Errorf("Memory usage should increase after adding vectors: %d -> %d", initialMemory, finalMemory)
	}
}

func TestHNSWNode(t *testing.T) {
	vector := []float32{1.0, 2.0, 3.0}
	metadata := map[string]interface{}{"tag": "test"}
	maxLayers := 3

	node := NewHNSWNode(123, vector, metadata, maxLayers)

	if node.ID != 123 {
		t.Errorf("Node ID = %d, want 123", node.ID)
	}
	if len(node.Vector) != 3 {
		t.Errorf("Node vector length = %d, want 3", len(node.Vector))
	}
	if node.Deleted {
		t.Error("New node should not be deleted")
	}
	if len(node.Connections) != maxLayers {
		t.Errorf("Node connections length = %d, want %d", len(node.Connections), maxLayers)
	}

	// Test connections
	node.AddConnection(0, 1)
	node.AddConnection(0, 2)
	node.AddConnection(1, 3)

	connections0 := node.GetConnections(0)
	if len(connections0) != 2 {
		t.Errorf("Layer 0 connections = %d, want 2", len(connections0))
	}

	connections1 := node.GetConnections(1)
	if len(connections1) != 1 {
		t.Errorf("Layer 1 connections = %d, want 1", len(connections1))
	}

	// Test removing connections
	node.RemoveConnection(0, 1)
	connections0 = node.GetConnections(0)
	if len(connections0) != 1 {
		t.Errorf("After removal, layer 0 connections = %d, want 1", len(connections0))
	}
}

// Helper function to create a test HNSW index
func createTestHNSW(t *testing.T) *HNSW {
	params := types.DefaultHNSWParams()
	index, err := NewHNSW(params, types.DistanceMetricL2)
	if err != nil {
		t.Fatalf("Failed to create test HNSW: %v", err)
	}
	return index.(*HNSW)
}

// Benchmark tests
func BenchmarkHNSW_Insert(b *testing.B) {
	index := createTestHNSWForBench(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		vector := types.Vector{
			ID:       generateID(i),
			Elements: generateRandomVector(128),
		}
		index.Insert(context.Background(), vector)
	}
}

func BenchmarkHNSW_Search(b *testing.B) {
	index := createTestHNSWForBench(b)

	// Pre-populate with some vectors
	vectors := make([]types.Vector, 1000)
	for i := range vectors {
		vectors[i] = types.Vector{
			ID:       generateID(i),
			Elements: generateRandomVector(128),
		}
	}
	index.Build(context.Background(), vectors)

	query := generateRandomVector(128)
	params := types.SearchParams{TopK: 10}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		index.Search(context.Background(), query, params)
	}
}

// Helper functions for benchmarks
func createTestHNSWForBench(b *testing.B) *HNSW {
	params := types.DefaultHNSWParams()
	index, err := NewHNSW(params, types.DistanceMetricL2)
	if err != nil {
		b.Fatalf("Failed to create benchmark HNSW: %v", err)
	}
	return index.(*HNSW)
}

func generateID(i int) uint64 {
	return uint64(i + 1) // Add 1 to avoid 0 which is treated as empty ID
}

func generateRandomVector(dim int) []float32 {
	vector := make([]float32, dim)
	for i := range vector {
		vector[i] = float32(i) / float32(dim) // Simple deterministic pattern
	}
	return vector
}
