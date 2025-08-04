# System Requirements

This document describes the system requirements and recommended configurations for running Scintirete.

## 📋 Basic Requirements

### Operating System Support

Scintirete supports the following operating systems:

| Operating System | Architecture | Minimum Version | Recommended Version |
|------------------|--------------|-----------------|-------------------|
| **Linux** | x86_64 | Ubuntu 18.04, CentOS 7 | Ubuntu 20.04+, CentOS 8+ |
| **Linux** | ARM64 | Ubuntu 18.04, CentOS 7 | Ubuntu 20.04+, CentOS 8+ |
| **macOS** | x86_64 | macOS 10.15 (Catalina) | macOS 12.0+ (Monterey) |
| **macOS** | ARM64 (M1/M2) | macOS 11.0 (Big Sur) | macOS 12.0+ (Monterey) |
| **Windows** | x86_64 | Windows 10 | Windows 11 |

### Hardware Requirements

#### Minimum Configuration
- **CPU**: 1 core (x86_64 or ARM64)
- **Memory**: 512 MB RAM
- **Storage**: 100 MB available disk space
- **Network**: TCP/IP support

#### Recommended Configuration
- **CPU**: 2+ cores (x86_64 or ARM64)
- **Memory**: 2 GB+ RAM
- **Storage**: 1 GB+ SSD storage
- **Network**: Gigabit Ethernet

## 🧮 Memory Calculation Guide

### Vector Storage Memory Requirements

Scintirete stores all vector data in memory to ensure optimal performance. Memory requirements are primarily determined by the following factors:

```
Total Memory Required = Vector Data + HNSW Index + Metadata + System Overhead
```

#### Vector Data Memory

```
Vector Data Memory = Number of Vectors × Vector Dimension × 4 bytes (float32)
```

**Example Calculations**:
- 1 million 768-dimensional vectors: 1,000,000 × 768 × 4 = ~2.9 GB
- 1 million 1536-dimensional vectors: 1,000,000 × 1536 × 4 = ~5.7 GB

#### HNSW Index Memory

HNSW index typically requires an additional 30-50% memory overhead:

```
Index Memory ≈ Vector Data Memory × 0.3 to 0.5
```

#### Complete Memory Estimation Table

| Vector Count | Dimension | Vector Data | Index Overhead | Recommended Total Memory |
|--------------|-----------|-------------|----------------|-------------------------|
| 10,000 | 768 | ~29 MB | ~12 MB | 512 MB |
| 100,000 | 768 | ~290 MB | ~120 MB | 1 GB |
| 1,000,000 | 768 | ~2.9 GB | ~1.2 GB | 8 GB |
| 10,000,000 | 768 | ~29 GB | ~12 GB | 64 GB |
| 1,000,000 | 1536 | ~5.7 GB | ~2.3 GB | 16 GB |

### Memory Optimization Recommendations

1. **Reserve Memory**: Reserve at least 25% of memory for system and other processes
2. **Batch Import**: Import large amounts of data in batches to avoid memory peaks
3. **Monitor Usage**: Use the `/metrics` endpoint to monitor memory usage
4. **Regular Rebuild**: Periodically rebuild indexes to clean up marked-deleted vectors

## 🔧 Runtime Dependencies

### No External Dependencies

Scintirete is a **statically compiled single binary** with no additional runtime dependencies required:

- ✅ **No Python Environment Required**
- ✅ **No Go Runtime Required**
- ✅ **No Database Software Required**
- ✅ **No Complex System Libraries Required**

### Optional Dependencies (for Integration Features)

| Feature | Dependency | Description |
|---------|------------|-------------|
| **Docker Deployment** | Docker 20.10+ | Containerized deployment |
| **Monitoring Integration** | Prometheus, Grafana | Observability stack |
| **Text Vectorization** | OpenAI API or compatible service | Automatic text embedding |
| **Reverse Proxy** | Nginx, Traefik, etc. | Production load balancing |

## 🌐 Network Requirements

### Port Configuration

| Port | Protocol | Purpose | Default | Configurable |
|------|----------|---------|---------|--------------|
| **HTTP API** | TCP | RESTful API service | 8080 | ✅ |
| **gRPC** | TCP | gRPC service | 9090 | ✅ |
| **Metrics Monitoring** | TCP | Prometheus metrics | 8080 (built-in) | ❌ |

### Firewall Configuration

**Minimal configuration** (local access only):
```bash
# Allow local loopback access
sudo ufw allow from 127.0.0.1
```

**Internal network service configuration**:
```bash
# Allow internal network access to HTTP API
sudo ufw allow from 192.168.0.0/16 to any port 8080

# Allow internal network access to gRPC
sudo ufw allow from 192.168.0.0/16 to any port 9090
```

**Public network service configuration** (not recommended, use reverse proxy and TLS):
```bash
# Caution: Only open public access with appropriate security measures
sudo ufw allow 8080
sudo ufw allow 9090
```

### External Network Requirements

If using text vectorization features, access to OpenAI API is required. Please configure `base_url` and `api_key` accordingly.

## 💾 Storage Requirements

### Disk Space Planning

```
Total Storage Required = Data Files + Log Files + Configuration Files + System Overhead
```

#### Data Storage Estimation

| Component | Storage Required | Description |
|-----------|------------------|-------------|
| **RDB Snapshots** | ~Memory data size | Compressed data snapshots |
| **AOF Logs** | Variable | Operation logs, configurable rotation |
| **Metadata** | < 1% of data size | Collection and database metadata |
| **System Logs** | Configurable | Structured logs and audit logs |

#### Storage Configuration Recommendations

| Data Scale | Recommended Storage | Storage Type | IOPS Requirement |
|------------|-------------------|--------------|------------------|
| **< 1GB data** | 10 GB | SSD | Basic |
| **1-10GB data** | 100 GB | SSD | Medium |
| **10-100GB data** | 1 TB | NVMe SSD | High |
| **> 100GB data** | Data size × 3 | Enterprise SSD | Very High |

### File System Recommendations

| Operating System | Recommended File System | Alternatives |
|------------------|------------------------|--------------|
| **Linux** | ext4, XFS | Btrfs, ZFS |
| **macOS** | APFS | HFS+ |
| **Windows** | NTFS | ReFS |

## 🔒 Security Requirements

### Network Security

1. **Firewall**: Configure appropriate firewall rules
2. **TLS**: Use HTTPS/TLS in production environments
3. **Authentication**: Configure strong passwords or key authentication
4. **Network Isolation**: Place database services in protected network segments

### File System Permissions

```bash
# Create dedicated user (recommended)
sudo useradd -r -s /bin/false scintirete

# Set data directory permissions
sudo mkdir -p /var/lib/scintirete
sudo chown scintirete:scintirete /var/lib/scintirete
sudo chmod 750 /var/lib/scintirete

# Set configuration file permissions
sudo chmod 640 /etc/scintirete/scintirete.toml
sudo chown root:scintirete /etc/scintirete/scintirete.toml
```

## 🚀 Performance Tuning

### System-Level Optimization

#### Linux System Parameters

```bash
# Increase file descriptor limits
echo "* soft nofile 65536" >> /etc/security/limits.conf
echo "* hard nofile 65536" >> /etc/security/limits.conf

# Optimize memory management
echo 'vm.swappiness = 1' >> /etc/sysctl.conf
echo 'vm.dirty_ratio = 15' >> /etc/sysctl.conf
echo 'vm.dirty_background_ratio = 5' >> /etc/sysctl.conf

# Apply configuration
sysctl -p
```

#### Docker Optimization

```bash
# Limit container memory (prevent OOM)
docker run -m 4g scintirete/scintirete:latest

# Use local SSD volume
docker run -v /fast-ssd/scintirete:/app/data scintirete/scintirete:latest
```

### Application-Level Optimization

1. **HNSW Parameter Tuning**: Refer to [HNSW Parameter Tuning](user-guides/3_hnsw-parameter-tuning.md)
2. **Batch Operations**: Use batch insert and search interfaces
3. **Connection Pooling**: Client-side connection pool management
4. **Monitoring-Based Tuning**: Adjust configuration based on Prometheus metrics

## 📊 Environment Verification

### System Information Check

```bash
# Check basic system information
uname -a
cat /etc/os-release

# Check hardware resources
free -h              # Memory
df -h               # Disk space
lscpu               # CPU information
```

### Network Connectivity Test

```bash
# Test port availability
netstat -tuln | grep 8080
netstat -tuln | grep 9090

# Test network latency
ping your-scintirete-server
```

## 🔧 Deployment Mode Comparison

| Deployment Mode | Use Case | Resource Requirements | Complexity |
|-----------------|----------|----------------------|------------|
| **Single Binary** | Development, testing, small-scale production | Minimal | Simple |
| **Docker Container** | Standardized deployment | Low | Medium |
| **Docker Compose** | Complete stack with monitoring | Medium | Medium |
| **Kubernetes** | Large-scale, high availability | High | Complex |

Choose the appropriate deployment mode to meet your requirements and resource constraints.