// Package rdb provides Redis-style database snapshot functionality for Scintirete using FlatBuffers.
package rdb

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	flatbuffers "github.com/google/flatbuffers/go"
	fbrdb "github.com/scintirete/scintirete/internal/flatbuffers/rdb"
	"github.com/scintirete/scintirete/internal/utils"
	"github.com/scintirete/scintirete/pkg/types"
)

// RDBSnapshot represents a point-in-time snapshot of the database
type RDBSnapshot struct {
	Version   string                      `json:"version"`
	Timestamp time.Time                   `json:"timestamp"`
	Databases map[string]DatabaseSnapshot `json:"databases"`
	Metadata  map[string]interface{}      `json:"metadata,omitempty"`
}

// DatabaseSnapshot represents a snapshot of a single database
type DatabaseSnapshot struct {
	Name        string                        `json:"name"`
	Collections map[string]CollectionSnapshot `json:"collections"`
	CreatedAt   time.Time                     `json:"created_at"`
}

// CollectionSnapshot represents a snapshot of a single collection
type CollectionSnapshot struct {
	Name         string                 `json:"name"`
	Config       types.CollectionConfig `json:"config"`
	Vectors      []types.Vector         `json:"vectors"`
	VectorCount  int64                  `json:"vector_count"`
	DeletedCount int64                  `json:"deleted_count"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

// RDBInfo contains information about the RDB file
type RDBInfo struct {
	Exists  bool      `json:"exists"`
	Size    int64     `json:"size,omitempty"`
	ModTime time.Time `json:"mod_time,omitempty"`
	Path    string    `json:"path,omitempty"`
}

// DatabaseState represents the current state of a database for snapshotting
type DatabaseState struct {
	Name        string                     `json:"name"`
	Collections map[string]CollectionState `json:"collections"`
	CreatedAt   time.Time                  `json:"created_at"`
}

// CollectionState represents the current state of a collection for snapshotting
type CollectionState struct {
	Name         string                 `json:"name"`
	Config       types.CollectionConfig `json:"config"`
	Vectors      []types.Vector         `json:"vectors"`
	VectorCount  int64                  `json:"vector_count"`
	DeletedCount int64                  `json:"deleted_count"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

// BackupInfo contains information about a backup file
type BackupInfo struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
}

// RDBManager handles RDB snapshot operations using FlatBuffers
type RDBManager struct {
	mu       sync.RWMutex
	filePath string
	tempDir  string
}

// NewRDBManager creates a new RDB manager
func NewRDBManager(filePath string) (*RDBManager, error) {
	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, utils.ErrPersistenceFailedWithCause("failed to create RDB directory", err)
	}

	tempDir := filepath.Join(dir, "temp")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return nil, utils.ErrPersistenceFailedWithCause("failed to create RDB temp directory", err)
	}

	return &RDBManager{
		filePath: filePath,
		tempDir:  tempDir,
	}, nil
}

// Save creates and saves an RDB snapshot using FlatBuffers
func (r *RDBManager) Save(ctx context.Context, snapshot RDBSnapshot) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Set snapshot metadata
	snapshot.Version = "1.0"
	snapshot.Timestamp = time.Now()
	if snapshot.Metadata == nil {
		snapshot.Metadata = make(map[string]interface{})
	}
	snapshot.Metadata["created_by"] = "scintirete"

	// Create temporary file
	tempPath := filepath.Join(r.tempDir, fmt.Sprintf("rdb_%d.tmp", time.Now().UnixNano()))
	tempFile, err := os.Create(tempPath)
	if err != nil {
		return utils.ErrPersistenceFailedWithCause("failed to create temporary RDB file", err)
	}
	defer func() {
		tempFile.Close()
		os.Remove(tempPath) // Clean up on error
	}()

	// Convert to FlatBuffers and write
	if err := r.writeSnapshotFlatBuffers(tempFile, snapshot); err != nil {
		return err
	}

	// Sync to disk
	if err := tempFile.Sync(); err != nil {
		return utils.ErrPersistenceFailedWithCause("failed to sync RDB file", err)
	}
	if err := tempFile.Close(); err != nil {
		return utils.ErrPersistenceFailedWithCause("failed to close RDB file", err)
	}

	// Atomically replace the old file
	if err := os.Rename(tempPath, r.filePath); err != nil {
		return utils.ErrPersistenceFailedWithCause("failed to replace RDB file", err)
	}

	return nil
}

// Load loads an RDB snapshot from disk using FlatBuffers
func (r *RDBManager) Load(ctx context.Context) (*RDBSnapshot, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Check if file exists
	if _, err := os.Stat(r.filePath); os.IsNotExist(err) {
		return nil, nil // No snapshot exists, that's OK
	} else if err != nil {
		return nil, utils.ErrRecoveryFailed("failed to check RDB file: " + err.Error())
	}

	// Read all data
	data, err := os.ReadFile(r.filePath)
	if err != nil {
		return nil, utils.ErrRecoveryFailed("failed to read RDB file: " + err.Error())
	}

	// Parse FlatBuffers data
	fbSnapshot := fbrdb.GetRootAsRDBSnapshot(data, 0)

	// Convert to Go struct
	snapshot := &RDBSnapshot{
		Version:   string(fbSnapshot.Version()),
		Timestamp: time.Unix(fbSnapshot.Timestamp(), 0),
		Databases: make(map[string]DatabaseSnapshot),
	}

	// Parse metadata
	if metadataBytes := fbSnapshot.Metadata(); metadataBytes != nil {
		if err := json.Unmarshal(metadataBytes, &snapshot.Metadata); err != nil {
			return nil, utils.ErrCorruptedData("failed to parse metadata: " + err.Error())
		}
	}

	// Parse databases
	for i := 0; i < fbSnapshot.DatabasesLength(); i++ {
		fbDb := new(fbrdb.DatabaseSnapshot)
		if !fbSnapshot.Databases(fbDb, i) {
			return nil, utils.ErrCorruptedData("failed to parse database")
		}

		dbSnapshot, err := r.parseDatabaseSnapshot(fbDb)
		if err != nil {
			return nil, err
		}

		snapshot.Databases[dbSnapshot.Name] = *dbSnapshot
	}

	// Validate snapshot
	if err := r.validateSnapshot(snapshot); err != nil {
		return nil, err
	}

	return snapshot, nil
}

// writeSnapshotFlatBuffers writes the snapshot using FlatBuffers format
func (r *RDBManager) writeSnapshotFlatBuffers(file *os.File, snapshot RDBSnapshot) error {
	builder := flatbuffers.NewBuilder(0)

	// Convert metadata to JSON string
	metadataBytes, err := json.Marshal(snapshot.Metadata)
	if err != nil {
		return utils.ErrPersistenceFailedWithCause("failed to marshal metadata", err)
	}

	// Create databases vector
	var dbSnapshots []flatbuffers.UOffsetT
	for _, dbSnapshot := range snapshot.Databases {
		dbOffset, err := r.createDatabaseSnapshot(builder, dbSnapshot)
		if err != nil {
			return err
		}
		dbSnapshots = append(dbSnapshots, dbOffset)
	}

	// Create databases vector
	fbrdb.RDBSnapshotStartDatabasesVector(builder, len(dbSnapshots))
	for i := len(dbSnapshots) - 1; i >= 0; i-- {
		builder.PrependUOffsetT(dbSnapshots[i])
	}
	databasesVector := builder.EndVector(len(dbSnapshots))

	// Create strings
	versionStr := builder.CreateString(snapshot.Version)
	metadataStr := builder.CreateString(string(metadataBytes))

	// Create RDB snapshot
	fbrdb.RDBSnapshotStart(builder)
	fbrdb.RDBSnapshotAddVersion(builder, versionStr)
	fbrdb.RDBSnapshotAddTimestamp(builder, snapshot.Timestamp.Unix())
	fbrdb.RDBSnapshotAddDatabases(builder, databasesVector)
	fbrdb.RDBSnapshotAddMetadata(builder, metadataStr)
	rdbSnapshot := fbrdb.RDBSnapshotEnd(builder)

	// Finish the FlatBuffer
	builder.Finish(rdbSnapshot)

	// Write to file
	if _, err := file.Write(builder.FinishedBytes()); err != nil {
		return utils.ErrPersistenceFailedWithCause("failed to write FlatBuffers data", err)
	}

	return nil
}

// createDatabaseSnapshot creates a FlatBuffers DatabaseSnapshot
func (r *RDBManager) createDatabaseSnapshot(builder *flatbuffers.Builder, dbSnapshot DatabaseSnapshot) (flatbuffers.UOffsetT, error) {
	// Create collections vector
	var collSnapshots []flatbuffers.UOffsetT
	for _, collSnapshot := range dbSnapshot.Collections {
		collOffset, err := r.createCollectionSnapshot(builder, collSnapshot)
		if err != nil {
			return 0, err
		}
		collSnapshots = append(collSnapshots, collOffset)
	}

	// Create collections vector
	fbrdb.DatabaseSnapshotStartCollectionsVector(builder, len(collSnapshots))
	for i := len(collSnapshots) - 1; i >= 0; i-- {
		builder.PrependUOffsetT(collSnapshots[i])
	}
	collectionsVector := builder.EndVector(len(collSnapshots))

	// Create name string
	nameStr := builder.CreateString(dbSnapshot.Name)

	// Create database snapshot
	fbrdb.DatabaseSnapshotStart(builder)
	fbrdb.DatabaseSnapshotAddName(builder, nameStr)
	fbrdb.DatabaseSnapshotAddCollections(builder, collectionsVector)
	fbrdb.DatabaseSnapshotAddCreatedAt(builder, dbSnapshot.CreatedAt.Unix())

	return fbrdb.DatabaseSnapshotEnd(builder), nil
}

// createCollectionSnapshot creates a FlatBuffers CollectionSnapshot
func (r *RDBManager) createCollectionSnapshot(builder *flatbuffers.Builder, collSnapshot CollectionSnapshot) (flatbuffers.UOffsetT, error) {
	// Create vectors vector
	var vectors []flatbuffers.UOffsetT
	for _, vector := range collSnapshot.Vectors {
		vectorOffset, err := r.createVector(builder, vector)
		if err != nil {
			return 0, err
		}
		vectors = append(vectors, vectorOffset)
	}

	// Create vectors vector
	fbrdb.CollectionSnapshotStartVectorsVector(builder, len(vectors))
	for i := len(vectors) - 1; i >= 0; i-- {
		builder.PrependUOffsetT(vectors[i])
	}
	vectorsVector := builder.EndVector(len(vectors))

	// Create collection config
	configOffset, err := r.createCollectionConfig(builder, collSnapshot.Config)
	if err != nil {
		return 0, err
	}

	// Create name string
	nameStr := builder.CreateString(collSnapshot.Name)

	// Create collection snapshot
	fbrdb.CollectionSnapshotStart(builder)
	fbrdb.CollectionSnapshotAddName(builder, nameStr)
	fbrdb.CollectionSnapshotAddConfig(builder, configOffset)
	fbrdb.CollectionSnapshotAddVectors(builder, vectorsVector)
	fbrdb.CollectionSnapshotAddVectorCount(builder, collSnapshot.VectorCount)
	fbrdb.CollectionSnapshotAddDeletedCount(builder, collSnapshot.DeletedCount)
	fbrdb.CollectionSnapshotAddCreatedAt(builder, collSnapshot.CreatedAt.Unix())
	fbrdb.CollectionSnapshotAddUpdatedAt(builder, collSnapshot.UpdatedAt.Unix())

	return fbrdb.CollectionSnapshotEnd(builder), nil
}

// createVector creates a FlatBuffers Vector
func (r *RDBManager) createVector(builder *flatbuffers.Builder, vector types.Vector) (flatbuffers.UOffsetT, error) {
	// Create elements vector
	fbrdb.VectorStartElementsVector(builder, len(vector.Elements))
	for i := len(vector.Elements) - 1; i >= 0; i-- {
		builder.PrependFloat32(vector.Elements[i])
	}
	elementsVector := builder.EndVector(len(vector.Elements))

	// Convert metadata to JSON string
	metadataBytes, err := json.Marshal(vector.Metadata)
	if err != nil {
		return 0, utils.ErrPersistenceFailedWithCause("failed to marshal vector metadata", err)
	}

	// Create strings
	idStr := builder.CreateString(vector.ID)
	metadataStr := builder.CreateString(string(metadataBytes))

	// Create vector
	fbrdb.VectorStart(builder)
	fbrdb.VectorAddId(builder, idStr)
	fbrdb.VectorAddElements(builder, elementsVector)
	fbrdb.VectorAddMetadata(builder, metadataStr)

	return fbrdb.VectorEnd(builder), nil
}

// createCollectionConfig creates a FlatBuffers CollectionConfig
func (r *RDBManager) createCollectionConfig(builder *flatbuffers.Builder, config types.CollectionConfig) (flatbuffers.UOffsetT, error) {
	// Create HNSW params
	hnswOffset, err := r.createHNSWParams(builder, config.HNSWParams)
	if err != nil {
		return 0, err
	}

	// Create name string
	nameStr := builder.CreateString(config.Name)

	// Create collection config
	fbrdb.CollectionConfigStart(builder)
	fbrdb.CollectionConfigAddName(builder, nameStr)
	fbrdb.CollectionConfigAddMetric(builder, fbrdb.DistanceMetric(config.Metric))
	fbrdb.CollectionConfigAddHnswParams(builder, hnswOffset)

	return fbrdb.CollectionConfigEnd(builder), nil
}

// createHNSWParams creates a FlatBuffers HNSWParams
func (r *RDBManager) createHNSWParams(builder *flatbuffers.Builder, params types.HNSWParams) (flatbuffers.UOffsetT, error) {
	fbrdb.HNSWParamsStart(builder)
	fbrdb.HNSWParamsAddM(builder, int32(params.M))
	fbrdb.HNSWParamsAddEfConstruction(builder, int32(params.EfConstruction))
	fbrdb.HNSWParamsAddEfSearch(builder, int32(params.EfSearch))
	fbrdb.HNSWParamsAddMaxLayers(builder, int32(params.MaxLayers))
	fbrdb.HNSWParamsAddSeed(builder, params.Seed)

	return fbrdb.HNSWParamsEnd(builder), nil
}

// parseDatabaseSnapshot parses a FlatBuffers DatabaseSnapshot to Go struct
func (r *RDBManager) parseDatabaseSnapshot(fbDb *fbrdb.DatabaseSnapshot) (*DatabaseSnapshot, error) {
	dbSnapshot := &DatabaseSnapshot{
		Name:        string(fbDb.Name()),
		Collections: make(map[string]CollectionSnapshot),
		CreatedAt:   time.Unix(fbDb.CreatedAt(), 0),
	}

	// Parse collections
	for i := 0; i < fbDb.CollectionsLength(); i++ {
		fbColl := new(fbrdb.CollectionSnapshot)
		if !fbDb.Collections(fbColl, i) {
			return nil, utils.ErrCorruptedData("failed to parse collection")
		}

		collSnapshot, err := r.parseCollectionSnapshot(fbColl)
		if err != nil {
			return nil, err
		}

		dbSnapshot.Collections[collSnapshot.Name] = *collSnapshot
	}

	return dbSnapshot, nil
}

// parseCollectionSnapshot parses a FlatBuffers CollectionSnapshot to Go struct
func (r *RDBManager) parseCollectionSnapshot(fbColl *fbrdb.CollectionSnapshot) (*CollectionSnapshot, error) {
	// Parse config
	config, err := r.parseCollectionConfig(fbColl.Config(nil))
	if err != nil {
		return nil, err
	}

	collSnapshot := &CollectionSnapshot{
		Name:         string(fbColl.Name()),
		Config:       *config,
		VectorCount:  fbColl.VectorCount(),
		DeletedCount: fbColl.DeletedCount(),
		CreatedAt:    time.Unix(fbColl.CreatedAt(), 0),
		UpdatedAt:    time.Unix(fbColl.UpdatedAt(), 0),
	}

	// Parse vectors
	for i := 0; i < fbColl.VectorsLength(); i++ {
		fbVec := new(fbrdb.Vector)
		if !fbColl.Vectors(fbVec, i) {
			return nil, utils.ErrCorruptedData("failed to parse vector")
		}

		vector, err := r.parseVector(fbVec)
		if err != nil {
			return nil, err
		}

		collSnapshot.Vectors = append(collSnapshot.Vectors, *vector)
	}

	return collSnapshot, nil
}

// parseVector parses a FlatBuffers Vector to Go struct
func (r *RDBManager) parseVector(fbVec *fbrdb.Vector) (*types.Vector, error) {
	// Parse elements
	elements := make([]float32, fbVec.ElementsLength())
	for i := 0; i < fbVec.ElementsLength(); i++ {
		elements[i] = fbVec.Elements(i)
	}

	// Parse metadata
	var metadata map[string]interface{}
	if metadataBytes := fbVec.Metadata(); metadataBytes != nil {
		if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
			return nil, utils.ErrCorruptedData("failed to parse vector metadata: " + err.Error())
		}
	}

	return &types.Vector{
		ID:       string(fbVec.Id()),
		Elements: elements,
		Metadata: metadata,
	}, nil
}

// parseCollectionConfig parses a FlatBuffers CollectionConfig to Go struct
func (r *RDBManager) parseCollectionConfig(fbConfig *fbrdb.CollectionConfig) (*types.CollectionConfig, error) {
	// Parse HNSW params
	hnswParams, err := r.parseHNSWParams(fbConfig.HnswParams(nil))
	if err != nil {
		return nil, err
	}

	return &types.CollectionConfig{
		Name:       string(fbConfig.Name()),
		Metric:     types.DistanceMetric(fbConfig.Metric()),
		HNSWParams: *hnswParams,
	}, nil
}

// parseHNSWParams parses a FlatBuffers HNSWParams to Go struct
func (r *RDBManager) parseHNSWParams(fbParams *fbrdb.HNSWParams) (*types.HNSWParams, error) {
	return &types.HNSWParams{
		M:              int(fbParams.M()),
		EfConstruction: int(fbParams.EfConstruction()),
		EfSearch:       int(fbParams.EfSearch()),
		MaxLayers:      int(fbParams.MaxLayers()),
		Seed:           fbParams.Seed(),
	}, nil
}

// validateSnapshot validates the integrity of a loaded snapshot
func (r *RDBManager) validateSnapshot(snapshot *RDBSnapshot) error {
	if snapshot.Version == "" {
		return utils.ErrCorruptedData("RDB snapshot missing version")
	}

	if snapshot.Timestamp.IsZero() {
		return utils.ErrCorruptedData("RDB snapshot missing timestamp")
	}

	// Validate version compatibility
	if snapshot.Version != "1.0" {
		return utils.ErrCorruptedData(fmt.Sprintf("unsupported RDB version: %s", snapshot.Version))
	}

	// Validate databases
	for dbName, dbSnapshot := range snapshot.Databases {
		if dbSnapshot.Name != dbName {
			return utils.ErrCorruptedData(fmt.Sprintf("database name mismatch: key=%s, name=%s", dbName, dbSnapshot.Name))
		}

		// Validate collections
		for collName, collSnapshot := range dbSnapshot.Collections {
			if collSnapshot.Name != collName {
				return utils.ErrCorruptedData(fmt.Sprintf("collection name mismatch: key=%s, name=%s", collName, collSnapshot.Name))
			}

			if len(collSnapshot.Vectors) != int(collSnapshot.VectorCount) {
				return utils.ErrCorruptedData(fmt.Sprintf("vector count mismatch in collection %s: expected=%d, actual=%d",
					collName, collSnapshot.VectorCount, len(collSnapshot.Vectors)))
			}

			// Validate vectors have consistent dimensions if any exist
			if len(collSnapshot.Vectors) > 0 {
				expectedDim := len(collSnapshot.Vectors[0].Elements)
				for i, vector := range collSnapshot.Vectors {
					if len(vector.Elements) != expectedDim {
						return utils.ErrCorruptedData(fmt.Sprintf("inconsistent vector dimension in collection %s: vector[%d] has %d dimensions, expected %d",
							collName, i, len(vector.Elements), expectedDim))
					}
				}
			}
		}
	}

	return nil
}

// Exists checks if an RDB snapshot file exists
func (r *RDBManager) Exists() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, err := os.Stat(r.filePath)
	return err == nil
}

// GetInfo returns information about the RDB file
func (r *RDBManager) GetInfo() (*RDBInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	info, err := os.Stat(r.filePath)
	if os.IsNotExist(err) {
		return &RDBInfo{
			Exists: false,
		}, nil
	} else if err != nil {
		return nil, utils.ErrPersistenceFailedWithCause("failed to get RDB file info", err)
	}

	return &RDBInfo{
		Exists:  true,
		Size:    info.Size(),
		ModTime: info.ModTime(),
		Path:    r.filePath,
	}, nil
}

// Remove deletes the RDB snapshot file
func (r *RDBManager) Remove() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := os.Remove(r.filePath); err != nil && !os.IsNotExist(err) {
		return utils.ErrPersistenceFailedWithCause("failed to remove RDB file", err)
	}

	return nil
}

// CreateSnapshot creates a snapshot from current database state
func (r *RDBManager) CreateSnapshot(databases map[string]DatabaseState) RDBSnapshot {
	snapshot := RDBSnapshot{
		Version:   "1.0",
		Timestamp: time.Now(),
		Databases: make(map[string]DatabaseSnapshot),
		Metadata:  make(map[string]interface{}),
	}

	for dbName, dbState := range databases {
		dbSnapshot := DatabaseSnapshot{
			Name:        dbName,
			Collections: make(map[string]CollectionSnapshot),
			CreatedAt:   dbState.CreatedAt,
		}

		for collName, collState := range dbState.Collections {
			collSnapshot := CollectionSnapshot{
				Name:         collName,
				Config:       collState.Config,
				Vectors:      collState.Vectors,
				VectorCount:  collState.VectorCount,
				DeletedCount: collState.DeletedCount,
				CreatedAt:    collState.CreatedAt,
				UpdatedAt:    collState.UpdatedAt,
			}
			dbSnapshot.Collections[collName] = collSnapshot
		}

		snapshot.Databases[dbName] = dbSnapshot
	}

	// Add metadata
	snapshot.Metadata["total_databases"] = len(databases)
	totalCollections := 0
	totalVectors := int64(0)
	for _, db := range databases {
		totalCollections += len(db.Collections)
		for _, coll := range db.Collections {
			totalVectors += coll.VectorCount
		}
	}
	snapshot.Metadata["total_collections"] = totalCollections
	snapshot.Metadata["total_vectors"] = totalVectors

	return snapshot
}

// BackupManager handles RDB backup operations
type BackupManager struct {
	rdbManager *RDBManager
	backupDir  string
}

// NewBackupManager creates a new backup manager
func NewBackupManager(rdbManager *RDBManager, backupDir string) (*BackupManager, error) {
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return nil, utils.ErrPersistenceFailedWithCause("failed to create backup directory", err)
	}

	return &BackupManager{
		rdbManager: rdbManager,
		backupDir:  backupDir,
	}, nil
}

// CreateBackup creates a timestamped backup of the current RDB file
func (bm *BackupManager) CreateBackup(ctx context.Context) (string, error) {
	// Load current snapshot
	snapshot, err := bm.rdbManager.Load(ctx)
	if err != nil {
		return "", err
	}
	if snapshot == nil {
		return "", utils.ErrPersistenceFailed("no RDB snapshot to backup")
	}

	// Create backup filename with timestamp
	timestamp := time.Now().Format("20060102_150405")
	backupFilename := fmt.Sprintf("rdb_backup_%s.flatbuf", timestamp)
	backupPath := filepath.Join(bm.backupDir, backupFilename)

	// Save backup using FlatBuffers format
	tempManager, err := NewRDBManager(backupPath)
	if err != nil {
		return "", err
	}

	if err := tempManager.Save(ctx, *snapshot); err != nil {
		return "", err
	}

	return backupPath, nil
}

// ListBackups returns a list of available backups
func (bm *BackupManager) ListBackups() ([]BackupInfo, error) {
	entries, err := os.ReadDir(bm.backupDir)
	if err != nil {
		return nil, utils.ErrPersistenceFailedWithCause("failed to read backup directory", err)
	}

	var backups []BackupInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if filepath.Ext(entry.Name()) == ".flatbuf" {
			info, err := entry.Info()
			if err != nil {
				continue
			}

			backups = append(backups, BackupInfo{
				Name:    entry.Name(),
				Path:    filepath.Join(bm.backupDir, entry.Name()),
				Size:    info.Size(),
				ModTime: info.ModTime(),
			})
		}
	}

	return backups, nil
}

// RestoreFromBackup restores the database from a backup file
func (bm *BackupManager) RestoreFromBackup(ctx context.Context, backupPath string) error {
	// Load backup
	backupManager, err := NewRDBManager(backupPath)
	if err != nil {
		return err
	}

	snapshot, err := backupManager.Load(ctx)
	if err != nil {
		return err
	}
	if snapshot == nil {
		return utils.ErrRecoveryFailed("backup file is empty")
	}

	// Save as current RDB
	return bm.rdbManager.Save(ctx, *snapshot)
}
