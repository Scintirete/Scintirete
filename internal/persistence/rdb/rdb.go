// Package rdb provides Redis-style database snapshot functionality for Scintirete.
package rdb

import (
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

// RDBManager handles RDB snapshot operations
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

// Save creates and saves an RDB snapshot
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

	// Encode snapshot to JSON with pretty printing for debugging
	encoder := json.NewEncoder(tempFile)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(snapshot); err != nil {
		return utils.ErrPersistenceFailedWithCause("failed to encode RDB snapshot", err)
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

// Load loads an RDB snapshot from disk
func (r *RDBManager) Load(ctx context.Context) (*RDBSnapshot, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Check if file exists
	if _, err := os.Stat(r.filePath); os.IsNotExist(err) {
		return nil, nil // No snapshot exists, that's OK
	} else if err != nil {
		return nil, utils.ErrRecoveryFailed("failed to check RDB file: " + err.Error())
	}

	// Open and read file
	file, err := os.Open(r.filePath)
	if err != nil {
		return nil, utils.ErrRecoveryFailed("failed to open RDB file: " + err.Error())
	}
	defer file.Close()

	// Decode snapshot
	var snapshot RDBSnapshot
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&snapshot); err != nil {
		return nil, utils.ErrCorruptedData("failed to decode RDB snapshot: " + err.Error())
	}

	// Validate snapshot
	if err := r.validateSnapshot(&snapshot); err != nil {
		return nil, err
	}

	return &snapshot, nil
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
	backupFilename := fmt.Sprintf("rdb_backup_%s.json", timestamp)
	backupPath := filepath.Join(bm.backupDir, backupFilename)

	// Save backup
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

		if filepath.Ext(entry.Name()) == ".json" {
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

// BackupInfo contains information about a backup file
type BackupInfo struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
}
