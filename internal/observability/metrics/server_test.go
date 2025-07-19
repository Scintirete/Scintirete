package metrics

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewMetricsServer(t *testing.T) {
	config := ServerConfig{
		Port:    9100,
		Path:    "/metrics",
		Enabled: true,
	}

	mockCollector := NewPrometheusCollector(Config{Enabled: true})
	server := NewMetricsServer(config, mockCollector)

	if server == nil {
		t.Fatal("Expected non-nil server")
	}

	if server.port != 9100 {
		t.Errorf("Expected port 9100, got %d", server.port)
	}

	if server.path != "/metrics" {
		t.Errorf("Expected path '/metrics', got '%s'", server.path)
	}
}

func TestNewMetricsServerWithDefaults(t *testing.T) {
	config := ServerConfig{
		Enabled: true,
	}

	mockCollector := NewPrometheusCollector(Config{Enabled: true})
	server := NewMetricsServer(config, mockCollector)

	if server.port != 9100 {
		t.Errorf("Expected default port 9100, got %d", server.port)
	}

	if server.path != "/metrics" {
		t.Errorf("Expected default path '/metrics', got '%s'", server.path)
	}
}

func TestMetricsServerHandleMetrics(t *testing.T) {
	config := ServerConfig{
		Port:    9100,
		Path:    "/metrics",
		Enabled: true,
	}

	mockCollector := NewPrometheusCollector(Config{Enabled: true})
	server := NewMetricsServer(config, mockCollector)

	// Add some test metrics
	server.RegisterSystemMetrics()

	// Create a test request
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	// Handle the request
	server.handleMetrics(w, req)

	// Check response
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	expectedContentType := "text/plain; version=0.0.4; charset=utf-8"
	if contentType != expectedContentType {
		t.Errorf("Expected content type '%s', got '%s'", expectedContentType, contentType)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	bodyStr := string(body)

	// Check that some expected metrics are present
	expectedMetrics := []string{
		"scintirete_uptime_seconds",
		"scintirete_build_info",
	}

	for _, metric := range expectedMetrics {
		if !strings.Contains(bodyStr, metric) {
			t.Errorf("Expected metric '%s' in response body", metric)
		}
	}

	// Check HELP and TYPE comments are present
	if !strings.Contains(bodyStr, "# HELP") {
		t.Error("Expected HELP comments in metrics output")
	}

	if !strings.Contains(bodyStr, "# TYPE") {
		t.Error("Expected TYPE comments in metrics output")
	}
}

func TestMetricsServerHandleHealth(t *testing.T) {
	config := ServerConfig{
		Port:    9100,
		Path:    "/metrics",
		Enabled: true,
	}

	mockCollector := NewPrometheusCollector(Config{Enabled: true})
	server := NewMetricsServer(config, mockCollector)

	// Create a test request
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	// Handle the request
	server.handleHealth(w, req)

	// Check response
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected content type 'application/json', got '%s'", contentType)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	bodyStr := string(body)

	// Check that health response contains expected fields
	expectedFields := []string{
		"status",
		"timestamp",
		"uptime_seconds",
	}

	for _, field := range expectedFields {
		if !strings.Contains(bodyStr, field) {
			t.Errorf("Expected field '%s' in health response", field)
		}
	}

	// Check that status is healthy
	if !strings.Contains(bodyStr, `"status":"healthy"`) {
		t.Error("Expected status to be 'healthy'")
	}
}

func TestMetricsServerMethodNotAllowed(t *testing.T) {
	config := ServerConfig{
		Port:    9100,
		Path:    "/metrics",
		Enabled: true,
	}

	mockCollector := NewPrometheusCollector(Config{Enabled: true})
	server := NewMetricsServer(config, mockCollector)

	// Test POST to metrics endpoint
	req := httptest.NewRequest("POST", "/metrics", nil)
	w := httptest.NewRecorder()

	server.handleMetrics(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", resp.StatusCode)
	}

	// Test PUT to health endpoint
	req = httptest.NewRequest("PUT", "/health", nil)
	w = httptest.NewRecorder()

	server.handleHealth(w, req)

	resp = w.Result()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", resp.StatusCode)
	}
}

func TestFormatLabels(t *testing.T) {
	tests := []struct {
		name     string
		labels   []Label
		expected string
	}{
		{
			name:     "empty labels",
			labels:   []Label{},
			expected: "",
		},
		{
			name: "single label",
			labels: []Label{
				{Name: "method", Value: "GET"},
			},
			expected: `method="GET"`,
		},
		{
			name: "multiple labels",
			labels: []Label{
				{Name: "method", Value: "POST"},
				{Name: "status", Value: "200"},
			},
			expected: `method="POST",status="200"`,
		},
		{
			name: "labels with special characters",
			labels: []Label{
				{Name: "path", Value: "/api/v1/test"},
				{Name: "message", Value: "hello \"world\""},
			},
			expected: `path="/api/v1/test",message="hello \"world\""`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := formatMetricLabels(test.labels)
			if result != test.expected {
				t.Errorf("Expected '%s', got '%s'", test.expected, result)
			}
		})
	}
}

func TestFormatValue(t *testing.T) {
	tests := []struct {
		value    float64
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{-1, "-1"},
		{3.14159, "3.14159"},
		{1000000, "1000000"},
		{0.000001, "0.000001"},
	}

	for _, test := range tests {
		result := formatValue(test.value)
		if result != test.expected {
			t.Errorf("For value %f, expected '%s', got '%s'", test.value, test.expected, result)
		}
	}
}

func TestEscapeValue(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"hello world", "hello world"},
		{`hello "world"`, `hello \"world\"`},
		{"line1\nline2", "line1\\nline2"},
		{`path\to\file`, `path\\to\\file`},
		{`complex "test" with\nnewlines`, `complex \"test\" with\\nnewlines`},
	}

	for _, test := range tests {
		result := escapeValue(test.input)
		if result != test.expected {
			t.Errorf("For input '%s', expected '%s', got '%s'", test.input, test.expected, result)
		}
	}
}

func TestMetricsServerStartStop(t *testing.T) {
	config := ServerConfig{
		Port:    0, // Use any available port
		Path:    "/metrics",
		Enabled: true,
	}

	mockCollector := NewPrometheusCollector(Config{Enabled: true})
	server := NewMetricsServer(config, mockCollector)

	ctx := context.Background()

	// Start server
	err := server.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start metrics server: %v", err)
	}

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Stop server
	err = server.Stop(ctx)
	if err != nil {
		t.Fatalf("Failed to stop metrics server: %v", err)
	}
}

func TestMetricsServerGetters(t *testing.T) {
	config := ServerConfig{
		Port:    9999,
		Path:    "/custom-metrics",
		Enabled: true,
	}

	mockCollector := NewPrometheusCollector(Config{Enabled: true})
	server := NewMetricsServer(config, mockCollector)

	if server.GetPort() != 9999 {
		t.Errorf("Expected port 9999, got %d", server.GetPort())
	}

	if server.GetPath() != "/custom-metrics" {
		t.Errorf("Expected path '/custom-metrics', got '%s'", server.GetPath())
	}
}
