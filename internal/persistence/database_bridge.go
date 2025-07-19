// Package persistence provides database bridge interfaces.
package persistence

import (
	"context"

	"github.com/scintirete/scintirete/internal/persistence/rdb"
	"github.com/scintirete/scintirete/pkg/types"
)

// DatabaseEngine represents the interface that the database engine must implement
// for persistence operations
type DatabaseEngine interface {
	// Snapshot operations
	GetDatabaseState(ctx context.Context) (map[string]rdb.DatabaseState, error)
	RestoreFromSnapshot(ctx context.Context, snapshot *rdb.RDBSnapshot) error

	// Command replay operations
	ApplyCommand(ctx context.Context, command types.AOFCommand) error

	// AOF rewrite operations
	GetOptimizedCommands(ctx context.Context) ([]types.AOFCommand, error)
}

// CommandApplier handles applying commands to the database engine
type CommandApplier struct {
	engine DatabaseEngine
}

// NewCommandApplier creates a new command applier
func NewCommandApplier(engine DatabaseEngine) *CommandApplier {
	return &CommandApplier{
		engine: engine,
	}
}

// ApplySnapshot applies an RDB snapshot to the database engine
func (ca *CommandApplier) ApplySnapshot(ctx context.Context, snapshot *rdb.RDBSnapshot) error {
	if snapshot == nil {
		return nil // Nothing to apply
	}

	return ca.engine.RestoreFromSnapshot(ctx, snapshot)
}

// ApplyCommand applies a single AOF command to the database engine
func (ca *CommandApplier) ApplyCommand(ctx context.Context, command types.AOFCommand) error {
	return ca.engine.ApplyCommand(ctx, command)
}

// GetDatabaseState gets the current state of all databases for snapshotting
func (ca *CommandApplier) GetDatabaseState(ctx context.Context) (map[string]rdb.DatabaseState, error) {
	return ca.engine.GetDatabaseState(ctx)
}

// GetOptimizedCommands gets a list of optimized commands for AOF rewrite
func (ca *CommandApplier) GetOptimizedCommands(ctx context.Context) ([]types.AOFCommand, error) {
	return ca.engine.GetOptimizedCommands(ctx)
}
