# Quick Start

This guide will help you launch and experience Scintirete within 3 minutes.

## 🚀 Quick Launch

### Option 1: Using Docker (Recommended)

The simplest way to start is using the pre-built Docker image:

```bash
# Pull the latest image
docker pull scintirete/scintirete:latest

wget https://raw.githubusercontent.com/Scintirete/Scintirete/refs/heads/main/configs/scintirete.template.toml -O scintirete.toml

# Start the service
docker run -d \
  --name scintirete \
  -p 8080:8080 \
  -p 9090:9090 \
  -v $(pwd)/data:/app/data \
  -v $(pwd)/scintirete.toml:/app/configs/scintirete.toml \
  scintirete/scintirete:latest
```

### Option 2: Using Docker Compose

If you need the complete monitoring stack (optional Prometheus and Grafana):

```bash
# Download the project
git clone https://github.com/scintirete/scintirete.git
cd scintirete
cp configs/scintirete.template.toml configs/scintirete.toml

# Start the complete service stack
docker-compose up -d
```

After services start:
- **HTTP API**: http://localhost:8080
- **gRPC Service**: localhost:9090
- **Prometheus**: http://localhost:9091 (optional)
- **Grafana**: http://localhost:3000 (optional)

### Option 3: Using Pre-compiled Binaries

Download the appropriate binary for your operating system from the [Releases page](https://github.com/scintirete/scintirete/releases):

```bash
# Download and extract (Linux amd64 example)
wget https://github.com/scintirete/scintirete/releases/latest/download/scintirete-linux-amd64.tar.gz
wget https://raw.githubusercontent.com/Scintirete/Scintirete/refs/heads/main/configs/scintirete.template.toml -O scintirete.toml
tar -xzf scintirete-linux-amd64.tar.gz
cd scintirete-linux-amd64

# Start the server
./scintirete-server -c scintirete.toml
```

## 📊 Management Interface

Access the Web management interface:

**Cloud Address**: [http://scintirete-manager-ui.cloud.wj2015.com/](http://scintirete-manager-ui.cloud.wj2015.com/)

> Note: The client direct connection mode can be used for Scintirete services that the local network can connect to. The server forwarding mode is suitable for production intranet environments. For security reasons, the cloud address does not support server forwarding mode.

In the management interface, you can:
- Visually manage local or remote Scintirete services
- Insert vector data
- Search vector data

Detailed documentation: [ManagerUI User Guide](user-guides/5_manager-ui-guide.md)

## 📈 Monitoring and Metrics

If started with Docker Compose, you can access:

- **Grafana Monitoring Dashboard**: http://localhost:3000 (admin/admin) (optional)
- **Prometheus Metrics**: http://localhost:9091 (optional)
- **Service Metrics Endpoint**: http://localhost:8080/metrics

## 🎯 First Experience

### 1. Verify Service Status

```bash
curl http://localhost:8080/api/v1/health
```

Expected response:
```json
{
  "status": "healthy",
  "service": "scintirete",
  "version": "1.0.0"
}
```

### 2. Create Database and Collection

```bash
# Create database
curl -X POST http://localhost:8080/api/v1/databases \
  -H "Content-Type: application/json" \
  -d '{"name": "demo_db"}'

# Create collection (using cosine similarity)
curl -X POST http://localhost:8080/api/v1/databases/demo_db/collections \
  -H "Content-Type: application/json" \
  -d '{
    "name": "documents",
    "distance_metric": "cosine"
  }'
```

### 3. Insert Vector Data

```bash
# Insert some sample vectors
curl -X POST http://localhost:8080/api/v1/databases/demo_db/collections/documents/vectors \
  -H "Content-Type: application/json" \
  -d '{
    "vectors": [
      {
        "data": [0.1, 0.2, 0.3, 0.4],
        "metadata": {"text": "First document", "category": "technology"}
      },
      {
        "data": [0.2, 0.3, 0.4, 0.5],
        "metadata": {"text": "Second document", "category": "science"}
      },
      {
        "data": [0.9, 0.8, 0.7, 0.6],
        "metadata": {"text": "Third document", "category": "art"}
      }
    ]
  }'
```

### 4. Search Similar Vectors

```bash
# Search for the 2 most similar vectors
curl -X POST http://localhost:8080/api/v1/databases/demo_db/collections/documents/search \
  -H "Content-Type: application/json" \
  -d '{
    "query_vector": [0.15, 0.25, 0.35, 0.45],
    "limit": 2
  }'
```

Expected response:
```json
{
  "success": true,
  "data": {
    "results": [
      {
        "id": 2,
        "score": 0.9998,
        "metadata": {"text": "Second document", "category": "science"}
      },
      {
        "id": 1,
        "score": 0.9995,
        "metadata": {"text": "First document", "category": "technology"}
      }
    ]
  }
}
```

## 🎉 Experience Text Vectorization (Optional)

If you have an OpenAI API Key, you can experience automatic text vectorization:

### 1. Configure API Key

Modify the configuration file or set environment variable:

```bash
export OPENAI_API_KEY="your-api-key-here"
```

Or edit the configuration file `configs/scintirete.toml`:

```toml
[embedding]
provider = "openai"
api_key = "your-api-key-here"
base_url = "https://api.openai.com/v1"
model = "text-embedding-ada-002"
```

### 2. Insert Text Directly

```bash
# Automatically convert text to vectors and insert
curl -X POST http://localhost:8080/api/v1/databases/demo_db/collections/documents/texts \
  -H "Content-Type: application/json" \
  -d '{
    "texts": [
      {
        "content": "Artificial intelligence is changing the world",
        "metadata": {"source": "news", "topic": "AI"}
      },
      {
        "content": "Machine learning is the core technology of AI",
        "metadata": {"source": "book", "topic": "ML"}
      }
    ]
  }'
```

### 3. Text Semantic Search

```bash
# Search using natural language
curl -X POST http://localhost:8080/api/v1/databases/demo_db/collections/documents/search-text \
  -H "Content-Type: application/json" \
  -d '{
    "query_text": "AI technology development",
    "limit": 5
  }'
```

## 🔧 Using Command Line Tools

Scintirete also provides powerful command-line tools:

```bash
# Use CLI to connect to server
./scintirete-cli --server localhost:9090

# Execute commands in interactive mode
> list databases
> use demo_db
> list collections
> describe documents
```

## 🔄 Next Steps

After completing the quick experience, we recommend:

1. Read [System Requirements](2_system-requirements.md) to understand production environment deployment requirements
2. Check [HTTP API Documentation](user-guides/1_http-api-reference.md) for complete API reference
3. Learn [HNSW Parameter Tuning](user-guides/3_hnsw-parameter-tuning.md) to optimize performance
4. Use [ManagerUI](user-guides/5_manager-ui-guide.md) for visual management

## 💡 Frequently Asked Questions

**Q: Why are my vector search results inaccurate?**
A: Ensure consistent distance metrics and vector dimensions. Cosine similarity is suitable for most text vectors, Euclidean distance is suitable for image feature vectors.

**Q: How to improve search performance?**
A: Adjust HNSW hyperparameters `efConstruction` and `maxConnections`. Higher values provide better accuracy but consume more memory.

**Q: Will data persist after service restart?**
A: Yes, Scintirete uses AOF + RDB persistence mechanism to ensure data safety.

**Q: Can it be used in production environments?**
A: Scintirete is designed for production environments with complete monitoring, logging, and error handling, but it is currently in rapid development phase, please pay attention to version compatibility.