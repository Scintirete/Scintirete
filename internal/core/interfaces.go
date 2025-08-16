// Package core defines the core interfaces and abstractions for Scintirete.
package core

import (
	"context"

	"github.com/scintirete/scintirete/pkg/types"
)

// DatabaseEngine is the top-level interface for the vector database engine.
// It manages multiple databases and provides lifecycle management.
type DatabaseEngine interface {
	// CreateDatabase creates a new database with the specified name.
	CreateDatabase(ctx context.Context, name string) error

	// DropDatabase removes a database and all its collections.
	DropDatabase(ctx context.Context, name string) error

	// ListDatabases returns a list of all database names.
	ListDatabases(ctx context.Context) ([]string, error)

	// GetDatabase retrieves a database by name.
	GetDatabase(ctx context.Context, name string) (Database, error)

	// Start initializes the database engine and starts background services.
	Start(ctx context.Context) error

	// Stop gracefully shuts down the database engine.
	Stop(ctx context.Context) error

	// Health returns the current health status of the engine.
	Health(ctx context.Context) error
}

// Database represents a single database containing multiple collections.
type Database interface {
	// Name returns the database name.
	Name() string

	// CreateCollection creates a new collection with the specified configuration.
	CreateCollection(ctx context.Context, config types.CollectionConfig) error

	// DropCollection removes a collection and all its data.
	DropCollection(ctx context.Context, name string) error

	// ListCollections returns information about all collections in this database.
	ListCollections(ctx context.Context) ([]types.CollectionInfo, error)

	// GetCollection retrieves a collection by name.
	GetCollection(ctx context.Context, name string) (Collection, error)

	// GetCollectionInfo returns metadata about a specific collection.
	GetCollectionInfo(ctx context.Context, name string) (types.CollectionInfo, error)
}

// Collection represents a single collection of vectors with the same dimension.
type Collection interface {
	// Info returns metadata about this collection.
	Info() types.CollectionInfo

	// Insert adds vectors to the collection. All vectors must have the same dimension.
	Insert(ctx context.Context, vectors []types.Vector) error

	// Delete marks vectors as deleted by their IDs. Returns the number of vectors deleted.
	Delete(ctx context.Context, ids []string) (int, error)

	// Search finds the most similar vectors to the query vector.
	Search(ctx context.Context, query []float32, params types.SearchParams) ([]types.SearchResult, error)

	// Get retrieves a specific vector by ID.
	Get(ctx context.Context, id string) (*types.Vector, error)

	// GetMultiple retrieves multiple vectors by their IDs.
	GetMultiple(ctx context.Context, ids []string) ([]types.Vector, error)

	// Count returns the total number of vectors in the collection (excluding deleted).
	Count(ctx context.Context) (int64, error)

	// Compact removes deleted vectors and rebuilds the index for better performance.
	Compact(ctx context.Context) error

	// Close closes the collection and releases resources.
	Close() error
}

// VectorIndex represents a vector indexing algorithm (e.g., HNSW).
type VectorIndex interface {
	// Build constructs the index from the given vectors.
	Build(ctx context.Context, vectors []types.Vector) error

	// Insert adds a single vector to the index.
	Insert(ctx context.Context, vector types.Vector) error

	// Delete marks a vector as deleted by ID.
	Delete(ctx context.Context, id string) error

	// Search finds the most similar vectors to the query.
	Search(ctx context.Context, query []float32, params types.SearchParams) ([]types.SearchResult, error)

	// Get retrieves a vector by ID.
	Get(ctx context.Context, id string) (*types.Vector, error)

	// Size returns the number of vectors in the index.
	Size() int

	// MemoryUsage returns the memory usage in bytes.
	MemoryUsage() int64

	// GetStatistics returns index-specific statistics.
	GetStatistics() interface{}
}

// HNSWIndex extends VectorIndex with HNSW-specific functionality.
type HNSWIndex interface {
	VectorIndex

	// GetParameters returns the HNSW configuration parameters.
	GetParameters() types.HNSWParams

	// GetLayers returns the number of layers in the HNSW graph.
	GetLayers() int

	// GetGraphStatistics returns detailed graph statistics.
	GetGraphStatistics() types.GraphStats

	// SetEfSearch dynamically updates the ef_search parameter for queries.
	SetEfSearch(efSearch int)

	// ExportGraphState exports the current graph structure for persistence.
	ExportGraphState() HNSWGraphState

	// ImportGraphState imports graph structure from persistence, avoiding rebuild.
	ImportGraphState(state HNSWGraphState) error
}

// HNSWGraphState represents the internal state of an HNSW graph for serialization.
type HNSWGraphState struct {
	Nodes      map[uint64]*HNSWNodeState // All nodes indexed by ID
	EntryPoint uint64                    // ID of the entry point node
	MaxLayer   int                       // Current maximum layer
	Size       int                       // Number of active nodes
}

// HNSWNodeState represents the internal state of an HNSW node for serialization.
type HNSWNodeState struct {
	ID          uint64                 // Vector ID
	Vector      []float32              // Vector data
	Metadata    map[string]interface{} // Associated metadata
	Deleted     bool                   // Soft delete flag
	Connections [][]uint64             // Connections at each layer (optimized from map)
}

// DistanceCalculator defines the interface for distance/similarity calculations.
type DistanceCalculator interface {
	// Distance calculates the distance between two vectors.
	Distance(a, b []float32) float32

	// DistanceType returns the type of distance metric used.
	DistanceType() types.DistanceMetric

	// IsSimilarity returns true if higher values indicate higher similarity.
	IsSimilarity() bool
}

// Persistence handles data persistence, recovery, and durability.
type Persistence interface {
	// WriteAOF writes a command to the append-only file.
	WriteAOF(ctx context.Context, command types.AOFCommand) error

	// LoadFromRDB loads data from the latest RDB snapshot.
	LoadFromRDB(ctx context.Context) error

	// SaveRDB creates a new RDB snapshot.
	SaveRDB(ctx context.Context) error

	// Recover replays the AOF log to restore the latest state.
	Recover(ctx context.Context) error

	// StartBackgroundTasks starts periodic snapshot and AOF rewrite tasks.
	StartBackgroundTasks(ctx context.Context) error

	// Stop gracefully stops all persistence operations.
	Stop(ctx context.Context) error
}

// IndexFactory creates vector indexes based on configuration.
type IndexFactory interface {
	// CreateIndex creates a new vector index with the specified configuration.
	CreateIndex(config types.IndexConfig) (VectorIndex, error)

	// SupportedMetrics returns the distance metrics supported by this factory.
	SupportedMetrics() []types.DistanceMetric

	// DefaultParameters returns default parameters for the index type.
	DefaultParameters() map[string]interface{}
}

// EmbeddingClient handles communication with external embedding services.
type EmbeddingClient interface {
	// Embed converts text to vectors using the specified model.
	Embed(ctx context.Context, texts []string, model string) ([][]float32, error)

	// EmbedSingle converts a single text to a vector.
	EmbedSingle(ctx context.Context, text string, model string) ([]float32, error)

	// GetSupportedModels returns a list of supported embedding models.
	GetSupportedModels(ctx context.Context) ([]string, error)

	// GetDefaultModel returns the default embedding model.
	GetDefaultModel() string
}

// AuthService handles authentication and authorization.
type AuthService interface {
	// Authenticate validates the provided credentials.
	Authenticate(ctx context.Context, password string) error

	// IsAuthorized checks if a user has permission for a specific operation.
	IsAuthorized(ctx context.Context, userID, operation, resource string) error
}

// MetricsCollector collects and provides system metrics.
type MetricsCollector interface {
	// RecordRequest records a request with its duration and status.
	RecordRequest(operation string, duration float64, success bool)

	// RecordVectorOperation records vector operations (insert, delete, search).
	RecordVectorOperation(operation string, count int64, duration float64)

	// UpdateGaugeMetrics updates gauge metrics like memory usage, vector count.
	UpdateGaugeMetrics(snapshot types.MetricsSnapshot)

	// GetMetrics returns current metrics in Prometheus format.
	GetMetrics() ([]byte, error)
}

// Logger provides structured logging functionality.
type Logger interface {
	// Debug logs debug-level messages.
	Debug(ctx context.Context, message string, fields map[string]interface{})

	// Info logs info-level messages.
	Info(ctx context.Context, message string, fields map[string]interface{})

	// Warn logs warning-level messages.
	Warn(ctx context.Context, message string, fields map[string]interface{})

	// Error logs error-level messages.
	Error(ctx context.Context, message string, err error, fields map[string]interface{})

	// WithFields returns a logger with additional fields.
	WithFields(fields map[string]interface{}) Logger
}

// AuditLogger logs audit events for compliance and debugging.
type AuditLogger interface {
	// LogOperation logs database operations for audit purposes.
	LogOperation(ctx context.Context, operation, database, collection, userID string, metadata map[string]interface{})

	// LogAccess logs access attempts (successful and failed).
	LogAccess(ctx context.Context, userID, operation, resource string, success bool, metadata map[string]interface{})
}

// RateLimiter controls the rate of requests to prevent abuse.
type RateLimiter interface {
	// Allow checks if a request should be allowed based on the rate limit.
	Allow(ctx context.Context, key string) error

	// GetLimit returns the current rate limit for a key.
	GetLimit(ctx context.Context, key string) (requests int, window int, err error)

	// SetLimit updates the rate limit for a key.
	SetLimit(ctx context.Context, key string, requests int, window int) error
}
