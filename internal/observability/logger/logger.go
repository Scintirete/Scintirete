// Package logger provides structured logging functionality for Scintirete.
package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/scintirete/scintirete/internal/core"
)

// LogLevel represents the severity level of a log message
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)

// String returns the string representation of a log level
func (l LogLevel) String() string {
	switch l {
	case LogLevelDebug:
		return "DEBUG"
	case LogLevelInfo:
		return "INFO"
	case LogLevelWarn:
		return "WARN"
	case LogLevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// LogFormat represents the output format of log messages
type LogFormat int

const (
	LogFormatText LogFormat = iota
	LogFormatJSON
)

// Config contains logger configuration
type Config struct {
	Level  LogLevel  // Minimum log level to output
	Format LogFormat // Output format (text or JSON)
	Output io.Writer // Output destination (default: os.Stdout)
}

// StructuredLogger implements the core.Logger interface
type StructuredLogger struct {
	config Config
	fields map[string]interface{} // Additional fields for this logger instance
}

// LogEntry represents a single log entry
type LogEntry struct {
	Timestamp string                 `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
	Error     string                 `json:"error,omitempty"`
}

// NewStructuredLogger creates a new structured logger with the given configuration
func NewStructuredLogger(config Config) *StructuredLogger {
	if config.Output == nil {
		config.Output = os.Stdout
	}

	return &StructuredLogger{
		config: config,
		fields: make(map[string]interface{}),
	}
}

// NewFromConfigString creates a logger from string configuration
func NewFromConfigString(level, format string) (*StructuredLogger, error) {
	config := Config{}

	// Parse level
	switch level {
	case "debug":
		config.Level = LogLevelDebug
	case "info":
		config.Level = LogLevelInfo
	case "warn":
		config.Level = LogLevelWarn
	case "error":
		config.Level = LogLevelError
	default:
		return nil, fmt.Errorf("invalid log level: %s", level)
	}

	// Parse format
	switch format {
	case "text":
		config.Format = LogFormatText
	case "json":
		config.Format = LogFormatJSON
	default:
		return nil, fmt.Errorf("invalid log format: %s", format)
	}

	return NewStructuredLogger(config), nil
}

// Debug logs debug-level messages
func (l *StructuredLogger) Debug(ctx context.Context, message string, fields map[string]interface{}) {
	l.log(LogLevelDebug, message, "", fields)
}

// Info logs info-level messages
func (l *StructuredLogger) Info(ctx context.Context, message string, fields map[string]interface{}) {
	l.log(LogLevelInfo, message, "", fields)
}

// Warn logs warning-level messages
func (l *StructuredLogger) Warn(ctx context.Context, message string, fields map[string]interface{}) {
	l.log(LogLevelWarn, message, "", fields)
}

// Error logs error-level messages
func (l *StructuredLogger) Error(ctx context.Context, message string, err error, fields map[string]interface{}) {
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	l.log(LogLevelError, message, errMsg, fields)
}

// WithFields returns a logger with additional fields
func (l *StructuredLogger) WithFields(fields map[string]interface{}) core.Logger {
	newFields := make(map[string]interface{})

	// Copy existing fields
	for k, v := range l.fields {
		newFields[k] = v
	}

	// Add new fields
	for k, v := range fields {
		newFields[k] = v
	}

	return &StructuredLogger{
		config: l.config,
		fields: newFields,
	}
}

// log is the internal logging method
func (l *StructuredLogger) log(level LogLevel, message, errorMsg string, fields map[string]interface{}) {
	// Check if we should log this level
	if level < l.config.Level {
		return
	}

	// Create log entry
	entry := LogEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Level:     level.String(),
		Message:   message,
		Fields:    l.mergeFields(fields),
	}

	if errorMsg != "" {
		entry.Error = errorMsg
	}

	// Format and write output
	switch l.config.Format {
	case LogFormatJSON:
		l.writeJSON(entry)
	case LogFormatText:
		l.writeText(entry)
	}
}

// mergeFields combines logger fields with provided fields
func (l *StructuredLogger) mergeFields(fields map[string]interface{}) map[string]interface{} {
	if len(l.fields) == 0 && len(fields) == 0 {
		return nil
	}

	merged := make(map[string]interface{})

	// Add logger fields first
	for k, v := range l.fields {
		merged[k] = v
	}

	// Add provided fields (will override logger fields if same key)
	for k, v := range fields {
		merged[k] = v
	}

	return merged
}

// writeJSON writes the log entry in JSON format
func (l *StructuredLogger) writeJSON(entry LogEntry) {
	data, err := json.Marshal(entry)
	if err != nil {
		// Fallback to text format if JSON marshaling fails
		l.writeText(entry)
		return
	}

	fmt.Fprintln(l.config.Output, string(data))
}

// writeText writes the log entry in human-readable text format
func (l *StructuredLogger) writeText(entry LogEntry) {
	output := fmt.Sprintf("[%s] %s %s", entry.Timestamp, entry.Level, entry.Message)

	if entry.Error != "" {
		output += fmt.Sprintf(" error=%s", entry.Error)
	}

	if len(entry.Fields) > 0 {
		for k, v := range entry.Fields {
			output += fmt.Sprintf(" %s=%v", k, v)
		}
	}

	fmt.Fprintln(l.config.Output, output)
}

// SetLevel dynamically changes the log level
func (l *StructuredLogger) SetLevel(level LogLevel) {
	l.config.Level = level
}

// GetLevel returns the current log level
func (l *StructuredLogger) GetLevel() LogLevel {
	return l.config.Level
}
