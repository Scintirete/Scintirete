// Package audit provides audit logging functionality for Scintirete.
package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AuditLevel represents different types of audit events
type AuditLevel string

const (
	AuditLevelOperation AuditLevel = "OPERATION" // Database operations
	AuditLevelAccess    AuditLevel = "ACCESS"    // Access attempts
	AuditLevelSecurity  AuditLevel = "SECURITY"  // Security-related events
)

// AuditEvent represents a single audit event
type AuditEvent struct {
	Timestamp  string                 `json:"timestamp"`
	Level      AuditLevel             `json:"level"`
	Operation  string                 `json:"operation"`
	Database   string                 `json:"database,omitempty"`
	Collection string                 `json:"collection,omitempty"`
	UserID     string                 `json:"user_id,omitempty"`
	Resource   string                 `json:"resource,omitempty"`
	Success    bool                   `json:"success"`
	Duration   int64                  `json:"duration_ms,omitempty"` // Duration in milliseconds
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	Error      string                 `json:"error,omitempty"`
}

// Config contains audit logger configuration
type Config struct {
	Enabled    bool      // Whether audit logging is enabled
	OutputPath string    // File path for audit logs (empty = stdout)
	MaxSize    int64     // Maximum size of audit log file in bytes before rotation
	MaxFiles   int       // Maximum number of rotated files to keep
	output     io.Writer // Internal output writer
}

// Logger implements the core.AuditLogger interface
type Logger struct {
	mu     sync.Mutex
	config Config
	file   *os.File // File handle for audit log file
}

// NewLogger creates a new audit logger with the given configuration
func NewLogger(config Config) (*Logger, error) {
	logger := &Logger{
		config: config,
	}

	if !config.Enabled {
		return logger, nil
	}

	// Set up output
	if config.OutputPath != "" {
		// Ensure directory exists
		dir := filepath.Dir(config.OutputPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create audit log directory: %w", err)
		}

		// Open or create audit log file
		file, err := os.OpenFile(config.OutputPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open audit log file: %w", err)
		}

		logger.file = file
		logger.config.output = file
	} else {
		// Use stdout if no file path is specified
		logger.config.output = os.Stdout
	}

	return logger, nil
}

// LogOperation logs database operations for audit purposes
func (l *Logger) LogOperation(ctx context.Context, operation, database, collection, userID string, metadata map[string]interface{}) {
	if !l.config.Enabled {
		return
	}

	event := AuditEvent{
		Timestamp:  time.Now().UTC().Format(time.RFC3339Nano),
		Level:      AuditLevelOperation,
		Operation:  operation,
		Database:   database,
		Collection: collection,
		UserID:     userID,
		Success:    true, // Assume success unless error is provided
		Metadata:   metadata,
	}

	// Extract duration from metadata if available
	if metadata != nil {
		if duration, ok := metadata["duration_ms"]; ok {
			if d, ok := duration.(int64); ok {
				event.Duration = d
			}
		}
	}

	l.writeEvent(event)
}

// LogAccess logs access attempts (successful and failed)
func (l *Logger) LogAccess(ctx context.Context, userID, operation, resource string, success bool, metadata map[string]interface{}) {
	if !l.config.Enabled {
		return
	}

	event := AuditEvent{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Level:     AuditLevelAccess,
		Operation: operation,
		UserID:    userID,
		Resource:  resource,
		Success:   success,
		Metadata:  metadata,
	}

	// If not successful, include error message from metadata
	if !success && metadata != nil {
		if errMsg, ok := metadata["error"]; ok {
			if err, ok := errMsg.(string); ok {
				event.Error = err
			}
		}
	}

	l.writeEvent(event)
}

// LogSecurityEvent logs security-related events
func (l *Logger) LogSecurityEvent(ctx context.Context, operation, userID string, metadata map[string]interface{}) {
	if !l.config.Enabled {
		return
	}

	event := AuditEvent{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Level:     AuditLevelSecurity,
		Operation: operation,
		UserID:    userID,
		Success:   true,
		Metadata:  metadata,
	}

	l.writeEvent(event)
}

// writeEvent writes an audit event to the configured output
func (l *Logger) writeEvent(event AuditEvent) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.config.output == nil {
		return
	}

	// Serialize event to JSON
	data, err := json.Marshal(event)
	if err != nil {
		// If we can't serialize, write a simple error message
		fallback := fmt.Sprintf(`{"timestamp":"%s","level":"ERROR","operation":"audit_error","error":"failed to serialize audit event: %s"}`,
			time.Now().UTC().Format(time.RFC3339Nano), err.Error())
		fmt.Fprintln(l.config.output, fallback)
		return
	}

	// Write JSON line
	fmt.Fprintln(l.config.output, string(data))

	// Check if we need to rotate the file
	if l.file != nil && l.config.MaxSize > 0 {
		if info, err := l.file.Stat(); err == nil && info.Size() > l.config.MaxSize {
			l.rotateFile()
		}
	}
}

// rotateFile rotates the audit log file when it becomes too large
func (l *Logger) rotateFile() {
	if l.file == nil || l.config.OutputPath == "" {
		return
	}

	// Close current file
	l.file.Close()

	// Rotate existing files
	for i := l.config.MaxFiles - 1; i > 0; i-- {
		oldPath := fmt.Sprintf("%s.%d", l.config.OutputPath, i)
		newPath := fmt.Sprintf("%s.%d", l.config.OutputPath, i+1)

		if i == l.config.MaxFiles-1 {
			// Remove the oldest file
			os.Remove(newPath)
		}

		// Rename file
		os.Rename(oldPath, newPath)
	}

	// Move current file to .1
	os.Rename(l.config.OutputPath, l.config.OutputPath+".1")

	// Create new file
	file, err := os.OpenFile(l.config.OutputPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		// Fallback to stdout if we can't create new file
		l.config.output = os.Stdout
		l.file = nil
		return
	}

	l.file = file
	l.config.output = file
}

// Close closes the audit logger and any open files
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		err := l.file.Close()
		l.file = nil
		return err
	}

	return nil
}

// IsEnabled returns whether audit logging is enabled
func (l *Logger) IsEnabled() bool {
	return l.config.Enabled
}

// GetConfig returns the current audit logger configuration
func (l *Logger) GetConfig() Config {
	return l.config
}

// Helper functions for common audit events

// LogDatabaseOperation logs database-level operations (create, drop, list)
func (l *Logger) LogDatabaseOperation(ctx context.Context, operation, database, userID string, success bool, duration time.Duration, err error) {
	metadata := map[string]interface{}{
		"duration_ms": duration.Milliseconds(),
	}

	if err != nil {
		metadata["error"] = err.Error()
		success = false
	}

	l.LogOperation(ctx, operation, database, "", userID, metadata)
}

// LogCollectionOperation logs collection-level operations (create, drop, list, info)
func (l *Logger) LogCollectionOperation(ctx context.Context, operation, database, collection, userID string, success bool, duration time.Duration, err error) {
	metadata := map[string]interface{}{
		"duration_ms": duration.Milliseconds(),
	}

	if err != nil {
		metadata["error"] = err.Error()
		success = false
	}

	l.LogOperation(ctx, operation, database, collection, userID, metadata)
}

// LogVectorOperation logs vector-level operations (insert, delete, search)
func (l *Logger) LogVectorOperation(ctx context.Context, operation, database, collection, userID string, vectorCount int64, success bool, duration time.Duration, err error) {
	metadata := map[string]interface{}{
		"duration_ms":  duration.Milliseconds(),
		"vector_count": vectorCount,
	}

	if err != nil {
		metadata["error"] = err.Error()
		success = false
	}

	l.LogOperation(ctx, operation, database, collection, userID, metadata)
}
