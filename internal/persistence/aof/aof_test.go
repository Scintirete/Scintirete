package aof

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/scintirete/scintirete/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAOFLogger_WriteAndReplay(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.aof")

	// Create logger
	logger, err := NewAOFLogger(filePath, SyncAlways)
	require.NoError(t, err)
	defer logger.Close()

	// Create test commands
	commands := []types.AOFCommand{
		{
			Timestamp:  time.Now(),
			Command:    "CREATE_DATABASE",
			Args:       map[string]interface{}{"name": "test_db"},
			Database:   "test_db",
			Collection: "",
		},
		{
			Timestamp: time.Now(),
			Command:   "CREATE_COLLECTION",
			Args: map[string]interface{}{
				"name": "test_collection",
				"config": types.CollectionConfig{
					Name:   "test_collection",
					Metric: types.DistanceMetricL2,
					HNSWParams: types.HNSWParams{
						M:              16,
						EfConstruction: 200,
						EfSearch:       50,
						MaxLayers:      16,
						Seed:           12345,
					},
				},
			},
			Database:   "test_db",
			Collection: "test_collection",
		},
		{
			Timestamp: time.Now(),
			Command:   "INSERT_VECTORS",
			Args: map[string]interface{}{
				"vectors": []types.Vector{
					{
						ID:       1,
						Elements: []float32{1.0, 2.0, 3.0},
						Metadata: map[string]interface{}{"label": "test"},
					},
					{
						ID:       2,
						Elements: []float32{4.0, 5.0, 6.0},
						Metadata: map[string]interface{}{"label": "test2"},
					},
				},
			},
			Database:   "test_db",
			Collection: "test_collection",
		},
		{
			Timestamp: time.Now(),
			Command:   "DELETE_VECTORS",
			Args: map[string]interface{}{
				"ids": []string{"vector1"},
			},
			Database:   "test_db",
			Collection: "test_collection",
		},
		{
			Timestamp:  time.Now(),
			Command:    "DROP_COLLECTION",
			Args:       map[string]interface{}{"name": "test_collection"},
			Database:   "test_db",
			Collection: "test_collection",
		},
		{
			Timestamp:  time.Now(),
			Command:    "DROP_DATABASE",
			Args:       map[string]interface{}{"name": "test_db"},
			Database:   "test_db",
			Collection: "",
		},
	}

	// Write commands
	ctx := context.Background()
	for _, command := range commands {
		err := logger.WriteCommand(ctx, command)
		require.NoError(t, err)
	}

	// Close logger to ensure all data is flushed
	err = logger.Close()
	require.NoError(t, err)

	// Create new logger for replay
	replayLogger, err := NewAOFLogger(filePath, SyncAlways)
	require.NoError(t, err)
	defer replayLogger.Close()

	// Replay commands
	var replayedCommands []types.AOFCommand
	err = replayLogger.Replay(ctx, func(command types.AOFCommand) error {
		replayedCommands = append(replayedCommands, command)
		return nil
	})
	require.NoError(t, err)

	// Verify replayed commands
	assert.Len(t, replayedCommands, len(commands))

	for i, original := range commands {
		replayed := replayedCommands[i]

		// Verify basic command properties
		assert.Equal(t, original.Command, replayed.Command)
		assert.Equal(t, original.Database, replayed.Database)
		assert.Equal(t, original.Collection, replayed.Collection)
		assert.Equal(t, original.Timestamp.Unix(), replayed.Timestamp.Unix())

		// Verify args are present (detailed verification would depend on full implementation)
		assert.NotNil(t, replayed.Args)
	}
}

func TestAOFLogger_Stats(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.aof")

	// Create logger
	logger, err := NewAOFLogger(filePath, SyncAlways)
	require.NoError(t, err)
	defer logger.Close()

	// Get initial stats
	stats := logger.GetStats()
	assert.Equal(t, int64(0), stats.CommandCount)
	assert.Equal(t, int64(0), stats.FileSize)
	assert.Equal(t, "always", stats.SyncStrategy)

	// Write a command
	ctx := context.Background()
	command := types.AOFCommand{
		Timestamp:  time.Now(),
		Command:    "CREATE_DATABASE",
		Args:       map[string]interface{}{"name": "test_db"},
		Database:   "test_db",
		Collection: "",
	}

	err = logger.WriteCommand(ctx, command)
	require.NoError(t, err)

	// Get updated stats
	stats = logger.GetStats()
	assert.Equal(t, int64(1), stats.CommandCount)
	assert.Greater(t, stats.FileSize, int64(0))
}

func TestAOFLogger_Truncate(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.aof")

	// Create logger
	logger, err := NewAOFLogger(filePath, SyncAlways)
	require.NoError(t, err)
	defer logger.Close()

	// Write a command
	ctx := context.Background()
	command := types.AOFCommand{
		Timestamp:  time.Now(),
		Command:    "CREATE_DATABASE",
		Args:       map[string]interface{}{"name": "test_db"},
		Database:   "test_db",
		Collection: "",
	}

	err = logger.WriteCommand(ctx, command)
	require.NoError(t, err)

	// Verify command count
	stats := logger.GetStats()
	assert.Equal(t, int64(1), stats.CommandCount)

	// Truncate file
	err = logger.Truncate()
	require.NoError(t, err)

	// Verify file is empty
	stats = logger.GetStats()
	assert.Equal(t, int64(0), stats.CommandCount)
	assert.Equal(t, int64(0), stats.FileSize)

	// Verify replay returns no commands
	var replayedCommands []types.AOFCommand
	err = logger.Replay(ctx, func(command types.AOFCommand) error {
		replayedCommands = append(replayedCommands, command)
		return nil
	})
	require.NoError(t, err)
	assert.Len(t, replayedCommands, 0)
}

func TestAOFLogger_Rewrite(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.aof")

	// Create logger
	logger, err := NewAOFLogger(filePath, SyncAlways)
	require.NoError(t, err)
	defer logger.Close()

	// Write multiple commands
	ctx := context.Background()
	commands := []types.AOFCommand{
		{
			Timestamp:  time.Now(),
			Command:    "CREATE_DATABASE",
			Args:       map[string]interface{}{"name": "test_db"},
			Database:   "test_db",
			Collection: "",
		},
		{
			Timestamp:  time.Now(),
			Command:    "CREATE_DATABASE", // Duplicate command
			Args:       map[string]interface{}{"name": "test_db"},
			Database:   "test_db",
			Collection: "",
		},
		{
			Timestamp: time.Now(),
			Command:   "CREATE_COLLECTION",
			Args: map[string]interface{}{
				"name": "test_collection",
				"config": types.CollectionConfig{
					Name:       "test_collection",
					Metric:     types.DistanceMetricL2,
					HNSWParams: types.DefaultHNSWParams(),
				},
			},
			Database:   "test_db",
			Collection: "test_collection",
		},
	}

	for _, command := range commands {
		err := logger.WriteCommand(ctx, command)
		require.NoError(t, err)
	}

	// Verify initial command count
	stats := logger.GetStats()
	assert.Equal(t, int64(3), stats.CommandCount)

	// Rewrite with optimized commands (remove duplicate)
	optimizedCommands := []types.AOFCommand{
		commands[0], // Only first CREATE_DATABASE
		commands[2], // CREATE_COLLECTION
	}

	err = logger.Rewrite(ctx, optimizedCommands)
	require.NoError(t, err)

	// Verify new command count
	stats = logger.GetStats()
	assert.Equal(t, int64(2), stats.CommandCount)

	// Verify replay returns optimized commands
	var replayedCommands []types.AOFCommand
	err = logger.Replay(ctx, func(command types.AOFCommand) error {
		replayedCommands = append(replayedCommands, command)
		return nil
	})
	require.NoError(t, err)
	assert.Len(t, replayedCommands, 2)
}

func TestAOFLogger_SyncStrategies(t *testing.T) {
	// Test different sync strategies
	strategies := []SyncStrategy{SyncAlways, SyncEverySec, SyncNo}

	for _, strategy := range strategies {
		t.Run(string(strategy), func(t *testing.T) {
			// Create temporary directory
			tempDir := t.TempDir()
			filePath := filepath.Join(tempDir, "test.aof")

			// Create logger with strategy
			logger, err := NewAOFLogger(filePath, strategy)
			require.NoError(t, err)
			defer logger.Close()

			// Verify sync strategy in stats
			stats := logger.GetStats()
			assert.Equal(t, string(strategy), stats.SyncStrategy)

			// Write a command
			ctx := context.Background()
			command := types.AOFCommand{
				Timestamp:  time.Now(),
				Command:    "CREATE_DATABASE",
				Args:       map[string]interface{}{"name": "test_db"},
				Database:   "test_db",
				Collection: "",
			}

			err = logger.WriteCommand(ctx, command)
			require.NoError(t, err)

			// Command should be written regardless of sync strategy
			stats = logger.GetStats()
			assert.Equal(t, int64(1), stats.CommandCount)
		})
	}
}
