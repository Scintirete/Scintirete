### Scintirete Python SDK 设计方案

定位：面向 Python 3.9+，基于 `grpcio` 与 `grpcio-tools` 生成客户端，提供同步与可选异步（`grpc.aio`）封装，自动注入 `AuthInfo`，统一重试/超时/压缩，便于在数据科学、服务端任务与脚本中使用。

---

### 安装

- 运行时：`grpcio>=1.60`
- 代码生成（开发时）：`grpcio-tools`

```bash
pip install grpcio
pip install grpcio-tools
```

---

### 代码生成

从 `schemas/proto/scintirete/v1/scintirete.proto` 生成 Python stub：

```bash
python -m grpc_tools.protoc \
  -I ./schemas/proto \
  --python_out=./gen/python \
  --grpc_python_out=./gen/python \
  ./schemas/proto/scintirete/v1/scintirete.proto
```

生成产物示例：`gen/python/scintirete/v1/scintirete_pb2.py` 与 `scintirete_pb2_grpc.py`。

建议在 `pyproject.toml` 定义生成脚本，或提供 `Makefile` 目标。

---

### 初始化

提供 `ScintireteClient` 封装：

```python
# sdk/client.py
import grpc
from typing import Optional, Dict, Any
from gen.python.scintirete.v1 import scintirete_pb2 as pb
from gen.python.scintirete.v1 import scintirete_pb2_grpc as pbgrpc


class ScintireteClient:
    def __init__(
        self,
        address: str,
        password: Optional[str] = None,
        use_tls: bool = False,
        options: Optional[list[tuple[str, Any]]] = None,
        default_timeout: Optional[float] = 10.0,
        enable_gzip: bool = False,
    ) -> None:
        self._password = password
        compression = grpc.Compression.Gzip if enable_gzip else grpc.Compression.NoCompression
        opts = options or [
            ("grpc.keepalive_time_ms", 30_000),
            ("grpc.keepalive_timeout_ms", 10_000),
            ("grpc.max_receive_message_length", 64 * 1024 * 1024),
            ("grpc.max_send_message_length", 64 * 1024 * 1024),
        ]
        if use_tls:
            creds = grpc.ssl_channel_credentials()
            self._channel = grpc.secure_channel(address, creds, options=opts, compression=compression)
        else:
            self._channel = grpc.insecure_channel(address, options=opts, compression=compression)

        self._stub = pbgrpc.ScintireteServiceStub(self._channel)
        self._default_timeout = default_timeout

    def _with_auth(self, payload: dict) -> dict:
        if not self._password:
            return payload
        return {"auth": {"password": self._password}, **payload}

    def close(self) -> None:
        self._channel.close()

    # --- 示例 API ---
    def create_database(self, name: str, timeout: Optional[float] = None):
        req = pb.CreateDatabaseRequest(**self._with_auth({"name": name}))
        return self._stub.CreateDatabase(req, timeout=timeout or self._default_timeout)

    def create_collection(self, db_name: str, collection_name: str, metric_type: int, hnsw_config: Optional[dict] = None):
        payload = {"db_name": db_name, "collection_name": collection_name, "metric_type": metric_type}
        if hnsw_config:
            payload["hnsw_config"] = pb.HnswConfig(**hnsw_config)
        req = pb.CreateCollectionRequest(**self._with_auth(payload))
        return self._stub.CreateCollection(req, timeout=self._default_timeout)

    def search(self, db_name: str, collection_name: str, query_vector: list[float], top_k: int, include_vector: bool = False):
        req = pb.SearchRequest(**self._with_auth({
            "db_name": db_name,
            "collection_name": collection_name,
            "query_vector": query_vector,
            "top_k": top_k,
            "include_vector": include_vector,
        }))
        return self._stub.Search(req, timeout=self._default_timeout)

    # 其他接口同理封装
```

异步版本可提供 `ScintireteAsyncClient`，使用 `grpc.aio.insecure_channel` 与 `await stub.Method(...)`。

---

### 底层实现的高效封装（gRPC）

- **连接复用**：单例 `Channel`，进程内 HTTP/2 复用。
- **超时/截止时间**：统一 `timeout`，避免悬挂。
- **压缩**：可选 `grpc.Compression.Gzip`。
- **大报文**：放宽 `max_{receive,send}_message_length`。
- **错误处理**：统一捕获 `grpc.RpcError`，解析 `code()` 与 `details()`，返回领域错误。
- **Auth 注入**：因 `AuthInfo` 在请求体内，统一在封装层扩展 payload。

---

### 使用示例

```python
from sdk.client import ScintireteClient

client = ScintireteClient("127.0.0.1:50051", password="secret")

client.create_database("demo")
client.create_collection("demo", "docs", metric_type=2)

res = client.search("demo", "docs", query_vector=[0.1,0.2,0.3], top_k=5, include_vector=True)
print(res)

client.close()
```

---

### 开发者如何发布（PyPI）

- 目录建议：
  - 源码：`scintirete_sdk/`（封装层），生成：`gen/python/`
  - 配置：`pyproject.toml`（`build-system = hatchling` 或 `setuptools`）
  - 类型：可选 `py.typed` 以暴露类型信息
- 关键配置：
  - `dependencies = ["grpcio>=1.60"]`
  - `include` 将 `gen/python` 打包进发行物
  - `scripts`: `gen:py`（执行上面的 protoc 命令）

发布流程：

```bash
python -m build
twine upload dist/*
```

语义化版本与 proto 版本对齐；在 `CHANGELOG.md` 记录 API 变更。

---

### 测试与兼容

- 使用 docker-compose 启服务端做集成测试
- 覆盖大批量插入/检索与断线重连
- 在 Linux/macOS/Windows（WSL）上验证


