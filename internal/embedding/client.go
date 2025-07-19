package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/scintirete/scintirete/pkg/types"
)

// Client represents an embedding API client
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client

	// Rate limiting
	rpmLimit   int
	tpmLimit   int
	rpmCounter *rateCounter
	tpmCounter *rateCounter

	mu sync.RWMutex
}

// rateCounter implements simple rate limiting
type rateCounter struct {
	count    int
	limit    int
	window   time.Time
	duration time.Duration
	mu       sync.Mutex
}

// Config contains embedding client configuration
type Config struct {
	BaseURL      string
	APIKeyEnvVar string
	RPMLimit     int
	TPMLimit     int
	Timeout      time.Duration
}

// NewClient creates a new embedding client
func NewClient(config Config) (*Client, error) {
	apiKey := os.Getenv(config.APIKeyEnvVar)
	if apiKey == "" {
		return nil, fmt.Errorf("API key not found in environment variable: %s", config.APIKeyEnvVar)
	}

	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	client := &Client{
		baseURL: strings.TrimSuffix(config.BaseURL, "/"),
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		rpmLimit: config.RPMLimit,
		tpmLimit: config.TPMLimit,
		rpmCounter: &rateCounter{
			limit:    config.RPMLimit,
			duration: time.Minute,
		},
		tpmCounter: &rateCounter{
			limit:    config.TPMLimit,
			duration: time.Minute,
		},
	}

	return client, nil
}

// newRateCounter creates a new rate counter
func (rc *rateCounter) canProceed(tokens int) bool {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	now := time.Now()

	// Reset counter if window has passed
	if now.Sub(rc.window) >= rc.duration {
		rc.count = 0
		rc.window = now
	}

	// Check if adding tokens would exceed limit
	if rc.count+tokens > rc.limit {
		return false
	}

	rc.count += tokens
	return true
}

// GetEmbeddings calls the embedding API to get embeddings for the given texts
func (c *Client) GetEmbeddings(ctx context.Context, texts []string, model string) (*types.EmbeddingResponse, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("no texts provided")
	}

	// Estimate token count (rough approximation: 1 token ≈ 4 characters)
	totalChars := 0
	for _, text := range texts {
		totalChars += len(text)
	}
	estimatedTokens := totalChars / 4

	// Check rate limits
	if !c.rpmCounter.canProceed(1) {
		return nil, fmt.Errorf("RPM limit exceeded")
	}

	if !c.tpmCounter.canProceed(estimatedTokens) {
		return nil, fmt.Errorf("TPM limit exceeded")
	}

	// Prepare request
	reqBody := types.EmbeddingRequest{
		Texts: texts,
		Model: model,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var embeddingResp types.EmbeddingResponse
	if err := json.Unmarshal(respBody, &embeddingResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &embeddingResp, nil
}

// GetSingleEmbedding is a convenience method for getting embedding of a single text
func (c *Client) GetSingleEmbedding(ctx context.Context, text string, model string) ([]float32, error) {
	resp, err := c.GetEmbeddings(ctx, []string{text}, model)
	if err != nil {
		return nil, err
	}

	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("no embedding data returned")
	}

	return resp.Data[0].Embedding, nil
}

// ConvertTextsToVectors converts texts with metadata to vectors using embedding API
func (c *Client) ConvertTextsToVectors(ctx context.Context, texts []types.TextWithMetadata, model string) ([]types.Vector, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("no texts provided")
	}

	// Extract text content
	textContent := make([]string, len(texts))
	for i, t := range texts {
		textContent[i] = t.Text
	}

	// Get embeddings
	embeddingResp, err := c.GetEmbeddings(ctx, textContent, model)
	if err != nil {
		return nil, err
	}

	if len(embeddingResp.Data) != len(texts) {
		return nil, fmt.Errorf("embedding count mismatch: expected %d, got %d", len(texts), len(embeddingResp.Data))
	}

	// Convert to vectors
	vectors := make([]types.Vector, len(texts))
	for i, text := range texts {
		vectors[i] = types.Vector{
			ID:       text.ID,
			Elements: embeddingResp.Data[i].Embedding,
			Metadata: text.Metadata,
		}
	}

	return vectors, nil
}
