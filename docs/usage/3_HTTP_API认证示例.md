# HTTP API 认证示例

## Bearer Token 认证

Scintirete HTTP API 除了 `/health` 端点外，所有其他端点都需要 Bearer token 认证。

### 认证格式

```
Authorization: Bearer {your-password}
```

### 示例请求

#### 1. 健康检查（无需认证）

```bash
curl -X GET http://localhost:8080/api/v1/health
```

#### 2. 创建数据库（需要认证）

```bash
curl -X POST http://localhost:8080/api/v1/databases \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-password" \
  -d '{"name": "my_database"}'
```

#### 3. 列出数据库（需要认证）

```bash
curl -X GET http://localhost:8080/api/v1/databases \
  -H "Authorization: Bearer your-password"
```

#### 4. 创建集合（需要认证）

```bash
curl -X POST http://localhost:8080/api/v1/databases/my_database/collections \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-password" \
  -d '{
    "collection_name": "my_collection",
    "metric_type": "DISTANCE_METRIC_COSINE",
    "hnsw_config": {
      "m": 16,
      "ef_construction": 200
    }
  }'
```

#### 5. 插入向量（需要认证）

```bash
curl -X POST http://localhost:8080/api/v1/databases/my_database/collections/my_collection/vectors \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-password" \
  -d '{
    "vectors": [
      {
        "elements": [0.1, 0.2, 0.3, 0.4],
        "metadata": {"text": "example vector"}
      }
    ]
  }'
```

#### 6. 搜索向量（需要认证）

```bash
curl -X POST http://localhost:8080/api/v1/databases/my_database/collections/my_collection/search \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-password" \
  -d '{
    "query_vector": [0.1, 0.2, 0.3, 0.4],
    "top_k": 10
  }'
```

### 错误处理

#### 缺少认证头

```bash
curl -X GET http://localhost:8080/api/v1/databases
```

响应：
```json
{
  "success": false,
  "error": "Authorization header required"
}
```

#### 错误的认证格式

```bash
curl -X GET http://localhost:8080/api/v1/databases \
  -H "Authorization: your-password"
```

响应：
```json
{
  "success": false,
  "error": "Invalid authorization format. Expected: Bearer {token}"
}
```

#### 错误的密码

```bash
curl -X GET http://localhost:8080/api/v1/databases \
  -H "Authorization: Bearer wrong-password"
```

响应：
```json
{
  "success": false,
  "error": "invalid credentials"
}
```

### 支持的端点

#### 公开端点（无需认证）
- `GET /api/v1/health` - 健康检查

#### 受保护端点（需要认证）
- 数据库操作：
  - `POST /api/v1/databases` - 创建数据库
  - `GET /api/v1/databases` - 列出数据库
  - `DELETE /api/v1/databases/{db_name}` - 删除数据库

- 集合操作：
  - `POST /api/v1/databases/{db_name}/collections` - 创建集合
  - `GET /api/v1/databases/{db_name}/collections` - 列出集合
  - `GET /api/v1/databases/{db_name}/collections/{coll_name}` - 获取集合信息
  - `DELETE /api/v1/databases/{db_name}/collections/{coll_name}` - 删除集合

- 向量操作：
  - `POST /api/v1/databases/{db_name}/collections/{coll_name}/vectors` - 插入向量
  - `DELETE /api/v1/databases/{db_name}/collections/{coll_name}/vectors` - 删除向量
  - `POST /api/v1/databases/{db_name}/collections/{coll_name}/search` - 搜索向量

- 嵌入操作：
  - `POST /api/v1/databases/{db_name}/collections/{coll_name}/embed` - 文本嵌入并插入
  - `POST /api/v1/databases/{db_name}/collections/{coll_name}/embed/search` - 文本嵌入并搜索
  - `POST /api/v1/embed` - 文本嵌入
  - `GET /api/v1/embed/models` - 列出嵌入模型

### 配置密码

在服务器配置中设置密码：

```toml
[server]
passwords = ["your-secret-password", "another-password"]
``` 