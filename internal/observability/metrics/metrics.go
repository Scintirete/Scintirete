// Package metrics provides Prometheus metrics collection for Scintirete.
package metrics

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/scintirete/scintirete/pkg/types"
)

// Counter represents a monotonically increasing counter metric
type Counter struct {
	mu     sync.Mutex
	value  float64
	name   string
	help   string
	labels map[string]string
}

// Gauge represents a metric that can go up and down
type Gauge struct {
	mu     sync.Mutex
	value  float64
	name   string
	help   string
	labels map[string]string
}

// Histogram represents a metric that samples observations
type Histogram struct {
	mu      sync.Mutex
	count   uint64
	sum     float64
	buckets map[float64]uint64
	name    string
	help    string
	labels  map[string]string
}

// PrometheusCollector implements the core.MetricsCollector interface
type PrometheusCollector struct {
	mu sync.RWMutex

	// Request metrics
	requestTotal    *Counter
	requestDuration *Histogram
	requestErrors   *Counter

	// Vector operation metrics
	vectorOpsTotal    *Counter
	vectorOpsDuration *Histogram
	vectorCount       *Gauge
	deletedCount      *Gauge
	memoryUsage       *Gauge

	// System metrics
	uptime    *Gauge
	startTime time.Time

	// Custom metrics
	customCounters   map[string]*Counter
	customGauges     map[string]*Gauge
	customHistograms map[string]*Histogram
}

// Config contains metrics collector configuration
type Config struct {
	Enabled   bool   // Whether metrics collection is enabled
	Namespace string // Metrics namespace prefix (default: "scintirete")
	Subsystem string // Metrics subsystem prefix (default: "")
}

// NewPrometheusCollector creates a new Prometheus metrics collector
func NewPrometheusCollector(config Config) *PrometheusCollector {
	if config.Namespace == "" {
		config.Namespace = "scintirete"
	}

	prefix := config.Namespace
	if config.Subsystem != "" {
		prefix = config.Namespace + "_" + config.Subsystem
	}

	// Default histogram buckets for duration metrics (in seconds)
	durationBuckets := []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0}

	collector := &PrometheusCollector{
		startTime:        time.Now(),
		customCounters:   make(map[string]*Counter),
		customGauges:     make(map[string]*Gauge),
		customHistograms: make(map[string]*Histogram),
	}

	// Initialize standard metrics
	collector.requestTotal = newCounter(
		prefix+"_requests_total",
		"Total number of requests processed",
		map[string]string{},
	)

	collector.requestDuration = newHistogram(
		prefix+"_request_duration_seconds",
		"Request duration in seconds",
		map[string]string{},
		durationBuckets,
	)

	collector.requestErrors = newCounter(
		prefix+"_request_errors_total",
		"Total number of request errors",
		map[string]string{},
	)

	collector.vectorOpsTotal = newCounter(
		prefix+"_vector_operations_total",
		"Total number of vector operations",
		map[string]string{},
	)

	collector.vectorOpsDuration = newHistogram(
		prefix+"_vector_operation_duration_seconds",
		"Vector operation duration in seconds",
		map[string]string{},
		durationBuckets,
	)

	collector.vectorCount = newGauge(
		prefix+"_vectors_total",
		"Total number of vectors in the database",
		map[string]string{},
	)

	collector.deletedCount = newGauge(
		prefix+"_vectors_deleted_total",
		"Total number of deleted vectors",
		map[string]string{},
	)

	collector.memoryUsage = newGauge(
		prefix+"_memory_usage_bytes",
		"Memory usage in bytes",
		map[string]string{},
	)

	collector.uptime = newGauge(
		prefix+"_uptime_seconds",
		"Server uptime in seconds",
		map[string]string{},
	)

	return collector
}

// RecordRequest records a request with its duration and status
func (c *PrometheusCollector) RecordRequest(operation string, duration float64, success bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Record total requests
	c.requestTotal.Inc()

	// Record request duration
	c.requestDuration.Observe(duration)

	// Record errors if not successful
	if !success {
		c.requestErrors.Inc()
	}
}

// RecordVectorOperation records vector operations (insert, delete, search)
func (c *PrometheusCollector) RecordVectorOperation(operation string, count int64, duration float64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Record total vector operations
	c.vectorOpsTotal.Add(float64(count))

	// Record operation duration
	c.vectorOpsDuration.Observe(duration)
}

// UpdateGaugeMetrics updates gauge metrics like memory usage, vector count
func (c *PrometheusCollector) UpdateGaugeMetrics(snapshot types.MetricsSnapshot) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Update vector count
	c.vectorCount.Set(float64(snapshot.TotalVectors))

	// Note: DeletedVectors is not currently in MetricsSnapshot, will need to be added later
	// c.deletedCount.Set(float64(snapshot.DeletedVectors))

	// Update memory usage
	c.memoryUsage.Set(float64(snapshot.MemoryUsage))

	// Update uptime
	c.uptime.Set(time.Since(c.startTime).Seconds())
}

// GetMetrics returns current metrics in Prometheus format
func (c *PrometheusCollector) GetMetrics() ([]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var output strings.Builder

	// Helper function to write metric
	writeMetric := func(metric interface{}) {
		switch m := metric.(type) {
		case *Counter:
			output.WriteString(m.String())
			output.WriteString("\n")
		case *Gauge:
			output.WriteString(m.String())
			output.WriteString("\n")
		case *Histogram:
			output.WriteString(m.String())
			output.WriteString("\n")
		}
	}

	// Write standard metrics
	writeMetric(c.requestTotal)
	writeMetric(c.requestDuration)
	writeMetric(c.requestErrors)
	writeMetric(c.vectorOpsTotal)
	writeMetric(c.vectorOpsDuration)
	writeMetric(c.vectorCount)
	writeMetric(c.deletedCount)
	writeMetric(c.memoryUsage)
	writeMetric(c.uptime)

	// Write custom metrics
	for _, counter := range c.customCounters {
		writeMetric(counter)
	}
	for _, gauge := range c.customGauges {
		writeMetric(gauge)
	}
	for _, histogram := range c.customHistograms {
		writeMetric(histogram)
	}

	return []byte(output.String()), nil
}

// Helper functions to create metrics

func newCounter(name, help string, labels map[string]string) *Counter {
	return &Counter{
		name:   name,
		help:   help,
		labels: labels,
	}
}

func newGauge(name, help string, labels map[string]string) *Gauge {
	return &Gauge{
		name:   name,
		help:   help,
		labels: labels,
	}
}

func newHistogram(name, help string, labels map[string]string, buckets []float64) *Histogram {
	bucketMap := make(map[float64]uint64)
	for _, bucket := range buckets {
		bucketMap[bucket] = 0
	}
	return &Histogram{
		name:    name,
		help:    help,
		labels:  labels,
		buckets: bucketMap,
	}
}

// Counter methods

func (c *Counter) Inc() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.value++
}

func (c *Counter) Add(value float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.value += value
}

func (c *Counter) String() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	var output strings.Builder
	output.WriteString(fmt.Sprintf("# HELP %s %s\n", c.name, c.help))
	output.WriteString(fmt.Sprintf("# TYPE %s counter\n", c.name))

	labelStr := formatLabels(c.labels)
	output.WriteString(fmt.Sprintf("%s%s %g", c.name, labelStr, c.value))

	return output.String()
}

// Gauge methods

func (g *Gauge) Set(value float64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.value = value
}

func (g *Gauge) Inc() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.value++
}

func (g *Gauge) Dec() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.value--
}

func (g *Gauge) Add(value float64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.value += value
}

func (g *Gauge) String() string {
	g.mu.Lock()
	defer g.mu.Unlock()

	var output strings.Builder
	output.WriteString(fmt.Sprintf("# HELP %s %s\n", g.name, g.help))
	output.WriteString(fmt.Sprintf("# TYPE %s gauge\n", g.name))

	labelStr := formatLabels(g.labels)
	output.WriteString(fmt.Sprintf("%s%s %g", g.name, labelStr, g.value))

	return output.String()
}

// Histogram methods

func (h *Histogram) Observe(value float64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.count++
	h.sum += value

	// Update buckets
	for bucket := range h.buckets {
		if value <= bucket {
			h.buckets[bucket]++
		}
	}
}

func (h *Histogram) String() string {
	h.mu.Lock()
	defer h.mu.Unlock()

	var output strings.Builder
	output.WriteString(fmt.Sprintf("# HELP %s %s\n", h.name, h.help))
	output.WriteString(fmt.Sprintf("# TYPE %s histogram\n", h.name))

	labelStr := formatLabels(h.labels)

	// Write buckets
	for bucket, count := range h.buckets {
		bucketLabels := make(map[string]string)
		for k, v := range h.labels {
			bucketLabels[k] = v
		}
		bucketLabels["le"] = strconv.FormatFloat(bucket, 'g', -1, 64)
		bucketLabelStr := formatLabels(bucketLabels)
		output.WriteString(fmt.Sprintf("%s_bucket%s %d\n", h.name, bucketLabelStr, count))
	}

	// Write +Inf bucket
	infLabels := make(map[string]string)
	for k, v := range h.labels {
		infLabels[k] = v
	}
	infLabels["le"] = "+Inf"
	infLabelStr := formatLabels(infLabels)
	output.WriteString(fmt.Sprintf("%s_bucket%s %d\n", h.name, infLabelStr, h.count))

	// Write sum and count
	output.WriteString(fmt.Sprintf("%s_sum%s %g\n", h.name, labelStr, h.sum))
	output.WriteString(fmt.Sprintf("%s_count%s %d", h.name, labelStr, h.count))

	return output.String()
}

// Helper function to format labels
func formatLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}

	var parts []string
	for k, v := range labels {
		parts = append(parts, fmt.Sprintf(`%s="%s"`, k, v))
	}

	return "{" + strings.Join(parts, ",") + "}"
}

// Additional helper methods

// GetStartTime returns the collector's start time
func (c *PrometheusCollector) GetStartTime() time.Time {
	return c.startTime
}

// AddCustomCounter adds a custom counter metric
func (c *PrometheusCollector) AddCustomCounter(name, help string, labels map[string]string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.customCounters[name] = newCounter(name, help, labels)
}

// AddCustomGauge adds a custom gauge metric
func (c *PrometheusCollector) AddCustomGauge(name, help string, labels map[string]string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.customGauges[name] = newGauge(name, help, labels)
}

// GetCustomCounter returns a custom counter by name
func (c *PrometheusCollector) GetCustomCounter(name string) *Counter {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.customCounters[name]
}

// GetCustomGauge returns a custom gauge by name
func (c *PrometheusCollector) GetCustomGauge(name string) *Gauge {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.customGauges[name]
}
