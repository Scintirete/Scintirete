// Package cli provides the command-line interface for Scintirete.
package cli

import (
	pb "github.com/scintirete/scintirete/gen/go/scintirete/v1"
	"google.golang.org/grpc"
)

// CLI represents the command-line interface
type CLI struct {
	client   pb.ScintireteServiceClient
	conn     *grpc.ClientConn
	password string
	prompt   string
}

// Command represents a CLI command
type Command struct {
	Name        string
	Description string
	Usage       string
	Handler     func(*CLI, []string) error
}

// NewCLI creates a new CLI instance
func NewCLI(password string) *CLI {
	return &CLI{
		password: password,
		prompt:   "scintirete> ",
	}
}

// SetPrompt sets the CLI prompt
func (c *CLI) SetPrompt(prompt string) {
	c.prompt = prompt
}

// GetPrompt returns the current prompt
func (c *CLI) GetPrompt() string {
	return c.prompt
}

// GetClient returns the gRPC client
func (c *CLI) GetClient() pb.ScintireteServiceClient {
	return c.client
}

// GetPassword returns the password for authentication
func (c *CLI) GetPassword() string {
	return c.password
}
