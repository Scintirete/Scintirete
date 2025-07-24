// Package database provides collection management functionality.
package database

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/scintirete/scintirete/internal/core"
	"github.com/scintirete/scintirete/internal/core/algorithm"
	"github.com/scintirete/scintirete/internal/utils"
	"github.com/scintirete/scintirete/pkg/types"
)

// Collection represents a collection of vectors with an associated index
type Collection struct {
	mu         sync.RWMutex
	name       string
	config     types.CollectionConfig
	vectors    map[uint64]*types.Vector // vector ID -> vector
	deletedIDs map[uint64]bool          // soft deletion tracking
	index      core.VectorIndex
	createdAt  time.Time
	updatedAt  time.Time

	// Statistics
	vectorCount  int64
	deletedCount int64
	memoryBytes  int64
}

// NewCollection creates a new collection with the specified configuration
func NewCollection(name string, config types.CollectionConfig) (*Collection, error) {
	// Validate configuration
	if err := validateCollectionConfig(config); err != nil {
		return nil, err
	}

	// Set name in config if not set
	if config.Name == "" {
		config.Name = name
	}

	now := time.Now()
	collection := &Collection{
		name:       name,
		config:     config,
		vectors:    make(map[uint64]*types.Vector),
		deletedIDs: make(map[uint64]bool),
		createdAt:  now,
		updatedAt:  now,
	}

	// Create index based on configuration
	index, err := algorithm.NewHNSW(config.HNSWParams, config.Metric)
	if err != nil {
		return nil, utils.ErrInvalidInput("failed to create HNSW index: " + err.Error())
	}

	collection.index = index
	return collection, nil
}

// Insert adds vectors to the collection
func (c *Collection) Insert(ctx context.Context, vectors []types.Vector) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(vectors) == 0 {
		return utils.ErrInvalidInput("no vectors provided")
	}

	// Validate dimensions
	if c.vectorCount > 0 {
		// Existing collection - check dimensions match
		expectedDim := c.config.HNSWParams.EfConstruction // This should be dimension, but let's check first vector
		if len(c.vectors) > 0 {
			for _, existingVector := range c.vectors {
				expectedDim = len(existingVector.Elements)
				break
			}
		}

		for i, vector := range vectors {
			if len(vector.Elements) != expectedDim {
				return utils.ErrInvalidVectorDimension(fmt.Sprintf("vector[%d] has dimension %d, expected %d",
					i, len(vector.Elements), expectedDim))
			}

			if vector.ID == 0 {
				return utils.ErrInvalidInput(fmt.Sprintf("vector[%d] has empty ID", i))
			}
		}
	} else {
		// For first insertion, just validate that all vectors have the same dimension and non-empty IDs
		if len(vectors) > 0 {
			firstDim := len(vectors[0].Elements)
			for i, vector := range vectors {
				if len(vector.Elements) != firstDim {
					return utils.ErrInvalidVectorDimension(fmt.Sprintf("vector[%d] has dimension %d, expected %d (from first vector)",
						i, len(vector.Elements), firstDim))
				}

				if vector.ID == 0 {
					return utils.ErrInvalidInput(fmt.Sprintf("vector[%d] has empty ID", i))
				}
			}
		}
	}

	// Insert vectors
	var insertedCount int64
	for _, vector := range vectors {
		// Create a copy to avoid mutation
		vectorCopy := types.Vector{
			ID:       vector.ID,
			Elements: make([]float32, len(vector.Elements)),
			Metadata: make(map[string]interface{}),
		}
		copy(vectorCopy.Elements, vector.Elements)
		for k, v := range vector.Metadata {
			vectorCopy.Metadata[k] = v
		}

		// Check if vector already exists
		if existingVector, exists := c.vectors[vector.ID]; exists {
			// Update existing vector
			*existingVector = vectorCopy
			if c.deletedIDs[vector.ID] {
				// Un-delete if it was soft deleted
				delete(c.deletedIDs, vector.ID)
				c.deletedCount--
			}
		} else {
			// Insert new vector
			c.vectors[vector.ID] = &vectorCopy
			insertedCount++
		}

		// Add to index if not deleted
		if !c.deletedIDs[vector.ID] {
			if c.index != nil {
				if err := c.index.Insert(ctx, vectorCopy); err != nil {
					return utils.ErrIndexOperationFailed("failed to insert into index: " + err.Error())
				}
			}
		}
	}

	c.vectorCount += insertedCount
	c.updatedAt = time.Now()
	c.updateMemoryUsage()

	return nil
}

// Delete marks vectors as deleted by their IDs
func (c *Collection) Delete(ctx context.Context, ids []string) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(ids) == 0 {
		return 0, utils.ErrInvalidInput("no IDs provided")
	}

	var deletedCount int
	for _, idStr := range ids {
		// Convert string ID to uint64
		id, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			continue // Skip invalid IDs
		}

		if _, exists := c.vectors[id]; !exists {
			continue // Skip non-existent vectors
		}

		if !c.deletedIDs[id] {
			c.deletedIDs[id] = true
			c.deletedCount++
			deletedCount++

			// Remove from index
			if c.index != nil {
				if err := c.index.Delete(ctx, idStr); err != nil {
					return deletedCount, utils.ErrIndexOperationFailed("failed to delete from index: " + err.Error())
				}
			}
		}
	}

	c.updatedAt = time.Now()
	c.updateMemoryUsage()

	return deletedCount, nil
}

// Search finds the most similar vectors to the query
func (c *Collection) Search(ctx context.Context, query []float32, params types.SearchParams) ([]types.SearchResult, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.index == nil {
		return nil, utils.ErrInvalidInput("index not initialized")
	}

	// Perform search using the index
	results, err := c.index.Search(ctx, query, params)
	if err != nil {
		return nil, err
	}

	// Filter out deleted vectors
	var filteredResults []types.SearchResult
	for _, result := range results {
		if !c.deletedIDs[result.Vector.ID] {
			filteredResults = append(filteredResults, result)
		}
	}

	return filteredResults, nil
}

// Get retrieves a vector by ID
func (c *Collection) Get(ctx context.Context, id string) (*types.Vector, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Convert string ID to uint64
	vectorID, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return nil, utils.ErrInvalidParameters(fmt.Sprintf("invalid ID format: %s", id))
	}

	vector, exists := c.vectors[vectorID]
	if !exists {
		return nil, utils.ErrVectorNotFound(id)
	}

	if c.deletedIDs[vectorID] {
		return nil, utils.ErrVectorNotFound(id)
	}

	// Return a copy to prevent external mutation
	vectorCopy := types.Vector{
		ID:       vector.ID,
		Elements: make([]float32, len(vector.Elements)),
		Metadata: make(map[string]interface{}),
	}
	copy(vectorCopy.Elements, vector.Elements)
	for k, v := range vector.Metadata {
		vectorCopy.Metadata[k] = v
	}

	return &vectorCopy, nil
}

// Count returns the total number of vectors in the collection (excluding deleted)
func (c *Collection) Count(ctx context.Context) (int64, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.vectorCount - c.deletedCount, nil
}

// GetMultiple retrieves multiple vectors by their IDs
func (c *Collection) GetMultiple(ctx context.Context, ids []string) ([]types.Vector, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var results []types.Vector
	for _, idStr := range ids {
		// Convert string ID to uint64
		id, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			continue // Skip invalid IDs
		}

		vector, exists := c.vectors[id]
		if !exists || c.deletedIDs[id] {
			continue // Skip deleted or non-existent vectors
		}

		// Return a copy to prevent external mutation
		vectorCopy := types.Vector{
			ID:       vector.ID,
			Elements: make([]float32, len(vector.Elements)),
			Metadata: make(map[string]interface{}),
		}
		copy(vectorCopy.Elements, vector.Elements)
		for k, v := range vector.Metadata {
			vectorCopy.Metadata[k] = v
		}
		results = append(results, vectorCopy)
	}

	return results, nil
}

// Compact removes deleted vectors and rebuilds the index
func (c *Collection) Compact(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Remove deleted vectors from memory
	for id := range c.deletedIDs {
		delete(c.vectors, id)
	}

	// Clear deleted IDs
	c.deletedIDs = make(map[uint64]bool)
	c.deletedCount = 0
	c.vectorCount = int64(len(c.vectors))

	// Rebuild index if it exists
	if c.index != nil {
		vectors := make([]types.Vector, 0, len(c.vectors))
		for _, vector := range c.vectors {
			vectors = append(vectors, *vector)
		}

		if err := c.index.Build(ctx, vectors); err != nil {
			return utils.ErrIndexOperationFailed("failed to rebuild index: " + err.Error())
		}
	}

	c.updatedAt = time.Now()
	c.updateMemoryUsage()

	return nil
}

// Close releases resources used by the collection
func (c *Collection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Clear data structures
	c.vectors = nil
	c.deletedIDs = nil
	c.index = nil

	return nil
}

// Info returns metadata about this collection
func (c *Collection) Info() types.CollectionInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var dimension int
	if len(c.vectors) > 0 {
		for _, vector := range c.vectors {
			dimension = len(vector.Elements)
			break
		}
	}

	return types.CollectionInfo{
		Name:         c.name,
		Dimension:    dimension,
		VectorCount:  c.vectorCount - c.deletedCount,
		DeletedCount: c.deletedCount,
		MemoryBytes:  c.memoryBytes,
		MetricType:   c.config.Metric,
		HNSWConfig:   c.config.HNSWParams,
		CreatedAt:    c.createdAt,
		UpdatedAt:    c.updatedAt,
	}
}

// updateMemoryUsage calculates and updates the memory usage estimate
func (c *Collection) updateMemoryUsage() {
	// Rough estimation of memory usage
	var totalBytes int64

	// Vector data
	for _, vector := range c.vectors {
		totalBytes += 8                                // ID (uint64 = 8 bytes)
		totalBytes += int64(len(vector.Elements) * 4)  // float32 elements
		totalBytes += int64(len(vector.Metadata) * 32) // rough metadata size
	}

	// Deleted IDs map
	for range c.deletedIDs {
		totalBytes += 8 // uint64 = 8 bytes
	}

	// Index memory (rough estimation)
	if c.index != nil {
		// HNSW typically uses 4-8 bytes per vector per connection
		avgConnections := c.config.HNSWParams.M * 2 // rough estimate
		totalBytes += c.vectorCount * int64(avgConnections) * 8
	}

	c.memoryBytes = totalBytes
}

// validateCollectionConfig validates the collection configuration
func validateCollectionConfig(config types.CollectionConfig) error {
	if config.Name == "" {
		return utils.ErrInvalidInput("collection name cannot be empty")
	}

	if config.Metric == types.DistanceMetricUnspecified {
		return utils.ErrInvalidInput("distance metric must be specified")
	}

	// Validate HNSW parameters
	if config.HNSWParams.M <= 0 {
		return utils.ErrInvalidInput("HNSW M parameter must be positive")
	}

	if config.HNSWParams.EfConstruction <= 0 {
		return utils.ErrInvalidInput("HNSW EfConstruction parameter must be positive")
	}

	return nil
}
