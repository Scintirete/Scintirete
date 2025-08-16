// Package server provides common interfaces and types for server implementations.
package server

import (
	"context"
	"time"

	"github.com/scintirete/scintirete/internal/config"
	"github.com/scintirete/scintirete/internal/core"
	"github.com/scintirete/scintirete/internal/core/database"
	"github.com/scintirete/scintirete/internal/embedding"
	"github.com/scintirete/scintirete/internal/persistence"
)

// ServerConfig contains server configuration shared by gRPC and HTTP servers
type ServerConfig struct {
	// Authentication
	Passwords []string `toml:"passwords"`

	// Persistence
	PersistenceConfig persistence.Config `toml:"persistence"`

	// Embedding
	EmbeddingConfig embedding.Config `toml:"embedding"`

	// Features
	EnableMetrics  bool `toml:"enable_metrics"`
	EnableAuditLog bool `toml:"enable_audit_log"`

	// Monitoring
	MonitoringConfig config.RuntimeMonitoringConfig `toml:"monitoring"`
}

// Stats contains server statistics
type Stats struct {
	StartTime    time.Time     `json:"start_time"`
	Uptime       time.Duration `json:"uptime"`
	RequestCount int64         `json:"request_count"`
}

// Dependencies contains the core dependencies for both gRPC and HTTP servers
type Dependencies struct {
	Engine      *database.Engine
	Persistence *persistence.Manager
	Embedding   *embedding.Client
	Logger      core.Logger
}

// Server represents a common server interface
type Server interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	GetStats() Stats
}

// Authenticator provides authentication functionality
type Authenticator interface {
	Authenticate(password string) error
}

// BasicAuthenticator implements password-based authentication
type BasicAuthenticator struct {
	validPasswords map[string]bool
}

// NewBasicAuthenticator creates a new basic authenticator
func NewBasicAuthenticator(passwords []string) *BasicAuthenticator {
	validPwds := make(map[string]bool)
	for _, pwd := range passwords {
		validPwds[pwd] = true
	}
	return &BasicAuthenticator{
		validPasswords: validPwds,
	}
}

// Authenticate checks if the provided password is valid
func (a *BasicAuthenticator) Authenticate(password string) error {
	if password == "" || !a.validPasswords[password] {
		return ErrInvalidCredentials
	}
	return nil
}
