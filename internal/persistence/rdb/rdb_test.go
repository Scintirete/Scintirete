package rdb

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/scintirete/scintirete/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRDBManager_SaveAndLoad(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.rdb")

	// Create manager
	manager, err := NewRDBManager(filePath)
	require.NoError(t, err)

	// Create test snapshot
	snapshot := RDBSnapshot{
		Version:   "1.0",
		Timestamp: time.Now().Truncate(time.Second), // Truncate for comparison
		Databases: map[string]DatabaseSnapshot{
			"test_db": {
				Name:      "test_db",
				CreatedAt: time.Now().Truncate(time.Second),
				Collections: map[string]CollectionSnapshot{
					"test_collection": {
						Name: "test_collection",
						Config: types.CollectionConfig{
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
						Vectors: []types.Vector{
							{
								ID:       "vector1",
								Elements: []float32{1.0, 2.0, 3.0},
								Metadata: map[string]interface{}{
									"label": "test",
								},
							},
							{
								ID:       "vector2",
								Elements: []float32{4.0, 5.0, 6.0},
								Metadata: map[string]interface{}{
									"label": "test2",
								},
							},
						},
						VectorCount:  2,
						DeletedCount: 0,
						CreatedAt:    time.Now().Truncate(time.Second),
						UpdatedAt:    time.Now().Truncate(time.Second),
					},
				},
			},
		},
		Metadata: map[string]interface{}{
			"test_key": "test_value",
		},
	}

	// Save snapshot
	ctx := context.Background()
	err = manager.Save(ctx, snapshot)
	require.NoError(t, err)

	// Verify file exists
	assert.True(t, manager.Exists())

	// Load snapshot
	loadedSnapshot, err := manager.Load(ctx)
	require.NoError(t, err)
	require.NotNil(t, loadedSnapshot)

	// Verify basic properties
	assert.Equal(t, snapshot.Version, loadedSnapshot.Version)
	assert.Equal(t, snapshot.Timestamp.Unix(), loadedSnapshot.Timestamp.Unix())

	// Verify database structure
	assert.Len(t, loadedSnapshot.Databases, 1)
	testDB, exists := loadedSnapshot.Databases["test_db"]
	require.True(t, exists)
	assert.Equal(t, "test_db", testDB.Name)

	// Verify collection structure
	assert.Len(t, testDB.Collections, 1)
	testColl, exists := testDB.Collections["test_collection"]
	require.True(t, exists)
	assert.Equal(t, "test_collection", testColl.Name)
	assert.Equal(t, int64(2), testColl.VectorCount)
	assert.Equal(t, int64(0), testColl.DeletedCount)

	// Verify collection config
	assert.Equal(t, "test_collection", testColl.Config.Name)
	assert.Equal(t, types.DistanceMetricL2, testColl.Config.Metric)
	assert.Equal(t, 16, testColl.Config.HNSWParams.M)
	assert.Equal(t, 200, testColl.Config.HNSWParams.EfConstruction)

	// Verify vectors
	assert.Len(t, testColl.Vectors, 2)

	vector1 := testColl.Vectors[0]
	assert.Equal(t, "vector1", vector1.ID)
	assert.Equal(t, []float32{1.0, 2.0, 3.0}, vector1.Elements)
	assert.NotNil(t, vector1.Metadata)

	vector2 := testColl.Vectors[1]
	assert.Equal(t, "vector2", vector2.ID)
	assert.Equal(t, []float32{4.0, 5.0, 6.0}, vector2.Elements)
	assert.NotNil(t, vector2.Metadata)

	// Verify metadata
	assert.NotNil(t, loadedSnapshot.Metadata)
}

func TestRDBManager_FileOperations(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.rdb")

	// Create manager
	manager, err := NewRDBManager(filePath)
	require.NoError(t, err)

	// Initially file should not exist
	assert.False(t, manager.Exists())

	// Get info when file doesn't exist
	info, err := manager.GetInfo()
	require.NoError(t, err)
	assert.False(t, info.Exists)

	// Create minimal snapshot
	snapshot := RDBSnapshot{
		Version:   "1.0",
		Timestamp: time.Now(),
		Databases: make(map[string]DatabaseSnapshot),
		Metadata:  make(map[string]interface{}),
	}

	// Save snapshot
	ctx := context.Background()
	err = manager.Save(ctx, snapshot)
	require.NoError(t, err)

	// Now file should exist
	assert.True(t, manager.Exists())

	// Get file info
	info, err = manager.GetInfo()
	require.NoError(t, err)
	assert.True(t, info.Exists)
	assert.Greater(t, info.Size, int64(0))
	assert.Equal(t, filePath, info.Path)

	// Remove file
	err = manager.Remove()
	require.NoError(t, err)

	// File should no longer exist
	assert.False(t, manager.Exists())
}

func TestRDBManager_LoadNonExistentFile(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "nonexistent.rdb")

	// Create manager
	manager, err := NewRDBManager(filePath)
	require.NoError(t, err)

	// Load from non-existent file should return nil without error
	ctx := context.Background()
	snapshot, err := manager.Load(ctx)
	require.NoError(t, err)
	assert.Nil(t, snapshot)
}

func TestRDBManager_CreateSnapshot(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.rdb")

	// Create manager
	manager, err := NewRDBManager(filePath)
	require.NoError(t, err)

	// Create test database state
	databases := map[string]DatabaseState{
		"test_db": {
			Name:      "test_db",
			CreatedAt: time.Now(),
			Collections: map[string]CollectionState{
				"test_collection": {
					Name: "test_collection",
					Config: types.CollectionConfig{
						Name:       "test_collection",
						Metric:     types.DistanceMetricCosine,
						HNSWParams: types.DefaultHNSWParams(),
					},
					Vectors: []types.Vector{
						{
							ID:       "v1",
							Elements: []float32{0.1, 0.2, 0.3},
							Metadata: nil,
						},
					},
					VectorCount:  1,
					DeletedCount: 0,
					CreatedAt:    time.Now(),
					UpdatedAt:    time.Now(),
				},
			},
		},
	}

	// Create snapshot from database state
	snapshot := manager.CreateSnapshot(databases)

	// Verify snapshot structure
	assert.Equal(t, "1.0", snapshot.Version)
	assert.NotZero(t, snapshot.Timestamp)
	assert.Len(t, snapshot.Databases, 1)
	assert.NotNil(t, snapshot.Metadata)

	// Verify metadata contains statistics
	assert.Equal(t, 1, snapshot.Metadata["total_databases"])
	assert.Equal(t, 1, snapshot.Metadata["total_collections"])
	assert.Equal(t, int64(1), snapshot.Metadata["total_vectors"])

	// Verify database snapshot
	dbSnapshot, exists := snapshot.Databases["test_db"]
	require.True(t, exists)
	assert.Equal(t, "test_db", dbSnapshot.Name)
	assert.Len(t, dbSnapshot.Collections, 1)

	// Verify collection snapshot
	collSnapshot, exists := dbSnapshot.Collections["test_collection"]
	require.True(t, exists)
	assert.Equal(t, "test_collection", collSnapshot.Name)
	assert.Equal(t, int64(1), collSnapshot.VectorCount)
	assert.Equal(t, int64(0), collSnapshot.DeletedCount)
	assert.Len(t, collSnapshot.Vectors, 1)

	// Verify vector
	vector := collSnapshot.Vectors[0]
	assert.Equal(t, "v1", vector.ID)
	assert.Equal(t, []float32{0.1, 0.2, 0.3}, vector.Elements)
}
