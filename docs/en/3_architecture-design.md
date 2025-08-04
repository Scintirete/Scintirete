# Scintirete Architecture Design

## 1. Overall Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Client Layer                             │
├─────────────────────────────────────────────────────────────┤
│  scintirete-cli  │  HTTP/JSON API  │  gRPC Client SDKs      │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│                    Network Layer                            │
├─────────────────────────────────────────────────────────────┤
│    HTTP Gateway    │         gRPC Server                    │
│  (gRPC-Gateway)    │    (scintirete.proto)                 │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│                   Service Layer                             │
├─────────────────────────────────────────────────────────────┤
│  Authentication  │  Database Mgmt  │  Collection Mgmt       │
│     Service      │     Service     │      Service           │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│                    Core Layer                               │
├─────────────────────────────────────────────────────────────┤
│   Vector Store   │   Index Engine  │   Embedding Client     │
│    Manager       │    (HNSW)       │     (OpenAI API)       │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│                 Persistence Layer                           │
├─────────────────────────────────────────────────────────────┤
│     AOF Logger   │   RDB Snapshots │   Config Manager       │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│                Infrastructure Layer                         │
├─────────────────────────────────────────────────────────────┤
│   Metrics/Logs   │   Error Handler │   Rate Limiter         │
└─────────────────────────────────────────────────────────────┘
```

## 2. Directory Structure Planning

```
scintirete/
├── cmd/                          # Command-line tool entry points
│   ├── scintirete-server/        # Server startup tool
│   │   └── main.go
│   └── scintirete-cli/           # Client CLI tool
│       └── main.go
├── internal/                     # Internal implementation, not exposed
│   ├── server/                   # Server core implementation
│   │   ├── grpc/                 # gRPC service implementation
│   │   ├── http/                 # HTTP gateway implementation
│   │   └── service/              # Business logic services
│   ├── core/                     # Core business logic
│   │   ├── database/             # Database management
│   │   ├── collection/           # Collection management
│   │   ├── vector/               # Vector operations
│   │   └── algorithm/            # Algorithm implementation (HNSW)
│   ├── persistence/              # Persistence layer
│   │   ├── aof/                  # AOF log implementation
│   │   ├── rdb/                  # RDB snapshot implementation
│   │   └── wal/                  # Write-ahead log
│   ├── embedding/                # Embedding API client
│   ├── auth/                     # Authentication management
│   ├── config/                   # Configuration management
│   ├── observability/            # Observability (logs, metrics)
│   │   ├── logger/
│   │   ├── metrics/
│   │   └── audit/
│   └── utils/                    # Utility functions
├── pkg/                          # Publicly exposed packages
│   ├── client/                   # Go client SDK
│   └── types/                    # Common type definitions
├── schemas/                      # Schema definitions
│   ├── proto/                    # protobuf definitions
│   │   └── scintirete/
│   │       └── v1/
│   │           └── scintirete.proto
│   └── flatbuffers/              # FlatBuffers schema
│       ├── aof.fbs               # AOF log structure
│       └── rdb.fbs               # RDB snapshot structure
├── gen/                          # Generated code
│   └── go/                       # Go code generation directory
├── configs/                      # Configuration file templates and examples
│   ├── scintirete.template.toml  # Configuration file template (copy and modify to scintirete.toml)
│   └── docker-compose.yml
├── scripts/                      # Build and deployment scripts
│   └── docker/
├── test/                         # Test related
│   ├── integration/              # Integration tests
│   ├── benchmark/                # Performance tests
│   └── testdata/                 # Test data
├── docs/                         # Documentation
├── go.mod
├── go.sum
├── Makefile                      # Build commands
├── Dockerfile
├── docker-compose.yml
└── README.md
```

## 3. Core Module Design

### 3.1 Interface Abstraction Layers

```go
// Top-level abstraction: Database Engine
type DatabaseEngine interface {
    CreateDatabase(name string) error
    DropDatabase(name string) error
    ListDatabases() ([]string, error)
    GetDatabase(name string) (Database, error)
}

// Database abstraction
type Database interface {
    CreateCollection(name string, config CollectionConfig) error
    DropCollection(name string) error
    ListCollections() ([]CollectionInfo, error)
    GetCollection(name string) (Collection, error)
}

// Collection abstraction
type Collection interface {
    Insert(vectors []Vector) error
    Delete(ids []string) (int, error)
    Search(query []float32, topK int, params SearchParams) ([]SearchResult, error)
    GetInfo() CollectionInfo
}

// Vector index abstraction
type VectorIndex interface {
    Build(vectors []Vector) error
    Insert(vector Vector) error
    Delete(id string) error
    Search(query []float32, topK int, params SearchParams) ([]SearchResult, error)
    Size() int
    MemoryUsage() int64
}

// Persistence abstraction
type Persistence interface {
    WriteAOF(command AOFCommand) error
    LoadFromRDB() error
    SaveRDB() error
    Recover() error
}
```

### 3.2 Module Responsibility Division

#### 3.2.1 Core Layer
- **DatabaseManager**: Manages multiple database instances
- **CollectionManager**: Manages collection CRUD operations
- **VectorStore**: In-memory storage and retrieval of vector data
- **IndexEngine**: Vector index algorithm implementation (primarily HNSW)

#### 3.2.2 Service Layer
- **AuthService**: Password verification and authorization
- **DatabaseService**: Database-level operation services
- **CollectionService**: Collection-level operation services
- **VectorService**: Vector operation services
- **EmbeddingService**: Text embedding services

#### 3.2.3 Persistence Layer
- **AOFLogger**: Append-only log recorder
- **RDBManager**: Snapshot manager
- **ConfigManager**: Configuration file management

#### 3.2.4 Infrastructure Layer
- **Logger**: Structured logging
- **MetricsCollector**: Metrics collection
- **RateLimiter**: Request rate limiting
- **ErrorHandler**: Unified error handling

## 4. HNSW Algorithm Abstraction Design

```go
// HNSW algorithm core interface
type HNSWIndex interface {
    VectorIndex
    
    // HNSW-specific methods
    GetParameters() HNSWParams
    GetLayers() int
    GetGraphStatistics() GraphStats
}

// HNSW configuration parameters
type HNSWParams struct {
    M              int     // Maximum connections
    EfConstruction int     // Construction-time search range
    EfSearch       int     // Search-time range
    MaxLayers      int     // Maximum layers
    Seed           int64   // Random seed
}

// Distance calculation abstraction
type DistanceCalculator interface {
    Distance(a, b []float32) float32
    DistanceType() DistanceMetric
}
```

## 5. Error Handling Strategy

```go
// Unified error type
type ScintireteError struct {
    Code    ErrorCode
    Message string
    Cause   error
}

// Error code classification
type ErrorCode int

const (
    // System errors (1000-1999)
    ErrorCodeInternal ErrorCode = 1000
    ErrorCodeConfig   ErrorCode = 1001
    
    // Authentication errors (2000-2999)
    ErrorCodeUnauthorized ErrorCode = 2000
    ErrorCodeForbidden    ErrorCode = 2001
    
    // Business errors (3000-3999)
    ErrorCodeDatabaseNotFound   ErrorCode = 3000
    ErrorCodeCollectionNotFound ErrorCode = 3001
    ErrorCodeVectorNotFound     ErrorCode = 3002
    ErrorCodeDimensionMismatch  ErrorCode = 3003
    
    // Persistence errors (4000-4999)
    ErrorCodePersistenceFailed ErrorCode = 4000
    ErrorCodeRecoveryFailed    ErrorCode = 4001
)
```

## 6. Concurrency Model

### 6.1 Read-Write Separation Strategy
- **Read Operations**: Support high concurrency, use read locks
- **Write Operations**: Execute mutually exclusively, use write locks
- **Index Rebuild**: Execute asynchronously in background, doesn't block read operations

### 6.2 Lock Granularity Design
```go
type SafeCollection struct {
    mu          sync.RWMutex     // Collection-level lock
    vectorStore VectorStore      // Vector storage
    index       VectorIndex      // Vector index
    metadata    CollectionInfo   // Metadata
}
```

## 7. Testing Strategy

### 7.1 Unit Test Coverage Requirements
- **Core Algorithms**: ≥95% coverage
- **Business Logic**: ≥90% coverage
- **Utility Functions**: ≥85% coverage

### 7.2 Testing Layers
```go
// Unit tests: Test individual functions/methods
func TestHNSWInsert(t *testing.T) {}

// Integration tests: Test module interactions
func TestDatabaseServiceIntegration(t *testing.T) {}

// Benchmark tests: Performance testing
func BenchmarkHNSWSearch(b *testing.B) {}

// End-to-end tests: Complete workflow testing
func TestE2EWorkflow(t *testing.T) {}
```

### 7.3 Mock and Dependency Injection
- Use Mock for external dependencies (embedding APIs)
- Use in-memory implementations for file system operations
- Use interfaces for dependency injection

## 8. Development Standards

### 8.1 Code Style
- Strictly follow `gofmt` and `golint` standards
- Use `go vet` for static checking
- Function naming follows Go conventions (capitalized for public)

### 8.2 Comment Standards
```go
// PackageName provides vector database functionality
// with HNSW indexing algorithm.
package core

// DatabaseManager manages multiple database instances
// and provides thread-safe operations.
type DatabaseManager struct {
    // Field comments
}

// CreateDatabase creates a new database with the specified name.
// It returns an error if the database already exists.
func (dm *DatabaseManager) CreateDatabase(name string) error {
    // Implementation...
}
```

### 8.3 Git Commit Standards
```
feat: new feature
fix: bug fix
docs: documentation update
style: code formatting adjustment
refactor: code refactoring
test: test related
chore: build, configuration, etc.

Example: feat: implement HNSW algorithm core structure
```

## 9. Performance Requirements

### 9.1 Response Time Targets
- **Insert Operations**: <10ms (single vector)
- **Search Operations**: <50ms (top-10, 1M vectors)
- **Batch Insert**: <100ms (100 vectors)

### 9.2 Memory Usage Optimization
- Vector data uses compact storage format
- Index structure optimizes memory layout
- Timely reclamation of marked-deleted vectors

### 9.3 Concurrency Performance
- Support 1000+ concurrent read operations
- Write operations serialized but efficiently executed

## 10. Extensibility Design

### 10.1 Algorithm Pluginization
```go
// Algorithm factory interface
type IndexFactory interface {
    CreateIndex(config IndexConfig) VectorIndex
    SupportedMetrics() []DistanceMetric
}

// Registration mechanism
func RegisterIndexFactory(name string, factory IndexFactory)
```

### 10.2 Distance Metric Extension
```go
// Support for new distance metrics
type CustomDistanceCalculator struct{}

func (c CustomDistanceCalculator) Distance(a, b []float32) float32 {
    // Custom distance calculation
}
```

This architectural design ensures the project's maintainability, testability, and extensibility, providing clear guidance for subsequent development work.