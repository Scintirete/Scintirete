// Package config provides configuration management for Scintirete.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config represents the complete Scintirete configuration.
type Config struct {
	Server        ServerConfig        `toml:"server"`
	Log           LogConfig           `toml:"log"`
	Persistence   PersistenceConfig   `toml:"persistence"`
	Embedding     EmbeddingConfig     `toml:"embedding"`
	Observability ObservabilityConfig `toml:"observability"`
	Algorithm     AlgorithmConfig     `toml:"algorithm"`
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

// PersistenceConfig contains data persistence settings.
type PersistenceConfig struct {
	DataDir         string `toml:"data_dir"`
	RDBFilename     string `toml:"rdb_filename"`
	AOFFilename     string `toml:"aof_filename"`
	AOFSyncStrategy string `toml:"aof_sync_strategy"`
}

// EmbeddingConfig contains external embedding service settings.
type EmbeddingConfig struct {
	BaseURL      string `toml:"base_url"`
	APIKeyEnvVar string `toml:"api_key_env_var"`
	RPMLimit     int    `toml:"rpm_limit"`
	TPMLimit     int    `toml:"tpm_limit"`
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
			DataDir:         "./data",
			RDBFilename:     "dump.rdb",
			AOFFilename:     "appendonly.aof",
			AOFSyncStrategy: "everysec",
		},
		Embedding: EmbeddingConfig{
			BaseURL:      "https://api.openai.com/v1/embeddings",
			APIKeyEnvVar: "OPENAI_API_KEY",
			RPMLimit:     3500,
			TPMLimit:     90000,
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
	
	return nil
}

// resolvePaths converts relative paths to absolute paths based on config file location.
func (c *Config) resolvePaths(configFilePath string) error {
	configDir := filepath.Dir(configFilePath)
	
	// Resolve data directory path
	if !filepath.IsAbs(c.Persistence.DataDir) {
		c.Persistence.DataDir = filepath.Join(configDir, c.Persistence.DataDir)
	}
	
	return nil
}

// GetEmbeddingAPIKey retrieves the embedding API key from environment variable.
func (c *Config) GetEmbeddingAPIKey() string {
	if c.Embedding.APIKeyEnvVar == "" {
		return ""
	}
	return os.Getenv(c.Embedding.APIKeyEnvVar)
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