// Package http provides unit tests for the HTTP server.
package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"context"

	pb "github.com/scintirete/scintirete/gen/go/scintirete/v1"
	"github.com/scintirete/scintirete/internal/embedding"
	"github.com/scintirete/scintirete/internal/persistence"
	"github.com/scintirete/scintirete/internal/server"
	grpcserver "github.com/scintirete/scintirete/internal/server/grpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/protobuf/types/known/emptypb"
)

// MockGRPCServer is a mock implementation of the gRPC server for testing
type MockGRPCServer struct {
	mock.Mock
}

func (m *MockGRPCServer) CreateDatabase(ctx context.Context, req *pb.CreateDatabaseRequest) (*emptypb.Empty, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*emptypb.Empty), args.Error(1)
}

func (m *MockGRPCServer) ListDatabases(ctx context.Context, req *pb.ListDatabasesRequest) (*pb.ListDatabasesResponse, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*pb.ListDatabasesResponse), args.Error(1)
}

func (m *MockGRPCServer) DropDatabase(ctx context.Context, req *pb.DropDatabaseRequest) (*emptypb.Empty, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*emptypb.Empty), args.Error(1)
}

// Add other gRPC methods as needed for comprehensive testing

func TestNewHTTPServer(t *testing.T) {
	// Create a real gRPC server for this test
	config := server.ServerConfig{
		Passwords: []string{"test-password"},
		PersistenceConfig: persistence.Config{
			DataDir:     "/tmp/test",
			RDBFilename: "test.rdb",
			AOFFilename: "test.aof",
		},
		EmbeddingConfig: embedding.Config{
			BaseURL: "http://localhost:8080",
			APIKey:  "test-key",
		},
	}

	grpcSrv, err := grpcserver.NewServer(config)
	if err != nil {
		t.Fatalf("Failed to create gRPC server: %v", err)
	}

	// Create HTTP server
	httpSrv := NewServer(grpcSrv)

	assert.NotNil(t, httpSrv)
	assert.NotNil(t, httpSrv.engine)
	assert.NotNil(t, httpSrv.grpcServer)
}

func TestHealthEndpoint(t *testing.T) {
	// Create a real gRPC server
	config := server.ServerConfig{
		Passwords: []string{"test-password"},
		PersistenceConfig: persistence.Config{
			DataDir:     "/tmp/test",
			RDBFilename: "test.rdb",
			AOFFilename: "test.aof",
		},
		EmbeddingConfig: embedding.Config{
			BaseURL: "http://localhost:8080",
			APIKey:  "test-key",
		},
	}

	grpcSrv, err := grpcserver.NewServer(config)
	if err != nil {
		t.Fatalf("Failed to create gRPC server: %v", err)
	}

	httpSrv := NewServer(grpcSrv)

	// Create test request
	req, _ := http.NewRequest("GET", "/api/v1/health", nil)
	w := httptest.NewRecorder()

	// Perform request
	httpSrv.ServeHTTP(w, req)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "healthy", response["status"])
	assert.Equal(t, "scintirete", response["service"])
}

func TestCORSMiddleware(t *testing.T) {
	// Create a real gRPC server
	config := server.ServerConfig{
		Passwords: []string{"test-password"},
		PersistenceConfig: persistence.Config{
			DataDir:     "/tmp/test",
			RDBFilename: "test.rdb",
			AOFFilename: "test.aof",
		},
		EmbeddingConfig: embedding.Config{
			BaseURL: "http://localhost:8080",
			APIKey:  "test-key",
		},
	}

	grpcSrv, err := grpcserver.NewServer(config)
	if err != nil {
		t.Fatalf("Failed to create gRPC server: %v", err)
	}

	httpSrv := NewServer(grpcSrv)

	// Test CORS headers
	req, _ := http.NewRequest("OPTIONS", "/api/v1/health", nil)
	w := httptest.NewRecorder()

	httpSrv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "GET")
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "POST")
}

func TestAuthMiddleware(t *testing.T) {
	// Create a real gRPC server
	config := server.ServerConfig{
		Passwords: []string{"test-password"},
		PersistenceConfig: persistence.Config{
			DataDir:     "/tmp/test",
			RDBFilename: "test.rdb",
			AOFFilename: "test.aof",
		},
		EmbeddingConfig: embedding.Config{
			BaseURL: "http://localhost:8080",
			APIKey:  "test-key",
		},
	}

	grpcSrv, err := grpcserver.NewServer(config)
	if err != nil {
		t.Fatalf("Failed to create gRPC server: %v", err)
	}

	httpSrv := NewServer(grpcSrv)

	tests := []struct {
		name           string
		authHeader     string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "Valid Bearer token",
			authHeader:     "Bearer test-password",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Missing Authorization header",
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "Authorization header required",
		},
		{
			name:           "Invalid format - no Bearer prefix",
			authHeader:     "test-password",
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "Invalid authorization format. Expected: Bearer {token}",
		},
		{
			name:           "Invalid format - wrong prefix",
			authHeader:     "Basic test-password",
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "Invalid authorization format. Expected: Bearer {token}",
		},
		{
			name:           "Empty token",
			authHeader:     "Bearer ",
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "Token cannot be empty",
		},
		{
			name:           "Bearer only",
			authHeader:     "Bearer",
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "Invalid authorization format. Expected: Bearer {token}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test with protected endpoint
			req, _ := http.NewRequest("GET", "/api/v1/databases", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			w := httptest.NewRecorder()

			httpSrv.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedError != "" {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.False(t, response["success"].(bool))
				assert.Contains(t, response["error"], tt.expectedError)
			}
		})
	}
}

func TestPublicEndpointsNoAuth(t *testing.T) {
	// Create a real gRPC server
	config := server.ServerConfig{
		Passwords: []string{"test-password"},
		PersistenceConfig: persistence.Config{
			DataDir:     "/tmp/test",
			RDBFilename: "test.rdb",
			AOFFilename: "test.aof",
		},
		EmbeddingConfig: embedding.Config{
			BaseURL: "http://localhost:8080",
			APIKey:  "test-key",
		},
	}

	grpcSrv, err := grpcserver.NewServer(config)
	if err != nil {
		t.Fatalf("Failed to create gRPC server: %v", err)
	}

	httpSrv := NewServer(grpcSrv)

	// Test public endpoints that should not require authentication
	publicEndpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/api/v1/health"},
	}

	for _, endpoint := range publicEndpoints {
		t.Run(endpoint.method+" "+endpoint.path, func(t *testing.T) {
			req, _ := http.NewRequest(endpoint.method, endpoint.path, nil)
			w := httptest.NewRecorder()
			httpSrv.ServeHTTP(w, req)

			// Should not return 401 Unauthorized
			assert.NotEqual(t, http.StatusUnauthorized, w.Code)
		})
	}
}

func TestErrorHandling(t *testing.T) {
	// Create a real gRPC server
	config := server.ServerConfig{
		Passwords: []string{"test-password"},
		PersistenceConfig: persistence.Config{
			DataDir:     "/tmp/test",
			RDBFilename: "test.rdb",
			AOFFilename: "test.aof",
		},
		EmbeddingConfig: embedding.Config{
			BaseURL: "http://localhost:8080",
			APIKey:  "test-key",
		},
	}

	grpcSrv, err := grpcserver.NewServer(config)
	if err != nil {
		t.Fatalf("Failed to create gRPC server: %v", err)
	}

	httpSrv := NewServer(grpcSrv)

	// Test invalid JSON with a protected endpoint (databases with auth)
	invalidJSON := `{"invalid": json}`
	req, _ := http.NewRequest("POST", "/api/v1/databases", bytes.NewBuffer([]byte(invalidJSON)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-password")
	w := httptest.NewRecorder()

	httpSrv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.False(t, response["success"].(bool))
	assert.NotEmpty(t, response["error"])
}
