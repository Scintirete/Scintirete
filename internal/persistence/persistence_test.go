package persistence

import (
	"context"
	"testing"
	"time"

	"github.com/scintirete/scintirete/internal/persistence/rdb"
	"github.com/scintirete/scintirete/pkg/types"
)

func TestNewManager(t *testing.T) {
	tempDir := t.TempDir()
	config := DefaultConfig(tempDir)

	manager, err := NewManager(config)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	defer manager.Stop(context.Background())

	if manager.config.DataDir != tempDir {
		t.Errorf("DataDir = %s, want %s", manager.config.DataDir, tempDir)
	}
}

func TestDefaultConfig(t *testing.T) {
	dataDir := "/test/data"
	config := DefaultConfig(dataDir)

	if config.DataDir != dataDir {
		t.Errorf("DataDir = %s, want %s", config.DataDir, dataDir)
	}

	if config.RDBFilename != "dump.rdb" {
		t.Errorf("RDBFilename = %s, want dump.rdb", config.RDBFilename)
	}

	if config.AOFFilename != "appendonly.aof" {
		t.Errorf("AOFFilename = %s, want appendonly.aof", config.AOFFilename)
	}

	if config.AOFSyncStrategy != "everysec" {
		t.Errorf("AOFSyncStrategy = %s, want everysec", config.AOFSyncStrategy)
	}

	if config.RDBInterval != 5*time.Minute {
		t.Errorf("RDBInterval = %v, want 5m", config.RDBInterval)
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid config",
			config:  DefaultConfig("/tmp/test"),
			wantErr: false,
		},
		{
			name: "empty data dir",
			config: Config{
				DataDir:         "",
				RDBFilename:     "dump.rdb",
				AOFFilename:     "appendonly.aof",
				AOFSyncStrategy: "everysec",
				RDBInterval:     time.Minute,
				AOFRewriteSize:  1024,
				BackupRetention: 1,
			},
			wantErr: true,
			errMsg:  "data directory cannot be empty",
		},
		{
			name: "empty RDB filename",
			config: Config{
				DataDir:         "/tmp/test",
				RDBFilename:     "",
				AOFFilename:     "appendonly.aof",
				AOFSyncStrategy: "everysec",
				RDBInterval:     time.Minute,
				AOFRewriteSize:  1024,
				BackupRetention: 1,
			},
			wantErr: true,
			errMsg:  "RDB filename cannot be empty",
		},
		{
			name: "invalid sync strategy",
			config: Config{
				DataDir:         "/tmp/test",
				RDBFilename:     "dump.rdb",
				AOFFilename:     "appendonly.aof",
				AOFSyncStrategy: "invalid",
				RDBInterval:     time.Minute,
				AOFRewriteSize:  1024,
				BackupRetention: 1,
			},
			wantErr: true,
			errMsg:  "invalid AOF sync strategy",
		},
		{
			name: "negative RDB interval",
			config: Config{
				DataDir:         "/tmp/test",
				RDBFilename:     "dump.rdb",
				AOFFilename:     "appendonly.aof",
				AOFSyncStrategy: "everysec",
				RDBInterval:     -time.Minute,
				AOFRewriteSize:  1024,
				BackupRetention: 1,
			},
			wantErr: true,
			errMsg:  "RDB interval must be positive",
		},
		{
			name: "negative AOF rewrite size",
			config: Config{
				DataDir:         "/tmp/test",
				RDBFilename:     "dump.rdb",
				AOFFilename:     "appendonly.aof",
				AOFSyncStrategy: "everysec",
				RDBInterval:     time.Minute,
				AOFRewriteSize:  -1024,
				BackupRetention: 1,
			},
			wantErr: true,
			errMsg:  "AOF rewrite size must be positive",
		},
		{
			name: "negative backup retention",
			config: Config{
				DataDir:         "/tmp/test",
				RDBFilename:     "dump.rdb",
				AOFFilename:     "appendonly.aof",
				AOFSyncStrategy: "everysec",
				RDBInterval:     time.Minute,
				AOFRewriteSize:  1024,
				BackupRetention: -1,
			},
			wantErr: true,
			errMsg:  "backup retention must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfig(tt.config)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateConfig() should have failed for %s", tt.name)
				} else if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateConfig() error = %v, should contain %s", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateConfig() failed for valid config: %v", err)
				}
			}
		})
	}
}

func TestManager_AOFOperations(t *testing.T) {
	tempDir := t.TempDir()
	config := DefaultConfig(tempDir)

	manager, err := NewManager(config)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	defer manager.Stop(context.Background())

	ctx := context.Background()

	// Test logging different commands
	tests := []struct {
		name string
		fn   func() error
	}{
		{
			name: "LogCreateDatabase",
			fn: func() error {
				return manager.LogCreateDatabase(ctx, "test_db")
			},
		},
		{
			name: "LogDropDatabase",
			fn: func() error {
				return manager.LogDropDatabase(ctx, "test_db")
			},
		},
		{
			name: "LogCreateCollection",
			fn: func() error {
				config := types.CollectionConfig{
					Name:   "test_coll",
					Metric: types.DistanceMetricL2,
				}
				return manager.LogCreateCollection(ctx, "test_db", "test_coll", config)
			},
		},
		{
			name: "LogDropCollection",
			fn: func() error {
				return manager.LogDropCollection(ctx, "test_db", "test_coll")
			},
		},
		{
			name: "LogInsertVectors",
			fn: func() error {
				vectors := []types.Vector{
					{ID: "v1", Elements: []float32{1, 2, 3}},
				}
				return manager.LogInsertVectors(ctx, "test_db", "test_coll", vectors)
			},
		},
		{
			name: "LogDeleteVectors",
			fn: func() error {
				return manager.LogDeleteVectors(ctx, "test_db", "test_coll", []string{"v1"})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn()
			if err != nil {
				t.Errorf("%s failed: %v", tt.name, err)
			}
		})
	}

	// Check statistics
	stats := manager.GetStats()
	if stats.AOFStats.CommandCount != int64(len(tests)) {
		t.Errorf("AOF command count = %d, want %d", stats.AOFStats.CommandCount, len(tests))
	}
}

func TestManager_SaveSnapshot(t *testing.T) {
	tempDir := t.TempDir()
	config := DefaultConfig(tempDir)

	manager, err := NewManager(config)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	defer manager.Stop(context.Background())

	// Create test database state
	databases := map[string]rdb.DatabaseState{
		"test_db": {
			Name: "test_db",
			Collections: map[string]rdb.CollectionState{
				"test_coll": {
					Name: "test_coll",
					Config: types.CollectionConfig{
						Name:   "test_coll",
						Metric: types.DistanceMetricL2,
					},
					Vectors: []types.Vector{
						{ID: "v1", Elements: []float32{1, 2, 3}},
					},
					VectorCount:  1,
					DeletedCount: 0,
					CreatedAt:    time.Now(),
					UpdatedAt:    time.Now(),
				},
			},
			CreatedAt: time.Now(),
		},
	}

	// Save snapshot
	err = manager.SaveSnapshot(context.Background(), databases)
	if err != nil {
		t.Errorf("SaveSnapshot failed: %v", err)
	}

	// Check statistics
	stats := manager.GetStats()
	if stats.LastRDBSave.IsZero() {
		t.Error("LastRDBSave should be set after SaveSnapshot")
	}

	if stats.RDBInfo == nil || !stats.RDBInfo.Exists {
		t.Error("RDB file should exist after SaveSnapshot")
	}
}

func TestManager_BackupOperations(t *testing.T) {
	tempDir := t.TempDir()
	config := DefaultConfig(tempDir)

	manager, err := NewManager(config)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	defer manager.Stop(context.Background())

	// First create a snapshot to backup
	databases := map[string]rdb.DatabaseState{
		"test_db": {
			Name:        "test_db",
			Collections: map[string]rdb.CollectionState{},
			CreatedAt:   time.Now(),
		},
	}

	err = manager.SaveSnapshot(context.Background(), databases)
	if err != nil {
		t.Fatalf("SaveSnapshot failed: %v", err)
	}

	// Create backup
	backupPath, err := manager.CreateBackup(context.Background())
	if err != nil {
		t.Errorf("CreateBackup failed: %v", err)
	}

	if backupPath == "" {
		t.Error("CreateBackup should return non-empty path")
	}

	// List backups
	backups, err := manager.ListBackups()
	if err != nil {
		t.Errorf("ListBackups failed: %v", err)
	}

	if len(backups) != 1 {
		t.Errorf("Backup count = %d, want 1", len(backups))
	}

	// Restore from backup
	err = manager.RestoreFromBackup(context.Background(), backupPath)
	if err != nil {
		t.Errorf("RestoreFromBackup failed: %v", err)
	}
}

func TestManager_Recover(t *testing.T) {
	tempDir := t.TempDir()
	config := DefaultConfig(tempDir)

	manager, err := NewManager(config)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	defer manager.Stop(context.Background())

	ctx := context.Background()

	// Write some AOF commands
	err = manager.LogCreateDatabase(ctx, "test_db")
	if err != nil {
		t.Fatalf("LogCreateDatabase failed: %v", err)
	}

	err = manager.LogCreateCollection(ctx, "test_db", "test_coll", types.CollectionConfig{
		Name:   "test_coll",
		Metric: types.DistanceMetricL2,
	})
	if err != nil {
		t.Fatalf("LogCreateCollection failed: %v", err)
	}

	// Recover (should replay AOF commands)
	err = manager.Recover(ctx)
	if err != nil {
		t.Errorf("Recover failed: %v", err)
	}

	// Check recovery statistics
	stats := manager.GetStats()
	if stats.RecoveredCommands != 2 {
		t.Errorf("RecoveredCommands = %d, want 2", stats.RecoveredCommands)
	}

	if stats.RecoveryTime <= 0 {
		t.Error("RecoveryTime should be positive")
	}
}

func TestManager_StartStopBackgroundTasks(t *testing.T) {
	tempDir := t.TempDir()
	config := DefaultConfig(tempDir)
	config.RDBInterval = 100 * time.Millisecond // Short interval for testing

	manager, err := NewManager(config)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	ctx := context.Background()

	// Start background tasks
	err = manager.StartBackgroundTasks(ctx)
	if err != nil {
		t.Errorf("StartBackgroundTasks failed: %v", err)
	}

	// Let tasks run briefly
	time.Sleep(200 * time.Millisecond)

	// Stop tasks
	err = manager.Stop(ctx)
	if err != nil {
		t.Errorf("Stop failed: %v", err)
	}
}

func TestManager_RewriteAOF(t *testing.T) {
	tempDir := t.TempDir()
	config := DefaultConfig(tempDir)

	manager, err := NewManager(config)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	defer manager.Stop(context.Background())

	// Create optimized commands
	commands := []types.AOFCommand{
		{
			Command:  "CREATE_DATABASE",
			Args:     map[string]interface{}{"name": "db1"},
			Database: "db1",
		},
		{
			Command:    "INSERT_VECTORS",
			Args:       map[string]interface{}{"vectors": []string{"v1"}},
			Database:   "db1",
			Collection: "coll1",
		},
	}

	// Rewrite AOF
	err = manager.RewriteAOF(context.Background(), commands)
	if err != nil {
		t.Errorf("RewriteAOF failed: %v", err)
	}

	// Check statistics
	stats := manager.GetStats()
	if stats.LastAOFRewrite.IsZero() {
		t.Error("LastAOFRewrite should be set after RewriteAOF")
	}
}

func TestManager_SaveSnapshotTruncatesAOF(t *testing.T) {
	tempDir := t.TempDir()
	config := DefaultConfig(tempDir)
	config.AOFSyncStrategy = "always" // Force immediate sync for testing

	manager, err := NewManager(config)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	defer manager.Stop(context.Background())

	ctx := context.Background()

	// Step 1: Write some AOF commands first
	err = manager.LogCreateDatabase(ctx, "test_db")
	if err != nil {
		t.Fatalf("LogCreateDatabase failed: %v", err)
	}

	err = manager.LogCreateCollection(ctx, "test_db", "test_coll", types.CollectionConfig{
		Name:   "test_coll",
		Metric: types.DistanceMetricL2,
		HNSWParams: types.HNSWParams{
			M:              16,
			EfConstruction: 200,
			EfSearch:       50,
		},
	})
	if err != nil {
		t.Fatalf("LogCreateCollection failed: %v", err)
	}

	// Step 2: Verify AOF has some content
	aofStats := manager.GetStats().AOFStats
	if aofStats.CommandCount == 0 {
		t.Error("AOF should contain commands before snapshot")
	}
	if aofStats.FileSize == 0 {
		t.Error("AOF file should have content before snapshot")
	}

	// Step 3: Create and save RDB snapshot
	databases := map[string]rdb.DatabaseState{
		"test_db": {
			Name: "test_db",
			Collections: map[string]rdb.CollectionState{
				"test_coll": {
					Name: "test_coll",
					Config: types.CollectionConfig{
						Name:   "test_coll",
						Metric: types.DistanceMetricL2,
						HNSWParams: types.HNSWParams{
							M:              16,
							EfConstruction: 200,
							EfSearch:       50,
						},
					},
					Vectors:      []types.Vector{},
					VectorCount:  0,
					DeletedCount: 0,
					CreatedAt:    time.Now(),
					UpdatedAt:    time.Now(),
				},
			},
			CreatedAt: time.Now(),
		},
	}

	err = manager.SaveSnapshot(ctx, databases)
	if err != nil {
		t.Fatalf("SaveSnapshot failed: %v", err)
	}

	// Step 4: Verify AOF was truncated after RDB save
	aofStatsAfter := manager.GetStats().AOFStats
	if aofStatsAfter.CommandCount != 0 {
		t.Errorf("AOF command count should be 0 after RDB save, got %d", aofStatsAfter.CommandCount)
	}

	// Note: FileSize might not be exactly 0 due to buffering, but it should be very small
	if aofStatsAfter.FileSize > 100 {
		t.Errorf("AOF file size should be small after truncation, got %d bytes", aofStatsAfter.FileSize)
	}

	// Step 5: Verify RDB was created successfully
	stats := manager.GetStats()
	if stats.LastRDBSave.IsZero() {
		t.Error("LastRDBSave should be set after SaveSnapshot")
	}

	if stats.RDBInfo == nil || !stats.RDBInfo.Exists {
		t.Error("RDB file should exist after SaveSnapshot")
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && s[:len(substr)] == substr) ||
		(len(s) > len(substr) && s[len(s)-len(substr):] == substr) ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}
