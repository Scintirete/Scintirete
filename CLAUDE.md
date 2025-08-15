# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Scintirete is a lightweight, production-ready vector database built on the HNSW (Hierarchical Navigable Small World) algorithm. It provides high-performance vector search with dual gRPC and HTTP/JSON APIs, Redis-like persistence (AOF + RDB), and OpenAI-compatible embedding integration.

## Development Commands

### Building and Testing
```bash
# Full build with code generation
make all

# Build specific components
make server        # Build server binary
make cli           # Build CLI binary
make build         # Build both server and CLI

# Run tests
make test                          # Run all tests with race detection
make test-coverage                 # Run tests with coverage report
make test-integration              # Run integration tests only
make benchmark                     # Run performance benchmarks

# Code quality
make check                         # Run format, vet, lint, and test
make format                        # Format code with gofmt
make vet                           # Run go vet static analysis
make lint                          # Run linter (golangci-lint if available)

# Development tools
make tools                         # Install protoc, flatc, golangci-lint
make proto-gen                     # Generate protobuf code
make flatbuffers-gen               # Generate FlatBuffers code
```

### Running Services
```bash
# Development mode
make run-server                    # Start server (requires configs/scintirete.toml)
make run-cli                      # Run CLI help

# Docker operations
make docker-build                  # Build Docker image
make docker-run                    # Run Docker container
make docker-compose-up             # Start with docker-compose
make docker-compose-monitoring     # Start with Prometheus/Grafana monitoring
```

### Configuration
- Copy `configs/scintirete.template.toml` to `configs/scintirete.toml` before running
- Key configuration sections: `[server]`, `[persistence]`, `[embedding]`, `[algorithm]`
- Default ports: gRPC 9090, HTTP 8080, Metrics 9100

## Architecture Overview

### Core Components
- **DatabaseEngine** (`internal/core/interfaces.go:12`): Top-level interface managing multiple databases
- **Database** (`internal/core/interfaces.go:36`): Single database containing multiple collections
- **Collection** (`internal/core/interfaces.go:57`): Vector collection with HNSW indexing
- **VectorIndex** (`internal/core/interfaces.go:87`): Vector indexing abstraction (HNSW implementation)
- **Persistence** (`internal/core/interfaces.go:143`): Redis-like AOF + RDB persistence layer

### Key Implementation Files
- **Server**: `cmd/scintirete-server/main.go` - Main server entry point with gRPC/HTTP setup
- **Core Algorithm**: `internal/core/algorithm/hnsw.go` - HNSW algorithm implementation
- **Distance Metrics**: `internal/core/algorithm/distance.go` - Distance calculations
- **Persistence**: `internal/persistence/` - AOF and RDB implementations
- **API Handlers**: `internal/server/grpc/` and `internal/server/http/` - gRPC and HTTP implementations

### Directory Structure
```
cmd/                    # Server and CLI entry points
internal/
  core/                 # Core interfaces and HNSW algorithm
  server/               # gRPC and HTTP server implementations
  persistence/          # AOF and RDB persistence layer
  embedding/            # OpenAI-compatible embedding client
  observability/        # Logging, metrics, audit
  flatbuffers/          # Generated FlatBuffers code
pkg/types/              # Public type definitions
schemas/                # Protocol buffers and FlatBuffers schemas
```

## Code Generation

### Protocol Buffers
- Schema location: `schemas/proto/scintirete/v1/scintirete.proto`
- Generated to: `gen/go/scintirete/v1/`
- Run: `make proto-gen` (requires protoc)

### FlatBuffers
- Schema location: `schemas/flatbuffers/aof.fbs` and `rdb.fbs`
- Generated to: `internal/flatbuffers/aof/` and `rdb/`
- Run: `make flatbuffers-gen` (requires flatc)

## Testing Strategy

### Test Structure
- Unit tests: Co-located with source files (`*_test.go`)
- Integration tests: `test/integration/`
- Benchmarks: `test/benchmark/`
- Coverage requirement: ≥90% for core logic

### Running Tests
```bash
# All tests with coverage
make test-coverage

# Integration tests only
make test-integration

# Performance benchmarks
make benchmark
```

## Development Guidelines

### Code Style
- Follow Go standard formatting (`gofmt`)
- Use `go vet` for static analysis
- Structured logging with context
- Interface-based design for testability

### Error Handling
- Use defined error types from `internal/utils/errors.go`
- Include context in error messages
- Proper error propagation through layers

### Concurrency
- Collection-level RWMutex for thread safety
- Read operations support high concurrency
- Write operations are serialized but efficient
- Background tasks for persistence and maintenance

## Performance Considerations

### HNSW Parameters
- Default: `M=16`, `ef_construction=200`, `ef_search=50`
- Tuning guidance available in docs/
- Parameters affect accuracy vs. speed trade-offs

### Memory Usage
- Vectors stored in memory for optimal performance
- Compact storage formats used
- Regular compaction to reclaim space

### Persistence Strategy
- AOF: Real-time command logging (configurable sync)
- RDB: Periodic snapshots (default 5-minute intervals)
- Configurable durability vs. performance balance

## Configuration Management

### Key Configuration Sections
- `[server]`: Network settings and authentication
- `[persistence]`: Data directory and sync strategies
- `[embedding]`: OpenAI-compatible API settings
- `[algorithm]`: HNSW default parameters
- `[observability]`: Metrics and logging configuration

### Environment Variables
- Override config values with environment variables
- Use secrets management for API keys and passwords