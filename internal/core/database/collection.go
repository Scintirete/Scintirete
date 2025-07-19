// Package database provides collection management functionality.
package database

import (
	"context"
	"fmt"
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
	vectors    map[string]*types.Vector // vector ID -> vector
	deletedIDs map[string]bool          // soft deletion tracking
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
		vectors:    make(map[string]*types.Vector),
		deletedIDs: make(map[string]bool),
		createdAt:  now,
		updatedAt:  now,
	}

	return collection, nil
}

// GetName returns the collection name
func (c *Collection) GetName() string {
	return c.name
}

// GetConfig returns the collection configuration
func (c *Collection) GetConfig() types.CollectionConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.config
}

// Insert adds vectors to the collection
func (c *Collection) Insert(ctx context.Context, vectors []types.Vector) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(vectors) == 0 {
		return utils.ErrInvalidInput("no vectors provided")
	}

	// Initialize index on first insertion
	if len(c.vectors) == 0 && len(vectors) > 0 {
		dimension := len(vectors[0].Elements)
		if err := c.initializeIndex(dimension); err != nil {
			return utils.ErrCollectionOperationFailed("failed to initialize index: " + err.Error())
		}
	}

	// Validate vector dimensions (only if we have existing vectors or this is not the first insertion)
	expectedDim := c.getDimension()
	if expectedDim > 0 {
		for i, vector := range vectors {
			if len(vector.Elements) != expectedDim {
				return utils.ErrInvalidVectorDimension(fmt.Sprintf("vector[%d] has dimension %d, expected %d",
					i, len(vector.Elements), expectedDim))
			}

			if vector.ID == "" {
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

				if vector.ID == "" {
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

// Delete marks vectors as deleted (soft deletion)
func (c *Collection) Delete(ctx context.Context, ids []string) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(ids) == 0 {
		return 0, utils.ErrInvalidInput("no IDs provided")
	}

	var deletedCount int
	for _, id := range ids {
		if _, exists := c.vectors[id]; !exists {
			continue // Skip non-existent vectors
		}

		if !c.deletedIDs[id] {
			c.deletedIDs[id] = true
			c.deletedCount++
			deletedCount++

			// Remove from index
			if c.index != nil {
				if err := c.index.Delete(ctx, id); err != nil {
					return deletedCount, utils.ErrIndexOperationFailed("failed to delete from index: " + err.Error())
				}
			}
		}
	}

	if deletedCount > 0 {
		c.updatedAt = time.Now()
		c.updateMemoryUsage()
	}

	return deletedCount, nil
}

// Search performs vector similarity search
func (c *Collection) Search(ctx context.Context, queryVector []float32, params types.SearchParams) ([]types.SearchResult, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.index == nil {
		return nil, utils.ErrCollectionEmpty("collection has no index")
	}

	// Validate query vector dimension
	expectedDim := c.getDimension()
	if len(queryVector) != expectedDim {
		return nil, utils.ErrInvalidVectorDimension(fmt.Sprintf("query vector has dimension %d, expected %d",
			len(queryVector), expectedDim))
	}

	// Perform index search
	indexResults, err := c.index.Search(ctx, queryVector, params)
	if err != nil {
		return nil, utils.ErrSearchFailed("index search failed: " + err.Error())
	}

	// The index already returns the correct format
	results := make([]types.SearchResult, 0, len(indexResults))
	for _, result := range indexResults {
		// Check if vector is deleted
		if c.deletedIDs[result.Vector.ID] {
			continue // Skip deleted vectors
		}
		results = append(results, result)
	}

	return results, nil
}

// Get retrieves a vector by ID
func (c *Collection) Get(ctx context.Context, id string) (*types.Vector, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	vector, exists := c.vectors[id]
	if !exists {
		return nil, utils.ErrVectorNotFound(id)
	}

	if c.deletedIDs[id] {
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

// Info returns collection metadata
func (c *Collection) Info() types.CollectionInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var dimension int
	if len(c.vectors) > 0 {
		// Get dimension from first vector
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

// Compact removes soft-deleted vectors permanently
func (c *Collection) Compact(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.deletedCount == 0 {
		return nil // Nothing to compact
	}

	// Remove deleted vectors from storage
	for id := range c.deletedIDs {
		delete(c.vectors, id)
	}

	// Clear deleted IDs
	c.deletedIDs = make(map[string]bool)
	c.deletedCount = 0
	c.vectorCount = int64(len(c.vectors))
	c.updatedAt = time.Now()
	c.updateMemoryUsage()

	// TODO: Rebuild index if needed
	// This is a complex operation that might require recreating the entire index

	return nil
}

// Close closes the collection and cleans up resources
func (c *Collection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Close index if it exists
	if c.index != nil {
		if closer, ok := c.index.(interface{ Close() error }); ok {
			if err := closer.Close(); err != nil {
				return fmt.Errorf("failed to close index: %w", err)
			}
		}
	}

	// Clear data to help with GC
	c.vectors = nil
	c.deletedIDs = nil
	c.index = nil

	return nil
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
	for _, id := range ids {
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

// Private methods

// initializeIndex creates and initializes the vector index
func (c *Collection) initializeIndex(dimension int) error {
	if c.index != nil {
		return nil // Already initialized
	}

	// Create HNSW index
	hnswIndex, err := algorithm.NewHNSW(c.config.HNSWParams, c.config.Metric)
	if err != nil {
		return err
	}
	c.index = hnswIndex

	return nil
}

// getDimension returns the expected vector dimension
func (c *Collection) getDimension() int {
	if len(c.vectors) == 0 {
		return 0
	}

	// Get dimension from first vector
	for _, vector := range c.vectors {
		return len(vector.Elements)
	}

	return 0
}

// updateMemoryUsage calculates and updates memory usage statistics
func (c *Collection) updateMemoryUsage() {
	// Rough estimation of memory usage
	var totalBytes int64

	// Vector data
	for _, vector := range c.vectors {
		totalBytes += int64(len(vector.ID))            // ID string
		totalBytes += int64(len(vector.Elements) * 4)  // float32 elements
		totalBytes += int64(len(vector.Metadata) * 32) // rough metadata size
	}

	// Deleted IDs map
	for id := range c.deletedIDs {
		totalBytes += int64(len(id))
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

	switch config.Metric {
	case types.DistanceMetricL2, types.DistanceMetricCosine, types.DistanceMetricInnerProduct:
		// Valid metrics
	default:
		return utils.ErrInvalidInput("invalid distance metric")
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
