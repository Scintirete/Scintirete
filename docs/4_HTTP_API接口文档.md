# HTTP API 接口文档

## 概述

Scintirete 提供基于 Gin 框架的 RESTful API 接口，用于向量数据库的管理和操作。所有 API 都遵循 REST 设计原则，使用 JSON 格式进行数据交换。

**基础URL**: `/api/v1`

**认证**: 除健康检查接口外，所有 API 都需要认证

## 认证方式

所有受保护的接口都需要在请求头中包含认证信息：

```
Authorization: Bearer <token>
```

## 响应格式

所有 API 响应都使用 JSON 格式：

```json
{
  "success": true,
  "data": {},
  "error": null
}
```

## 接口列表

### 1. 健康检查

**接口**: `GET /api/v1/health`

**描述**: 检查服务状态

**认证**: 不需要

**响应示例**:
```json
{
  "status": "healthy",
  "service": "scintirete",
  "version": "1.0.0"
}
```

---

### 2. 数据库管理

#### 2.1 创建数据库

**接口**: `POST /api/v1/databases`

**描述**: 创建新的数据库

**认证**: 需要

**请求体**:
```json
{
  "name": "database_name"
}
```

**响应**: 201 Created

#### 2.2 删除数据库

**接口**: `DELETE /api/v1/databases/:db_name`

**描述**: 删除指定数据库

**认证**: 需要

**参数**:
- `db_name`: 数据库名称（路径参数）

**响应**: 200 OK

#### 2.3 列出数据库

**接口**: `GET /api/v1/databases`

**描述**: 获取所有数据库列表

**认证**: 需要

**响应**: 200 OK

---

### 3. 集合管理

#### 3.1 创建集合

**接口**: `POST /api/v1/databases/:db_name/collections`

**描述**: 在指定数据库中创建集合

**认证**: 需要

**参数**:
- `db_name`: 数据库名称（路径参数）

**请求体**:
```json
{
  "collection_name": "collection_name",
  "metric_type": "COSINE",
  "dimension": 768
}
```

**响应**: 201 Created

#### 3.2 删除集合

**接口**: `DELETE /api/v1/databases/:db_name/collections/:coll_name`

**描述**: 删除指定集合

**认证**: 需要

**参数**:
- `db_name`: 数据库名称（路径参数）
- `coll_name`: 集合名称（路径参数）

**响应**: 200 OK

#### 3.3 获取集合信息

**接口**: `GET /api/v1/databases/:db_name/collections/:coll_name`

**描述**: 获取指定集合的详细信息

**认证**: 需要

**参数**:
- `db_name`: 数据库名称（路径参数）
- `coll_name`: 集合名称（路径参数）

**响应**: 200 OK

#### 3.4 列出集合

**接口**: `GET /api/v1/databases/:db_name/collections`

**描述**: 获取指定数据库中的所有集合

**认证**: 需要

**参数**:
- `db_name`: 数据库名称（路径参数）

**响应**: 200 OK

---

### 4. 向量操作

#### 4.1 插入向量

**接口**: `POST /api/v1/databases/:db_name/collections/:coll_name/vectors`

**描述**: 向指定集合中插入向量数据

**认证**: 需要

**参数**:
- `db_name`: 数据库名称（路径参数）
- `coll_name`: 集合名称（路径参数）

**请求体**:
```json
{
  "vectors": [
    {
      "id": "vector_id_1",
      "values": [0.1, 0.2, 0.3, ...],
      "metadata": {"key": "value"}
    }
  ]
}
```

**响应**: 201 Created

#### 4.2 删除向量

**接口**: `DELETE /api/v1/databases/:db_name/collections/:coll_name/vectors`

**描述**: 从指定集合中删除向量

**认证**: 需要

**参数**:
- `db_name`: 数据库名称（路径参数）
- `coll_name`: 集合名称（路径参数）

**请求体**:
```json
{
  "ids": ["vector_id_1", "vector_id_2"]
}
```

**响应**: 200 OK

#### 4.3 向量搜索

**接口**: `POST /api/v1/databases/:db_name/collections/:coll_name/search`

**描述**: 在指定集合中进行向量搜索

**认证**: 需要

**参数**:
- `db_name`: 数据库名称（路径参数）
- `coll_name`: 集合名称（路径参数）

**请求体**:
```json
{
  "query_vector": [0.1, 0.2, 0.3, ...],
  "top_k": 10,
  "filter": {"key": "value"}
}
```

**响应**: 200 OK

---

### 5. 文本嵌入

#### 5.1 嵌入并插入

**接口**: `POST /api/v1/databases/:db_name/collections/:coll_name/embed`

**描述**: 对文本进行嵌入并插入到向量数据库

**认证**: 需要

**参数**:
- `db_name`: 数据库名称（路径参数）
- `coll_name`: 集合名称（路径参数）

**请求体**:
```json
{
  "texts": [
    {
      "id": "text_id_1",
      "content": "文本内容",
      "metadata": {"key": "value"}
    }
  ]
}
```

**响应**: 201 Created

#### 5.2 嵌入并搜索

**接口**: `POST /api/v1/databases/:db_name/collections/:coll_name/embed/search`

**描述**: 对查询文本进行嵌入并搜索相似向量

**认证**: 需要

**参数**:
- `db_name`: 数据库名称（路径参数）
- `coll_name`: 集合名称（路径参数）

**请求体**:
```json
{
  "query_text": "查询文本",
  "top_k": 10,
  "filter": {"key": "value"}
}
```

**响应**: 200 OK

#### 5.3 文本嵌入

**接口**: `POST /api/v1/embed`

**描述**: 对文本进行嵌入处理

**认证**: 需要

**请求体**:
```json
{
  "texts": ["文本1", "文本2", "文本3"]
}
```

**响应**: 200 OK

#### 5.4 列出嵌入模型

**接口**: `GET /api/v1/embed/models`

**描述**: 获取可用的嵌入模型列表

**认证**: 需要

**响应**: 200 OK

---

## 错误处理

所有 API 在出错时都会返回相应的 HTTP 状态码和错误信息：

- **400 Bad Request**: 请求参数错误
- **401 Unauthorized**: 认证失败
- **404 Not Found**: 资源不存在
- **500 Internal Server Error**: 服务器内部错误

错误响应格式：
```json
{
  "error": {
    "code": "ERROR_CODE",
    "message": "错误描述"
  }
}
```

## 示例请求

### 创建数据库

```bash
curl -X POST http://localhost:8080/api/v1/databases \
  -H "Authorization: Bearer your_token" \
  -H "Content-Type: application/json" \
  -d '{"name": "my_database"}'
```

### 插入向量

```bash
curl -X POST http://localhost:8080/api/v1/databases/my_database/collections/my_collection/vectors \
  -H "Authorization: Bearer your_token" \
  -H "Content-Type: application/json" \
  -d '{
    "vectors": [
      {
        "id": "vec1",
        "values": [0.1, 0.2, 0.3],
        "metadata": {"category": "test"}
      }
    ]
  }'
```

### 向量搜索

```bash
curl -X POST http://localhost:8080/api/v1/databases/my_database/collections/my_collection/search \
  -H "Authorization: Bearer your_token" \
  -H "Content-Type: application/json" \
  -d '{
    "query_vector": [0.1, 0.2, 0.3],
    "top_k": 5
  }'
```

## 版本信息

- **API 版本**: v1
- **文档版本**: 1.0.0
- **最后更新**: 2024-07-29