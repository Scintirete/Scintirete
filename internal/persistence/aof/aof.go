// Package aof provides Append-Only File logging for Scintirete using FlatBuffers.
package aof

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	flatbuffers "github.com/google/flatbuffers/go"
	fbaof "github.com/scintirete/scintirete/internal/flatbuffers/aof"
	"github.com/scintirete/scintirete/internal/utils"
	"github.com/scintirete/scintirete/pkg/types"
)

// SyncStrategy defines how frequently AOF data is synced to disk
type SyncStrategy string

const (
	SyncAlways   SyncStrategy = "always"   // Sync after every write
	SyncEverySec SyncStrategy = "everysec" // Sync every second
	SyncNo       SyncStrategy = "no"       // Let OS decide when to sync
)

// AOFStats contains statistics about the AOF log
type AOFStats struct {
	CommandCount int64     `json:"command_count"`
	FileSize     int64     `json:"file_size"`
	LastSync     time.Time `json:"last_sync"`
	SyncStrategy string    `json:"sync_strategy"`
}

// AOFLogger handles append-only file logging using FlatBuffers with Length-Prefix format
type AOFLogger struct {
	mu           sync.Mutex
	file         *os.File
	writer       *bufio.Writer
	filePath     string
	syncStrategy SyncStrategy

	// Background sync for everysec strategy
	syncTicker *time.Ticker
	stopSync   chan struct{}
	syncWG     sync.WaitGroup

	// Statistics
	commandCount int64
	lastSync     time.Time
}

// NewAOFLogger creates a new FlatBuffers AOF logger with Length-Prefix format
func NewAOFLogger(filePath string, syncStrategy SyncStrategy) (*AOFLogger, error) {
	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, utils.ErrPersistenceFailedWithCause("failed to create AOF directory", err)
	}

	// Open file for append
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, utils.ErrPersistenceFailedWithCause("failed to open AOF file", err)
	}

	logger := &AOFLogger{
		file:         file,
		writer:       bufio.NewWriter(file),
		filePath:     filePath,
		syncStrategy: syncStrategy,
		stopSync:     make(chan struct{}),
		lastSync:     time.Now(),
	}

	// Start background sync if needed
	if syncStrategy == SyncEverySec {
		logger.startBackgroundSync()
	}

	return logger, nil
}

// WriteCommand writes a command to the AOF log using FlatBuffers + Length-Prefix format
func (a *AOFLogger) WriteCommand(ctx context.Context, command types.AOFCommand) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Set timestamp if not provided
	if command.Timestamp.IsZero() {
		command.Timestamp = time.Now()
	}

	// Convert command to FlatBuffers
	data, err := a.commandToFlatBuffers(command)
	if err != nil {
		return utils.ErrPersistenceFailedWithCause("failed to serialize AOF command", err)
	}

	// Write length prefix (4 bytes, little-endian)
	length := uint32(len(data))
	if err := binary.Write(a.writer, binary.LittleEndian, length); err != nil {
		return utils.ErrPersistenceFailedWithCause("failed to write AOF length prefix", err)
	}

	// Write FlatBuffers data
	if _, err := a.writer.Write(data); err != nil {
		return utils.ErrPersistenceFailedWithCause("failed to write AOF command data", err)
	}

	a.commandCount++

	// Sync based on strategy
	switch a.syncStrategy {
	case SyncAlways:
		if err := a.syncToFile(); err != nil {
			return err
		}
	case SyncEverySec:
		// Background sync handles this
	case SyncNo:
		// OS will sync when it wants
	}

	return nil
}

// Replay reads and replays all commands from the AOF file using Length-Prefix format
func (a *AOFLogger) Replay(ctx context.Context, handler func(types.AOFCommand) error) error {
	// Close current file handle for reading
	a.mu.Lock()
	if a.file != nil {
		a.syncToFile() // Ensure any buffered data is written
	}
	a.mu.Unlock()

	// Open file for reading
	file, err := os.Open(a.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No AOF file exists yet, that's OK
		}
		return utils.ErrRecoveryFailed("failed to open AOF file for replay: " + err.Error())
	}
	defer file.Close()

	commandNum := 0

	for {
		commandNum++

		// Read length prefix (4 bytes, little-endian) directly from file
		var length uint32
		if err := binary.Read(file, binary.LittleEndian, &length); err != nil {
			if errors.Is(err, io.EOF) {
				break // End of file
			}
			return utils.ErrCorruptedData(fmt.Sprintf("invalid length prefix at command %d: %v", commandNum, err))
		}

		// Validate length
		if length == 0 || length > 100*1024*1024 { // Max 100MB per command
			return utils.ErrCorruptedData(fmt.Sprintf("invalid command length %d at command %d", length, commandNum))
		}

		// Read FlatBuffers data directly from file
		data := make([]byte, length)
		if _, err := io.ReadFull(file, data); err != nil {
			return utils.ErrCorruptedData(fmt.Sprintf("failed to read command data at command %d: %v", commandNum, err))
		}

		// Parse FlatBuffers command
		command, err := a.flatBuffersToCommand(data)
		if err != nil {
			return utils.ErrCorruptedData(fmt.Sprintf("invalid FlatBuffers command at command %d: %v", commandNum, err))
		}

		// Execute command
		if err := handler(*command); err != nil {
			return utils.ErrRecoveryFailed(fmt.Sprintf("failed to replay AOF command at command %d: %v", commandNum, err))
		}

		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}

	return nil
}

// Rewrite creates a new AOF file with optimized commands using FlatBuffers + Length-Prefix
func (a *AOFLogger) Rewrite(ctx context.Context, snapshotCommands []types.AOFCommand) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Create temporary file
	tempPath := a.filePath + ".tmp"
	tempFile, err := os.Create(tempPath)
	if err != nil {
		return utils.ErrPersistenceFailedWithCause("failed to create temporary AOF file", err)
	}
	defer func() {
		tempFile.Close()
		os.Remove(tempPath) // Clean up on error
	}()

	writer := bufio.NewWriter(tempFile)

	// Write optimized commands
	for _, command := range snapshotCommands {
		// Convert command to FlatBuffers
		data, err := a.commandToFlatBuffers(command)
		if err != nil {
			return utils.ErrPersistenceFailedWithCause("failed to serialize rewrite command", err)
		}

		// Write length prefix
		length := uint32(len(data))
		if err := binary.Write(writer, binary.LittleEndian, length); err != nil {
			return utils.ErrPersistenceFailedWithCause("failed to write rewrite length prefix", err)
		}

		// Write FlatBuffers data
		if _, err := writer.Write(data); err != nil {
			return utils.ErrPersistenceFailedWithCause("failed to write rewrite command data", err)
		}

		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}

	// Flush and sync
	if err := writer.Flush(); err != nil {
		return utils.ErrPersistenceFailedWithCause("failed to flush rewrite buffer", err)
	}
	if err := tempFile.Sync(); err != nil {
		return utils.ErrPersistenceFailedWithCause("failed to sync rewrite file", err)
	}
	if err := tempFile.Close(); err != nil {
		return utils.ErrPersistenceFailedWithCause("failed to close rewrite file", err)
	}

	// Close current file
	if err := a.syncToFile(); err != nil {
		return err
	}
	if err := a.file.Close(); err != nil {
		return utils.ErrPersistenceFailedWithCause("failed to close current AOF file", err)
	}

	// Replace old file with new one
	if err := os.Rename(tempPath, a.filePath); err != nil {
		return utils.ErrPersistenceFailedWithCause("failed to replace AOF file", err)
	}

	// Reopen file for writing
	a.file, err = os.OpenFile(a.filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return utils.ErrPersistenceFailedWithCause("failed to reopen AOF file after rewrite", err)
	}
	a.writer = bufio.NewWriter(a.file)
	a.commandCount = int64(len(snapshotCommands))

	return nil
}

// commandToFlatBuffers converts a types.AOFCommand to FlatBuffers data
func (a *AOFLogger) commandToFlatBuffers(command types.AOFCommand) ([]byte, error) {
	builder := flatbuffers.NewBuilder(0)

	// Create command args based on command type
	var argsOffset flatbuffers.UOffsetT
	var commandType fbaof.CommandType
	var err error

	switch command.Command {
	case "CREATE_DATABASE":
		commandType = fbaof.CommandTypeCREATE_DATABASE
		argsOffset, err = a.createDatabaseArgs(builder, command.Args)
	case "DROP_DATABASE":
		commandType = fbaof.CommandTypeDROP_DATABASE
		argsOffset, err = a.dropDatabaseArgs(builder, command.Args)
	case "CREATE_COLLECTION":
		commandType = fbaof.CommandTypeCREATE_COLLECTION
		argsOffset, err = a.createCollectionArgs(builder, command.Args)
	case "DROP_COLLECTION":
		commandType = fbaof.CommandTypeDROP_COLLECTION
		argsOffset, err = a.dropCollectionArgs(builder, command.Args)
	case "INSERT_VECTORS":
		commandType = fbaof.CommandTypeINSERT_VECTORS
		argsOffset, err = a.insertVectorsArgs(builder, command.Args)
	case "DELETE_VECTORS":
		commandType = fbaof.CommandTypeDELETE_VECTORS
		argsOffset, err = a.deleteVectorsArgs(builder, command.Args)
	default:
		return nil, fmt.Errorf("unsupported command type: %s", command.Command)
	}

	if err != nil {
		return nil, err
	}

	// Create strings
	databaseStr := builder.CreateString(command.Database)
	collectionStr := builder.CreateString(command.Collection)

	// Create AOF command
	fbaof.AOFCommandStart(builder)
	fbaof.AOFCommandAddTimestamp(builder, command.Timestamp.Unix())
	fbaof.AOFCommandAddCommandType(builder, commandType)
	fbaof.AOFCommandAddArgsType(builder, a.getArgsType(command.Command))
	fbaof.AOFCommandAddArgs(builder, argsOffset)
	fbaof.AOFCommandAddDatabase(builder, databaseStr)
	fbaof.AOFCommandAddCollection(builder, collectionStr)
	aofCommand := fbaof.AOFCommandEnd(builder)

	// Finish the FlatBuffer
	builder.Finish(aofCommand)

	return builder.FinishedBytes(), nil
}

// flatBuffersToCommand converts FlatBuffers data to types.AOFCommand
func (a *AOFLogger) flatBuffersToCommand(data []byte) (*types.AOFCommand, error) {
	fbCommand := fbaof.GetRootAsAOFCommand(data, 0)

	command := &types.AOFCommand{
		Timestamp:  time.Unix(fbCommand.Timestamp(), 0),
		Database:   string(fbCommand.Database()),
		Collection: string(fbCommand.Collection()),
		Args:       make(map[string]interface{}),
	}

	// Convert command type
	switch fbCommand.CommandType() {
	case fbaof.CommandTypeCREATE_DATABASE:
		command.Command = "CREATE_DATABASE"
	case fbaof.CommandTypeDROP_DATABASE:
		command.Command = "DROP_DATABASE"
	case fbaof.CommandTypeCREATE_COLLECTION:
		command.Command = "CREATE_COLLECTION"
	case fbaof.CommandTypeDROP_COLLECTION:
		command.Command = "DROP_COLLECTION"
	case fbaof.CommandTypeINSERT_VECTORS:
		command.Command = "INSERT_VECTORS"
	case fbaof.CommandTypeDELETE_VECTORS:
		command.Command = "DELETE_VECTORS"
	default:
		return nil, fmt.Errorf("unknown command type: %d", fbCommand.CommandType())
	}

	// Parse command args
	argsTable := new(flatbuffers.Table)
	if fbCommand.Args(argsTable) {
		if err := a.parseCommandArgs(command, argsTable); err != nil {
			return nil, err
		}
	}

	return command, nil
}

// Helper methods for creating FlatBuffers command arguments
func (a *AOFLogger) createDatabaseArgs(builder *flatbuffers.Builder, args map[string]interface{}) (flatbuffers.UOffsetT, error) {
	name, ok := args["name"].(string)
	if !ok {
		return 0, fmt.Errorf("missing or invalid database name")
	}

	nameStr := builder.CreateString(name)
	fbaof.CreateDatabaseArgsStart(builder)
	fbaof.CreateDatabaseArgsAddName(builder, nameStr)
	return fbaof.CreateDatabaseArgsEnd(builder), nil
}

func (a *AOFLogger) dropDatabaseArgs(builder *flatbuffers.Builder, args map[string]interface{}) (flatbuffers.UOffsetT, error) {
	name, ok := args["name"].(string)
	if !ok {
		return 0, fmt.Errorf("missing or invalid database name")
	}

	nameStr := builder.CreateString(name)
	fbaof.DropDatabaseArgsStart(builder)
	fbaof.DropDatabaseArgsAddName(builder, nameStr)
	return fbaof.DropDatabaseArgsEnd(builder), nil
}

func (a *AOFLogger) createCollectionArgs(builder *flatbuffers.Builder, args map[string]interface{}) (flatbuffers.UOffsetT, error) {
	name, ok := args["name"].(string)
	if !ok {
		return 0, fmt.Errorf("missing or invalid collection name")
	}

	configInterface, ok := args["config"]
	if !ok {
		return 0, fmt.Errorf("missing collection config")
	}

	config, ok := configInterface.(types.CollectionConfig)
	if !ok {
		return 0, fmt.Errorf("invalid collection config type")
	}

	// Create collection config
	configOffset, err := a.createCollectionConfig(builder, config)
	if err != nil {
		return 0, err
	}

	nameStr := builder.CreateString(name)
	fbaof.CreateCollectionArgsStart(builder)
	fbaof.CreateCollectionArgsAddName(builder, nameStr)
	fbaof.CreateCollectionArgsAddConfig(builder, configOffset)
	return fbaof.CreateCollectionArgsEnd(builder), nil
}

func (a *AOFLogger) dropCollectionArgs(builder *flatbuffers.Builder, args map[string]interface{}) (flatbuffers.UOffsetT, error) {
	name, ok := args["name"].(string)
	if !ok {
		return 0, fmt.Errorf("missing or invalid collection name")
	}

	nameStr := builder.CreateString(name)
	fbaof.DropCollectionArgsStart(builder)
	fbaof.DropCollectionArgsAddName(builder, nameStr)
	return fbaof.DropCollectionArgsEnd(builder), nil
}

func (a *AOFLogger) insertVectorsArgs(builder *flatbuffers.Builder, args map[string]interface{}) (flatbuffers.UOffsetT, error) {
	vectorsInterface, ok := args["vectors"]
	if !ok {
		return 0, fmt.Errorf("missing vectors")
	}

	vectors, ok := vectorsInterface.([]types.Vector)
	if !ok {
		return 0, fmt.Errorf("invalid vectors type")
	}

	// Create vectors vector
	var vectorOffsets []flatbuffers.UOffsetT
	for _, vector := range vectors {
		vectorOffset, err := a.createVector(builder, vector)
		if err != nil {
			return 0, err
		}
		vectorOffsets = append(vectorOffsets, vectorOffset)
	}

	fbaof.InsertVectorsArgsStartVectorsVector(builder, len(vectorOffsets))
	for i := len(vectorOffsets) - 1; i >= 0; i-- {
		builder.PrependUOffsetT(vectorOffsets[i])
	}
	vectorsVector := builder.EndVector(len(vectorOffsets))

	fbaof.InsertVectorsArgsStart(builder)
	fbaof.InsertVectorsArgsAddVectors(builder, vectorsVector)
	return fbaof.InsertVectorsArgsEnd(builder), nil
}

func (a *AOFLogger) deleteVectorsArgs(builder *flatbuffers.Builder, args map[string]interface{}) (flatbuffers.UOffsetT, error) {
	idsInterface, ok := args["ids"]
	if !ok {
		return 0, fmt.Errorf("missing ids")
	}

	ids, ok := idsInterface.([]string)
	if !ok {
		return 0, fmt.Errorf("invalid ids type")
	}

	// Create ids vector
	var idOffsets []flatbuffers.UOffsetT
	for _, id := range ids {
		idStr := builder.CreateString(id)
		idOffsets = append(idOffsets, idStr)
	}

	fbaof.DeleteVectorsArgsStartIdsVector(builder, len(idOffsets))
	for i := len(idOffsets) - 1; i >= 0; i-- {
		builder.PrependUOffsetT(idOffsets[i])
	}
	idsVector := builder.EndVector(len(idOffsets))

	fbaof.DeleteVectorsArgsStart(builder)
	fbaof.DeleteVectorsArgsAddIds(builder, idsVector)
	return fbaof.DeleteVectorsArgsEnd(builder), nil
}

// Helper methods for creating complex types
func (a *AOFLogger) createVector(builder *flatbuffers.Builder, vector types.Vector) (flatbuffers.UOffsetT, error) {
	// Create elements vector
	fbaof.VectorStartElementsVector(builder, len(vector.Elements))
	for i := len(vector.Elements) - 1; i >= 0; i-- {
		builder.PrependFloat32(vector.Elements[i])
	}
	elementsVector := builder.EndVector(len(vector.Elements))

	// Convert metadata to JSON (simplified for now)
	metadataStr := builder.CreateString("{}")
	if vector.Metadata != nil {
		// In a real implementation, you might want to serialize metadata properly
		metadataStr = builder.CreateString("{}")
	}

	idStr := builder.CreateString(fmt.Sprintf("%d", vector.ID))

	fbaof.VectorStart(builder)
	fbaof.VectorAddId(builder, idStr)
	fbaof.VectorAddElements(builder, elementsVector)
	fbaof.VectorAddMetadata(builder, metadataStr)
	return fbaof.VectorEnd(builder), nil
}

func (a *AOFLogger) createCollectionConfig(builder *flatbuffers.Builder, config types.CollectionConfig) (flatbuffers.UOffsetT, error) {
	// Create HNSW params
	hnswOffset, err := a.createHNSWParams(builder, config.HNSWParams)
	if err != nil {
		return 0, err
	}

	nameStr := builder.CreateString(config.Name)

	fbaof.CollectionConfigStart(builder)
	fbaof.CollectionConfigAddName(builder, nameStr)
	fbaof.CollectionConfigAddMetric(builder, fbaof.DistanceMetric(config.Metric))
	fbaof.CollectionConfigAddHnswParams(builder, hnswOffset)
	return fbaof.CollectionConfigEnd(builder), nil
}

func (a *AOFLogger) createHNSWParams(builder *flatbuffers.Builder, params types.HNSWParams) (flatbuffers.UOffsetT, error) {
	fbaof.HNSWParamsStart(builder)
	fbaof.HNSWParamsAddM(builder, int32(params.M))
	fbaof.HNSWParamsAddEfConstruction(builder, int32(params.EfConstruction))
	fbaof.HNSWParamsAddEfSearch(builder, int32(params.EfSearch))
	fbaof.HNSWParamsAddMaxLayers(builder, int32(params.MaxLayers))
	fbaof.HNSWParamsAddSeed(builder, params.Seed)
	return fbaof.HNSWParamsEnd(builder), nil
}

// getArgsType returns the appropriate CommandArgs type for the command
func (a *AOFLogger) getArgsType(command string) fbaof.CommandArgs {
	switch command {
	case "CREATE_DATABASE":
		return fbaof.CommandArgsCreateDatabaseArgs
	case "DROP_DATABASE":
		return fbaof.CommandArgsDropDatabaseArgs
	case "CREATE_COLLECTION":
		return fbaof.CommandArgsCreateCollectionArgs
	case "DROP_COLLECTION":
		return fbaof.CommandArgsDropCollectionArgs
	case "INSERT_VECTORS":
		return fbaof.CommandArgsInsertVectorsArgs
	case "DELETE_VECTORS":
		return fbaof.CommandArgsDeleteVectorsArgs
	default:
		return fbaof.CommandArgsNONE
	}
}

// parseCommandArgs parses command arguments from FlatBuffers table
func (a *AOFLogger) parseCommandArgs(command *types.AOFCommand, argsTable *flatbuffers.Table) error {
	command.Args = make(map[string]interface{})

	// Parse arguments based on command type
	switch command.Command {
	case "CREATE_DATABASE":
		args := &fbaof.CreateDatabaseArgs{}
		args.Init(argsTable.Bytes, argsTable.Pos)
		command.Args["name"] = string(args.Name())

	case "DROP_DATABASE":
		args := &fbaof.DropDatabaseArgs{}
		args.Init(argsTable.Bytes, argsTable.Pos)
		command.Args["name"] = string(args.Name())

	case "CREATE_COLLECTION":
		args := &fbaof.CreateCollectionArgs{}
		args.Init(argsTable.Bytes, argsTable.Pos)
		command.Args["name"] = string(args.Name())

		// Parse collection config
		config := args.Config(nil)
		if config != nil {
			hnswParams := config.HnswParams(nil)
			if hnswParams != nil {
				collectionConfig := types.CollectionConfig{
					Name:   string(config.Name()),
					Metric: types.DistanceMetric(config.Metric()),
					HNSWParams: types.HNSWParams{
						M:              int(hnswParams.M()),
						EfConstruction: int(hnswParams.EfConstruction()),
						EfSearch:       int(hnswParams.EfSearch()),
						MaxLayers:      int(hnswParams.MaxLayers()),
						Seed:           hnswParams.Seed(),
					},
				}
				command.Args["config"] = collectionConfig
			}
		}

	case "DROP_COLLECTION":
		args := &fbaof.DropCollectionArgs{}
		args.Init(argsTable.Bytes, argsTable.Pos)
		command.Args["name"] = string(args.Name())

	case "INSERT_VECTORS":
		args := &fbaof.InsertVectorsArgs{}
		args.Init(argsTable.Bytes, argsTable.Pos)

		vectors := make([]types.Vector, args.VectorsLength())
		for i := 0; i < args.VectorsLength(); i++ {
			vector := &fbaof.Vector{}
			if args.Vectors(vector, i) {
				elements := make([]float32, vector.ElementsLength())
				for j := 0; j < vector.ElementsLength(); j++ {
					elements[j] = vector.Elements(j)
				}

				// Convert string ID to uint64
				var vectorID uint64
				if _, err := fmt.Sscanf(string(vector.Id()), "%d", &vectorID); err != nil {
					// Handle error - skip this vector or log error
					continue
				}

				vectors[i] = types.Vector{
					ID:       vectorID,
					Elements: elements,
					// Note: Metadata parsing could be enhanced for JSON
					Metadata: nil,
				}
			}
		}
		command.Args["vectors"] = vectors

	case "DELETE_VECTORS":
		args := &fbaof.DeleteVectorsArgs{}
		args.Init(argsTable.Bytes, argsTable.Pos)

		ids := make([]string, args.IdsLength())
		for i := 0; i < args.IdsLength(); i++ {
			ids[i] = string(args.Ids(i))
		}
		command.Args["ids"] = ids

	default:
		return fmt.Errorf("unknown command type for argument parsing: %s", command.Command)
	}

	return nil
}

// Truncate removes all content from the AOF file
func (a *AOFLogger) Truncate() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Close current file
	if err := a.file.Close(); err != nil {
		return utils.ErrPersistenceFailedWithCause("failed to close AOF file for truncation", err)
	}

	// Recreate empty file
	file, err := os.Create(a.filePath)
	if err != nil {
		return utils.ErrPersistenceFailedWithCause("failed to recreate AOF file", err)
	}

	a.file = file
	a.writer = bufio.NewWriter(file)
	a.commandCount = 0

	return nil
}

// Close closes the AOF logger and stops background sync
func (a *AOFLogger) Close() error {
	// Stop background sync
	if a.syncTicker != nil {
		close(a.stopSync)
		a.syncWG.Wait()
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	// Final sync and close
	if err := a.syncToFile(); err != nil {
		return err
	}

	return a.file.Close()
}

// GetStats returns AOF statistics
func (a *AOFLogger) GetStats() AOFStats {
	a.mu.Lock()
	defer a.mu.Unlock()

	fileInfo, _ := a.file.Stat()
	fileSize := int64(0)
	if fileInfo != nil {
		fileSize = fileInfo.Size()
	}

	return AOFStats{
		CommandCount: a.commandCount,
		FileSize:     fileSize,
		LastSync:     a.lastSync,
		SyncStrategy: string(a.syncStrategy),
	}
}

// Private methods

// syncToFile flushes the buffer and syncs to disk
func (a *AOFLogger) syncToFile() error {
	if err := a.writer.Flush(); err != nil {
		return utils.ErrPersistenceFailedWithCause("failed to flush AOF buffer", err)
	}
	if err := a.file.Sync(); err != nil {
		return utils.ErrPersistenceFailedWithCause("failed to sync AOF file", err)
	}
	a.lastSync = time.Now()
	return nil
}

// startBackgroundSync starts the background sync goroutine for everysec strategy
func (a *AOFLogger) startBackgroundSync() {
	a.syncTicker = time.NewTicker(time.Second)
	a.syncWG.Add(1)

	go func() {
		defer a.syncWG.Done()
		defer a.syncTicker.Stop()

		for {
			select {
			case <-a.syncTicker.C:
				a.mu.Lock()
				// 智能同步：只在有缓冲数据且距离上次同步超过1秒时才同步
				bufferedBytes := a.writer.Buffered()
				timeSinceLastSync := time.Since(a.lastSync)

				// 只有在以下情况才进行同步：
				// 1. 有缓冲的数据
				// 2. 距离上次同步已经超过5秒
				// 3. 缓冲数据超过4KB（避免频繁同步小量数据）
				if bufferedBytes > 0 && timeSinceLastSync >= time.Second &&
					(bufferedBytes >= 4096 || timeSinceLastSync >= 5*time.Second) {
					a.syncToFile() // Ignore errors in background sync
				}
				a.mu.Unlock()
			case <-a.stopSync:
				return
			}
		}
	}()
}

// CommandBuilder helps build AOF commands for different operations
type CommandBuilder struct{}

// NewCommandBuilder creates a new command builder
func NewCommandBuilder() *CommandBuilder {
	return &CommandBuilder{}
}

// CreateDatabase builds a command for database creation
func (cb *CommandBuilder) CreateDatabase(dbName string) types.AOFCommand {
	return types.AOFCommand{
		Timestamp: time.Now(),
		Command:   "CREATE_DATABASE",
		Args: map[string]interface{}{
			"name": dbName,
		},
		Database: dbName,
	}
}

// DropDatabase builds a command for database deletion
func (cb *CommandBuilder) DropDatabase(dbName string) types.AOFCommand {
	return types.AOFCommand{
		Timestamp: time.Now(),
		Command:   "DROP_DATABASE",
		Args: map[string]interface{}{
			"name": dbName,
		},
		Database: dbName,
	}
}

// CreateCollection builds a command for collection creation
func (cb *CommandBuilder) CreateCollection(dbName, collName string, config types.CollectionConfig) types.AOFCommand {
	return types.AOFCommand{
		Timestamp: time.Now(),
		Command:   "CREATE_COLLECTION",
		Args: map[string]interface{}{
			"name":   collName,
			"config": config,
		},
		Database:   dbName,
		Collection: collName,
	}
}

// DropCollection builds a command for collection deletion
func (cb *CommandBuilder) DropCollection(dbName, collName string) types.AOFCommand {
	return types.AOFCommand{
		Timestamp: time.Now(),
		Command:   "DROP_COLLECTION",
		Args: map[string]interface{}{
			"name": collName,
		},
		Database:   dbName,
		Collection: collName,
	}
}

// InsertVectors builds a command for vector insertion
func (cb *CommandBuilder) InsertVectors(dbName, collName string, vectors []types.Vector) types.AOFCommand {
	return types.AOFCommand{
		Timestamp: time.Now(),
		Command:   "INSERT_VECTORS",
		Args: map[string]interface{}{
			"vectors": vectors,
		},
		Database:   dbName,
		Collection: collName,
	}
}

// DeleteVectors builds a command for vector deletion
func (cb *CommandBuilder) DeleteVectors(dbName, collName string, ids []string) types.AOFCommand {
	return types.AOFCommand{
		Timestamp: time.Now(),
		Command:   "DELETE_VECTORS",
		Args: map[string]interface{}{
			"ids": ids,
		},
		Database:   dbName,
		Collection: collName,
	}
}
