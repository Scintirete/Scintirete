package database

import (
	"context"
	"testing"

	"github.com/scintirete/scintirete/internal/persistence/rdb"
	"github.com/scintirete/scintirete/pkg/types"
)

// TestVectorCountAfterDeleteAndRestore tests the specific bug where
// vector count becomes negative after inserting vectors, deleting them,
// and then restarting/restoring from snapshot
func TestVectorCountAfterDeleteAndRestore(t *testing.T) {
	ctx := context.Background()

	// Create a database engine
	engine := NewEngine()

	// Create test database
	err := engine.CreateDatabase(ctx, "test_db")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}

	db, err := engine.GetDatabase(ctx, "test_db")
	if err != nil {
		t.Fatalf("Failed to get database: %v", err)
	}

	// Create test collection
	config := types.CollectionConfig{
		Name:   "test_collection",
		Metric: types.DistanceMetricCosine,
		HNSWParams: types.HNSWParams{
			M:              16,
			EfConstruction: 200,
			EfSearch:       50,
			MaxLayers:      16,
			Seed:           12345,
		},
	}

	err = db.CreateCollection(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create collection: %v", err)
	}

	collection, err := db.GetCollection(ctx, "test_collection")
	if err != nil {
		t.Fatalf("Failed to get collection: %v", err)
	}

	// Insert 2 vectors
	vectors := []types.Vector{
		{
			Elements: []float32{1.0, 2.0, 3.0},
			Metadata: map[string]interface{}{"test": "vector1"},
		},
		{
			Elements: []float32{4.0, 5.0, 6.0},
			Metadata: map[string]interface{}{"test": "vector2"},
		},
	}

	err = collection.Insert(ctx, vectors)
	if err != nil {
		t.Fatalf("Failed to insert vectors: %v", err)
	}

	// Check count after insertion
	count, err := collection.Count(ctx)
	if err != nil {
		t.Fatalf("Failed to get count after insertion: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 vectors after insertion, got %d", count)
	}

	// Get info after insertion
	info := collection.Info()
	if info.VectorCount != 2 {
		t.Errorf("Expected VectorCount=2 after insertion, got %d", info.VectorCount)
	}
	if info.DeletedCount != 0 {
		t.Errorf("Expected DeletedCount=0 after insertion, got %d", info.DeletedCount)
	}

	// Delete both vectors (using the generated IDs)
	vectorIDs := []string{
		"1", "2", // Auto-generated IDs start from 1
	}

	deletedCount, err := collection.Delete(ctx, vectorIDs)
	if err != nil {
		t.Fatalf("Failed to delete vectors: %v", err)
	}
	if deletedCount != 2 {
		t.Errorf("Expected to delete 2 vectors, deleted %d", deletedCount)
	}

	// Check count after deletion
	count, err = collection.Count(ctx)
	if err != nil {
		t.Fatalf("Failed to get count after deletion: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 vectors after deletion, got %d", count)
	}

	// Get info after deletion
	info = collection.Info()
	if info.VectorCount != 0 {
		t.Errorf("Expected VectorCount=0 after deletion, got %d", info.VectorCount)
	}
	if info.DeletedCount != 2 {
		t.Errorf("Expected DeletedCount=2 after deletion, got %d", info.DeletedCount)
	}

	// Now simulate restart by creating snapshot and restoring
	// Get database state (simulate snapshot creation)
	databaseStates, err := engine.GetDatabaseState(ctx)
	if err != nil {
		t.Fatalf("Failed to get database state: %v", err)
	}

	// Convert DatabaseState to DatabaseSnapshot
	databases := make(map[string]rdb.DatabaseSnapshot)
	for dbName, dbState := range databaseStates {
		collections := make(map[string]rdb.CollectionSnapshot)
		for collName, collState := range dbState.Collections {
			// Convert HNSW graph state if present
			var hnswGraphSnapshot *rdb.HNSWGraphSnapshot
			if collState.HNSWGraph != nil {
				hnswGraphSnapshot = rdb.ConvertHNSWGraphState(collState.HNSWGraph)
			}

			collections[collName] = rdb.CollectionSnapshot{
				Name:         collState.Name,
				Config:       collState.Config,
				Vectors:      collState.Vectors,
				HNSWGraph:    hnswGraphSnapshot,
				VectorCount:  collState.VectorCount,
				DeletedCount: collState.DeletedCount,
				CreatedAt:    collState.CreatedAt,
				UpdatedAt:    collState.UpdatedAt,
			}
		}
		databases[dbName] = rdb.DatabaseSnapshot{
			Name:        dbState.Name,
			Collections: collections,
			CreatedAt:   dbState.CreatedAt,
		}
	}

	// Create a mock RDB snapshot
	snapshot := &rdb.RDBSnapshot{
		Version:   "1.0",
		Databases: databases,
	}

	// Create new engine (simulate restart)
	newEngine := NewEngine()

	// Restore from snapshot
	err = newEngine.RestoreFromSnapshot(ctx, snapshot)
	if err != nil {
		t.Fatalf("Failed to restore from snapshot: %v", err)
	}

	// Get the restored database and collection
	restoredDb, err := newEngine.GetDatabase(ctx, "test_db")
	if err != nil {
		t.Fatalf("Failed to get restored database: %v", err)
	}

	restoredCollection, err := restoredDb.GetCollection(ctx, "test_collection")
	if err != nil {
		t.Fatalf("Failed to get restored collection: %v", err)
	}

	// Check count after restoration
	restoredCount, err := restoredCollection.Count(ctx)
	if err != nil {
		t.Fatalf("Failed to get count after restoration: %v", err)
	}

	if restoredCount != 0 {
		t.Errorf("Expected 0 vectors after restoration, got %d", restoredCount)
	}

	// Most importantly, check that the info doesn't show negative count
	restoredInfo := restoredCollection.Info()
	if restoredInfo.VectorCount < 0 {
		t.Errorf("BUG: VectorCount is negative after restoration: %d", restoredInfo.VectorCount)
	}

	// The vector count should be 0 after restoration
	if restoredInfo.VectorCount != 0 {
		t.Errorf("Expected VectorCount=0 after restoration, got %d", restoredInfo.VectorCount)
	}

	// The deleted count should be 0 after restoration (since we only save active vectors)
	if restoredInfo.DeletedCount != 0 {
		t.Errorf("Expected DeletedCount=0 after restoration, got %d", restoredInfo.DeletedCount)
	}

	t.Logf("Test passed: Vector count after insert/delete/restore cycle is correct")
}
