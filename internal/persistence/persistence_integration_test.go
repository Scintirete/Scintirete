// Package persistence provides integration tests for AOF recovery functionality.
package persistence

import (
	"context"
	"testing"

	"github.com/scintirete/scintirete/internal/core/database"
	"github.com/scintirete/scintirete/internal/observability/logger"
	"github.com/scintirete/scintirete/pkg/types"
)

// TestAOFRecoveryWithDatabaseEngine tests that AOF commands are properly applied to database engine
func TestAOFRecoveryWithDatabaseEngine(t *testing.T) {
	tempDir := t.TempDir()

	// Create a test logger
	testLogger, err := logger.NewFromConfigString("debug", "text")
	if err != nil {
		t.Fatalf("Failed to create test logger: %v", err)
	}

	config := Config{
		DataDir:         tempDir,
		RDBFilename:     "test.rdb",
		AOFFilename:     "test.aof",
		AOFSyncStrategy: "always", // Force immediate sync for testing
		Logger:          testLogger,
	}

	// Step 1: Create persistence manager without database engine (simulating old behavior)
	managerWithoutEngine, err := NewManager(config)
	if err != nil {
		t.Fatalf("Failed to create persistence manager without engine: %v", err)
	}

	ctx := context.Background()

	// Write some AOF commands
	err = managerWithoutEngine.LogCreateDatabase(ctx, "test_db")
	if err != nil {
		t.Fatalf("Failed to log create database: %v", err)
	}

	err = managerWithoutEngine.LogCreateCollection(ctx, "test_db", "test_coll", types.CollectionConfig{
		Name:   "test_coll",
		Metric: types.DistanceMetricL2,
		HNSWParams: types.HNSWParams{
			M:              16,
			EfConstruction: 200,
			EfSearch:       50,
		},
	})
	if err != nil {
		t.Fatalf("Failed to log create collection: %v", err)
	}

	// Insert some test vectors
	testVectors := []types.Vector{
		{
			ID:       "vec1",
			Elements: []float32{1.0, 2.0, 3.0},
			Metadata: map[string]interface{}{"category": "test"},
		},
		{
			ID:       "vec2",
			Elements: []float32{4.0, 5.0, 6.0},
			Metadata: map[string]interface{}{"category": "test"},
		},
	}

	err = managerWithoutEngine.LogInsertVectors(ctx, "test_db", "test_coll", testVectors)
	if err != nil {
		t.Fatalf("Failed to log insert vectors: %v", err)
	}

	// Stop the manager to ensure AOF is flushed
	managerWithoutEngine.Stop(ctx)

	// Step 2: Create new database engine
	engine := database.NewEngine()

	// Step 3: Create new persistence manager WITH database engine (simulating fixed behavior)
	managerWithEngine, err := NewManagerWithEngine(config, engine)
	if err != nil {
		t.Fatalf("Failed to create persistence manager with engine: %v", err)
	}
	defer managerWithEngine.Stop(ctx)

	// Step 4: Recover data - this should now properly apply AOF commands to the database
	err = managerWithEngine.Recover(ctx)
	if err != nil {
		t.Fatalf("Failed to recover data: %v", err)
	}

	// Step 5: Verify that data was actually restored to the database engine

	// Check if database exists
	databases, err := engine.ListDatabases(ctx)
	if err != nil {
		t.Fatalf("Failed to list databases: %v", err)
	}

	found := false
	for _, dbName := range databases {
		if dbName == "test_db" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Database 'test_db' not found after recovery. Found databases: %v", databases)
	}

	// Get the database
	db, err := engine.GetDatabase(ctx, "test_db")
	if err != nil {
		t.Fatalf("Failed to get database 'test_db': %v", err)
	}

	// Check if collection exists
	collections, err := db.ListCollections(ctx)
	if err != nil {
		t.Fatalf("Failed to list collections: %v", err)
	}

	found = false
	for _, coll := range collections {
		if coll.Name == "test_coll" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Collection 'test_coll' not found after recovery. Found collections: %v", collections)
	}

	// Get the collection
	collection, err := db.GetCollection(ctx, "test_coll")
	if err != nil {
		t.Fatalf("Failed to get collection 'test_coll': %v", err)
	}

	// Check vector count
	vectorCount, err := collection.Count(ctx)
	if err != nil {
		t.Fatalf("Failed to get vector count: %v", err)
	}

	if vectorCount != int64(len(testVectors)) {
		t.Errorf("Expected %d vectors, got %d", len(testVectors), vectorCount)
	}

	// Check that we can retrieve the vectors
	for _, expectedVector := range testVectors {
		retrievedVector, err := collection.Get(ctx, expectedVector.ID)
		if err != nil {
			t.Errorf("Failed to retrieve vector %s: %v", expectedVector.ID, err)
			continue
		}

		if retrievedVector.ID != expectedVector.ID {
			t.Errorf("Vector ID mismatch: expected %s, got %s", expectedVector.ID, retrievedVector.ID)
		}

		if len(retrievedVector.Elements) != len(expectedVector.Elements) {
			t.Errorf("Vector %s element count mismatch: expected %d, got %d",
				expectedVector.ID, len(expectedVector.Elements), len(retrievedVector.Elements))
			continue
		}

		for i, expectedVal := range expectedVector.Elements {
			if retrievedVector.Elements[i] != expectedVal {
				t.Errorf("Vector %s element[%d] mismatch: expected %f, got %f",
					expectedVector.ID, i, expectedVal, retrievedVector.Elements[i])
			}
		}
	}

	// Step 6: Check recovery stats
	stats := managerWithEngine.GetStats()
	if stats.RecoveredCommands != 3 { // CREATE_DATABASE + CREATE_COLLECTION + INSERT_VECTORS
		t.Errorf("Expected 3 recovered commands, got %d", stats.RecoveredCommands)
	}

	if stats.RecoveryTime <= 0 {
		t.Error("Recovery time should be positive")
	}
}

// TestAOFRecoveryWithoutDatabaseEngine tests the behavior when database engine is not connected
func TestAOFRecoveryWithoutDatabaseEngine(t *testing.T) {
	tempDir := t.TempDir()

	// Create a test logger to capture warning messages
	testLogger, err := logger.NewFromConfigString("debug", "text")
	if err != nil {
		t.Fatalf("Failed to create test logger: %v", err)
	}

	config := Config{
		DataDir:         tempDir,
		RDBFilename:     "test.rdb",
		AOFFilename:     "test.aof",
		AOFSyncStrategy: "always",
		Logger:          testLogger,
	}

	// Step 1: Create persistence manager and write some AOF commands
	manager1, err := NewManager(config)
	if err != nil {
		t.Fatalf("Failed to create first persistence manager: %v", err)
	}

	ctx := context.Background()

	err = manager1.LogCreateDatabase(ctx, "test_db")
	if err != nil {
		t.Fatalf("Failed to log create database: %v", err)
	}

	manager1.Stop(ctx)

	// Step 2: Create new persistence manager WITHOUT database engine and try to recover
	manager2, err := NewManager(config) // Note: using NewManager, not NewManagerWithEngine
	if err != nil {
		t.Fatalf("Failed to create second persistence manager: %v", err)
	}
	defer manager2.Stop(ctx)

	// This should complete without error but log warnings about data loss
	err = manager2.Recover(ctx)
	if err != nil {
		t.Fatalf("Recovery should not fail even without database engine: %v", err)
	}

	// Check that commands were read but not applied
	stats := manager2.GetStats()
	if stats.RecoveredCommands != 1 {
		t.Errorf("Expected 1 recovered command (read but not applied), got %d", stats.RecoveredCommands)
	}

	// Verify that the manager knows it doesn't have a database engine
	if manager2.HasDatabaseEngine() {
		t.Error("Manager should report that it doesn't have a database engine")
	}
}

// TestAOFRecoveryRDBIntegration tests recovery with both RDB snapshot and AOF commands
func TestAOFRecoveryRDBIntegration(t *testing.T) {
	tempDir := t.TempDir()

	testLogger, err := logger.NewFromConfigString("debug", "text")
	if err != nil {
		t.Fatalf("Failed to create test logger: %v", err)
	}

	config := Config{
		DataDir:         tempDir,
		RDBFilename:     "test.rdb",
		AOFFilename:     "test.aof",
		AOFSyncStrategy: "always",
		Logger:          testLogger,
	}

	// Step 1: Create engine and persistence manager
	engine1 := database.NewEngine()
	manager1, err := NewManagerWithEngine(config, engine1)
	if err != nil {
		t.Fatalf("Failed to create persistence manager: %v", err)
	}

	ctx := context.Background()

	// Step 2: Create some data and save RDB snapshot
	err = engine1.CreateDatabase(ctx, "test_db")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}

	// Create RDB snapshot
	databases, err := engine1.GetDatabaseState(ctx)
	if err != nil {
		t.Fatalf("Failed to get database state: %v", err)
	}

	err = manager1.SaveSnapshot(ctx, databases)
	if err != nil {
		t.Fatalf("Failed to save RDB snapshot: %v", err)
	}

	// Step 3: Add more data after RDB snapshot (this will only be in AOF)
	err = manager1.LogCreateCollection(ctx, "test_db", "new_coll", types.CollectionConfig{
		Name:       "new_coll",
		Metric:     types.DistanceMetricL2,
		HNSWParams: types.HNSWParams{M: 16, EfConstruction: 200, EfSearch: 50},
	})
	if err != nil {
		t.Fatalf("Failed to log create collection: %v", err)
	}

	manager1.Stop(ctx)

	// Step 4: Create new engine and manager, recover data
	engine2 := database.NewEngine()
	manager2, err := NewManagerWithEngine(config, engine2)
	if err != nil {
		t.Fatalf("Failed to create second persistence manager: %v", err)
	}
	defer manager2.Stop(ctx)

	err = manager2.Recover(ctx)
	if err != nil {
		t.Fatalf("Failed to recover data: %v", err)
	}

	// Step 5: Verify both RDB and AOF data are present

	// Database should exist (from RDB)
	databases2, err := engine2.ListDatabases(ctx)
	if err != nil {
		t.Fatalf("Failed to list databases: %v", err)
	}

	found := false
	for _, dbName := range databases2 {
		if dbName == "test_db" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Database 'test_db' not found after RDB+AOF recovery")
	}

	// Collection should exist (from AOF)
	db, err := engine2.GetDatabase(ctx, "test_db")
	if err != nil {
		t.Fatalf("Failed to get database: %v", err)
	}

	collections, err := db.ListCollections(ctx)
	if err != nil {
		t.Fatalf("Failed to list collections: %v", err)
	}

	found = false
	for _, coll := range collections {
		if coll.Name == "new_coll" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Collection 'new_coll' not found after RDB+AOF recovery")
	}

	// Recovery stats should show at least 1 command (CREATE_COLLECTION)
	stats := manager2.GetStats()
	if stats.RecoveredCommands < 1 {
		t.Errorf("Expected at least 1 recovered command, got %d", stats.RecoveredCommands)
	}
}
