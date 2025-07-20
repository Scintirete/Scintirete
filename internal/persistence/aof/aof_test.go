package aof

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/scintirete/scintirete/pkg/types"
)

func TestNewAOFLogger(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.aof")

	logger, err := NewAOFLogger(filePath, SyncAlways)
	if err != nil {
		t.Fatalf("NewAOFLogger failed: %v", err)
	}
	defer logger.Close()

	if logger.filePath != filePath {
		t.Errorf("filePath = %s, want %s", logger.filePath, filePath)
	}

	if logger.syncStrategy != SyncAlways {
		t.Errorf("syncStrategy = %v, want %v", logger.syncStrategy, SyncAlways)
	}
}

func TestAOFLogger_WriteCommand(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.aof")

	logger, err := NewAOFLogger(filePath, SyncAlways)
	if err != nil {
		t.Fatalf("NewAOFLogger failed: %v", err)
	}
	defer logger.Close()

	command := types.AOFCommand{
		Command:  "CREATE_DATABASE",
		Args:     map[string]interface{}{"name": "test_db"},
		Database: "test_db",
	}

	err = logger.WriteCommand(context.Background(), command)
	if err != nil {
		t.Errorf("WriteCommand failed: %v", err)
	}

	stats := logger.GetStats()
	if stats.CommandCount != 1 {
		t.Errorf("CommandCount = %d, want 1", stats.CommandCount)
	}
}

func TestAOFLogger_Replay(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.aof")

	// Write some commands
	logger, err := NewAOFLogger(filePath, SyncAlways)
	if err != nil {
		t.Fatalf("NewAOFLogger failed: %v", err)
	}

	commands := []types.AOFCommand{
		{Command: "CREATE_DATABASE", Args: map[string]interface{}{"name": "db1"}, Database: "db1"},
		{Command: "CREATE_DATABASE", Args: map[string]interface{}{"name": "db2"}, Database: "db2"},
		{Command: "CREATE_COLLECTION", Args: map[string]interface{}{"name": "coll1"}, Database: "db1", Collection: "coll1"},
	}

	for _, cmd := range commands {
		if err := logger.WriteCommand(context.Background(), cmd); err != nil {
			t.Fatalf("WriteCommand failed: %v", err)
		}
	}
	logger.Close()

	// Replay commands
	logger2, err := NewAOFLogger(filePath, SyncAlways)
	if err != nil {
		t.Fatalf("NewAOFLogger failed: %v", err)
	}
	defer logger2.Close()

	var replayedCommands []types.AOFCommand
	err = logger2.Replay(context.Background(), func(cmd types.AOFCommand) error {
		replayedCommands = append(replayedCommands, cmd)
		return nil
	})

	if err != nil {
		t.Errorf("Replay failed: %v", err)
	}

	if len(replayedCommands) != len(commands) {
		t.Errorf("Replayed %d commands, want %d", len(replayedCommands), len(commands))
	}

	for i, cmd := range replayedCommands {
		if cmd.Command != commands[i].Command {
			t.Errorf("Command[%d] = %s, want %s", i, cmd.Command, commands[i].Command)
		}
		if cmd.Database != commands[i].Database {
			t.Errorf("Database[%d] = %s, want %s", i, cmd.Database, commands[i].Database)
		}
	}
}

func TestAOFLogger_Rewrite(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.aof")

	logger, err := NewAOFLogger(filePath, SyncAlways)
	if err != nil {
		t.Fatalf("NewAOFLogger failed: %v", err)
	}
	defer logger.Close()

	// Write original commands
	originalCommands := []types.AOFCommand{
		{Command: "CREATE_DATABASE", Args: map[string]interface{}{"name": "db1"}},
		{Command: "INSERT_VECTORS", Args: map[string]interface{}{"vectors": []string{"v1", "v2"}}},
		{Command: "DELETE_VECTORS", Args: map[string]interface{}{"ids": []string{"v1"}}},
	}

	for _, cmd := range originalCommands {
		if err := logger.WriteCommand(context.Background(), cmd); err != nil {
			t.Fatalf("WriteCommand failed: %v", err)
		}
	}

	// Rewrite with optimized commands
	optimizedCommands := []types.AOFCommand{
		{Command: "CREATE_DATABASE", Args: map[string]interface{}{"name": "db1"}},
		{Command: "INSERT_VECTORS", Args: map[string]interface{}{"vectors": []string{"v2"}}}, // v1 was deleted
	}

	err = logger.Rewrite(context.Background(), optimizedCommands)
	if err != nil {
		t.Errorf("Rewrite failed: %v", err)
	}

	// Verify rewritten content
	var replayedCommands []types.AOFCommand
	err = logger.Replay(context.Background(), func(cmd types.AOFCommand) error {
		replayedCommands = append(replayedCommands, cmd)
		return nil
	})

	if err != nil {
		t.Errorf("Replay after rewrite failed: %v", err)
	}

	if len(replayedCommands) != len(optimizedCommands) {
		t.Errorf("After rewrite, replayed %d commands, want %d", len(replayedCommands), len(optimizedCommands))
	}
}

func TestAOFLogger_SyncStrategies(t *testing.T) {
	tempDir := t.TempDir()

	strategies := []SyncStrategy{SyncAlways, SyncEverySec, SyncNo}

	for _, strategy := range strategies {
		t.Run(string(strategy), func(t *testing.T) {
			filePath := filepath.Join(tempDir, "test_"+string(strategy)+".aof")

			logger, err := NewAOFLogger(filePath, strategy)
			if err != nil {
				t.Fatalf("NewAOFLogger failed for strategy %s: %v", strategy, err)
			}
			defer logger.Close()

			command := types.AOFCommand{
				Command: "TEST_COMMAND",
				Args:    map[string]interface{}{"test": "data"},
			}

			err = logger.WriteCommand(context.Background(), command)
			if err != nil {
				t.Errorf("WriteCommand failed for strategy %s: %v", strategy, err)
			}

			stats := logger.GetStats()
			if stats.SyncStrategy != string(strategy) {
				t.Errorf("SyncStrategy = %s, want %s", stats.SyncStrategy, strategy)
			}
		})
	}
}

func TestAOFLogger_BackgroundSync(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.aof")

	logger, err := NewAOFLogger(filePath, SyncEverySec)
	if err != nil {
		t.Fatalf("NewAOFLogger failed: %v", err)
	}
	defer logger.Close()

	command := types.AOFCommand{
		Command: "TEST_COMMAND",
		Args:    map[string]interface{}{"test": "data"},
	}

	err = logger.WriteCommand(context.Background(), command)
	if err != nil {
		t.Errorf("WriteCommand failed: %v", err)
	}

	// Wait a bit to let background sync happen
	time.Sleep(1100 * time.Millisecond)

	stats := logger.GetStats()
	if stats.LastSync.IsZero() {
		t.Error("Background sync should have happened")
	}
}

func TestAOFLogger_Truncate(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.aof")

	logger, err := NewAOFLogger(filePath, SyncAlways)
	if err != nil {
		t.Fatalf("NewAOFLogger failed: %v", err)
	}
	defer logger.Close()

	// Write some commands
	command := types.AOFCommand{
		Command: "TEST_COMMAND",
		Args:    map[string]interface{}{"test": "data"},
	}

	for i := 0; i < 5; i++ {
		if err := logger.WriteCommand(context.Background(), command); err != nil {
			t.Fatalf("WriteCommand failed: %v", err)
		}
	}

	stats := logger.GetStats()
	if stats.CommandCount != 5 {
		t.Errorf("CommandCount before truncate = %d, want 5", stats.CommandCount)
	}

	// Truncate
	err = logger.Truncate()
	if err != nil {
		t.Errorf("Truncate failed: %v", err)
	}

	stats = logger.GetStats()
	if stats.CommandCount != 0 {
		t.Errorf("CommandCount after truncate = %d, want 0", stats.CommandCount)
	}

	// Verify file is empty
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		t.Errorf("Failed to stat file: %v", err)
	}
	if fileInfo.Size() != 0 {
		t.Errorf("File size after truncate = %d, want 0", fileInfo.Size())
	}
}

func TestCommandBuilder(t *testing.T) {
	builder := NewCommandBuilder()

	// Test CreateDatabase
	cmd := builder.CreateDatabase("test_db")
	if cmd.Command != "CREATE_DATABASE" {
		t.Errorf("CreateDatabase command = %s, want CREATE_DATABASE", cmd.Command)
	}
	if cmd.Database != "test_db" {
		t.Errorf("CreateDatabase database = %s, want test_db", cmd.Database)
	}

	// Test CreateCollection
	config := types.CollectionConfig{
		Name:   "test_coll",
		Metric: types.DistanceMetricL2,
	}
	cmd = builder.CreateCollection("test_db", "test_coll", config)
	if cmd.Command != "CREATE_COLLECTION" {
		t.Errorf("CreateCollection command = %s, want CREATE_COLLECTION", cmd.Command)
	}
	if cmd.Database != "test_db" {
		t.Errorf("CreateCollection database = %s, want test_db", cmd.Database)
	}
	if cmd.Collection != "test_coll" {
		t.Errorf("CreateCollection collection = %s, want test_coll", cmd.Collection)
	}

	// Test InsertVectors
	vectors := []types.Vector{
		{ID: "v1", Elements: []float32{1, 2, 3}},
		{ID: "v2", Elements: []float32{4, 5, 6}},
	}
	cmd = builder.InsertVectors("test_db", "test_coll", vectors)
	if cmd.Command != "INSERT_VECTORS" {
		t.Errorf("InsertVectors command = %s, want INSERT_VECTORS", cmd.Command)
	}

	// Test DeleteVectors
	ids := []string{"v1", "v2"}
	cmd = builder.DeleteVectors("test_db", "test_coll", ids)
	if cmd.Command != "DELETE_VECTORS" {
		t.Errorf("DeleteVectors command = %s, want DELETE_VECTORS", cmd.Command)
	}
}

func TestAOFLogger_NonExistentFile(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "nonexistent.aof")

	logger, err := NewAOFLogger(filePath, SyncAlways)
	if err != nil {
		t.Fatalf("NewAOFLogger failed: %v", err)
	}
	defer logger.Close()

	// Replay on non-existent file should not error
	err = logger.Replay(context.Background(), func(cmd types.AOFCommand) error {
		t.Error("Should not replay any commands from non-existent file")
		return nil
	})

	if err != nil {
		t.Errorf("Replay on non-existent file should not error: %v", err)
	}
}
