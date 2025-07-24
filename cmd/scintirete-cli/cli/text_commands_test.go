// Package cli provides unit tests for text commands.
package cli

import (
	"context"
	"errors"
	"testing"

	pb "github.com/scintirete/scintirete/gen/go/scintirete/v1"
	"google.golang.org/grpc"
)

// MockScintireteServiceClient is a mock implementation for testing
type MockScintireteServiceClient struct {
	pb.ScintireteServiceClient
	ListEmbeddingModelsFunc func(ctx context.Context, req *pb.ListEmbeddingModelsRequest, opts ...grpc.CallOption) (*pb.ListEmbeddingModelsResponse, error)
}

func (m *MockScintireteServiceClient) ListEmbeddingModels(ctx context.Context, req *pb.ListEmbeddingModelsRequest, opts ...grpc.CallOption) (*pb.ListEmbeddingModelsResponse, error) {
	if m.ListEmbeddingModelsFunc != nil {
		return m.ListEmbeddingModelsFunc(ctx, req, opts...)
	}
	return nil, errors.New("not implemented")
}

func TestTextCommand_Models(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		mockResponse   *pb.ListEmbeddingModelsResponse
		mockError      error
		expectedError  bool
		expectedErrMsg string
	}{
		{
			name: "successful models list",
			args: []string{"models"},
			mockResponse: &pb.ListEmbeddingModelsResponse{
				DefaultModel: "text-embedding-3-small",
				Models: []*pb.EmbeddingModel{
					{
						Id:          "text-embedding-3-small",
						Name:        "OpenAI Text Embedding 3 Small",
						Dimension:   1536,
						Available:   true,
						Description: "Small and fast embedding model",
					},
					{
						Id:          "text-embedding-3-large",
						Name:        "OpenAI Text Embedding 3 Large",
						Dimension:   3072,
						Available:   true,
						Description: "Large and high-performance embedding model",
					},
				},
			},
			mockError:     nil,
			expectedError: false,
		},
		{
			name: "empty models list",
			args: []string{"models"},
			mockResponse: &pb.ListEmbeddingModelsResponse{
				DefaultModel: "",
				Models:       []*pb.EmbeddingModel{},
			},
			mockError:     nil,
			expectedError: false,
		},
		{
			name:           "grpc error",
			args:           []string{"models"},
			mockResponse:   nil,
			mockError:      errors.New("connection failed"),
			expectedError:  true,
			expectedErrMsg: "failed to list embedding models",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockScintireteServiceClient{
				ListEmbeddingModelsFunc: func(ctx context.Context, req *pb.ListEmbeddingModelsRequest, opts ...grpc.CallOption) (*pb.ListEmbeddingModelsResponse, error) {
					// Verify the request has auth info
					if req.Auth == nil || req.Auth.Password == "" {
						t.Error("Request should include authentication")
					}
					return tt.mockResponse, tt.mockError
				},
			}

			cli := &CLI{
				client:   mockClient,
				password: "test-password",
			}

			err := cli.textCommand(tt.args)

			if tt.expectedError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.expectedErrMsg != "" && err.Error()[:len(tt.expectedErrMsg)] != tt.expectedErrMsg {
					t.Errorf("Expected error message to start with '%s', got '%s'", tt.expectedErrMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestTextCommand_ArgumentValidation(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		expectedError  bool
		expectedErrMsg string
	}{
		{
			name:           "no arguments",
			args:           []string{},
			expectedError:  true,
			expectedErrMsg: "usage: text <insert|search|models>",
		},
		{
			name:           "unknown subcommand",
			args:           []string{"unknown"},
			expectedError:  true,
			expectedErrMsg: "unknown text sub-command: unknown",
		},
		{
			name:          "valid models subcommand",
			args:          []string{"models"},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockScintireteServiceClient{
				ListEmbeddingModelsFunc: func(ctx context.Context, req *pb.ListEmbeddingModelsRequest, opts ...grpc.CallOption) (*pb.ListEmbeddingModelsResponse, error) {
					return &pb.ListEmbeddingModelsResponse{
						DefaultModel: "test-model",
						Models:       []*pb.EmbeddingModel{},
					}, nil
				},
			}

			cli := &CLI{
				client:   mockClient,
				password: "test-password",
			}

			err := cli.textCommand(tt.args)

			if tt.expectedError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.expectedErrMsg != "" && err.Error()[:len(tt.expectedErrMsg)] != tt.expectedErrMsg {
					t.Errorf("Expected error message to start with '%s', got '%s'", tt.expectedErrMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestTextModelsCommand_WithModelData(t *testing.T) {
	mockClient := &MockScintireteServiceClient{
		ListEmbeddingModelsFunc: func(ctx context.Context, req *pb.ListEmbeddingModelsRequest, opts ...grpc.CallOption) (*pb.ListEmbeddingModelsResponse, error) {
			return &pb.ListEmbeddingModelsResponse{
				DefaultModel: "text-embedding-3-small",
				Models: []*pb.EmbeddingModel{
					{
						Id:          "text-embedding-3-small",
						Name:        "OpenAI Text Embedding 3 Small",
						Dimension:   1536,
						Available:   true,
						Description: "Small and fast embedding model for general use",
					},
					{
						Id:          "text-embedding-3-large",
						Name:        "OpenAI Text Embedding 3 Large Super Long Name That Should Be Truncated",
						Dimension:   3072,
						Available:   false,
						Description: "Large and high-performance embedding model with very detailed description that should be truncated",
					},
				},
			}, nil
		},
	}

	cli := &CLI{
		client:   mockClient,
		password: "test-password",
	}

	// Test that textModelsCommand can be called directly without panicking
	err := cli.textModelsCommand([]string{})
	if err != nil {
		t.Errorf("textModelsCommand should not error with valid response, got: %v", err)
	}
}

func TestTextCommand_RegistrationAndHelp(t *testing.T) {
	// Test that text command is properly registered
	commands := GetCommands()

	textCmd, exists := commands["text"]
	if !exists {
		t.Error("Text command should be registered")
	}

	if textCmd.Name != "text" {
		t.Errorf("Expected command name 'text', got '%s'", textCmd.Name)
	}

	if textCmd.Description == "" {
		t.Error("Text command should have a description")
	}

	// Verify usage includes models subcommand
	expectedUsage := "text <insert|search|models> <args...>"
	if textCmd.Usage != expectedUsage {
		t.Errorf("Expected usage '%s', got '%s'", expectedUsage, textCmd.Usage)
	}
}
