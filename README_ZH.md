# Scintirete

[![Go](https://github.com/scintirete/scintirete/actions/workflows/ci.yml/badge.svg)](https://github.com/scintirete/scintirete/actions/workflows/ci.yml)
[![Release](https://github.com/scintirete/scintirete/actions/workflows/release.yml/badge.svg)](https://github.com/scintirete/scintirete/actions/workflows/release.yml)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

![](head-logo-zh.jpg)

[English](README.md)

Scintirete 是一款基于 HNSW（分层导航小世界）算法实现的轻量级、面向生产的向量数据库。它的名字源于拉丁语 Scintilla（火花）和 Rete（网络），意为闪光的火花之网，寓意着在庞杂的数据网络中，用数据间最深层的相似性点亮那些微小却关键的火花。

**核心理念：** 点亮数据之网，发现无限近邻。

## 开源生态

【临时】官网和文档：[scintirete.cloud.wj2015.com](http://scintirete.cloud.wj2015.com/zh/)（[开源地址](https://github.com/Scintirete/webapp)）

【临时】数据库管理系统（DBMS）：[scintirete-manager-ui.cloud.wj2015.com](http://scintirete-manager-ui.cloud.wj2015.com/)（[开源地址](https://github.com/Scintirete/scintirete-manager-ui)）

正式官网备案中，暂时无法访问：[scintirete.top](https://scintirete.top)

## 特性

- **简单轻量**: 核心逻辑自主实现，无冗余依赖，专注于向量搜索的核心功能
- **高性能**: 基于内存中的 HNSW 图索引，提供毫秒级的最近邻搜索
- **数据安全**: 基于 flatbuffers 实现了类似于 Redis 的 AOF + RDB 高效持久化机制，确保数据万无一失
- **现代接口**: 原生支持 gRPC 和 HTTP/JSON 双接口，易于集成到任何现代应用架构中
- **易于运维**: 提供结构化日志、审计日志、Prometheus 指标和便捷的命令行工具，为生产环境而设计
- **跨平台**: 支持 Linux、macOS、Windows 及 arm64 、amd64 架构开箱即用
- **支持文本嵌入**: 支持 OpenAI 兼容 API 集成，支持自动文本向量化

Scintirete 的目标是为中小型项目、边缘计算场景以及需要快速原型验证的开发者，提供一个开箱即用、性能卓越且易于维护的向量搜索解决方案。

## 路线图

- [ ] 提供上下游框架集成，如 langchain、langgraph 等
- [ ] 在 webapp 中实现一些杀手级功能供参考体验，如电影推荐、人脸识别、知识库问答等
- [ ] 在树莓派上流畅运行全套项目
- [ ] 基于 protobuf 提供多语言SDK


## 快速上手

### 环境要求

- Go 1.24+（从源码构建时需要）
- Docker（可选，用于容器化部署）

### 安装

#### 选项 1：下载预编译二进制文件

从 [releases 页面](https://github.com/scintirete/scintirete/releases) 下载最新版本。

#### 选项 2：从源码构建

```bash
git clone https://github.com/scintirete/scintirete.git
cd scintirete
make all
```

#### 选项 3：Docker

```bash
docker pull ghcr.io/scintirete/scintirete:latest
```

### 基本使用

#### 1. 启动服务器

```bash
# 使用二进制文件
./bin/scintirete-server

# 使用 Docker
docker run -p 8080:8080 -p 9090:9090 ghcr.io/scintirete/scintirete:latest

# 使用 docker-compose
docker-compose up -d
```

服务器将在以下端口启动：
- gRPC API：9090 端口
- HTTP/JSON API：8080 端口

#### 2. 环境初始化（支持文本嵌入功能）

要使用文本嵌入功能，请配置您的 OpenAI 兼容 API，配置文件 `configs/scintirete.toml` 中 `[embedding]` 表定义了与外部文本嵌入服务交互的配置

首先从模板创建配置文件，然后编辑：

```bash
cp configs/scintirete.template.toml configs/scintirete.toml
```

编辑配置文件 `configs/scintirete.toml`：

```toml
# [embedding] 表定义了与外部文本嵌入服务交互的配置
[embedding]
# 符合 OpenAI `embeddings` 接口规范的 API base URL
base_url = "https://api.openai.com/v1/embeddings"
# API Token/Key。为了安全，建议使用强密码或令牌。
api_key = ""
# 每分钟请求数限制 (RPM)
rpm_limit = 3500
# 每分钟 Token 数限制 (TPM)
tpm_limit = 90000
```

#### 3. 基本操作

使用命令行工具执行基本的向量操作：

```bash
# 创建数据库
./bin/scintirete-cli -p "your-password" db create my_app

# 为文档创建集合
./bin/scintirete-cli -p "your-password" collection create my_app documents --metric Cosine

# 插入文本并自动嵌入
./bin/scintirete-cli -p "your-password" text insert my_app documents \
  "doc1" \
  "Scintirete 是一个为生产环境优化的轻量级向量数据库。" \
  '{"source":"documentation","type":"intro"}'

# 插入更多文档
./bin/scintirete-cli -p "your-password" text insert my_app documents \
  "doc2" \
  "HNSW 算法提供高效的近似最近邻搜索。" \
  '{"source":"documentation","type":"technical"}'

# 搜索相似内容
./bin/scintirete-cli -p "your-password" text search my_app documents \
  "什么是 Scintirete？" \
  5

# 获取集合信息
./bin/scintirete-cli -p "your-password" collection info my_app documents
```

#### 4. 使用预计算向量

如果您有预计算的向量：

```bash
# 直接插入向量
./bin/scintirete-cli -p "your-password" vector insert my_app vectors \
  --id "vec1" \
  --vector '[0.1, 0.2, 0.3, 0.4]' \
  --metadata '{"category":"example"}'

# 使用向量搜索
./bin/scintirete-cli -p "your-password" vector search my_app vectors \
  --vector '[0.15, 0.25, 0.35, 0.45]' \
  --top-k 3
```

更多文档请参阅 [http://scintirete.cloud.wj2015.com/zh/docs](http://scintirete.cloud.wj2015.com/zh/docs) 目录。

## 架构

Scintirete 实现了现代向量数据库架构，包含以下组件：

- **核心引擎**: 内存中的 HNSW 图，支持可配置参数
- **持久化层**: AOF（实时）和 RDB（快照）双模式持久化策略
- **API 层**: 支持 gRPC（高性能）和 HTTP/JSON（易用性）双协议
- **嵌入集成**: OpenAI 兼容 API 集成，支持自动文本向量化
- **可观测性**: 全面的日志记录、审计日志、指标监控

详细的技术文档请参阅 [http://scintirete.cloud.wj2015.com/zh/docs](http://scintirete.cloud.wj2015.com/zh/docs) 目录。

## 配置

Scintirete 使用单一的 TOML 配置文件。默认配置为大多数用例提供了合理的默认值：

```toml
[server]
grpc_host = "127.0.0.1"
grpc_port = 9090
http_host = "127.0.0.1"
http_port = 8080
passwords = ["your-strong-password-here"]

[log]
level = "info"
format = "json"
enable_audit_log = true

[persistence]
data_dir = "./data"
aof_sync_strategy = "everysec"

[embedding]
base_url = "https://api.openai.com/v1/embeddings"
api_key = "your-openai-api-key"
rpm_limit = 3500
tpm_limit = 90000
```

## API 文档

Scintirete 提供 gRPC 和 HTTP/JSON 两种 API：

- **gRPC**: 高性能接口，定义在 [protobuf](schemas/proto/scintirete/v1/scintirete.proto) 中
- **HTTP/JSON**: RESTful 接口，可通过 `http://localhost:8080/` 访问

全面的 API 文档和使用示例请参考 [文档](http://scintirete.cloud.wj2015.com/zh/docs)。

## 性能考虑

- **内存使用**: 向量存储在内存中以获得最佳搜索性能
- **索引配置**: 根据精度/速度要求调优 HNSW 参数（`m`、`ef_construction`、`ef_search`）
- **持久化**: 根据持久性与性能需求配置 AOF 同步策略

## 参与贡献

我们欢迎对 Scintirete 的贡献！以下是您可以提供帮助的方式：

### 开发环境设置

1. **Fork 并克隆**
   ```bash
   git clone https://github.com/your-username/scintirete.git
   cd scintirete
   ```

2. **安装依赖并构建**
   ```bash
   brew install flatbuffers protobuf
   make all
   ```

3. **运行测试**
   ```bash
   make test
   ```

### 贡献指南

- **代码质量**: 确保您的代码通过所有测试并遵循 Go 约定
- **文档**: 为任何 API 或配置更改更新文档
- **测试**: 为新功能和错误修复添加测试
- **提交信息**: 使用清晰、描述性的提交信息
- **Pull Request**: 提供更改及其理由的详细描述

### 贡献领域

- **性能优化**: HNSW 算法改进、内存优化
- **功能**: 元数据过滤、其他距离度量、聚类算法
- **集成**: 不同语言的客户端库、框架集成
- **文档**: 教程、最佳实践、部署指南
- **测试**: 集成测试、基准测试、压力测试

### 行为准则

我们致力于提供一个热情和包容的环境。请以尊重和专业的态度对待所有贡献者。

## 许可证

此项目在 MIT 许可证下授权 - 详情请参阅 [LICENSE](LICENSE) 文件。

## 支持

- **文档**: [文档](http://scintirete.cloud.wj2015.com/zh/docs)
- **问题**: [GitHub Issues](https://github.com/scintirete/scintirete/issues)
- **讨论**: [GitHub Discussions](https://github.com/scintirete/scintirete/discussions)

---

*Scintirete: 点亮数据之网，发现无限近邻。* 