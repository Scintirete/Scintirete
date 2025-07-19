// Package database provides the core database engine implementation for Scintirete.
package database

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/scintirete/scintirete/internal/core"
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
