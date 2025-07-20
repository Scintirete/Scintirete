#!/bin/bash

# Generate FlatBuffers Go code from schema files

set -e

echo "Generating FlatBuffers Go code..."

# Clean existing generated files
rm -rf internal/flatbuffers/rdb/*.go internal/flatbuffers/aof/*.go

# Create directories
mkdir -p internal/flatbuffers/rdb internal/flatbuffers/aof

# Generate RDB FlatBuffers code
echo "Generating RDB FlatBuffers code..."
flatc --go -o internal/flatbuffers/rdb schemas/flatbuffers/rdb.fbs

# Generate AOF FlatBuffers code  
echo "Generating AOF FlatBuffers code..."
flatc --go -o internal/flatbuffers/aof schemas/flatbuffers/aof.fbs

# Move files from nested directories to correct location
if [ -d "internal/flatbuffers/rdb/scintirete" ]; then
    mv internal/flatbuffers/rdb/scintirete/rdb/* internal/flatbuffers/rdb/
    rm -rf internal/flatbuffers/rdb/scintirete
fi

if [ -d "internal/flatbuffers/aof/scintirete" ]; then
    mv internal/flatbuffers/aof/scintirete/aof/* internal/flatbuffers/aof/
    rm -rf internal/flatbuffers/aof/scintirete
fi

echo "FlatBuffers Go code generation completed!"
echo "Generated RDB files in internal/flatbuffers/rdb/:"
ls -la internal/flatbuffers/rdb/*.go | head -5
echo "Generated AOF files in internal/flatbuffers/aof/:"
ls -la internal/flatbuffers/aof/*.go | head -5 