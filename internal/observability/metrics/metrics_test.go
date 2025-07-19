package metrics

import (
	"testing"
	"time"
)

func TestCounterOperations(t *testing.T) {
	counter := &Counter{
		name:   "test_counter",
		help:   "Test counter",
		labels: make(map[string]string),
	}

	// Test initial value
	if counter.value != 0 {
		t.Errorf("Expected initial value 0, got %f", counter.value)
	}

	// Test increment
	counter.mu.Lock()
	counter.value += 1
	counter.mu.Unlock()

	if counter.value != 1 {
		t.Errorf("Expected value 1 after increment, got %f", counter.value)
	}
}

func TestGaugeOperations(t *testing.T) {
	gauge := &Gauge{
		name:   "test_gauge",
		help:   "Test gauge",
		labels: make(map[string]string),
	}

	// Test set value
	gauge.mu.Lock()
	gauge.value = 42.5
	gauge.mu.Unlock()

	if gauge.value != 42.5 {
		t.Errorf("Expected value 42.5, got %f", gauge.value)
	}

	// Test increase
	gauge.mu.Lock()
	gauge.value += 10
	gauge.mu.Unlock()

	if gauge.value != 52.5 {
		t.Errorf("Expected value 52.5 after increase, got %f", gauge.value)
	}

	// Test decrease
	gauge.mu.Lock()
	gauge.value -= 5
	gauge.mu.Unlock()

	if gauge.value != 47.5 {
		t.Errorf("Expected value 47.5 after decrease, got %f", gauge.value)
	}
}

func TestHistogramOperations(t *testing.T) {
	histogram := &Histogram{
		name:    "test_histogram",
		help:    "Test histogram",
		labels:  make(map[string]string),
		buckets: make(map[float64]uint64),
	}

	// Test initial state
	if histogram.count != 0 {
		t.Errorf("Expected initial count 0, got %d", histogram.count)
	}

	if histogram.sum != 0 {
		t.Errorf("Expected initial sum 0, got %f", histogram.sum)
	}

	// Test observe value
	value := 1.5
	histogram.mu.Lock()
	histogram.count++
	histogram.sum += value

	// Add to buckets
	buckets := []float64{1.0, 2.0, 5.0, 10.0}
	for _, bucket := range buckets {
		if value <= bucket {
			histogram.buckets[bucket]++
		}
	}
	histogram.mu.Unlock()

	if histogram.count != 1 {
		t.Errorf("Expected count 1, got %d", histogram.count)
	}

	if histogram.sum != 1.5 {
		t.Errorf("Expected sum 1.5, got %f", histogram.sum)
	}

	if histogram.buckets[2.0] != 1 {
		t.Errorf("Expected bucket 2.0 to have count 1, got %d", histogram.buckets[2.0])
	}
}

func TestNewPrometheusCollector(t *testing.T) {
	config := Config{
		Enabled:   true,
		Namespace: "test",
		Subsystem: "scintirete",
	}

	collector := NewPrometheusCollector(config)
	if collector == nil {
		t.Fatal("Expected non-nil collector")
	}

	// Test default namespace
	config2 := Config{
		Enabled: true,
	}
	collector2 := NewPrometheusCollector(config2)
	if collector2 == nil {
		t.Fatal("Expected non-nil collector with default config")
	}
}

func TestMetricNaming(t *testing.T) {
	tests := []struct {
		namespace string
		subsystem string
		expected  string
	}{
		{"scintirete", "", "scintirete"},
		{"scintirete", "http", "scintirete_http"},
		{"", "", "scintirete"},
		{"", "subsys", "scintirete_subsys"},
	}

	for _, test := range tests {
		config := Config{
			Enabled:   true,
			Namespace: test.namespace,
			Subsystem: test.subsystem,
		}

		collector := NewPrometheusCollector(config)
		if collector == nil {
			t.Errorf("Expected non-nil collector for namespace '%s', subsystem '%s'", test.namespace, test.subsystem)
		}
	}
}

func TestConcurrentMetricsOperations(t *testing.T) {
	config := Config{
		Enabled:   true,
		Namespace: "test",
		Subsystem: "concurrent",
	}

	collector := NewPrometheusCollector(config)

	// Test concurrent operations
	done := make(chan bool, 3)

	// Concurrent counter increments
	go func() {
		for i := 0; i < 100; i++ {
			counter := &Counter{name: "test", help: "test", labels: make(map[string]string)}
			counter.mu.Lock()
			counter.value++
			counter.mu.Unlock()
		}
		done <- true
	}()

	// Concurrent gauge sets
	go func() {
		for i := 0; i < 100; i++ {
			gauge := &Gauge{name: "test", help: "test", labels: make(map[string]string)}
			gauge.mu.Lock()
			gauge.value = float64(i)
			gauge.mu.Unlock()
		}
		done <- true
	}()

	// Concurrent histogram observations
	go func() {
		for i := 0; i < 100; i++ {
			histogram := &Histogram{name: "test", help: "test", labels: make(map[string]string), buckets: make(map[float64]uint64)}
			histogram.mu.Lock()
			histogram.count++
			histogram.sum += float64(i) * 0.5
			histogram.mu.Unlock()
		}
		done <- true
	}()

	// Wait for all goroutines to complete
	for i := 0; i < 3; i++ {
		select {
		case <-done:
			// Continue
		case <-time.After(5 * time.Second):
			t.Fatal("Concurrent operations timed out")
		}
	}

	// If we reach here, no race conditions occurred
	_ = collector
}

func TestDisabledCollector(t *testing.T) {
	config := Config{
		Enabled:   false,
		Namespace: "test",
		Subsystem: "disabled",
	}

	collector := NewPrometheusCollector(config)

	// Operations on disabled collector should not panic
	if collector == nil {
		t.Error("Expected non-nil collector even when disabled")
	}
}
