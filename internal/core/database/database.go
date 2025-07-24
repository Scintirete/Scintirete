// Package database provides the core database engine implementation for Scintirete.
package database

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/scintirete/scintirete/internal/core"
	"github.com/scintirete/scintirete/internal/persistence/rdb"
	"github.com/scintirete/scintirete/internal/utils"
	"github.com/scintirete/scintirete/pkg/types"
)

// Engine implements the DatabaseEngine interface
type Engine struct {
	mu        sync.RWMutex
	databases map[string]*Database

	// Statistics
	startTime       time.Time
	totalOps        int64
	lastOpTime      time.Time
	totalDuration   time.Duration
	averageDuration time.Duration
}

// NewEngine creates a new database engine
func NewEngine() *Engine {
	return &Engine{
		databases: make(map[string]*Database),
		startTime: time.Now(),
	}
}

// CreateDatabase creates a new database
func (e *Engine) CreateDatabase(ctx context.Context, name string) error {
	startTime := time.Now()
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.databases[name]; exists {
		return utils.ErrDatabaseExists(name)
	}

	db := NewDatabase(name)
	e.databases[name] = db
	e.updateStatsWithDuration(time.Since(startTime))

	return nil
}

// DropDatabase removes a database and all its collections
func (e *Engine) DropDatabase(ctx context.Context, name string) error {
	startTime := time.Now()
	e.mu.Lock()
	defer e.mu.Unlock()

	db, exists := e.databases[name]
	if !exists {
		return utils.ErrDatabaseNotFound(name)
	}

	// Close database and cleanup resources
	if err := db.Close(ctx); err != nil {
		return utils.ErrDatabaseOperationFailed("failed to close database: " + err.Error())
	}

	delete(e.databases, name)
	e.updateStatsWithDuration(time.Since(startTime))

	return nil
}

// GetDatabase retrieves a database by name
func (e *Engine) GetDatabase(ctx context.Context, name string) (core.Database, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	db, exists := e.databases[name]
	if !exists {
		return nil, utils.ErrDatabaseNotFound(name)
	}

	return db, nil
}

// ListDatabases returns a list of all database names
func (e *Engine) ListDatabases(ctx context.Context) ([]string, error) {
	startTime := time.Now()
	e.mu.RLock()
	defer func() {
		e.mu.RUnlock()
		// Update stats after unlock to avoid deadlock
		e.mu.Lock()
		e.updateStatsWithDuration(time.Since(startTime))
		e.mu.Unlock()
	}()

	names := make([]string, 0, len(e.databases))
	for name := range e.databases {
		names = append(names, name)
	}

	return names, nil
}

// GetStats returns engine statistics
func (e *Engine) GetStats(ctx context.Context) (types.DatabaseStats, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var totalVectors, totalCollections int64
	var totalMemory int64

	for _, db := range e.databases {
		dbStats := db.GetStats()
		totalCollections += int64(len(dbStats.Collections))
		for _, collStats := range dbStats.Collections {
			totalVectors += collStats.VectorCount
			totalMemory += collStats.MemoryBytes
		}
	}

	return types.DatabaseStats{
		TotalVectors:     totalVectors,
		TotalCollections: int(totalCollections),
		TotalDatabases:   len(e.databases),
		MemoryUsage:      totalMemory,
		RequestsTotal:    e.totalOps,
		RequestDuration:  float64(e.averageDuration.Nanoseconds()) / 1e6, // Convert to milliseconds
	}, nil
}

// Close shuts down the engine gracefully
func (e *Engine) Close(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	var errors []error
	for name, db := range e.databases {
		if err := db.Close(ctx); err != nil {
			errors = append(errors, fmt.Errorf("failed to close database %s: %w", name, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors closing databases: %v", errors)
	}

	return nil
}

// updateStats updates internal statistics
func (e *Engine) updateStats() {
	e.totalOps++
	e.lastOpTime = time.Now()
}

// updateStatsWithDuration updates internal statistics with request duration
func (e *Engine) updateStatsWithDuration(duration time.Duration) {
	e.totalOps++
	e.lastOpTime = time.Now()
	e.totalDuration += duration
	if e.totalOps > 0 {
		e.averageDuration = e.totalDuration / time.Duration(e.totalOps)
	}
}

// Database represents a single database containing multiple collections
type Database struct {
	mu          sync.RWMutex
	name        string
	collections map[string]*Collection
	createdAt   time.Time
	lastAccess  time.Time
}

// NewDatabase creates a new database
func NewDatabase(name string) *Database {
	now := time.Now()
	return &Database{
		name:        name,
		collections: make(map[string]*Collection),
		createdAt:   now,
		lastAccess:  now,
	}
}

// GetName returns the database name
func (d *Database) GetName() string {
	return d.name
}

// Name returns the database name (interface method)
func (d *Database) Name() string {
	return d.name
}

// CreateCollection creates a new collection in the database
func (d *Database) CreateCollection(ctx context.Context, config types.CollectionConfig) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, exists := d.collections[config.Name]; exists {
		return utils.ErrCollectionExists(config.Name)
	}

	collection, err := NewCollection(config.Name, config)
	if err != nil {
		return utils.ErrCollectionCreationFailed("failed to create collection: " + err.Error())
	}

	d.collections[config.Name] = collection
	d.lastAccess = time.Now()

	return nil
}

// DropCollection removes a collection from the database
func (d *Database) DropCollection(ctx context.Context, name string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	collection, exists := d.collections[name]
	if !exists {
		return utils.ErrCollectionNotFound(d.name, name)
	}

	// Close collection and cleanup resources
	if err := collection.Close(); err != nil {
		return utils.ErrCollectionOperationFailed("failed to close collection: " + err.Error())
	}

	delete(d.collections, name)
	d.lastAccess = time.Now()

	return nil
}

// GetCollection retrieves a collection by name
func (d *Database) GetCollection(ctx context.Context, name string) (core.Collection, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	collection, exists := d.collections[name]
	if !exists {
		return nil, utils.ErrCollectionNotFound(d.name, name)
	}

	d.lastAccess = time.Now()
	return collection, nil
}

// ListCollections returns information about all collections in the database
func (d *Database) ListCollections(ctx context.Context) ([]types.CollectionInfo, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	infos := make([]types.CollectionInfo, 0, len(d.collections))
	for _, collection := range d.collections {
		info := collection.Info()
		infos = append(infos, info)
	}

	return infos, nil
}

// GetCollectionInfo returns metadata about a specific collection
func (d *Database) GetCollectionInfo(ctx context.Context, name string) (types.CollectionInfo, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	collection, exists := d.collections[name]
	if !exists {
		return types.CollectionInfo{}, utils.ErrCollectionNotFound(d.name, name)
	}

	return collection.Info(), nil
}

// GetStats returns database statistics
func (d *Database) GetStats() types.DatabaseInfo {
	d.mu.RLock()
	defer d.mu.RUnlock()

	collections := make(map[string]types.CollectionInfo)
	for name, collection := range d.collections {
		collections[name] = collection.Info()
	}

	return types.DatabaseInfo{
		Name:        d.name,
		Collections: collections,
		CreatedAt:   d.createdAt,
		LastAccess:  d.lastAccess,
	}
}

// Close closes the database and all its collections
func (d *Database) Close(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	var errors []error
	for name, collection := range d.collections {
		if err := collection.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close collection %s: %w", name, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors closing collections: %v", errors)
	}

	return nil
}

// Persistence interface implementation for DatabaseEngine

// GetDatabaseState returns the current state of all databases for snapshotting
func (e *Engine) GetDatabaseState(ctx context.Context) (map[string]rdb.DatabaseState, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	databases := make(map[string]rdb.DatabaseState)

	for name, db := range e.databases {
		dbInfo := db.GetStats()

		// Convert collections to RDB format
		rdbCollections := make(map[string]rdb.CollectionState)
		for collName, collInfo := range dbInfo.Collections {
			// Get collection for vector extraction
			collection, err := db.GetCollection(ctx, collName)
			if err != nil {
				continue // Skip collections that can't be accessed
			}

			// Extract all vectors from collection
			vectors := make([]types.Vector, 0)
			if dbCollection, ok := collection.(*Collection); ok {
				dbCollection.mu.RLock()
				for _, vector := range dbCollection.vectors {
					if !dbCollection.deletedIDs[vector.ID] {
						// Create a copy of the vector
						vectorCopy := types.Vector{
							ID:       vector.ID,
							Elements: make([]float32, len(vector.Elements)),
							Metadata: make(map[string]interface{}),
						}
						copy(vectorCopy.Elements, vector.Elements)
						for k, v := range vector.Metadata {
							vectorCopy.Metadata[k] = v
						}
						vectors = append(vectors, vectorCopy)
					}
				}
				dbCollection.mu.RUnlock()
			}

			rdbCollections[collName] = rdb.CollectionState{
				Name:         collInfo.Name,
				Config:       convertCollectionInfoToConfig(collInfo),
				Vectors:      vectors,
				VectorCount:  collInfo.VectorCount,
				DeletedCount: collInfo.DeletedCount,
				CreatedAt:    collInfo.CreatedAt,
				UpdatedAt:    collInfo.UpdatedAt,
			}
		}

		databases[name] = rdb.DatabaseState{
			Name:        name,
			Collections: rdbCollections,
			CreatedAt:   dbInfo.CreatedAt,
		}
	}

	return databases, nil
}

// RestoreFromSnapshot restores the database state from an RDB snapshot
func (e *Engine) RestoreFromSnapshot(ctx context.Context, snapshot *rdb.RDBSnapshot) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Clear existing databases
	for name, db := range e.databases {
		if err := db.Close(ctx); err != nil {
			// Log error but continue
			fmt.Printf("Warning: failed to close database %s during restore: %v\n", name, err)
		}
		delete(e.databases, name)
	}

	// Restore databases from snapshot
	for dbName, dbSnapshot := range snapshot.Databases {
		// Create new database
		db := NewDatabase(dbName)
		db.createdAt = dbSnapshot.CreatedAt

		// Restore collections
		for collName, collSnapshot := range dbSnapshot.Collections {
			// Create collection with config
			config := collSnapshot.Config
			if err := db.CreateCollection(ctx, config); err != nil {
				return fmt.Errorf("failed to create collection %s in database %s: %w", collName, dbName, err)
			}

			// Get the created collection
			collection, err := db.GetCollection(ctx, collName)
			if err != nil {
				return fmt.Errorf("failed to get collection %s after creation: %w", collName, err)
			}

			// Insert vectors if any exist
			if len(collSnapshot.Vectors) > 0 {
				if err := collection.Insert(ctx, collSnapshot.Vectors); err != nil {
					return fmt.Errorf("failed to insert vectors into collection %s: %w", collName, err)
				}
			}

			// Update collection metadata
			if dbCollection, ok := collection.(*Collection); ok {
				dbCollection.mu.Lock()
				dbCollection.createdAt = collSnapshot.CreatedAt
				dbCollection.updatedAt = collSnapshot.UpdatedAt
				dbCollection.vectorCount = collSnapshot.VectorCount
				dbCollection.deletedCount = collSnapshot.DeletedCount
				dbCollection.mu.Unlock()
			}
		}

		e.databases[dbName] = db
	}

	return nil
}

// ApplyCommand applies an AOF command to the database engine
func (e *Engine) ApplyCommand(ctx context.Context, command types.AOFCommand) error {
	switch command.Command {
	case "CREATE_DATABASE":
		name, ok := command.Args["name"].(string)
		if !ok {
			return fmt.Errorf("invalid database name in CREATE_DATABASE command")
		}
		return e.CreateDatabase(ctx, name)

	case "DROP_DATABASE":
		name, ok := command.Args["name"].(string)
		if !ok {
			return fmt.Errorf("invalid database name in DROP_DATABASE command")
		}
		return e.DropDatabase(ctx, name)

	case "CREATE_COLLECTION":
		dbName := command.Database
		collName, ok := command.Args["name"].(string)
		if !ok {
			return fmt.Errorf("invalid collection name in CREATE_COLLECTION command")
		}

		// Get database
		db, err := e.GetDatabase(ctx, dbName)
		if err != nil {
			return fmt.Errorf("database %s not found for CREATE_COLLECTION: %w", dbName, err)
		}

		// Extract config from args
		configInterface, ok := command.Args["config"]
		if !ok {
			return fmt.Errorf("missing config in CREATE_COLLECTION command")
		}

		config, err := extractCollectionConfig(configInterface, collName)
		if err != nil {
			return fmt.Errorf("invalid config in CREATE_COLLECTION command: %w", err)
		}

		return db.CreateCollection(ctx, config)

	case "DROP_COLLECTION":
		dbName := command.Database
		collName, ok := command.Args["name"].(string)
		if !ok {
			return fmt.Errorf("invalid collection name in DROP_COLLECTION command")
		}

		// Get database
		db, err := e.GetDatabase(ctx, dbName)
		if err != nil {
			return fmt.Errorf("database %s not found for DROP_COLLECTION: %w", dbName, err)
		}

		return db.DropCollection(ctx, collName)

	case "INSERT_VECTORS":
		dbName := command.Database
		collName := command.Collection

		// Get collection
		db, err := e.GetDatabase(ctx, dbName)
		if err != nil {
			return fmt.Errorf("database %s not found for INSERT_VECTORS: %w", dbName, err)
		}

		collection, err := db.GetCollection(ctx, collName)
		if err != nil {
			return fmt.Errorf("collection %s not found for INSERT_VECTORS: %w", collName, err)
		}

		// Extract vectors from args
		vectorsInterface, ok := command.Args["vectors"]
		if !ok {
			return fmt.Errorf("missing vectors in INSERT_VECTORS command")
		}

		vectors, err := extractVectors(vectorsInterface)
		if err != nil {
			return fmt.Errorf("invalid vectors in INSERT_VECTORS command: %w", err)
		}

		return collection.Insert(ctx, vectors)

	case "DELETE_VECTORS":
		dbName := command.Database
		collName := command.Collection

		// Get collection
		db, err := e.GetDatabase(ctx, dbName)
		if err != nil {
			return fmt.Errorf("database %s not found for DELETE_VECTORS: %w", dbName, err)
		}

		collection, err := db.GetCollection(ctx, collName)
		if err != nil {
			return fmt.Errorf("collection %s not found for DELETE_VECTORS: %w", collName, err)
		}

		// Extract IDs from args
		idsInterface, ok := command.Args["ids"]
		if !ok {
			return fmt.Errorf("missing ids in DELETE_VECTORS command")
		}

		ids, err := extractStringSlice(idsInterface)
		if err != nil {
			return fmt.Errorf("invalid ids in DELETE_VECTORS command: %w", err)
		}

		_, err = collection.Delete(ctx, ids)
		return err

	default:
		return fmt.Errorf("unknown command: %s", command.Command)
	}
}

// GetOptimizedCommands returns optimized commands for AOF rewrite
func (e *Engine) GetOptimizedCommands(ctx context.Context) ([]types.AOFCommand, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var commands []types.AOFCommand

	// Generate commands to recreate current state
	for dbName, db := range e.databases {
		// Create database command
		commands = append(commands, types.AOFCommand{
			Timestamp: db.createdAt,
			Command:   "CREATE_DATABASE",
			Args: map[string]interface{}{
				"name": dbName,
			},
			Database: dbName,
		})

		// Create collection commands
		dbInfo := db.GetStats()
		for collName, collInfo := range dbInfo.Collections {
			// Get collection for detailed info
			collection, err := db.GetCollection(ctx, collName)
			if err != nil {
				continue // Skip collections that can't be accessed
			}

			config := convertCollectionInfoToConfig(collInfo)
			commands = append(commands, types.AOFCommand{
				Timestamp: collInfo.CreatedAt,
				Command:   "CREATE_COLLECTION",
				Args: map[string]interface{}{
					"name":   collName,
					"config": config,
				},
				Database:   dbName,
				Collection: collName,
			})

			// Insert vector commands (batch by reasonable size)
			if dbCollection, ok := collection.(*Collection); ok {
				dbCollection.mu.RLock()
				var batchVectors []types.Vector
				const batchSize = 100 // Insert in batches of 100

				for _, vector := range dbCollection.vectors {
					if !dbCollection.deletedIDs[vector.ID] {
						// Create a copy of the vector
						vectorCopy := types.Vector{
							ID:       vector.ID,
							Elements: make([]float32, len(vector.Elements)),
							Metadata: make(map[string]interface{}),
						}
						copy(vectorCopy.Elements, vector.Elements)
						for k, v := range vector.Metadata {
							vectorCopy.Metadata[k] = v
						}
						batchVectors = append(batchVectors, vectorCopy)

						// Insert batch when it reaches batchSize
						if len(batchVectors) >= batchSize {
							commands = append(commands, types.AOFCommand{
								Timestamp: collInfo.UpdatedAt,
								Command:   "INSERT_VECTORS",
								Args: map[string]interface{}{
									"vectors": batchVectors,
								},
								Database:   dbName,
								Collection: collName,
							})
							batchVectors = make([]types.Vector, 0, batchSize)
						}
					}
				}

				// Insert remaining vectors if any
				if len(batchVectors) > 0 {
					commands = append(commands, types.AOFCommand{
						Timestamp: collInfo.UpdatedAt,
						Command:   "INSERT_VECTORS",
						Args: map[string]interface{}{
							"vectors": batchVectors,
						},
						Database:   dbName,
						Collection: collName,
					})
				}

				dbCollection.mu.RUnlock()
			}
		}
	}

	return commands, nil
}

// Helper functions for data conversion

// convertCollectionInfoToConfig converts CollectionInfo to CollectionConfig
func convertCollectionInfoToConfig(info types.CollectionInfo) types.CollectionConfig {
	return types.CollectionConfig{
		Name:       info.Name,
		Metric:     info.MetricType,
		HNSWParams: info.HNSWConfig,
	}
}

// extractCollectionConfig extracts CollectionConfig from interface{}
func extractCollectionConfig(configInterface interface{}, collName string) (types.CollectionConfig, error) {
	switch config := configInterface.(type) {
	case types.CollectionConfig:
		config.Name = collName
		return config, nil
	case map[string]interface{}:
		// Parse from map
		collConfig := types.CollectionConfig{Name: collName}

		if metric, ok := config["metric"].(string); ok {
			collConfig.Metric = parseDistanceMetric(metric)
		} else {
			collConfig.Metric = types.DistanceMetricL2 // default
		}

		// Parse HNSW parameters
		if hnswInterface, ok := config["hnsw_params"]; ok {
			if hnswMap, ok := hnswInterface.(map[string]interface{}); ok {
				hnsw := types.HNSWParams{}
				if m, ok := hnswMap["m"].(int); ok {
					hnsw.M = m
				} else if m, ok := hnswMap["m"].(float64); ok {
					hnsw.M = int(m)
				} else {
					hnsw.M = 16 // default
				}

				if ef, ok := hnswMap["ef_construction"].(int); ok {
					hnsw.EfConstruction = ef
				} else if ef, ok := hnswMap["ef_construction"].(float64); ok {
					hnsw.EfConstruction = int(ef)
				} else {
					hnsw.EfConstruction = 200 // default
				}

				if ef, ok := hnswMap["ef_search"].(int); ok {
					hnsw.EfSearch = ef
				} else if ef, ok := hnswMap["ef_search"].(float64); ok {
					hnsw.EfSearch = int(ef)
				} else {
					hnsw.EfSearch = 50 // default
				}

				collConfig.HNSWParams = hnsw
			}
		} else {
			// Default HNSW parameters
			collConfig.HNSWParams = types.HNSWParams{
				M:              16,
				EfConstruction: 200,
				EfSearch:       50,
			}
		}

		return collConfig, nil
	default:
		return types.CollectionConfig{}, fmt.Errorf("unsupported config type: %T", configInterface)
	}
}

// extractVectors extracts []types.Vector from interface{}
func extractVectors(vectorsInterface interface{}) ([]types.Vector, error) {
	switch vectors := vectorsInterface.(type) {
	case []types.Vector:
		return vectors, nil
	case []interface{}:
		// Convert from []interface{} to []types.Vector
		result := make([]types.Vector, len(vectors))
		for i, vectorInterface := range vectors {
			vector, err := extractSingleVector(vectorInterface)
			if err != nil {
				return nil, fmt.Errorf("invalid vector at index %d: %w", i, err)
			}
			result[i] = vector
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unsupported vectors type: %T", vectorsInterface)
	}
}

// extractSingleVector extracts a single Vector from interface{}
func extractSingleVector(vectorInterface interface{}) (types.Vector, error) {
	switch vector := vectorInterface.(type) {
	case types.Vector:
		return vector, nil
	case map[string]interface{}:
		// Parse from map
		var result types.Vector

		if id, ok := vector["id"].(string); ok {
			// Convert string ID to uint64
			parsedID, err := strconv.ParseUint(id, 10, 64)
			if err != nil {
				return result, fmt.Errorf("invalid id format: %s", id)
			}
			result.ID = parsedID
		} else {
			return result, fmt.Errorf("missing or invalid id field")
		}

		if elementsInterface, ok := vector["elements"]; ok {
			elements, err := extractFloat32Slice(elementsInterface)
			if err != nil {
				return result, fmt.Errorf("invalid elements field: %w", err)
			}
			result.Elements = elements
		} else {
			return result, fmt.Errorf("missing elements field")
		}

		if metadataInterface, ok := vector["metadata"]; ok {
			if metadata, ok := metadataInterface.(map[string]interface{}); ok {
				result.Metadata = metadata
			}
		} else {
			result.Metadata = make(map[string]interface{})
		}

		return result, nil
	default:
		return types.Vector{}, fmt.Errorf("unsupported vector type: %T", vectorInterface)
	}
}

// extractFloat32Slice extracts []float32 from interface{}
func extractFloat32Slice(elementsInterface interface{}) ([]float32, error) {
	switch elements := elementsInterface.(type) {
	case []float32:
		return elements, nil
	case []interface{}:
		result := make([]float32, len(elements))
		for i, elementInterface := range elements {
			switch element := elementInterface.(type) {
			case float32:
				result[i] = element
			case float64:
				result[i] = float32(element)
			case int:
				result[i] = float32(element)
			case int64:
				result[i] = float32(element)
			default:
				return nil, fmt.Errorf("invalid element type at index %d: %T", i, element)
			}
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unsupported elements type: %T", elementsInterface)
	}
}

// extractStringSlice extracts []string from interface{}
func extractStringSlice(idsInterface interface{}) ([]string, error) {
	switch ids := idsInterface.(type) {
	case []string:
		return ids, nil
	case []interface{}:
		result := make([]string, len(ids))
		for i, idInterface := range ids {
			if id, ok := idInterface.(string); ok {
				result[i] = id
			} else {
				return nil, fmt.Errorf("invalid ID type at index %d: %T", i, idInterface)
			}
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unsupported IDs type: %T", idsInterface)
	}
}

// parseDistanceMetric converts a string to DistanceMetric
func parseDistanceMetric(metric string) types.DistanceMetric {
	switch metric {
	case "l2", "L2", "euclidean", "Euclidean":
		return types.DistanceMetricL2
	case "cosine", "Cosine":
		return types.DistanceMetricCosine
	case "inner_product", "InnerProduct", "dot", "Dot":
		return types.DistanceMetricInnerProduct
	default:
		return types.DistanceMetricL2 // default to L2
	}
}
