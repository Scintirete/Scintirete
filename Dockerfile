# Multi-stage Docker build for Scintirete
FROM golang:1.24-alpine AS builder

# Build arguments
ARG VERSION=dev
ARG COMMIT=unknown

# Install build dependencies
RUN apk add --no-cache \
    git \
    protobuf \
    protobuf-dev \
    flatbuffers \
    make \
    gcc \
    musl-dev

# Set working directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Install protoc plugins
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Generate protobuf code
RUN make proto-gen

# Generate flatbuffers code
RUN make flatbuffers-gen

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s -X main.version=${VERSION} -X main.commit=${COMMIT}" \
    -o scintirete-server ./cmd/scintirete-server

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s -X main.version=${VERSION} -X main.commit=${COMMIT}" \
    -o scintirete-cli ./cmd/scintirete-cli

# Production stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 scintirete && \
    adduser -D -s /bin/sh -u 1000 -G scintirete scintirete

# Set working directory
WORKDIR /app

# Copy binaries from builder stage
COPY --from=builder /app/scintirete-server /app/scintirete-cli ./

# Copy configuration template (users should create scintirete.toml from template)
COPY --chown=scintirete:scintirete configs/scintirete.template.toml ./configs/

# Create data directory
RUN mkdir -p /app/data && chown -R scintirete:scintirete /app

# Switch to non-root user
USER scintirete

# Expose ports
EXPOSE 8080 9090 9100

# Create volume for data persistence
VOLUME ["/app/data"]

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
    CMD ./scintirete-cli --help > /dev/null || exit 1

# Default command (expects scintirete.toml to be mounted or created from template)
CMD ["./scintirete-server", "--config", "configs/scintirete.toml"] 