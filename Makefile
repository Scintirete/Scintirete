# Scintirete Makefile

.PHONY: all build test test-coverage test-integration benchmark clean proto-gen flatbuffers-gen lint format vet help server cli deps docker docker-compose
.DEFAULT_GOAL := help

# 基本配置
GO := go
GOFMT := gofmt
GOLINT := golint
GOVET := go vet
PROTOC := protoc
FLATC := flatc

# 项目路径
PROJECT_ROOT := $(shell pwd)
API_DIR := schemas/proto
GEN_DIR := gen/go
BIN_DIR := bin
FLATBUFFERS_SCHEMA_DIR := schemas/flatbuffers
FLATBUFFERS_GEN_DIR := internal/flatbuffers

# 生成的protobuf文件
PROTO_FILES := $(shell find $(API_DIR) -name "*.proto")
PROTO_GO_FILES := $(patsubst $(API_DIR)/%.proto,$(GEN_DIR)/%.pb.go,$(PROTO_FILES))

# FlatBuffers schema 文件
FLATBUFFERS_SCHEMA_FILES := $(shell find $(FLATBUFFERS_SCHEMA_DIR) -name "*.fbs" 2>/dev/null || echo "")

# 二进制文件名
SERVER_BINARY := scintirete-server
CLI_BINARY := scintirete-cli

help: ## 显示帮助信息
	@echo "Scintirete 构建系统"
	@echo ""
	@echo "可用命令:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

all: proto-gen flatbuffers-gen deps build ## 完整构建流程

deps: ## 安装依赖
	@echo "Installing dependencies..."
	$(GO) mod download
	$(GO) mod tidy

proto-gen: ## 生成protobuf代码
	@echo "Generating protobuf code..."
	@mkdir -p $(GEN_DIR)
	@if ! command -v protoc > /dev/null; then \
		echo "Error: protoc not found. Please install Protocol Buffers compiler."; \
		exit 1; \
	fi
	@if ! $(GO) list -m google.golang.org/protobuf > /dev/null 2>&1; then \
		echo "Installing protobuf dependencies..."; \
		$(GO) get google.golang.org/protobuf/cmd/protoc-gen-go; \
		$(GO) get google.golang.org/grpc/cmd/protoc-gen-go-grpc; \
	fi
	$(PROTOC) \
		--proto_path=$(API_DIR) \
		--go_out=$(GEN_DIR) \
		--go_opt=paths=source_relative \
		--go-grpc_out=$(GEN_DIR) \
		--go-grpc_opt=paths=source_relative \
		$(PROTO_FILES)

flatbuffers-gen: ## 生成FlatBuffers代码
	@echo "Generating FlatBuffers code..."
	@if [ -z "$(FLATBUFFERS_SCHEMA_FILES)" ]; then \
		echo "No FlatBuffers schema files found in $(FLATBUFFERS_SCHEMA_DIR)"; \
		exit 0; \
	fi
	@if ! command -v $(FLATC) > /dev/null; then \
		echo "Error: flatc not found. Please install FlatBuffers compiler."; \
		echo "  macOS: brew install flatbuffers"; \
		echo "  Ubuntu: sudo apt-get install flatbuffers-compiler"; \
		exit 1; \
	fi
	@echo "Cleaning existing generated FlatBuffers files..."
	@rm -rf $(FLATBUFFERS_GEN_DIR)/rdb/*.go $(FLATBUFFERS_GEN_DIR)/aof/*.go
	@echo "Creating FlatBuffers directories..."
	@mkdir -p $(FLATBUFFERS_GEN_DIR)/rdb $(FLATBUFFERS_GEN_DIR)/aof
	@if [ -f "$(FLATBUFFERS_SCHEMA_DIR)/rdb.fbs" ]; then \
		echo "Generating RDB FlatBuffers code..."; \
		$(FLATC) --go -o $(FLATBUFFERS_GEN_DIR)/rdb $(FLATBUFFERS_SCHEMA_DIR)/rdb.fbs; \
		if [ -d "$(FLATBUFFERS_GEN_DIR)/rdb/scintirete" ]; then \
			mv $(FLATBUFFERS_GEN_DIR)/rdb/scintirete/rdb/* $(FLATBUFFERS_GEN_DIR)/rdb/; \
			rm -rf $(FLATBUFFERS_GEN_DIR)/rdb/scintirete; \
		fi; \
		echo "Generated RDB files in $(FLATBUFFERS_GEN_DIR)/rdb/:"; \
		ls -la $(FLATBUFFERS_GEN_DIR)/rdb/*.go 2>/dev/null | head -5 || echo "No .go files found"; \
	fi
	@if [ -f "$(FLATBUFFERS_SCHEMA_DIR)/aof.fbs" ]; then \
		echo "Generating AOF FlatBuffers code..."; \
		$(FLATC) --go -o $(FLATBUFFERS_GEN_DIR)/aof $(FLATBUFFERS_SCHEMA_DIR)/aof.fbs; \
		if [ -d "$(FLATBUFFERS_GEN_DIR)/aof/scintirete" ]; then \
			mv $(FLATBUFFERS_GEN_DIR)/aof/scintirete/aof/* $(FLATBUFFERS_GEN_DIR)/aof/; \
			rm -rf $(FLATBUFFERS_GEN_DIR)/aof/scintirete; \
		fi; \
		echo "Generated AOF files in $(FLATBUFFERS_GEN_DIR)/aof/:"; \
		ls -la $(FLATBUFFERS_GEN_DIR)/aof/*.go 2>/dev/null | head -5 || echo "No .go files found"; \
	fi
	@echo "FlatBuffers code generation completed!"

build: proto-gen flatbuffers-gen ## 构建所有二进制文件
	@echo "Building binaries..."
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_DIR)/$(SERVER_BINARY) ./cmd/scintirete-server
	$(GO) build -o $(BIN_DIR)/$(CLI_BINARY) ./cmd/scintirete-cli
	$(GO) build -o $(BIN_DIR)/cpu-monitor ./cmd/cpu-monitor

server: proto-gen flatbuffers-gen ## 只构建服务端
	@echo "Building server..."
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_DIR)/$(SERVER_BINARY) ./cmd/scintirete-server

cli: proto-gen flatbuffers-gen ## 只构建客户端
	@echo "Building CLI..."
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_DIR)/$(CLI_BINARY) ./cmd/scintirete-cli

cpu-monitor: proto-gen flatbuffers-gen ## 构建CPU监控工具
	@echo "Building CPU monitor..."
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_DIR)/cpu-monitor ./cmd/cpu-monitor

test: proto-gen flatbuffers-gen ## 运行测试
	@echo "Running tests..."
	$(GO) test -v -race -cover ./...

test-coverage: proto-gen flatbuffers-gen ## 运行测试并生成覆盖率报告
	@echo "Running tests with coverage..."
	$(GO) test -v -race -coverprofile=coverage.out -covermode=atomic ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-integration: proto-gen flatbuffers-gen ## 运行集成测试
	@echo "Running integration tests..."
	$(GO) test -v -tags=integration ./test/integration/...

benchmark: proto-gen flatbuffers-gen ## 运行基准测试  
	@echo "Running benchmark tests..."
	$(GO) test -bench=. -benchmem -timeout=300s ./test/benchmark/...
	$(GO) test -bench=. -benchmem -timeout=300s ./internal/core/algorithm/...

lint: ## 运行代码检查
	@echo "Running linter..."
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not found, using basic checks..."; \
		$(GOLINT) ./...; \
	fi

format: ## 格式化代码
	@echo "Formatting code..."
	$(GOFMT) -s -w .

vet: ## 运行go vet检查
	@echo "Running go vet..."
	$(GOVET) ./...

check: format vet lint test ## 运行所有检查

clean: ## 清理构建文件
	@echo "Cleaning up..."
	rm -rf $(BIN_DIR)
	rm -rf $(GEN_DIR)
	rm -f coverage.out coverage.html
	rm -rf $(FLATBUFFERS_GEN_DIR)/rdb/*.go $(FLATBUFFERS_GEN_DIR)/aof/*.go

install: build ## 安装到GOPATH/bin
	@echo "Installing binaries..."
	$(GO) install ./cmd/scintirete-server
	$(GO) install ./cmd/scintirete-cli

docker-build: ## 构建Docker镜像
	@echo "Building Docker image..."
	docker build -t scintirete:latest .

docker-run: ## 运行Docker容器
	@echo "Running Docker container..."
	docker run -p 8080:8080 -p 9090:9090 -p 9100:9100 -v $(PWD)/data:/app/data scintirete:latest

docker-compose-up: ## 启动docker-compose服务
	@echo "Starting services with docker-compose..."
	docker-compose up -d

docker-compose-down: ## 停止docker-compose服务
	@echo "Stopping docker-compose services..."
	docker-compose down

docker-compose-logs: ## 查看docker-compose日志
	@echo "Showing docker-compose logs..."
	docker-compose logs -f

docker-compose-monitoring: ## 启动包含监控的完整服务
	@echo "Starting services with monitoring..."
	docker-compose --profile monitoring up -d

tools: ## 安装开发工具
	@echo "Installing development tools..."
	$(GO) install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	$(GO) install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "To install FlatBuffers compiler (flatc):"
	@echo "  macOS: brew install flatbuffers"
	@echo "  Ubuntu: sudo apt-get install flatbuffers-compiler"
	@echo "  Other: Download from https://github.com/google/flatbuffers/releases"

.PHONY: run-server run-cli
run-server: server ## 运行服务端 (开发模式)
	@echo "Starting Scintirete server..."
	@if [ ! -f configs/scintirete.toml ]; then \
		echo "Warning: configs/scintirete.toml not found. Please copy from configs/scintirete.template.toml"; \
		echo "cp configs/scintirete.template.toml configs/scintirete.toml"; \
		exit 1; \
	fi
	./$(BIN_DIR)/$(SERVER_BINARY) --config configs/scintirete.toml

run-cli: cli ## 运行CLI客户端示例
	@echo "Running CLI example..."
	./$(BIN_DIR)/$(CLI_BINARY) --help 