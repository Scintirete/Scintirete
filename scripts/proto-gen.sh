#!/bin/bash

# Scintirete Protobuf 代码生成脚本

set -euo pipefail

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 目录配置
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
API_DIR="${PROJECT_ROOT}/api/proto"
GEN_DIR="${PROJECT_ROOT}/gen/go"

echo -e "${GREEN}Scintirete Protobuf Code Generation${NC}"
echo "Project root: ${PROJECT_ROOT}"
echo "API directory: ${API_DIR}"
echo "Generated code directory: ${GEN_DIR}"

# 检查protoc是否安装
if ! command -v protoc &> /dev/null; then
    echo -e "${RED}Error: protoc (Protocol Buffers compiler) is not installed.${NC}"
    echo "Please install it from: https://grpc.io/docs/protoc-installation/"
    exit 1
fi

# 检查Go protobuf插件
if ! command -v protoc-gen-go &> /dev/null; then
    echo -e "${YELLOW}Warning: protoc-gen-go not found. Installing...${NC}"
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
fi

if ! command -v protoc-gen-go-grpc &> /dev/null; then
    echo -e "${YELLOW}Warning: protoc-gen-go-grpc not found. Installing...${NC}"
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
fi

# 创建生成目录
echo "Creating generated code directory..."
mkdir -p "${GEN_DIR}"

# 查找所有proto文件
echo "Finding proto files..."
PROTO_FILES=$(find "${API_DIR}" -name "*.proto" | sort)

if [ -z "$PROTO_FILES" ]; then
    echo -e "${RED}Error: No .proto files found in ${API_DIR}${NC}"
    exit 1
fi

echo "Found proto files:"
for file in $PROTO_FILES; do
    echo "  - ${file#$PROJECT_ROOT/}"
done

# 生成Go代码
echo "Generating Go code..."
for proto_file in $PROTO_FILES; do
    echo "Processing: ${proto_file#$PROJECT_ROOT/}"
    
    protoc \
        --proto_path="${API_DIR}" \
        --go_out="${GEN_DIR}" \
        --go_opt=paths=source_relative \
        --go-grpc_out="${GEN_DIR}" \
        --go-grpc_opt=paths=source_relative \
        "${proto_file}"
        
    if [ $? -ne 0 ]; then
        echo -e "${RED}Error: Failed to generate code for ${proto_file}${NC}"
        exit 1
    fi
done

# 验证生成的文件
echo "Verifying generated files..."
GENERATED_FILES=$(find "${GEN_DIR}" -name "*.pb.go" | wc -l)
echo "Generated ${GENERATED_FILES} Go files"

if [ $GENERATED_FILES -eq 0 ]; then
    echo -e "${RED}Error: No Go files were generated${NC}"
    exit 1
fi

# 格式化生成的代码
echo "Formatting generated code..."
find "${GEN_DIR}" -name "*.pb.go" -exec gofmt -s -w {} \;

echo -e "${GREEN}✓ Protobuf code generation completed successfully!${NC}"
echo "Generated files in: ${GEN_DIR}" 