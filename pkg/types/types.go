// Package types defines the core types used throughout the Scintirete project.
package types

import (
	"context"
	"time"

	pb "github.com/scintirete/scintirete/gen/go/scintirete/v1"
)

// DistanceMetric represents the type of distance calculation
type DistanceMetric int32

const (
	DistanceMetricUnspecified  DistanceMetric = 0
	DistanceMetricL2           DistanceMetric = 1 // Euclidean distance
	DistanceMetricCosine       DistanceMetric = 2 // Cosine similarity
	DistanceMetricInnerProduct DistanceMetric = 3 // Inner product
)

// String returns the string representation of DistanceMetric
func (dm DistanceMetric) String() string {
	switch dm {
	case DistanceMetricL2:
		return "L2"
	case DistanceMetricCosine:
		return "Cosine"
	case DistanceMetricInnerProduct:
		return "InnerProduct"
	default:
		return "Unspecified"
	}
}

// ToProto converts DistanceMetric to protobuf enum
func (dm DistanceMetric) ToProto() pb.DistanceMetric {
	switch dm {
	case DistanceMetricL2:
		return pb.DistanceMetric_L2
	case DistanceMetricCosine:
		return pb.DistanceMetric_COSINE
	case DistanceMetricInnerProduct:
		return pb.DistanceMetric_INNER_PRODUCT
	default:
		return pb.DistanceMetric_DISTANCE_METRIC_UNSPECIFIED
	}
}

// DistanceMetricFromProto converts protobuf enum to DistanceMetric
func DistanceMetricFromProto(pbMetric pb.DistanceMetric) DistanceMetric {
	switch pbMetric {
	case pb.DistanceMetric_L2:
		return DistanceMetricL2
	case pb.DistanceMetric_COSINE:
		return DistanceMetricCosine
	case pb.DistanceMetric_INNER_PRODUCT:
		return DistanceMetricInnerProduct
	default:
		return DistanceMetricUnspecified
	}
}

// Vector represents a vector with ID, elements, and metadata
type Vector struct {
	ID       string                 `json:"id"`
	Elements []float32              `json:"elements"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Dimension returns the dimension of the vector
func (v *Vector) Dimension() int {
	return len(v.Elements)
}

// TextWithMetadata represents text data with metadata for embedding
type TextWithMetadata struct {
	ID       string                 `json:"id"`
	Text     string                 `json:"text"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// SearchResult represents a single search result
type SearchResult struct {
	Vector   Vector  `json:"vector"`
	Distance float32 `json:"distance"`
}

// SearchParams contains parameters for vector search
type SearchParams struct {
	TopK     int  `json:"top_k"`
	EfSearch *int `json:"ef_search,omitempty"` // HNSW-specific parameter
}

// HNSWParams contains HNSW algorithm parameters
type HNSWParams struct {
	M              int   `json:"m"`               // Maximum connections per node
	EfConstruction int   `json:"ef_construction"` // Search scope during construction
	EfSearch       int   `json:"ef_search"`       // Search scope during query
	MaxLayers      int   `json:"max_layers"`      // Maximum number of layers
	Seed           int64 `json:"seed"`            // Random seed for reproducibility
}

// DefaultHNSWParams returns default HNSW parameters
func DefaultHNSWParams() HNSWParams {
	return HNSWParams{
		M:              16,
		EfConstruction: 200,
		EfSearch:       50,
		MaxLayers:      16,
		Seed:           time.Now().UnixNano(),
	}
}

// ToProto converts HNSWParams to protobuf message
func (p HNSWParams) ToProto() *pb.HnswConfig {
	return &pb.HnswConfig{
		M:              int32(p.M),
		EfConstruction: int32(p.EfConstruction),
	}
}

// HNSWParamsFromProto converts protobuf message to HNSWParams
func HNSWParamsFromProto(pbConfig *pb.HnswConfig) HNSWParams {
	params := DefaultHNSWParams()
	if pbConfig != nil {
		if pbConfig.M > 0 {
			params.M = int(pbConfig.M)
		}
		if pbConfig.EfConstruction > 0 {
			params.EfConstruction = int(pbConfig.EfConstruction)
		}
	}
	return params
}

// CollectionConfig contains configuration for creating a collection
type CollectionConfig struct {
	Name       string         `json:"name"`
	Metric     DistanceMetric `json:"metric"`
	HNSWParams HNSWParams     `json:"hnsw_params"`
}

// CollectionInfo contains metadata about a collection
type CollectionInfo struct {
	Name         string         `json:"name"`
	Dimension    int            `json:"dimension"`
	VectorCount  int64          `json:"vector_count"`
	DeletedCount int64          `json:"deleted_count"`
	MemoryBytes  int64          `json:"memory_bytes"`
	MetricType   DistanceMetric `json:"metric_type"`
	HNSWConfig   HNSWParams     `json:"hnsw_config"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

// ToProto converts CollectionInfo to protobuf message
func (info CollectionInfo) ToProto() *pb.CollectionInfo {
	return &pb.CollectionInfo{
		Name:         info.Name,
		Dimension:    int32(info.Dimension),
		VectorCount:  info.VectorCount,
		DeletedCount: info.DeletedCount,
		MemoryBytes:  info.MemoryBytes,
		MetricType:   info.MetricType.ToProto(),
		HnswConfig:   info.HNSWConfig.ToProto(),
	}
}

// GraphStats contains statistics about the HNSW graph
type GraphStats struct {
	Layers      int     `json:"layers"`
	Nodes       int     `json:"nodes"`
	Connections int     `json:"connections"`
	AvgDegree   float64 `json:"avg_degree"`
	MaxDegree   int     `json:"max_degree"`
	MemoryUsage int64   `json:"memory_usage"`
}

// IndexConfig contains configuration for creating an index
type IndexConfig struct {
	Type       string                 `json:"type"` // "hnsw", etc.
	Metric     DistanceMetric         `json:"metric"`
	Parameters map[string]interface{} `json:"parameters"`
}

// AOFCommand represents a command to be logged in AOF
type AOFCommand struct {
	Timestamp  time.Time              `json:"timestamp"`
	Command    string                 `json:"command"`
	Args       map[string]interface{} `json:"args"`
	Database   string                 `json:"database,omitempty"`
	Collection string                 `json:"collection,omitempty"`
}

// EmbeddingRequest represents a request to embedding service
type EmbeddingRequest struct {
	Texts []string `json:"input"`
	Model string   `json:"model"`
}

// EmbeddingResponse represents a response from embedding service
type EmbeddingResponse struct {
	Data  []EmbeddingData `json:"data"`
	Usage EmbeddingUsage  `json:"usage"`
}

// EmbeddingData represents a single embedding result
type EmbeddingData struct {
	Index     int       `json:"index"`
	Embedding []float32 `json:"embedding"`
}

// EmbeddingUsage represents token usage information
type EmbeddingUsage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// DatabaseInfo contains information about a single database
type DatabaseInfo struct {
	Name        string                    `json:"name"`
	Collections map[string]CollectionInfo `json:"collections"`
	CreatedAt   time.Time                 `json:"created_at"`
	LastAccess  time.Time                 `json:"last_access"`
}

// DatabaseStats contains overall database statistics
type DatabaseStats struct {
	// Overall statistics
	TotalVectors     int64 `json:"total_vectors"`
	TotalCollections int   `json:"total_collections"`
	TotalDatabases   int   `json:"total_databases"`
	MemoryUsage      int64 `json:"memory_usage_bytes"`

	// Request metrics
	RequestsTotal   int64   `json:"requests_total"`
	RequestDuration float64 `json:"request_duration_avg_ms"`
	ErrorsTotal     int64   `json:"errors_total"`

	// Search metrics
	SearchLatency    float64 `json:"search_latency_avg_ms"`
	SearchThroughput float64 `json:"search_throughput_qps"`

	// Insert metrics
	InsertLatency    float64 `json:"insert_latency_avg_ms"`
	InsertThroughput float64 `json:"insert_throughput_ops"`
}

// RequestContext contains information about the current request
type RequestContext struct {
	Context   context.Context
	RequestID string
	UserID    string
	StartTime time.Time
}

// MetricsSnapshot contains current metrics snapshot
type MetricsSnapshot struct {
	Timestamp        time.Time `json:"timestamp"`
	TotalVectors     int64     `json:"total_vectors"`
	TotalCollections int       `json:"total_collections"`
	TotalDatabases   int       `json:"total_databases"`
	MemoryUsage      int64     `json:"memory_usage_bytes"`

	// Request metrics
	RequestsTotal   int64   `json:"requests_total"`
	RequestDuration float64 `json:"request_duration_avg_ms"`
	ErrorsTotal     int64   `json:"errors_total"`

	// Search metrics
	SearchLatency    float64 `json:"search_latency_avg_ms"`
	SearchThroughput float64 `json:"search_throughput_qps"`

	// Insert metrics
	InsertLatency    float64 `json:"insert_latency_avg_ms"`
	InsertThroughput float64 `json:"insert_throughput_ops"`
}
