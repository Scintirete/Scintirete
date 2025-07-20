// Package persistence provides unified data persistence functionality for Scintirete.
package persistence

import (
	"context"
	"sync"
	"time"

	"github.com/scintirete/scintirete/internal/core"
	"github.com/scintirete/scintirete/internal/observability/logger"
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
	cmdApplier    *CommandApplier
	logger        core.Logger // Add logger field

	// Configuration
	config Config

	// Background tasks
	stopTasks chan struct{}
	taskWG    sync.WaitGroup

	// Statistics
	stats Stats
}

// Config contains persistence configuration
type Config struct {
	DataDir         string
	RDBFilename     string
	AOFFilename     string
	AOFSyncStrategy string

	// Background task intervals
	RDBInterval     time.Duration // How often to create RDB snapshots
	AOFRewriteSize  int64         // Rewrite AOF when it exceeds this size
	BackupRetention int           // Number of backups to keep

	// Optional: Logger for persistence component
	Logger core.Logger
}

// Stats contains persistence statistics
type Stats struct {
	AOFStats aof.AOFStats `json:"aof_stats"`
	RDBInfo  *rdb.RDBInfo `json:"rdb_info"`

	LastRDBSave    time.Time `json:"last_rdb_save"`
	LastAOFRewrite time.Time `json:"last_aof_rewrite"`

	RecoveryTime      time.Duration `json:"recovery_time"`
	RecoveredCommands int64         `json:"recovered_commands"`
}

// NewManager creates a new persistence manager
func NewManager(config Config) (*Manager, error) {
	return NewManagerWithEngine(config, nil)
}

// NewManagerWithEngine creates a new persistence manager with database engine support
func NewManagerWithEngine(config Config, dbEngine DatabaseEngine) (*Manager, error) {
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

	// Create default logger if not provided in config
	var persistenceLogger core.Logger
	if config.Logger != nil {
		persistenceLogger = config.Logger
	} else {
		// Create a default logger with INFO level and JSON format
		defaultLogger, err := logger.NewFromConfigString("info", "json")
		if err != nil {
			return nil, utils.ErrPersistenceFailedWithCause("failed to create default logger", err)
		}
		persistenceLogger = defaultLogger.WithFields(map[string]interface{}{
			"component": "persistence",
		})
	}

	manager := &Manager{
		aofLogger:     aofLogger,
		rdbManager:    rdbManager,
		backupManager: backupManager,
		cmdBuilder:    aof.NewCommandBuilder(),
		config:        config,
		stopTasks:     make(chan struct{}),
		logger:        persistenceLogger,
	}

	// Set up command applier if database engine is provided
	if dbEngine != nil {
		manager.cmdApplier = NewCommandApplier(dbEngine)
	}

	return manager, nil
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

	// Check if database engine is connected
	if m.cmdApplier == nil {
		m.logger.Error(ctx, "AOF recovery disabled: database engine not connected to persistence manager", nil, map[string]interface{}{
			"component": "persistence_recovery",
			"message":   "Data recovery will be incomplete - AOF commands will be read but not applied to database",
		})
	} else {
		m.logger.Info(ctx, "Starting data recovery with database engine connected", map[string]interface{}{
			"component": "persistence_recovery",
		})
	}

	// Step 1: Load RDB snapshot if it exists
	m.logger.Debug(ctx, "Attempting to load RDB snapshot", map[string]interface{}{
		"component": "persistence_recovery",
	})

	snapshot, err := m.rdbManager.Load(ctx)
	if err != nil {
		m.logger.Error(ctx, "Failed to load RDB snapshot", err, map[string]interface{}{
			"component": "persistence_recovery",
		})
		return utils.ErrRecoveryFailed("failed to load RDB snapshot: " + err.Error())
	}

	if snapshot != nil {
		m.logger.Info(ctx, "RDB snapshot found, applying to database", map[string]interface{}{
			"component":      "persistence_recovery",
			"snapshot_time":  snapshot.Timestamp,
			"database_count": len(snapshot.Databases),
		})

		// Apply RDB snapshot to database engine if command applier is available
		if m.cmdApplier != nil {
			if err := m.cmdApplier.ApplySnapshot(ctx, snapshot); err != nil {
				m.logger.Error(ctx, "Failed to apply RDB snapshot to database engine", err, map[string]interface{}{
					"component": "persistence_recovery",
				})
				return utils.ErrRecoveryFailed("failed to apply RDB snapshot: " + err.Error())
			}
			m.logger.Info(ctx, "RDB snapshot successfully applied to database engine", map[string]interface{}{
				"component": "persistence_recovery",
			})
		} else {
			m.logger.Warn(ctx, "RDB snapshot loaded but cannot be applied: database engine not connected", map[string]interface{}{
				"component": "persistence_recovery",
			})
		}
	} else {
		m.logger.Info(ctx, "No RDB snapshot found, proceeding with AOF-only recovery", map[string]interface{}{
			"component": "persistence_recovery",
		})
	}

	// Step 2: Replay AOF commands
	m.logger.Debug(ctx, "Starting AOF replay", map[string]interface{}{
		"component": "persistence_recovery",
	})

	err = m.aofLogger.Replay(ctx, func(command types.AOFCommand) error {
		commandCount++

		// Log every 1000 commands to show progress
		if commandCount%1000 == 0 {
			m.logger.Debug(ctx, "AOF replay progress", map[string]interface{}{
				"component":         "persistence_recovery",
				"commands_replayed": commandCount,
			})
		}

		// Apply command to database engine if command applier is available
		if m.cmdApplier != nil {
			if err := m.cmdApplier.ApplyCommand(ctx, command); err != nil {
				m.logger.Error(ctx, "Failed to apply AOF command to database engine", err, map[string]interface{}{
					"component":   "persistence_recovery",
					"command":     command.Command,
					"database":    command.Database,
					"collection":  command.Collection,
					"command_num": commandCount,
				})
				return utils.ErrRecoveryFailed("failed to apply AOF command: " + err.Error())
			}
		} else {
			// AOF command read but not applied - this is the data loss scenario!
			m.logger.Warn(ctx, "AOF command read but not applied to database (engine not connected)", map[string]interface{}{
				"component":   "persistence_recovery",
				"command":     command.Command,
				"database":    command.Database,
				"collection":  command.Collection,
				"command_num": commandCount,
			})
		}
		return nil
	})

	if err != nil {
		m.logger.Error(ctx, "Failed to replay AOF commands", err, map[string]interface{}{
			"component":         "persistence_recovery",
			"commands_replayed": commandCount,
		})
		return utils.ErrRecoveryFailed("failed to replay AOF: " + err.Error())
	}

	// Update statistics
	m.stats.RecoveryTime = time.Since(startTime)
	m.stats.RecoveredCommands = commandCount

	// Log recovery completion
	if m.cmdApplier != nil {
		m.logger.Info(ctx, "Data recovery completed successfully", map[string]interface{}{
			"component":         "persistence_recovery",
			"recovery_time":     m.stats.RecoveryTime.String(),
			"commands_replayed": commandCount,
			"has_rdb_snapshot":  snapshot != nil,
		})
	} else {
		m.logger.Warn(ctx, "Data recovery completed but with data loss", map[string]interface{}{
			"component":        "persistence_recovery",
			"recovery_time":    m.stats.RecoveryTime.String(),
			"commands_read":    commandCount,
			"commands_applied": 0,
			"has_rdb_snapshot": snapshot != nil,
			"data_loss_reason": "database engine not connected to persistence manager",
		})
	}

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

	// Since RDB snapshot now contains all current data,
	// truncate the AOF file to avoid replay conflicts
	if err := m.aofLogger.Truncate(); err != nil {
		m.logger.Error(ctx, "Failed to truncate AOF after RDB save", err, map[string]interface{}{
			"component": "persistence_rdb_save",
		})
		return utils.ErrPersistenceFailedWithCause("failed to truncate AOF after RDB save", err)
	}

	m.stats.LastRDBSave = time.Now()
	m.logger.Info(ctx, "RDB snapshot saved and AOF truncated successfully", map[string]interface{}{
		"component":      "persistence_rdb_save",
		"database_count": len(databases),
	})
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
	m.mu.Lock()
	defer m.mu.Unlock()

	// Stop background tasks if not already stopped
	select {
	case <-m.stopTasks:
		// Already closed
	default:
		close(m.stopTasks)
		m.taskWG.Wait()
	}

	// Close persistence components
	if err := m.aofLogger.Close(); err != nil {
		return utils.ErrPersistenceFailedWithCause("failed to close AOF logger", err)
	}

	return nil
}

// SetDatabaseEngine sets the database engine for persistence operations
func (m *Manager) SetDatabaseEngine(engine DatabaseEngine) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if engine != nil {
		m.cmdApplier = NewCommandApplier(engine)
	} else {
		m.cmdApplier = nil
	}
}

// HasDatabaseEngine returns true if a database engine is configured
func (m *Manager) HasDatabaseEngine() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.cmdApplier != nil
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

// TruncateAOF clears the AOF file (removes all content)
func (m *Manager) TruncateAOF(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.aofLogger.Truncate(); err != nil {
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
			// Get current database state and create snapshot if command applier is available
			if m.cmdApplier != nil {
				databases, err := m.cmdApplier.GetDatabaseState(ctx)
				if err != nil {
					// Log error but continue running
					m.logger.Error(ctx, "failed to get database state for RDB snapshot", err, nil)
					continue
				}

				if err := m.SaveSnapshot(ctx, databases); err != nil {
					// Log error but continue running
					m.logger.Error(ctx, "failed to save RDB snapshot", err, nil)
				}
			}

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
				// Get optimized commands and rewrite AOF if command applier is available
				if m.cmdApplier != nil {
					commands, err := m.cmdApplier.GetOptimizedCommands(ctx)
					if err != nil {
						// Log error but continue running
						m.logger.Error(ctx, "failed to get optimized commands for AOF rewrite", err, nil)
						continue
					}

					if err := m.RewriteAOF(ctx, commands); err != nil {
						// Log error but continue running
						m.logger.Error(ctx, "failed to rewrite AOF", err, nil)
					}
				}
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
