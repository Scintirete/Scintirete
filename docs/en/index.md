# Scintirete

[![Go](https://github.com/scintirete/scintirete/actions/workflows/ci.yml/badge.svg)](https://github.com/scintirete/scintirete/actions/workflows/ci.yml)
[![Release](https://github.com/scintirete/scintirete/actions/workflows/release.yml/badge.svg)](https://github.com/scintirete/scintirete/actions/workflows/release.yml)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Scintirete is a lightweight, production-oriented vector database based on the HNSW (Hierarchical Navigable Small World) algorithm. Its name derives from the Latin words Scintilla (spark) and Rete (network), meaning "a network of sparkling sparks", symbolizing the illumination of those small yet crucial sparks through the deepest similarities between data in complex data networks.

**Core Philosophy:** Illuminate the network of data, discover infinite neighbors.

## Features

- **Simple & Lightweight**: Self-implemented core logic without redundant dependencies, focused on the core functionality of vector search
- **High Performance**: Memory-based HNSW graph indexing providing millisecond-level nearest neighbor search
- **Data Safety**: Efficient persistence mechanism based on flatbuffers implementing Redis-like AOF + RDB, ensuring data integrity
- **Modern Interfaces**: Native support for both gRPC and HTTP/JSON dual interfaces, easy integration into any modern application architecture
- **Easy Operations**: Provides structured logging, audit logs, Prometheus metrics, and convenient command-line tools, designed for production environments
- **Cross-Platform**: Out-of-the-box support for Linux, macOS, Windows, and both arm64 and amd64 architectures
- **Text Embedding Support**: Support for OpenAI-compatible API integration, enabling automatic text vectorization

Scintirete aims to provide an out-of-the-box, high-performance, and easy-to-maintain vector search solution for small to medium-sized projects, edge computing scenarios, and developers who need rapid prototype validation.

## Documentation Navigation

### 🚀 Quick Start
- [Quick Start](1_quick-start.md) - Experience Scintirete in 3 minutes
- [System Requirements](2_system-requirements.md) - Runtime environment and dependency requirements

### 📖 Core Documentation
- [Architecture Design](3_architecture-design.md) - Deep dive into system architecture

### 📚 User Guides
- [HTTP API Reference](user-guides/1_http-api-reference.md) - RESTful API complete reference
- [CLI Tool Parameters](user-guides/2_cli-tool-parameters.md) - CLI tool usage instructions
- [HNSW Parameter Tuning](user-guides/3_hnsw-parameter-tuning.md) - Performance optimization guide
- [gRPC Interface Usage](user-guides/4_grpc-interface-usage.md) - gRPC service usage guide
- [ManagerUI User Guide](user-guides/5_manager-ui-guide.md) - Web management interface usage instructions

## License

This project is licensed under the MIT License.

## Support

- **Issues**: [GitHub Issues](https://github.com/scintirete/scintirete/issues)
- **Discussions**: [GitHub Discussions](https://github.com/scintirete/scintirete/discussions)