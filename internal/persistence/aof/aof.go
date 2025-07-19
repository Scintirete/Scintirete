// Package aof provides Append-Only File logging for Scintirete.
package aof

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/scintirete/scintirete/internal/utils"
	"github.com/scintirete/scintirete/pkg/types"
)

// SyncStrategy defines how frequently AOF data is synced to disk
type SyncStrategy string

const (
	SyncAlways   SyncStrategy = "always"   // Sync after every write
	SyncEverySec SyncStrategy = "everysec" // Sync every second
	SyncNo       SyncStrategy = "no"       // Let OS decide when to sync
)

// AOFLogger handles append-only file logging
type AOFLogger struct {
	mu           sync.Mutex
	file         *os.File
	writer       *bufio.Writer
	filePath     string
	syncStrategy SyncStrategy
	
	// Background sync for everysec strategy
	syncTicker   *time.Ticker
	stopSync     chan struct{}
	syncWG       sync.WaitGroup
	
	// Statistics
	commandCount int64
	lastSync     time.Time
}

// NewAOFLogger creates a new AOF logger
func NewAOFLogger(filePath string, syncStrategy SyncStrategy) (*AOFLogger, error) {
	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, utils.ErrPersistenceFailedWithCause("failed to create AOF directory", err)
	}
	
	// Open file for append
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, utils.ErrPersistenceFailedWithCause("failed to open AOF file", err)
	}
	
	logger := &AOFLogger{
		file:         file,
		writer:       bufio.NewWriter(file),
		filePath:     filePath,
		syncStrategy: syncStrategy,
		stopSync:     make(chan struct{}),
		lastSync:     time.Now(),
	}
	
	// Start background sync if needed
	if syncStrategy == SyncEverySec {
		logger.startBackgroundSync()
	}
	
	return logger, nil
}

// WriteCommand writes a command to the AOF log
func (a *AOFLogger) WriteCommand(ctx context.Context, command types.AOFCommand) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	// Set timestamp if not provided
	if command.Timestamp.IsZero() {
		command.Timestamp = time.Now()
	}
	
	// Serialize command to JSON
	data, err := json.Marshal(command)
	if err != nil {
		return utils.ErrPersistenceFailedWithCause("failed to serialize AOF command", err)
	}
	
	// Write to buffer with newline
	if _, err := a.writer.Write(data); err != nil {
		return utils.ErrPersistenceFailedWithCause("failed to write AOF command", err)
	}
	if err := a.writer.WriteByte('\n'); err != nil {
		return utils.ErrPersistenceFailedWithCause("failed to write AOF newline", err)
	}
	
	a.commandCount++
	
	// Sync based on strategy
	switch a.syncStrategy {
	case SyncAlways:
		if err := a.syncToFile(); err != nil {
			return err
		}
	case SyncEverySec:
		// Background sync handles this
	case SyncNo:
		// OS will sync when it wants
	}
	
	return nil
}

// Replay reads and replays all commands from the AOF file
func (a *AOFLogger) Replay(ctx context.Context, handler func(types.AOFCommand) error) error {
	// Close current file handle for reading
	a.mu.Lock()
	if a.file != nil {
		a.syncToFile() // Ensure any buffered data is written
	}
	a.mu.Unlock()
	
	// Open file for reading
	file, err := os.Open(a.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No AOF file exists yet, that's OK
		}
		return utils.ErrRecoveryFailed("failed to open AOF file for replay: " + err.Error())
	}
	defer file.Close()
	
	scanner := bufio.NewScanner(file)
	lineNum := 0
	
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		
		// Skip empty lines
		if len(line) == 0 {
			continue
		}
		
		// Parse command
		var command types.AOFCommand
		if err := json.Unmarshal([]byte(line), &command); err != nil {
			return utils.ErrCorruptedData(fmt.Sprintf("invalid AOF command at line %d: %v", lineNum, err))
		}
		
		// Execute command
		if err := handler(command); err != nil {
			return utils.ErrRecoveryFailed(fmt.Sprintf("failed to replay AOF command at line %d: %v", lineNum, err))
		}
		
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
	
	if err := scanner.Err(); err != nil {
		return utils.ErrCorruptedData("failed to read AOF file: " + err.Error())
	}
	
	return nil
}

// Rewrite creates a new AOF file with optimized commands
func (a *AOFLogger) Rewrite(ctx context.Context, snapshotCommands []types.AOFCommand) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	// Create temporary file
	tempPath := a.filePath + ".tmp"
	tempFile, err := os.Create(tempPath)
	if err != nil {
		return utils.ErrPersistenceFailedWithCause("failed to create temporary AOF file", err)
	}
	defer func() {
		tempFile.Close()
		os.Remove(tempPath) // Clean up on error
	}()
	
	writer := bufio.NewWriter(tempFile)
	
	// Write optimized commands
	for _, command := range snapshotCommands {
		data, err := json.Marshal(command)
		if err != nil {
			return utils.ErrPersistenceFailedWithCause("failed to serialize rewrite command", err)
		}
		
		if _, err := writer.Write(data); err != nil {
			return utils.ErrPersistenceFailedWithCause("failed to write rewrite command", err)
		}
		if err := writer.WriteByte('\n'); err != nil {
			return utils.ErrPersistenceFailedWithCause("failed to write rewrite newline", err)
		}
		
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
	
	// Flush and sync
	if err := writer.Flush(); err != nil {
		return utils.ErrPersistenceFailedWithCause("failed to flush rewrite buffer", err)
	}
	if err := tempFile.Sync(); err != nil {
		return utils.ErrPersistenceFailedWithCause("failed to sync rewrite file", err)
	}
	if err := tempFile.Close(); err != nil {
		return utils.ErrPersistenceFailedWithCause("failed to close rewrite file", err)
	}
	
	// Close current file
	if err := a.syncToFile(); err != nil {
		return err
	}
	if err := a.file.Close(); err != nil {
		return utils.ErrPersistenceFailedWithCause("failed to close current AOF file", err)
	}
	
	// Replace old file with new one
	if err := os.Rename(tempPath, a.filePath); err != nil {
		return utils.ErrPersistenceFailedWithCause("failed to replace AOF file", err)
	}
	
	// Reopen file for writing
	a.file, err = os.OpenFile(a.filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return utils.ErrPersistenceFailedWithCause("failed to reopen AOF file after rewrite", err)
	}
	a.writer = bufio.NewWriter(a.file)
	a.commandCount = int64(len(snapshotCommands))
	
	return nil
}

// Truncate removes all content from the AOF file
func (a *AOFLogger) Truncate() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	// Close current file
	if err := a.file.Close(); err != nil {
		return utils.ErrPersistenceFailedWithCause("failed to close AOF file for truncation", err)
	}
	
	// Recreate empty file
	file, err := os.Create(a.filePath)
	if err != nil {
		return utils.ErrPersistenceFailedWithCause("failed to recreate AOF file", err)
	}
	
	a.file = file
	a.writer = bufio.NewWriter(file)
	a.commandCount = 0
	
	return nil
}

// Close closes the AOF logger and stops background sync
func (a *AOFLogger) Close() error {
	// Stop background sync
	if a.syncTicker != nil {
		close(a.stopSync)
		a.syncWG.Wait()
	}
	
	a.mu.Lock()
	defer a.mu.Unlock()
	
	// Final sync and close
	if err := a.syncToFile(); err != nil {
		return err
	}
	
	return a.file.Close()
}

// GetStats returns AOF statistics
func (a *AOFLogger) GetStats() AOFStats {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	fileInfo, _ := a.file.Stat()
	fileSize := int64(0)
	if fileInfo != nil {
		fileSize = fileInfo.Size()
	}
	
	return AOFStats{
		CommandCount: a.commandCount,
		FileSize:     fileSize,
		LastSync:     a.lastSync,
		SyncStrategy: string(a.syncStrategy),
	}
}

// AOFStats contains statistics about the AOF log
type AOFStats struct {
	CommandCount int64     `json:"command_count"`
	FileSize     int64     `json:"file_size"`
	LastSync     time.Time `json:"last_sync"`
	SyncStrategy string    `json:"sync_strategy"`
}

// Private methods

// syncToFile flushes the buffer and syncs to disk
func (a *AOFLogger) syncToFile() error {
	if err := a.writer.Flush(); err != nil {
		return utils.ErrPersistenceFailedWithCause("failed to flush AOF buffer", err)
	}
	if err := a.file.Sync(); err != nil {
		return utils.ErrPersistenceFailedWithCause("failed to sync AOF file", err)
	}
	a.lastSync = time.Now()
	return nil
}

// startBackgroundSync starts the background sync goroutine for everysec strategy
func (a *AOFLogger) startBackgroundSync() {
	a.syncTicker = time.NewTicker(time.Second)
	a.syncWG.Add(1)
	
	go func() {
		defer a.syncWG.Done()
		defer a.syncTicker.Stop()
		
		for {
			select {
			case <-a.syncTicker.C:
				a.mu.Lock()
				a.syncToFile() // Ignore errors in background sync
				a.mu.Unlock()
			case <-a.stopSync:
				return
			}
		}
	}()
}

// CommandBuilder helps build AOF commands for different operations
type CommandBuilder struct{}

// NewCommandBuilder creates a new command builder
func NewCommandBuilder() *CommandBuilder {
	return &CommandBuilder{}
}

// CreateDatabase builds a command for database creation
func (cb *CommandBuilder) CreateDatabase(dbName string) types.AOFCommand {
	return types.AOFCommand{
		Timestamp: time.Now(),
		Command:   "CREATE_DATABASE",
		Args: map[string]interface{}{
			"name": dbName,
		},
		Database: dbName,
	}
}

// DropDatabase builds a command for database deletion
func (cb *CommandBuilder) DropDatabase(dbName string) types.AOFCommand {
	return types.AOFCommand{
		Timestamp: time.Now(),
		Command:   "DROP_DATABASE",
		Args: map[string]interface{}{
			"name": dbName,
		},
		Database: dbName,
	}
}

// CreateCollection builds a command for collection creation
func (cb *CommandBuilder) CreateCollection(dbName, collName string, config types.CollectionConfig) types.AOFCommand {
	return types.AOFCommand{
		Timestamp:  time.Now(),
		Command:    "CREATE_COLLECTION",
		Args: map[string]interface{}{
			"name":   collName,
			"config": config,
		},
		Database:   dbName,
		Collection: collName,
	}
}

// DropCollection builds a command for collection deletion
func (cb *CommandBuilder) DropCollection(dbName, collName string) types.AOFCommand {
	return types.AOFCommand{
		Timestamp: time.Now(),
		Command:   "DROP_COLLECTION",
		Args: map[string]interface{}{
			"name": collName,
		},
		Database:   dbName,
		Collection: collName,
	}
}

// InsertVectors builds a command for vector insertion
func (cb *CommandBuilder) InsertVectors(dbName, collName string, vectors []types.Vector) types.AOFCommand {
	return types.AOFCommand{
		Timestamp: time.Now(),
		Command:   "INSERT_VECTORS",
		Args: map[string]interface{}{
			"vectors": vectors,
		},
		Database:   dbName,
		Collection: collName,
	}
}

// DeleteVectors builds a command for vector deletion
func (cb *CommandBuilder) DeleteVectors(dbName, collName string, ids []string) types.AOFCommand {
	return types.AOFCommand{
		Timestamp: time.Now(),
		Command:   "DELETE_VECTORS",
		Args: map[string]interface{}{
			"ids": ids,
		},
		Database:   dbName,
		Collection: collName,
	}
} 