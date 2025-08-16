// Package config provides configuration management for Scintirete.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/scintirete/scintirete/internal/core"
	"github.com/scintirete/scintirete/internal/embedding"
	"github.com/scintirete/scintirete/internal/persistence"
)

// Config represents the complete Scintirete configuration.
type Config struct {
	Server        ServerConfig        `toml:"server"`
	Log           LogConfig           `toml:"log"`
	Persistence   PersistenceConfig   `toml:"persistence"`
	Embedding     EmbeddingConfig     `toml:"embedding"`
	Observability ObservabilityConfig `toml:"observability"`
	Algorithm     AlgorithmConfig     `toml:"algorithm"`
	Monitoring    MonitoringConfig    `toml:"monitoring"`
}

// ServerConfig contains network and authentication settings.
type ServerConfig struct {
	GRPCHost  string   `toml:"grpc_host"`
	GRPCPort  int      `toml:"grpc_port"`
	HTTPHost  string   `toml:"http_host"`
	HTTPPort  int      `toml:"http_port"`
	Passwords []string `toml:"passwords"`
}

// LogConfig contains logging settings.
type LogConfig struct {
	Level          string `toml:"level"`
	Format         string `toml:"format"`
	EnableAuditLog bool   `toml:"enable_audit_log"`
}

// PersistenceConfig contains persistence-related configuration
type PersistenceConfig struct {
	DataDir            string `toml:"data_dir"`             // Directory to store data files
	RDBFilename        string `toml:"rdb_filename"`         // RDB snapshot filename
	AOFFilename        string `toml:"aof_filename"`         // AOF log filename
	AOFSyncStrategy    string `toml:"aof_sync_strategy"`    // AOF sync strategy: always, everysec, no
	RDBIntervalMinutes int    `toml:"rdb_interval_minutes"` // How often to create RDB snapshots (in minutes)
	AOFRewriteSizeMB   int    `toml:"aof_rewrite_size_mb"`  // Rewrite AOF when it exceeds this size (in MB)
}

// EmbeddingConfig contains external embedding service settings.
type EmbeddingConfig struct {
	BaseURL      string           `toml:"base_url"`
	APIKey       string           `toml:"api_key"`
	RPMLimit     int              `toml:"rpm_limit"`
	TPMLimit     int              `toml:"tpm_limit"`
	Models       []EmbeddingModel `toml:"models"`
	DefaultModel string           `toml:"default_model"`
}

// EmbeddingModel represents an embedding model configuration
type EmbeddingModel struct {
	ID          string `toml:"id"`
	Name        string `toml:"name"`
	Dimension   int    `toml:"dimension"`
	Available   bool   `toml:"available"`
	Description string `toml:"description"`
}

// ObservabilityConfig contains metrics and monitoring settings.
type ObservabilityConfig struct {
	MetricsEnabled bool   `toml:"metrics_enabled"`
	MetricsPath    string `toml:"metrics_path"`
	MetricsPort    int    `toml:"metrics_port"`
}

// AlgorithmConfig contains algorithm-specific settings.
type AlgorithmConfig struct {
	HNSWDefaults HNSWDefaultsConfig `toml:"hnsw_defaults"`
}

// HNSWDefaultsConfig contains default HNSW parameters.
type HNSWDefaultsConfig struct {
	M              int `toml:"m"`
	EfConstruction int `toml:"ef_construction"`
	EfSearch       int `toml:"ef_search"`
}

// MonitoringConfig contains system monitoring settings.
type MonitoringConfig struct {
	Enabled         bool    `toml:"enabled"`          // 是否启用系统监控
	Interval        int     `toml:"interval"`         // 监控间隔（秒）
	CPUEnabled      bool    `toml:"cpu_enabled"`      // 是否启用CPU监控
	CPUThreshold    float64 `toml:"cpu_threshold"`    // CPU使用率阈值（0.0-1.0）
	MemoryEnabled   bool    `toml:"memory_enabled"`   // 是否启用内存监控
	MemoryThreshold int     `toml:"memory_threshold"` // 内存使用阈值（MB）
	DiskEnabled     bool    `toml:"disk_enabled"`     // 是否启用磁盘监控
	DiskThreshold   int     `toml:"disk_threshold"`   // 磁盘使用阈值（MB）
}

// DefaultConfig returns a configuration with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			GRPCHost:  "127.0.0.1",
			GRPCPort:  9090,
			HTTPHost:  "127.0.0.1",
			HTTPPort:  8080,
			Passwords: []string{"default-password"},
		},
		Log: LogConfig{
			Level:          "info",
			Format:         "json",
			EnableAuditLog: true,
		},
		Persistence: PersistenceConfig{
			DataDir:            "./data",
			RDBFilename:        "dump.rdb",
			AOFFilename:        "appendonly.aof",
			AOFSyncStrategy:    "everysec",
			RDBIntervalMinutes: 0,  // 0 minutes, consistent with persistence.DefaultConfig
			AOFRewriteSizeMB:   64, // 64MB, consistent with persistence.DefaultConfig
		},
		Embedding: EmbeddingConfig{
			BaseURL:  "https://api.openai.com/v1/embeddings",
			APIKey:   "",
			RPMLimit: 3500,
			TPMLimit: 90000,
			Models: []EmbeddingModel{
				{ID: "text-embedding-3-small", Name: "text-embedding-3-small", Dimension: 1536, Available: true, Description: "Small text embedding model"},
				{ID: "text-embedding-3-base", Name: "text-embedding-3-base", Dimension: 3072, Available: true, Description: "Base text embedding model"},
				{ID: "text-embedding-3-large", Name: "text-embedding-3-large", Dimension: 4096, Available: true, Description: "Large text embedding model"},
			},
			DefaultModel: "text-embedding-3-small",
		},
		Observability: ObservabilityConfig{
			MetricsEnabled: true,
			MetricsPath:    "/metrics",
			MetricsPort:    9100,
		},
		Algorithm: AlgorithmConfig{
			HNSWDefaults: HNSWDefaultsConfig{
				M:              16,
				EfConstruction: 200,
				EfSearch:       50,
			},
		},
		Monitoring: MonitoringConfig{
			Enabled:         false, // 默认关闭监控
			Interval:        30,    // 30秒间隔
			CPUEnabled:      true,  // 启用时默认监控CPU
			CPUThreshold:    0.8,   // 80%阈值
			MemoryEnabled:   true,  // 启用时默认监控内存
			MemoryThreshold: 1024,  // 1GB阈值
			DiskEnabled:     false, // 默认不监控磁盘
			DiskThreshold:   10240, // 10GB阈值
		},
	}
}

// LoadConfig loads configuration from a TOML file.
func LoadConfig(filePath string) (*Config, error) {
	config := DefaultConfig()

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found: %s", filePath)
	}

	// Decode TOML file
	if _, err := toml.DecodeFile(filePath, config); err != nil {
		return nil, fmt.Errorf("failed to decode config file: %w", err)
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Resolve relative paths
	if err := config.resolvePaths(filePath); err != nil {
		return nil, fmt.Errorf("failed to resolve paths: %w", err)
	}

	return config, nil
}

// LoadConfigFromString loads configuration from a TOML string.
func LoadConfigFromString(tomlData string) (*Config, error) {
	config := DefaultConfig()

	if _, err := toml.Decode(tomlData, config); err != nil {
		return nil, fmt.Errorf("failed to decode config: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	// Validate server config
	if c.Server.GRPCPort <= 0 || c.Server.GRPCPort > 65535 {
		return fmt.Errorf("invalid gRPC port: %d", c.Server.GRPCPort)
	}
	if c.Server.HTTPPort <= 0 || c.Server.HTTPPort > 65535 {
		return fmt.Errorf("invalid HTTP port: %d", c.Server.HTTPPort)
	}
	if len(c.Server.Passwords) == 0 {
		return fmt.Errorf("at least one password must be configured")
	}

	// Validate log config
	validLogLevels := map[string]bool{
		"debug": true, "info": true, "warn": true, "error": true,
	}
	if !validLogLevels[c.Log.Level] {
		return fmt.Errorf("invalid log level: %s", c.Log.Level)
	}

	validLogFormats := map[string]bool{
		"text": true, "json": true,
	}
	if !validLogFormats[c.Log.Format] {
		return fmt.Errorf("invalid log format: %s", c.Log.Format)
	}

	// Validate persistence config
	if c.Persistence.DataDir == "" {
		return fmt.Errorf("data directory cannot be empty")
	}
	if c.Persistence.RDBFilename == "" {
		return fmt.Errorf("RDB filename cannot be empty")
	}
	if c.Persistence.AOFFilename == "" {
		return fmt.Errorf("AOF filename cannot be empty")
	}

	validAOFStrategies := map[string]bool{
		"always": true, "everysec": true, "no": true,
	}
	if !validAOFStrategies[c.Persistence.AOFSyncStrategy] {
		return fmt.Errorf("invalid AOF sync strategy: %s", c.Persistence.AOFSyncStrategy)
	}

	if c.Persistence.RDBIntervalMinutes < 0 {
		return fmt.Errorf("RDB interval minutes must be non-negative: %d", c.Persistence.RDBIntervalMinutes)
	}
	if c.Persistence.AOFRewriteSizeMB <= 0 {
		return fmt.Errorf("AOF rewrite size must be positive: %d", c.Persistence.AOFRewriteSizeMB)
	}

	// Validate embedding config
	if c.Embedding.BaseURL == "" {
		return fmt.Errorf("embedding base URL cannot be empty")
	}
	if c.Embedding.RPMLimit <= 0 {
		return fmt.Errorf("RPM limit must be positive: %d", c.Embedding.RPMLimit)
	}
	if c.Embedding.TPMLimit <= 0 {
		return fmt.Errorf("TPM limit must be positive: %d", c.Embedding.TPMLimit)
	}

	// Validate observability config
	if c.Observability.MetricsEnabled {
		if c.Observability.MetricsPort <= 0 || c.Observability.MetricsPort > 65535 {
			return fmt.Errorf("invalid metrics port: %d", c.Observability.MetricsPort)
		}
		if c.Observability.MetricsPath == "" {
			return fmt.Errorf("metrics path cannot be empty when metrics are enabled")
		}
	}

	// Validate algorithm config
	if c.Algorithm.HNSWDefaults.M <= 0 {
		return fmt.Errorf("HNSW M parameter must be positive: %d", c.Algorithm.HNSWDefaults.M)
	}
	if c.Algorithm.HNSWDefaults.EfConstruction <= 0 {
		return fmt.Errorf("HNSW ef_construction must be positive: %d", c.Algorithm.HNSWDefaults.EfConstruction)
	}
	if c.Algorithm.HNSWDefaults.EfSearch <= 0 {
		return fmt.Errorf("HNSW ef_search must be positive: %d", c.Algorithm.HNSWDefaults.EfSearch)
	}

	// Validate monitoring config
	if c.Monitoring.Enabled {
		if c.Monitoring.Interval <= 0 {
			return fmt.Errorf("monitoring interval must be positive: %d", c.Monitoring.Interval)
		}
		if c.Monitoring.CPUThreshold < 0 || c.Monitoring.CPUThreshold > 1 {
			return fmt.Errorf("CPU threshold must be between 0.0 and 1.0: %f", c.Monitoring.CPUThreshold)
		}
		if c.Monitoring.MemoryThreshold <= 0 {
			return fmt.Errorf("memory threshold must be positive: %d", c.Monitoring.MemoryThreshold)
		}
		if c.Monitoring.DiskThreshold <= 0 {
			return fmt.Errorf("disk threshold must be positive: %d", c.Monitoring.DiskThreshold)
		}
	}

	return nil
}

// resolvePaths converts relative paths to absolute paths based on project root file location.
func (c *Config) resolvePaths(configFilePath string) error {
	configDir := filepath.Dir(configFilePath)
	rootDir := configDir + "/.."

	// Resolve data directory path
	if !filepath.IsAbs(c.Persistence.DataDir) {
		c.Persistence.DataDir = filepath.Join(rootDir, c.Persistence.DataDir)
	}

	return nil
}

// GetEmbeddingAPIKey retrieves the embedding API key.
func (c *Config) GetEmbeddingAPIKey() string {
	return c.Embedding.APIKey
}

// GetRDBPath returns the full path to the RDB file.
func (c *Config) GetRDBPath() string {
	return filepath.Join(c.Persistence.DataDir, c.Persistence.RDBFilename)
}

// GetAOFPath returns the full path to the AOF file.
func (c *Config) GetAOFPath() string {
	return filepath.Join(c.Persistence.DataDir, c.Persistence.AOFFilename)
}

// GetGRPCAddress returns the gRPC server address.
func (c *Config) GetGRPCAddress() string {
	return fmt.Sprintf("%s:%d", c.Server.GRPCHost, c.Server.GRPCPort)
}

// GetHTTPAddress returns the HTTP server address.
func (c *Config) GetHTTPAddress() string {
	return fmt.Sprintf("%s:%d", c.Server.HTTPHost, c.Server.HTTPPort)
}

// GetMetricsAddress returns the metrics server address.
func (c *Config) GetMetricsAddress() string {
	return fmt.Sprintf(":%d", c.Observability.MetricsPort)
}

// SaveConfig saves the current configuration to a TOML file.
func (c *Config) SaveConfig(filePath string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Create or truncate the file
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	defer file.Close()

	// Encode to TOML
	encoder := toml.NewEncoder(file)
	if err := encoder.Encode(c); err != nil {
		return fmt.Errorf("failed to encode config to TOML: %w", err)
	}

	return nil
}

// Clone creates a deep copy of the configuration.
func (c *Config) Clone() *Config {
	clone := *c

	// Deep copy slices
	clone.Server.Passwords = make([]string, len(c.Server.Passwords))
	copy(clone.Server.Passwords, c.Server.Passwords)

	return &clone
}

// ToPersistenceConfig converts internal config to persistence.Config
func (c *Config) ToPersistenceConfig(logger core.Logger) persistence.Config {
	return persistence.Config{
		DataDir:         c.Persistence.DataDir,
		RDBFilename:     c.Persistence.RDBFilename,
		AOFFilename:     c.Persistence.AOFFilename,
		AOFSyncStrategy: c.Persistence.AOFSyncStrategy,
		RDBInterval:     time.Duration(c.Persistence.RDBIntervalMinutes) * time.Minute,
		AOFRewriteSize:  int64(c.Persistence.AOFRewriteSizeMB) * 1024 * 1024, // Convert MB to bytes
		Logger:          logger,
	}
}

// ToEmbeddingConfig converts config.EmbeddingConfig to embedding.Config
func (c *Config) ToEmbeddingConfig() embedding.Config {
	embeddingModels := make([]embedding.EmbeddingModel, len(c.Embedding.Models))
	for i, model := range c.Embedding.Models {
		embeddingModels[i] = embedding.EmbeddingModel{
			ID:          model.ID,
			Name:        model.Name,
			Dimension:   model.Dimension,
			Available:   model.Available,
			Description: model.Description,
		}
	}

	return embedding.Config{
		BaseURL:      c.Embedding.BaseURL,
		APIKey:       c.Embedding.APIKey,
		RPMLimit:     c.Embedding.RPMLimit,
		TPMLimit:     c.Embedding.TPMLimit,
		Timeout:      30 * time.Second, // Default timeout
		Models:       embeddingModels,
		DefaultModel: c.Embedding.DefaultModel,
	}
}

// RuntimeMonitoringConfig is the monitoring configuration used by the monitoring package
type RuntimeMonitoringConfig struct {
	Enabled         bool
	Interval        time.Duration
	CPUEnabled      bool
	CPUThreshold    float64
	MemoryEnabled   bool
	MemoryThreshold uint64 // in bytes
	DiskEnabled     bool
	DiskThreshold   uint64 // in bytes
}

// ToMonitoringConfig converts to monitoring config with proper type conversions
func (c *Config) ToMonitoringConfig() RuntimeMonitoringConfig {
	return RuntimeMonitoringConfig{
		Enabled:         c.Monitoring.Enabled,
		Interval:        time.Duration(c.Monitoring.Interval) * time.Second,
		CPUEnabled:      c.Monitoring.CPUEnabled,
		CPUThreshold:    c.Monitoring.CPUThreshold,
		MemoryEnabled:   c.Monitoring.MemoryEnabled,
		MemoryThreshold: uint64(c.Monitoring.MemoryThreshold) * 1024 * 1024, // Convert MB to bytes
		DiskEnabled:     c.Monitoring.DiskEnabled,
		DiskThreshold:   uint64(c.Monitoring.DiskThreshold) * 1024 * 1024, // Convert MB to bytes
	}
}
