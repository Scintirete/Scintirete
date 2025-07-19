// Package benchmark provides performance benchmarks for Scintirete.
package benchmark

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/scintirete/scintirete/internal/core/algorithm"
	"github.com/scintirete/scintirete/internal/core/database"
	"github.com/scintirete/scintirete/pkg/types"
)

// Performance targets from architecture document:
// - Insert operation: <10ms (single vector)
// - Search operation: <50ms (top-10, 1M vectors)
// - Batch insert: <100ms (100 vectors)

const (
	// Test data dimensions
	testDimensions = 768

	// Performance targets (in nanoseconds)
	insertTargetNs      = 10 * time.Millisecond
	searchTargetNs      = 50 * time.Millisecond
	batchInsertTargetNs = 100 * time.Millisecond

	// Test data sizes
	smallDataset   = 1000
	mediumDataset  = 10000
	largeDataset   = 100000
	massiveDataset = 1000000
)

// generateRandomVector creates a random vector of specified dimension
func generateRandomVector(id string, dimension int) types.Vector {
	vector := make([]float32, dimension)
	for i := range vector {
		vector[i] = rand.Float32()*2 - 1 // Random values between -1 and 1
	}

	return types.Vector{
		ID:       id,
		Elements: vector,
		Metadata: map[string]interface{}{"benchmark": true, "id": id},
	}
}

// generateTestVectors creates a slice of test vectors
func generateTestVectors(count int, dimension int) []types.Vector {
	vectors := make([]types.Vector, count)
	for i := 0; i < count; i++ {
		vectors[i] = generateRandomVector(fmt.Sprintf("vec_%d", i), dimension)
	}
	return vectors
}

// setupHNSWIndex creates and builds an HNSW index with test data
func setupHNSWIndex(b *testing.B, vectorCount int) (*algorithm.HNSW, []types.Vector) {
	b.Helper()

	// Create HNSW index with optimized parameters
	params := types.HNSWParams{
		M:              16,
		EfConstruction: 200,
		EfSearch:       50,
		MaxLayers:      4,
		Seed:           42,
	}

	index, err := algorithm.NewHNSW(params, types.DistanceMetricL2)
	if err != nil {
		b.Fatalf("Failed to create HNSW index: %v", err)
	}

	// Generate test vectors
	vectors := generateTestVectors(vectorCount, testDimensions)

	// Build index
	ctx := context.Background()
	if err := index.Build(ctx, vectors); err != nil {
		b.Fatalf("Failed to build index: %v", err)
	}

	return index.(*algorithm.HNSW), vectors
}

// BenchmarkHNSWInsert tests single vector insertion performance
func BenchmarkHNSWInsert(b *testing.B) {
	testCases := []struct {
		name        string
		vectorCount int
	}{
		{"SmallIndex_1K", smallDataset},
		{"MediumIndex_10K", mediumDataset},
		{"LargeIndex_100K", largeDataset},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			index, _ := setupHNSWIndex(b, tc.vectorCount)
			ctx := context.Background()

			// Generate vectors for insertion
			insertVectors := generateTestVectors(b.N, testDimensions)

			b.ResetTimer()
			start := time.Now()

			for i := 0; i < b.N; i++ {
				err := index.Insert(ctx, insertVectors[i])
				if err != nil {
					b.Fatalf("Insert failed: %v", err)
				}
			}

			elapsed := time.Since(start)
			avgTime := elapsed / time.Duration(b.N)

			// Check performance target: <10ms per insert
			if avgTime > insertTargetNs {
				b.Errorf("Insert performance target missed: got %v, target <%v", avgTime, insertTargetNs)
			}

			b.ReportMetric(float64(avgTime.Nanoseconds()), "ns/insert")
		})
	}
}

// BenchmarkHNSWSearch tests search performance
func BenchmarkHNSWSearch(b *testing.B) {
	testCases := []struct {
		name        string
		vectorCount int
		topK        int
	}{
		{"SmallIndex_1K_Top10", smallDataset, 10},
		{"MediumIndex_10K_Top10", mediumDataset, 10},
		{"LargeIndex_100K_Top10", largeDataset, 10},
		{"MassiveIndex_1M_Top10", massiveDataset, 10},
		{"LargeIndex_100K_Top50", largeDataset, 50},
		{"LargeIndex_100K_Top100", largeDataset, 100},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			index, vectors := setupHNSWIndex(b, tc.vectorCount)
			ctx := context.Background()

			// Use random vectors from the dataset as queries
			queryVectors := make([][]float32, b.N)
			for i := 0; i < b.N; i++ {
				idx := rand.Intn(len(vectors))
				queryVectors[i] = vectors[idx].Elements
			}

			efSearch := 50
			searchParams := types.SearchParams{
				TopK:     tc.topK,
				EfSearch: &efSearch,
			}

			b.ResetTimer()
			start := time.Now()

			for i := 0; i < b.N; i++ {
				results, err := index.Search(ctx, queryVectors[i], searchParams)
				if err != nil {
					b.Fatalf("Search failed: %v", err)
				}
				if len(results) == 0 {
					b.Fatalf("No search results returned")
				}
			}

			elapsed := time.Since(start)
			avgTime := elapsed / time.Duration(b.N)

			// Check performance target: <50ms per search
			if avgTime > searchTargetNs {
				b.Errorf("Search performance target missed: got %v, target <%v", avgTime, searchTargetNs)
			}

			b.ReportMetric(float64(avgTime.Nanoseconds()), "ns/search")
		})
	}
}

// BenchmarkHNSWBatchInsert tests batch insertion performance
func BenchmarkHNSWBatchInsert(b *testing.B) {
	batchSizes := []int{10, 50, 100, 500}

	for _, batchSize := range batchSizes {
		b.Run(fmt.Sprintf("BatchSize_%d", batchSize), func(b *testing.B) {
			index, _ := setupHNSWIndex(b, smallDataset)
			ctx := context.Background()

			// Generate batches for insertion
			batches := make([][]types.Vector, b.N)
			for i := 0; i < b.N; i++ {
				batches[i] = generateTestVectors(batchSize, testDimensions)
			}

			b.ResetTimer()
			start := time.Now()

			for i := 0; i < b.N; i++ {
				for _, vector := range batches[i] {
					err := index.Insert(ctx, vector)
					if err != nil {
						b.Fatalf("Batch insert failed: %v", err)
					}
				}
			}

			elapsed := time.Since(start)
			avgTime := elapsed / time.Duration(b.N)

			// Check performance target for 100-vector batches: <100ms
			if batchSize == 100 && avgTime > batchInsertTargetNs {
				b.Errorf("Batch insert performance target missed: got %v, target <%v", avgTime, batchInsertTargetNs)
			}

			b.ReportMetric(float64(avgTime.Nanoseconds()), "ns/batch")
			b.ReportMetric(float64(avgTime.Nanoseconds())/float64(batchSize), "ns/vector")
		})
	}
}

// BenchmarkCollectionOperations tests collection-level operations
func BenchmarkCollectionOperations(b *testing.B) {
	ctx := context.Background()

	// Create collection configuration
	config := types.CollectionConfig{
		Name:   "benchmark_collection",
		Metric: types.DistanceMetricL2,
		HNSWParams: types.HNSWParams{
			M:              16,
			EfConstruction: 200,
			EfSearch:       50,
		},
	}

	b.Run("CollectionInsert", func(b *testing.B) {
		coll, err := database.NewCollection("benchmark_collection", config)
		if err != nil {
			b.Fatalf("Failed to create collection: %v", err)
		}

		vectors := generateTestVectors(b.N, testDimensions)

		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			err := coll.Insert(ctx, []types.Vector{vectors[i]})
			if err != nil {
				b.Fatalf("Collection insert failed: %v", err)
			}
		}
	})

	b.Run("CollectionSearch", func(b *testing.B) {
		coll, err := database.NewCollection("benchmark_collection", config)
		if err != nil {
			b.Fatalf("Failed to create collection: %v", err)
		}

		// Pre-populate collection
		vectors := generateTestVectors(10000, testDimensions)
		err = coll.Insert(ctx, vectors)
		if err != nil {
			b.Fatalf("Failed to populate collection: %v", err)
		}

		// Generate query vectors
		queryVectors := make([][]float32, b.N)
		for i := 0; i < b.N; i++ {
			idx := rand.Intn(len(vectors))
			queryVectors[i] = vectors[idx].Elements
		}

		efSearch := 50
		searchParams := types.SearchParams{
			TopK:     10,
			EfSearch: &efSearch,
		}

		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			results, err := coll.Search(ctx, queryVectors[i], searchParams)
			if err != nil {
				b.Fatalf("Collection search failed: %v", err)
			}
			if len(results) == 0 {
				b.Fatalf("No search results returned")
			}
		}
	})
}

// BenchmarkMemoryUsage tests memory efficiency
func BenchmarkMemoryUsage(b *testing.B) {
	vectorCounts := []int{1000, 10000, 100000}

	for _, count := range vectorCounts {
		b.Run(fmt.Sprintf("Vectors_%d", count), func(b *testing.B) {
			index, _ := setupHNSWIndex(b, count)

			memUsage := index.MemoryUsage()
			vectorsSize := int64(count * testDimensions * 4) // 4 bytes per float32

			// Memory overhead should be reasonable (less than 3x vector data size)
			if memUsage > vectorsSize*3 {
				b.Errorf("Memory usage too high: %d bytes for %d vectors (%.2fx vector data)",
					memUsage, count, float64(memUsage)/float64(vectorsSize))
			}

			b.ReportMetric(float64(memUsage), "bytes")
			b.ReportMetric(float64(memUsage)/float64(count), "bytes/vector")
		})
	}
}

// BenchmarkConcurrency tests concurrent operations
func BenchmarkConcurrency(b *testing.B) {
	index, vectors := setupHNSWIndex(b, mediumDataset)
	ctx := context.Background()

	b.Run("ConcurrentReads", func(b *testing.B) {
		efSearch := 50
		searchParams := types.SearchParams{
			TopK:     10,
			EfSearch: &efSearch,
		}

		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				idx := rand.Intn(len(vectors))
				_, err := index.Search(ctx, vectors[idx].Elements, searchParams)
				if err != nil {
					b.Fatalf("Concurrent search failed: %v", err)
				}
			}
		})
	})
}
