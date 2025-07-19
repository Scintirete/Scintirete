// Package persistence provides unified data persistence functionality for Scintirete.
package persistence

import (
	"context"
	"sync"
	"time"

	"github.com/scintirete/scintirete/internal/persistence/aof"
	"github.com/scintirete/scintirete/internal/persistence/rdb"
	"github.com/scintirete/scintirete/internal/utils"
	"github.com/scintirete/scintirete/pkg/types"
)

// Manager implements the unified persistence interface
type Manager struct {
	mu            sync.RWMutex
	aofLogger     *aof.AOFLogger
	rdbManager    *rdb.RDBManager
	backupManager *rdb.BackupManager
	cmdBuilder    *aof.CommandBuilder
	
	// Configuration
	config        Config
	
	// Background tasks
	stopTasks     chan struct{}
	taskWG        sync.WaitGroup
	
	// Statistics
	stats         Stats
}

// Config contains persistence configuration
type Config struct {
	DataDir         string
	RDBFilename     string
	AOFFilename     string
	AOFSyncStrategy string
	
	// Background task intervals
	RDBInterval    time.Duration // How often to create RDB snapshots
	AOFRewriteSize int64         // Rewrite AOF when it exceeds this size
	BackupRetention int          // Number of backups to keep
}

// Stats contains persistence statistics
type Stats struct {
	AOFStats aof.AOFStats `json:"aof_stats"`
	RDBInfo  *rdb.RDBInfo `json:"rdb_info"`
	
	LastRDBSave    time.Time `json:"last_rdb_save"`
	LastAOFRewrite time.Time `json:"last_aof_rewrite"`
	
	RecoveryTime   time.Duration `json:"recovery_time"`
	RecoveredCommands int64      `json:"recovered_commands"`
}

// NewManager creates a new persistence manager
func NewManager(config Config) (*Manager, error) {
	// Create AOF logger
	aofPath := config.DataDir + "/" + config.AOFFilename
	syncStrategy := aof.SyncStrategy(config.AOFSyncStrategy)
	aofLogger, err := aof.NewAOFLogger(aofPath, syncStrategy)
	if err != nil {
		return nil, utils.ErrPersistenceFailedWithCause("failed to create AOF logger", err)
	}
	
	// Create RDB manager
	rdbPath := config.DataDir + "/" + config.RDBFilename
	rdbManager, err := rdb.NewRDBManager(rdbPath)
	if err != nil {
		return nil, utils.ErrPersistenceFailedWithCause("failed to create RDB manager", err)
	}
	
	// Create backup manager
	backupDir := config.DataDir + "/backups"
	backupManager, err := rdb.NewBackupManager(rdbManager, backupDir)
	if err != nil {
		return nil, utils.ErrPersistenceFailedWithCause("failed to create backup manager", err)
	}
	
	// Set default intervals if not specified
	if config.RDBInterval == 0 {
		config.RDBInterval = 5 * time.Minute // Default: save RDB every 5 minutes
	}
	if config.AOFRewriteSize == 0 {
		config.AOFRewriteSize = 64 * 1024 * 1024 // Default: rewrite when AOF exceeds 64MB
	}
	if config.BackupRetention == 0 {
		config.BackupRetention = 7 // Keep 7 backups by default
	}
	
	return &Manager{
		aofLogger:     aofLogger,
		rdbManager:    rdbManager,
		backupManager: backupManager,
		cmdBuilder:    aof.NewCommandBuilder(),
		config:        config,
		stopTasks:     make(chan struct{}),
	}, nil
}

// WriteAOF writes a command to the append-only file
func (m *Manager) WriteAOF(ctx context.Context, command types.AOFCommand) error {
	return m.aofLogger.WriteCommand(ctx, command)
}

// LoadFromRDB loads data from the latest RDB snapshot
func (m *Manager) LoadFromRDB(ctx context.Context) error {
	return utils.ErrPersistenceFailed("LoadFromRDB should not be called directly - use Recover instead")
}

// SaveRDB creates a new RDB snapshot
func (m *Manager) SaveRDB(ctx context.Context) error {
	return utils.ErrPersistenceFailed("SaveRDB should not be called directly - use SaveSnapshot instead")
}

// Recover replays the AOF log to restore the latest state
func (m *Manager) Recover(ctx context.Context) error {
	startTime := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	
	commandCount := int64(0)
	
	// Step 1: Load RDB snapshot if it exists
	snapshot, err := m.rdbManager.Load(ctx)
	if err != nil {
		return utils.ErrRecoveryFailed("failed to load RDB snapshot: " + err.Error())
	}
	
	if snapshot != nil {
		// TODO: Apply RDB snapshot to database engine
		// This will be implemented when we have the database engine
	}
	
	// Step 2: Replay AOF commands
	err = m.aofLogger.Replay(ctx, func(command types.AOFCommand) error {
		commandCount++
		// TODO: Apply command to database engine
		// This will be implemented when we have the database engine
		return nil
	})
	
	if err != nil {
		return utils.ErrRecoveryFailed("failed to replay AOF: " + err.Error())
	}
	
	// Update statistics
	m.stats.RecoveryTime = time.Since(startTime)
	m.stats.RecoveredCommands = commandCount
	
	return nil
}

// SaveSnapshot creates an RDB snapshot with current database state
func (m *Manager) SaveSnapshot(ctx context.Context, databases map[string]rdb.DatabaseState) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Create snapshot
	snapshot := m.rdbManager.CreateSnapshot(databases)
	
	// Save to disk
	if err := m.rdbManager.Save(ctx, snapshot); err != nil {
		return err
	}
	
	m.stats.LastRDBSave = time.Now()
	return nil
}

// StartBackgroundTasks starts periodic snapshot and AOF rewrite tasks
func (m *Manager) StartBackgroundTasks(ctx context.Context) error {
	m.taskWG.Add(2)
	
	// Start RDB snapshot task
	go m.runRDBSnapshotTask(ctx)
	
	// Start AOF rewrite task
	go m.runAOFRewriteTask(ctx)
	
	return nil
}

// Stop gracefully stops all persistence operations
func (m *Manager) Stop(ctx context.Context) error {
	// Stop background tasks
	close(m.stopTasks)
	m.taskWG.Wait()
	
	// Close persistence components
	if err := m.aofLogger.Close(); err != nil {
		return utils.ErrPersistenceFailedWithCause("failed to close AOF logger", err)
	}
	
	return nil
}

// GetStats returns current persistence statistics
func (m *Manager) GetStats() Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	stats := m.stats
	stats.AOFStats = m.aofLogger.GetStats()
	
	if rdbInfo, err := m.rdbManager.GetInfo(); err == nil {
		stats.RDBInfo = rdbInfo
	}
	
	return stats
}

// CreateBackup creates a timestamped backup
func (m *Manager) CreateBackup(ctx context.Context) (string, error) {
	return m.backupManager.CreateBackup(ctx)
}

// ListBackups returns available backups
func (m *Manager) ListBackups() ([]rdb.BackupInfo, error) {
	return m.backupManager.ListBackups()
}

// RestoreFromBackup restores from a specific backup
func (m *Manager) RestoreFromBackup(ctx context.Context, backupPath string) error {
	return m.backupManager.RestoreFromBackup(ctx, backupPath)
}

// RewriteAOF triggers an AOF rewrite
func (m *Manager) RewriteAOF(ctx context.Context, commands []types.AOFCommand) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if err := m.aofLogger.Rewrite(ctx, commands); err != nil {
		return err
	}
	
	m.stats.LastAOFRewrite = time.Now()
	return nil
}

// Command builders for different operations

// LogCreateDatabase logs a database creation command
func (m *Manager) LogCreateDatabase(ctx context.Context, dbName string) error {
	command := m.cmdBuilder.CreateDatabase(dbName)
	return m.WriteAOF(ctx, command)
}

// LogDropDatabase logs a database deletion command
func (m *Manager) LogDropDatabase(ctx context.Context, dbName string) error {
	command := m.cmdBuilder.DropDatabase(dbName)
	return m.WriteAOF(ctx, command)
}

// LogCreateCollection logs a collection creation command
func (m *Manager) LogCreateCollection(ctx context.Context, dbName, collName string, config types.CollectionConfig) error {
	command := m.cmdBuilder.CreateCollection(dbName, collName, config)
	return m.WriteAOF(ctx, command)
}

// LogDropCollection logs a collection deletion command
func (m *Manager) LogDropCollection(ctx context.Context, dbName, collName string) error {
	command := m.cmdBuilder.DropCollection(dbName, collName)
	return m.WriteAOF(ctx, command)
}

// LogInsertVectors logs a vector insertion command
func (m *Manager) LogInsertVectors(ctx context.Context, dbName, collName string, vectors []types.Vector) error {
	command := m.cmdBuilder.InsertVectors(dbName, collName, vectors)
	return m.WriteAOF(ctx, command)
}

// LogDeleteVectors logs a vector deletion command
func (m *Manager) LogDeleteVectors(ctx context.Context, dbName, collName string, ids []string) error {
	command := m.cmdBuilder.DeleteVectors(dbName, collName, ids)
	return m.WriteAOF(ctx, command)
}

// Background task implementations

// runRDBSnapshotTask runs periodic RDB snapshots
func (m *Manager) runRDBSnapshotTask(ctx context.Context) {
	defer m.taskWG.Done()
	
	ticker := time.NewTicker(m.config.RDBInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			// TODO: Get current database state and create snapshot
			// This will be implemented when we have the database engine
			
		case <-m.stopTasks:
			return
		case <-ctx.Done():
			return
		}
	}
}

// runAOFRewriteTask monitors AOF size and triggers rewrites
func (m *Manager) runAOFRewriteTask(ctx context.Context) {
	defer m.taskWG.Done()
	
	ticker := time.NewTicker(time.Minute) // Check every minute
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			stats := m.aofLogger.GetStats()
			if stats.FileSize > m.config.AOFRewriteSize {
				// TODO: Get optimized commands and rewrite AOF
				// This will be implemented when we have the database engine
			}
			
		case <-m.stopTasks:
			return
		case <-ctx.Done():
			return
		}
	}
}

// Utility functions

// DefaultConfig returns a default persistence configuration
func DefaultConfig(dataDir string) Config {
	return Config{
		DataDir:         dataDir,
		RDBFilename:     "dump.rdb",
		AOFFilename:     "appendonly.aof",
		AOFSyncStrategy: "everysec",
		RDBInterval:     5 * time.Minute,
		AOFRewriteSize:  64 * 1024 * 1024, // 64MB
		BackupRetention: 7,
	}
}

// ValidateConfig validates persistence configuration
func ValidateConfig(config Config) error {
	if config.DataDir == "" {
		return utils.ErrConfig("data directory cannot be empty")
	}
	
	if config.RDBFilename == "" {
		return utils.ErrConfig("RDB filename cannot be empty")
	}
	
	if config.AOFFilename == "" {
		return utils.ErrConfig("AOF filename cannot be empty")
	}
	
	validStrategies := map[string]bool{
		"always": true, "everysec": true, "no": true,
	}
	if !validStrategies[config.AOFSyncStrategy] {
		return utils.ErrConfig("invalid AOF sync strategy: " + config.AOFSyncStrategy)
	}
	
	if config.RDBInterval <= 0 {
		return utils.ErrConfig("RDB interval must be positive")
	}
	
	if config.AOFRewriteSize <= 0 {
		return utils.ErrConfig("AOF rewrite size must be positive")
	}
	
	if config.BackupRetention <= 0 {
		return utils.ErrConfig("backup retention must be positive")
	}
	
	return nil
} 