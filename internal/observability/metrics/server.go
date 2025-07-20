// Package metrics provides HTTP server for exposing Prometheus metrics.
package metrics

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/scintirete/scintirete/internal/core"
	"github.com/scintirete/scintirete/pkg/types"
)

// MetricsServer serves metrics over HTTP in Prometheus format
type MetricsServer struct {
	mu        sync.RWMutex
	collector core.MetricsCollector
	server    *http.Server
	port      int
	path      string
	registry  map[string]MetricFamily
	startTime time.Time
}

// MetricFamily represents a group of related metrics
type MetricFamily struct {
	Name    string
	Help    string
	Type    string
	Metrics []Metric
}

// Metric represents a single metric with labels and value
type Metric struct {
	Labels []Label
	Value  float64
}

// Label represents a metric label
type Label struct {
	Name  string
	Value string
}

// ServerConfig contains configuration for the metrics server
type ServerConfig struct {
	Port    int    // Port to listen on (default: 9100)
	Path    string // Path to serve metrics (default: "/metrics")
	Enabled bool   // Whether to enable metrics server
}

// NewMetricsServer creates a new metrics server
func NewMetricsServer(config ServerConfig, collector core.MetricsCollector) *MetricsServer {
	if config.Port == 0 {
		config.Port = 9100
	}
	if config.Path == "" {
		config.Path = "/metrics"
	}

	server := &MetricsServer{
		collector: collector,
		port:      config.Port,
		path:      config.Path,
		registry:  make(map[string]MetricFamily),
		startTime: time.Now(),
	}

	// Setup HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc(config.Path, server.handleMetrics)
	mux.HandleFunc("/health", server.handleHealth)

	server.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", config.Port),
		Handler: mux,
	}

	return server
}

// Start starts the metrics server
func (s *MetricsServer) Start(ctx context.Context) error {
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// Log error but don't fail the entire application
			fmt.Printf("Metrics server error: %v\n", err)
		}
	}()
	return nil
}

// Stop gracefully stops the metrics server
func (s *MetricsServer) Stop(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// RegisterDatabaseStats registers database statistics
func (s *MetricsServer) RegisterDatabaseStats(stats types.DatabaseStats) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Total vectors
	s.registry["scintirete_vectors_total"] = MetricFamily{
		Name: "scintirete_vectors_total",
		Help: "Total number of vectors in the database",
		Type: "gauge",
		Metrics: []Metric{
			{Value: float64(stats.TotalVectors)},
		},
	}

	// Total collections
	s.registry["scintirete_collections_total"] = MetricFamily{
		Name: "scintirete_collections_total",
		Help: "Total number of collections in the database",
		Type: "gauge",
		Metrics: []Metric{
			{Value: float64(stats.TotalCollections)},
		},
	}

	// Total databases
	s.registry["scintirete_databases_total"] = MetricFamily{
		Name: "scintirete_databases_total",
		Help: "Total number of databases",
		Type: "gauge",
		Metrics: []Metric{
			{Value: float64(stats.TotalDatabases)},
		},
	}

	// Memory usage
	s.registry["scintirete_memory_bytes"] = MetricFamily{
		Name: "scintirete_memory_bytes",
		Help: "Memory usage in bytes",
		Type: "gauge",
		Metrics: []Metric{
			{Value: float64(stats.MemoryUsage)},
		},
	}

	// Request metrics
	s.registry["scintirete_requests_total"] = MetricFamily{
		Name: "scintirete_requests_total",
		Help: "Total number of requests processed",
		Type: "counter",
		Metrics: []Metric{
			{Value: float64(stats.RequestsTotal)},
		},
	}

	// Request duration
	s.registry["scintirete_request_duration_ms"] = MetricFamily{
		Name: "scintirete_request_duration_ms",
		Help: "Average request duration in milliseconds",
		Type: "gauge",
		Metrics: []Metric{
			{Value: stats.RequestDuration},
		},
	}

	// Error count
	s.registry["scintirete_errors_total"] = MetricFamily{
		Name: "scintirete_errors_total",
		Help: "Total number of errors",
		Type: "counter",
		Metrics: []Metric{
			{Value: float64(stats.ErrorsTotal)},
		},
	}

	// Search metrics
	s.registry["scintirete_search_latency_ms"] = MetricFamily{
		Name: "scintirete_search_latency_ms",
		Help: "Average search latency in milliseconds",
		Type: "gauge",
		Metrics: []Metric{
			{Value: stats.SearchLatency},
		},
	}

	s.registry["scintirete_search_throughput_qps"] = MetricFamily{
		Name: "scintirete_search_throughput_qps",
		Help: "Search throughput in queries per second",
		Type: "gauge",
		Metrics: []Metric{
			{Value: stats.SearchThroughput},
		},
	}

	// Insert metrics
	s.registry["scintirete_insert_latency_ms"] = MetricFamily{
		Name: "scintirete_insert_latency_ms",
		Help: "Average insert latency in milliseconds",
		Type: "gauge",
		Metrics: []Metric{
			{Value: stats.InsertLatency},
		},
	}

	s.registry["scintirete_insert_throughput_ops"] = MetricFamily{
		Name: "scintirete_insert_throughput_ops",
		Help: "Insert throughput in operations per second",
		Type: "gauge",
		Metrics: []Metric{
			{Value: stats.InsertThroughput},
		},
	}
}

// RegisterCollectionStats registers collection-specific statistics
func (s *MetricsServer) RegisterCollectionStats(dbName string, collectionInfo types.CollectionInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()

	labels := []Label{
		{Name: "database", Value: dbName},
		{Name: "collection", Value: collectionInfo.Name},
		{Name: "metric_type", Value: collectionInfo.MetricType.String()},
	}

	// Collection vector count
	s.addMetricToFamily("scintirete_collection_vectors", MetricFamily{
		Name: "scintirete_collection_vectors",
		Help: "Number of vectors in each collection",
		Type: "gauge",
	}, Metric{
		Labels: labels,
		Value:  float64(collectionInfo.VectorCount),
	})

	// Collection deleted count
	s.addMetricToFamily("scintirete_collection_deleted_vectors", MetricFamily{
		Name: "scintirete_collection_deleted_vectors",
		Help: "Number of deleted vectors in each collection",
		Type: "gauge",
	}, Metric{
		Labels: labels,
		Value:  float64(collectionInfo.DeletedCount),
	})

	// Collection memory usage
	s.addMetricToFamily("scintirete_collection_memory_bytes", MetricFamily{
		Name: "scintirete_collection_memory_bytes",
		Help: "Memory usage of each collection in bytes",
		Type: "gauge",
	}, Metric{
		Labels: labels,
		Value:  float64(collectionInfo.MemoryBytes),
	})

	// Collection dimension
	s.addMetricToFamily("scintirete_collection_dimension", MetricFamily{
		Name: "scintirete_collection_dimension",
		Help: "Vector dimension of each collection",
		Type: "gauge",
	}, Metric{
		Labels: labels,
		Value:  float64(collectionInfo.Dimension),
	})
}

// RegisterSystemMetrics registers system-level metrics
func (s *MetricsServer) RegisterSystemMetrics() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Uptime
	uptime := time.Since(s.startTime).Seconds()
	s.registry["scintirete_uptime_seconds"] = MetricFamily{
		Name: "scintirete_uptime_seconds",
		Help: "Uptime of the Scintirete server in seconds",
		Type: "counter",
		Metrics: []Metric{
			{Value: uptime},
		},
	}

	// Build info (static metric)
	s.registry["scintirete_build_info"] = MetricFamily{
		Name: "scintirete_build_info",
		Help: "Build information about the Scintirete server",
		Type: "gauge",
		Metrics: []Metric{
			{
				Labels: []Label{
					{Name: "version", Value: "dev"},
					{Name: "go_version", Value: "1.24"},
				},
				Value: 1,
			},
		},
	}
}

// addMetricToFamily adds a metric to an existing family or creates a new one
func (s *MetricsServer) addMetricToFamily(name string, family MetricFamily, metric Metric) {
	if existing, exists := s.registry[name]; exists {
		existing.Metrics = append(existing.Metrics, metric)
		s.registry[name] = existing
	} else {
		family.Metrics = []Metric{metric}
		s.registry[name] = family
	}
}

// handleMetrics handles the /metrics endpoint
func (s *MetricsServer) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

	// Sort metric families by name for consistent output
	var familyNames []string
	for name := range s.registry {
		familyNames = append(familyNames, name)
	}
	sort.Strings(familyNames)

	for _, name := range familyNames {
		family := s.registry[name]

		// Write HELP comment
		fmt.Fprintf(w, "# HELP %s %s\n", family.Name, family.Help)

		// Write TYPE comment
		fmt.Fprintf(w, "# TYPE %s %s\n", family.Name, family.Type)

		// Write metrics
		for _, metric := range family.Metrics {
			if len(metric.Labels) > 0 {
				fmt.Fprintf(w, "%s{%s} %s\n", family.Name, formatMetricLabels(metric.Labels), formatValue(metric.Value))
			} else {
				fmt.Fprintf(w, "%s %s\n", family.Name, formatValue(metric.Value))
			}
		}

		fmt.Fprintln(w) // Empty line between families
	}
}

// handleHealth handles the /health endpoint
func (s *MetricsServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"healthy","timestamp":"%s","uptime_seconds":%.0f}`,
		time.Now().UTC().Format(time.RFC3339),
		time.Since(s.startTime).Seconds())
}

// formatMetricLabels formats metric labels for Prometheus output
func formatMetricLabels(labels []Label) string {
	if len(labels) == 0 {
		return ""
	}

	var parts []string
	for _, label := range labels {
		parts = append(parts, fmt.Sprintf(`%s="%s"`, label.Name, escapeValue(label.Value)))
	}
	return strings.Join(parts, ",")
}

// formatValue formats a metric value
func formatValue(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

// escapeValue escapes special characters in label values
func escapeValue(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "\"", "\\\"")
	value = strings.ReplaceAll(value, "\n", "\\n")
	return value
}

// GetPort returns the port the metrics server is listening on
func (s *MetricsServer) GetPort() int {
	return s.port
}

// GetPath returns the path the metrics are served on
func (s *MetricsServer) GetPath() string {
	return s.path
}
