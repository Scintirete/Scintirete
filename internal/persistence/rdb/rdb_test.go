package rdb

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/scintirete/scintirete/pkg/types"
)

func TestNewRDBManager(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.rdb")

	manager, err := NewRDBManager(filePath)
	if err != nil {
		t.Fatalf("NewRDBManager failed: %v", err)
	}

	if manager.filePath != filePath {
		t.Errorf("filePath = %s, want %s", manager.filePath, filePath)
	}
}

func TestRDBManager_SaveAndLoad(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.rdb")

	manager, err := NewRDBManager(filePath)
	if err != nil {
		t.Fatalf("NewRDBManager failed: %v", err)
	}

	// Create test snapshot
	snapshot := RDBSnapshot{
		Version:   "1.0",
		Timestamp: time.Now(),
		Databases: map[string]DatabaseSnapshot{
			"test_db": {
				Name: "test_db",
				Collections: map[string]CollectionSnapshot{
					"test_coll": {
						Name: "test_coll",
						Config: types.CollectionConfig{
							Name:   "test_coll",
							Metric: types.DistanceMetricL2,
							HNSWParams: types.HNSWParams{
								M:              16,
								EfConstruction: 200,
							},
						},
						Vectors: []types.Vector{
							{ID: "v1", Elements: []float32{1.0, 2.0, 3.0}},
							{ID: "v2", Elements: []float32{4.0, 5.0, 6.0}},
						},
						VectorCount:  2,
						DeletedCount: 0,
						CreatedAt:    time.Now(),
						UpdatedAt:    time.Now(),
					},
				},
				CreatedAt: time.Now(),
			},
		},
	}

	// Save snapshot
	err = manager.Save(context.Background(), snapshot)
	if err != nil {
		t.Errorf("Save failed: %v", err)
	}

	// Load snapshot
	loadedSnapshot, err := manager.Load(context.Background())
	if err != nil {
		t.Errorf("Load failed: %v", err)
	}

	if loadedSnapshot == nil {
		t.Fatal("Loaded snapshot is nil")
	}

	// Verify basic fields
	if loadedSnapshot.Version != "1.0" {
		t.Errorf("Version = %s, want 1.0", loadedSnapshot.Version)
	}

	// Verify databases
	if len(loadedSnapshot.Databases) != 1 {
		t.Errorf("Database count = %d, want 1", len(loadedSnapshot.Databases))
	}

	testDB, exists := loadedSnapshot.Databases["test_db"]
	if !exists {
		t.Fatal("test_db not found in loaded snapshot")
	}

	if testDB.Name != "test_db" {
		t.Errorf("Database name = %s, want test_db", testDB.Name)
	}

	// Verify collections
	if len(testDB.Collections) != 1 {
		t.Errorf("Collection count = %d, want 1", len(testDB.Collections))
	}

	testColl, exists := testDB.Collections["test_coll"]
	if !exists {
		t.Fatal("test_coll not found in loaded snapshot")
	}

	if testColl.VectorCount != 2 {
		t.Errorf("VectorCount = %d, want 2", testColl.VectorCount)
	}

	if len(testColl.Vectors) != 2 {
		t.Errorf("Vector array length = %d, want 2", len(testColl.Vectors))
	}
}

func TestRDBManager_LoadNonExistentFile(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "nonexistent.rdb")

	manager, err := NewRDBManager(filePath)
	if err != nil {
		t.Fatalf("NewRDBManager failed: %v", err)
	}

	// Load non-existent file should return nil without error
	snapshot, err := manager.Load(context.Background())
	if err != nil {
		t.Errorf("Load non-existent file should not error: %v", err)
	}

	if snapshot != nil {
		t.Error("Load non-existent file should return nil")
	}
}

func TestRDBManager_Exists(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.rdb")

	manager, err := NewRDBManager(filePath)
	if err != nil {
		t.Fatalf("NewRDBManager failed: %v", err)
	}

	// File should not exist initially
	if manager.Exists() {
		t.Error("File should not exist initially")
	}

	// Create file by saving snapshot
	snapshot := RDBSnapshot{
		Version:   "1.0",
		Timestamp: time.Now(),
		Databases: make(map[string]DatabaseSnapshot),
	}

	err = manager.Save(context.Background(), snapshot)
	if err != nil {
		t.Errorf("Save failed: %v", err)
	}

	// File should exist now
	if !manager.Exists() {
		t.Error("File should exist after save")
	}
}

func TestRDBManager_GetInfo(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.rdb")

	manager, err := NewRDBManager(filePath)
	if err != nil {
		t.Fatalf("NewRDBManager failed: %v", err)
	}

	// Get info for non-existent file
	info, err := manager.GetInfo()
	if err != nil {
		t.Errorf("GetInfo failed: %v", err)
	}

	if info.Exists {
		t.Error("Info should show file does not exist")
	}

	// Create file
	snapshot := RDBSnapshot{
		Version:   "1.0",
		Timestamp: time.Now(),
		Databases: make(map[string]DatabaseSnapshot),
	}

	err = manager.Save(context.Background(), snapshot)
	if err != nil {
		t.Errorf("Save failed: %v", err)
	}

	// Get info for existing file
	info, err = manager.GetInfo()
	if err != nil {
		t.Errorf("GetInfo failed: %v", err)
	}

	if !info.Exists {
		t.Error("Info should show file exists")
	}

	if info.Size <= 0 {
		t.Errorf("File size should be positive, got %d", info.Size)
	}

	if info.Path != filePath {
		t.Errorf("Path = %s, want %s", info.Path, filePath)
	}
}

func TestRDBManager_Remove(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.rdb")

	manager, err := NewRDBManager(filePath)
	if err != nil {
		t.Fatalf("NewRDBManager failed: %v", err)
	}

	// Create file
	snapshot := RDBSnapshot{
		Version:   "1.0",
		Timestamp: time.Now(),
		Databases: make(map[string]DatabaseSnapshot),
	}

	err = manager.Save(context.Background(), snapshot)
	if err != nil {
		t.Errorf("Save failed: %v", err)
	}

	// Verify file exists
	if !manager.Exists() {
		t.Error("File should exist before remove")
	}

	// Remove file
	err = manager.Remove()
	if err != nil {
		t.Errorf("Remove failed: %v", err)
	}

	// Verify file is gone
	if manager.Exists() {
		t.Error("File should not exist after remove")
	}

	// Remove non-existent file should not error
	err = manager.Remove()
	if err != nil {
		t.Errorf("Remove non-existent file should not error: %v", err)
	}
}

func TestRDBManager_CreateSnapshot(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.rdb")

	manager, err := NewRDBManager(filePath)
	if err != nil {
		t.Fatalf("NewRDBManager failed: %v", err)
	}

	// Create test database state
	databases := map[string]DatabaseState{
		"db1": {
			Name: "db1",
			Collections: map[string]CollectionState{
				"coll1": {
					Name: "coll1",
					Config: types.CollectionConfig{
						Name:   "coll1",
						Metric: types.DistanceMetricCosine,
					},
					Vectors: []types.Vector{
						{ID: "v1", Elements: []float32{1, 0, 0}},
						{ID: "v2", Elements: []float32{0, 1, 0}},
					},
					VectorCount:  2,
					DeletedCount: 1,
					CreatedAt:    time.Now(),
					UpdatedAt:    time.Now(),
				},
			},
			CreatedAt: time.Now(),
		},
		"db2": {
			Name: "db2",
			Collections: map[string]CollectionState{
				"coll2": {
					Name: "coll2",
					Config: types.CollectionConfig{
						Name:   "coll2",
						Metric: types.DistanceMetricInnerProduct,
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

	// Create snapshot
	snapshot := manager.CreateSnapshot(databases)

	// Verify snapshot structure
	if snapshot.Version != "1.0" {
		t.Errorf("Version = %s, want 1.0", snapshot.Version)
	}

	if len(snapshot.Databases) != 2 {
		t.Errorf("Database count = %d, want 2", len(snapshot.Databases))
	}

	// Check metadata
	totalDBs, ok := snapshot.Metadata["total_databases"]
	if !ok || totalDBs != 2 {
		t.Errorf("total_databases metadata = %v, want 2", totalDBs)
	}

	totalColls, ok := snapshot.Metadata["total_collections"]
	if !ok || totalColls != 2 {
		t.Errorf("total_collections metadata = %v, want 2", totalColls)
	}

	totalVectors, ok := snapshot.Metadata["total_vectors"]
	if !ok || totalVectors != int64(2) {
		t.Errorf("total_vectors metadata = %v, want 2", totalVectors)
	}

	// Verify database content
	db1, exists := snapshot.Databases["db1"]
	if !exists {
		t.Fatal("db1 not found in snapshot")
	}

	coll1, exists := db1.Collections["coll1"]
	if !exists {
		t.Fatal("coll1 not found in db1")
	}

	if coll1.VectorCount != 2 {
		t.Errorf("coll1 VectorCount = %d, want 2", coll1.VectorCount)
	}

	if coll1.DeletedCount != 1 {
		t.Errorf("coll1 DeletedCount = %d, want 1", coll1.DeletedCount)
	}
}

func TestRDBManager_ValidateSnapshot(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.rdb")

	manager, err := NewRDBManager(filePath)
	if err != nil {
		t.Fatalf("NewRDBManager failed: %v", err)
	}

	tests := []struct {
		name     string
		snapshot *RDBSnapshot
		wantErr  bool
		errMsg   string
	}{
		{
			name: "valid snapshot",
			snapshot: &RDBSnapshot{
				Version:   "1.0",
				Timestamp: time.Now(),
				Databases: map[string]DatabaseSnapshot{
					"db1": {
						Name: "db1",
						Collections: map[string]CollectionSnapshot{
							"coll1": {
								Name:        "coll1",
								Vectors:     []types.Vector{{ID: "v1", Elements: []float32{1, 2, 3}}},
								VectorCount: 1,
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing version",
			snapshot: &RDBSnapshot{
				Timestamp: time.Now(),
				Databases: make(map[string]DatabaseSnapshot),
			},
			wantErr: true,
			errMsg:  "missing version",
		},
		{
			name: "missing timestamp",
			snapshot: &RDBSnapshot{
				Version:   "1.0",
				Databases: make(map[string]DatabaseSnapshot),
			},
			wantErr: true,
			errMsg:  "missing timestamp",
		},
		{
			name: "unsupported version",
			snapshot: &RDBSnapshot{
				Version:   "2.0",
				Timestamp: time.Now(),
				Databases: make(map[string]DatabaseSnapshot),
			},
			wantErr: true,
			errMsg:  "unsupported RDB version",
		},
		{
			name: "database name mismatch",
			snapshot: &RDBSnapshot{
				Version:   "1.0",
				Timestamp: time.Now(),
				Databases: map[string]DatabaseSnapshot{
					"db1": {
						Name:        "db2", // Wrong name
						Collections: make(map[string]CollectionSnapshot),
					},
				},
			},
			wantErr: true,
			errMsg:  "database name mismatch",
		},
		{
			name: "vector count mismatch",
			snapshot: &RDBSnapshot{
				Version:   "1.0",
				Timestamp: time.Now(),
				Databases: map[string]DatabaseSnapshot{
					"db1": {
						Name: "db1",
						Collections: map[string]CollectionSnapshot{
							"coll1": {
								Name:        "coll1",
								Vectors:     []types.Vector{{ID: "v1", Elements: []float32{1, 2, 3}}},
								VectorCount: 2, // Wrong count
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "vector count mismatch",
		},
		{
			name: "inconsistent vector dimensions",
			snapshot: &RDBSnapshot{
				Version:   "1.0",
				Timestamp: time.Now(),
				Databases: map[string]DatabaseSnapshot{
					"db1": {
						Name: "db1",
						Collections: map[string]CollectionSnapshot{
							"coll1": {
								Name: "coll1",
								Vectors: []types.Vector{
									{ID: "v1", Elements: []float32{1, 2, 3}},    // 3 dimensions
									{ID: "v2", Elements: []float32{1, 2, 3, 4}}, // 4 dimensions
								},
								VectorCount: 2,
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "inconsistent vector dimension",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.validateSnapshot(tt.snapshot)
			if tt.wantErr {
				if err == nil {
					t.Errorf("validateSnapshot() should have failed for %s", tt.name)
				} else if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("validateSnapshot() error = %v, should contain %s", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("validateSnapshot() failed for valid snapshot: %v", err)
				}
			}
		})
	}
}

func TestBackupManager(t *testing.T) {
	tempDir := t.TempDir()
	rdbPath := filepath.Join(tempDir, "test.rdb")
	backupDir := filepath.Join(tempDir, "backups")

	// Create RDB manager and save a snapshot
	rdbManager, err := NewRDBManager(rdbPath)
	if err != nil {
		t.Fatalf("NewRDBManager failed: %v", err)
	}

	snapshot := RDBSnapshot{
		Version:   "1.0",
		Timestamp: time.Now(),
		Databases: map[string]DatabaseSnapshot{
			"test_db": {
				Name:        "test_db",
				Collections: map[string]CollectionSnapshot{},
				CreatedAt:   time.Now(),
			},
		},
	}

	err = rdbManager.Save(context.Background(), snapshot)
	if err != nil {
		t.Fatalf("RDB Save failed: %v", err)
	}

	// Create backup manager
	backupManager, err := NewBackupManager(rdbManager, backupDir)
	if err != nil {
		t.Fatalf("NewBackupManager failed: %v", err)
	}

	// Create backup
	backupPath, err := backupManager.CreateBackup(context.Background())
	if err != nil {
		t.Errorf("CreateBackup failed: %v", err)
	}

	if backupPath == "" {
		t.Error("CreateBackup should return non-empty path")
	}

	// List backups
	backups, err := backupManager.ListBackups()
	if err != nil {
		t.Errorf("ListBackups failed: %v", err)
	}

	if len(backups) != 1 {
		t.Errorf("Backup count = %d, want 1", len(backups))
	}

	if backups[0].Size <= 0 {
		t.Errorf("Backup size should be positive, got %d", backups[0].Size)
	}

	// Test restore from backup
	err = backupManager.RestoreFromBackup(context.Background(), backupPath)
	if err != nil {
		t.Errorf("RestoreFromBackup failed: %v", err)
	}

	// Verify restored snapshot
	restoredSnapshot, err := rdbManager.Load(context.Background())
	if err != nil {
		t.Errorf("Load after restore failed: %v", err)
	}

	if restoredSnapshot == nil {
		t.Fatal("Restored snapshot is nil")
	}

	if len(restoredSnapshot.Databases) != 1 {
		t.Errorf("Restored database count = %d, want 1", len(restoredSnapshot.Databases))
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
