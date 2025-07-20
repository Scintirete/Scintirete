package embedding

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/scintirete/scintirete/pkg/types"
)

func TestNewClient(t *testing.T) {
	// Set environment variable for testing
	os.Setenv("TEST_API_KEY", "test-key")
	defer os.Unsetenv("TEST_API_KEY")

	config := Config{
		BaseURL:      "https://api.test.com/v1/embeddings",
		APIKeyEnvVar: "TEST_API_KEY",
		RPMLimit:     100,
		TPMLimit:     10000,
		Timeout:      5 * time.Second,
	}

	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	if client == nil {
		t.Fatal("Client is nil")
	}

	if client.baseURL != "https://api.test.com/v1/embeddings" {
		t.Errorf("Expected baseURL %s, got %s", "https://api.test.com/v1/embeddings", client.baseURL)
	}

	if client.apiKey != "test-key" {
		t.Errorf("Expected apiKey %s, got %s", "test-key", client.apiKey)
	}
}

func TestNewClient_MissingAPIKey(t *testing.T) {
	config := Config{
		BaseURL:      "https://api.test.com/v1/embeddings",
		APIKeyEnvVar: "NON_EXISTENT_KEY",
		RPMLimit:     100,
		TPMLimit:     10000,
	}

	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("Client creation should succeed even without API key, got error: %v", err)
	}

	if client == nil {
		t.Fatal("Client should not be nil")
	}

	// Verify that embedding operations fail when API key is missing
	_, err = client.GetEmbeddings(context.Background(), []string{"test"}, "text-embedding-ada-002")
	if err == nil {
		t.Fatal("Expected error when calling GetEmbeddings without API key")
	}

	expectedErrMsg := "embedding functionality requires API key to be configured"
	if err.Error() != expectedErrMsg {
		t.Errorf("Expected error message %q, got %q", expectedErrMsg, err.Error())
	}
}

func TestRateCounter(t *testing.T) {
	counter := &rateCounter{
		limit:    10,
		duration: time.Second,
	}

	// Should allow up to limit
	for i := 0; i < 10; i++ {
		if !counter.canProceed(1) {
			t.Fatalf("Should allow request %d", i+1)
		}
	}

	// Should deny next request
	if counter.canProceed(1) {
		t.Fatal("Should deny request exceeding limit")
	}

	// Wait for window to reset
	time.Sleep(time.Second + 10*time.Millisecond)

	// Should allow requests again
	if !counter.canProceed(1) {
		t.Fatal("Should allow request after window reset")
	}
}

func TestGetEmbeddings(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and headers
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("Expected Authorization Bearer test-key, got %s", r.Header.Get("Authorization"))
		}

		// Return mock response
		response := types.EmbeddingResponse{
			Data: []types.EmbeddingData{
				{
					Index:     0,
					Embedding: []float32{0.1, 0.2, 0.3},
				},
				{
					Index:     1,
					Embedding: []float32{0.4, 0.5, 0.6},
				},
			},
			Usage: types.EmbeddingUsage{
				PromptTokens: 10,
				TotalTokens:  10,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Set environment variable for testing
	os.Setenv("TEST_API_KEY", "test-key")
	defer os.Unsetenv("TEST_API_KEY")

	config := Config{
		BaseURL:      server.URL,
		APIKeyEnvVar: "TEST_API_KEY",
		RPMLimit:     100,
		TPMLimit:     10000,
		Timeout:      5 * time.Second,
	}

	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	texts := []string{"Hello world", "Test text"}
	resp, err := client.GetEmbeddings(context.Background(), texts, "text-embedding-ada-002")
	if err != nil {
		t.Fatalf("Failed to get embeddings: %v", err)
	}

	if len(resp.Data) != 2 {
		t.Fatalf("Expected 2 embeddings, got %d", len(resp.Data))
	}

	if len(resp.Data[0].Embedding) != 3 {
		t.Fatalf("Expected embedding dimension 3, got %d", len(resp.Data[0].Embedding))
	}

	expectedEmbedding := []float32{0.1, 0.2, 0.3}
	for i, val := range resp.Data[0].Embedding {
		if val != expectedEmbedding[i] {
			t.Errorf("Expected embedding[%d] = %f, got %f", i, expectedEmbedding[i], val)
		}
	}
}

func TestGetSingleEmbedding(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := types.EmbeddingResponse{
			Data: []types.EmbeddingData{
				{
					Index:     0,
					Embedding: []float32{0.1, 0.2, 0.3},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Set environment variable for testing
	os.Setenv("TEST_API_KEY", "test-key")
	defer os.Unsetenv("TEST_API_KEY")

	config := Config{
		BaseURL:      server.URL,
		APIKeyEnvVar: "TEST_API_KEY",
		RPMLimit:     100,
		TPMLimit:     10000,
	}

	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	embedding, err := client.GetSingleEmbedding(context.Background(), "Hello world", "text-embedding-ada-002")
	if err != nil {
		t.Fatalf("Failed to get single embedding: %v", err)
	}

	expectedEmbedding := []float32{0.1, 0.2, 0.3}
	if len(embedding) != len(expectedEmbedding) {
		t.Fatalf("Expected embedding dimension %d, got %d", len(expectedEmbedding), len(embedding))
	}

	for i, val := range embedding {
		if val != expectedEmbedding[i] {
			t.Errorf("Expected embedding[%d] = %f, got %f", i, expectedEmbedding[i], val)
		}
	}
}

func TestConvertTextsToVectors(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := types.EmbeddingResponse{
			Data: []types.EmbeddingData{
				{
					Index:     0,
					Embedding: []float32{0.1, 0.2, 0.3},
				},
				{
					Index:     1,
					Embedding: []float32{0.4, 0.5, 0.6},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Set environment variable for testing
	os.Setenv("TEST_API_KEY", "test-key")
	defer os.Unsetenv("TEST_API_KEY")

	config := Config{
		BaseURL:      server.URL,
		APIKeyEnvVar: "TEST_API_KEY",
		RPMLimit:     100,
		TPMLimit:     10000,
	}

	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	texts := []types.TextWithMetadata{
		{
			ID:   "doc1",
			Text: "Hello world",
			Metadata: map[string]interface{}{
				"source": "test1",
			},
		},
		{
			ID:   "doc2",
			Text: "Test text",
			Metadata: map[string]interface{}{
				"source": "test2",
			},
		},
	}

	vectors, err := client.ConvertTextsToVectors(context.Background(), texts, "text-embedding-ada-002")
	if err != nil {
		t.Fatalf("Failed to convert texts to vectors: %v", err)
	}

	if len(vectors) != 2 {
		t.Fatalf("Expected 2 vectors, got %d", len(vectors))
	}

	// Check first vector
	if vectors[0].ID != "doc1" {
		t.Errorf("Expected vector ID doc1, got %s", vectors[0].ID)
	}

	if len(vectors[0].Elements) != 3 {
		t.Fatalf("Expected vector dimension 3, got %d", len(vectors[0].Elements))
	}

	expectedElements := []float32{0.1, 0.2, 0.3}
	for i, val := range vectors[0].Elements {
		if val != expectedElements[i] {
			t.Errorf("Expected elements[%d] = %f, got %f", i, expectedElements[i], val)
		}
	}

	// Check metadata
	if vectors[0].Metadata["source"] != "test1" {
		t.Errorf("Expected metadata source test1, got %v", vectors[0].Metadata["source"])
	}
}
