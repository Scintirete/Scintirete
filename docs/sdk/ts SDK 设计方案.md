### Scintirete TypeScript SDK 设计方案

面向 Node.js 环境，基于 gRPC 使用 `@grpc/grpc-js` 与 `ts-proto` 生成强类型客户端，并在其上提供轻量封装，自动注入 `AuthInfo`、管理长连接、统一超时/重试/压缩。目标：开箱即用、类型安全、可与 bundler 兼容（Node 18+）。

---

### 安装

- 必备：
  - Node.js 18+
  - `protoc` 3.21+（建议通过 Homebrew 安装：`brew install protobuf`）
- 依赖：
  - 运行时：`@grpc/grpc-js`
  - 代码生成（开发时）：`ts-proto`

```bash
pnpm add @grpc/grpc-js
pnpm add -D ts-proto
# 或 npm/yarn 等价命令
```

---

### 代码生成

从 `schemas/proto/scintirete/v1/scintirete.proto` 生成 TS 类型与 gRPC 客户端：

```bash
protoc \
  --plugin=./node_modules/.bin/protoc-gen-ts_proto \
  --ts_proto_out=./gen/ts \
  --ts_proto_opt=esModuleInterop=true,outputServices=grpc-js,env=node,outputJsonMethods=true,forceLong=string \
  --proto_path=./schemas/proto \
  ./schemas/proto/scintirete/v1/scintirete.proto
```

生成产物（示例）路径：`gen/ts/scintirete/v1/scintirete.ts`，其中包含 `ScintireteServiceClient`、请求/响应类型等。

建议在 `package.json` 增加脚本：

```json
{
  "scripts": {
    "gen:ts": "protoc --plugin=./node_modules/.bin/protoc-gen-ts_proto --ts_proto_out=./gen/ts --ts_proto_opt=esModuleInterop=true,outputServices=grpc-js,env=node,outputJsonMethods=true,forceLong=string --proto_path=./schemas/proto ./schemas/proto/scintirete/v1/scintirete.proto"
  }
}
```

---

### 初始化

提供一个高级封装工厂：`createScintireteClient(options)`，返回具备默认 `AuthInfo` 注入、超时、压缩与连接复用的客户端实例。

```ts
// sdk/client.ts（SDK 暴露）
import { credentials, ChannelCredentials, ChannelOptions } from '@grpc/grpc-js';
import { ScintireteServiceClient } from '../../gen/ts/scintirete/v1/scintirete';

export interface ScintireteClientOptions {
  address: string;               // 例如 '127.0.0.1:50051'
  password?: string;             // AuthInfo.password；可为空
  useTLS?: boolean;              // 是否启用 TLS
  channelOptions?: ChannelOptions; // keepalive、最大报文等
  defaultDeadlineMs?: number;    // 默认调用超时
  enableGzip?: boolean;          // 是否启用 gzip 压缩
}

export interface Client {
  raw: ScintireteServiceClient;
  withAuth<T extends object>(req: T): T; // 向请求体注入 auth
  close(): void;
}

export function createScintireteClient(opts: ScintireteClientOptions): Client {
  const creds: ChannelCredentials = opts.useTLS ? credentials.createSsl() : credentials.createInsecure();
  const client = new ScintireteServiceClient(opts.address, creds, opts.channelOptions);

  const withAuth = <T extends any>(req: T): T => {
    if (!opts.password) return req;
    return { auth: { password: opts.password }, ...req } as T;
  };

  return {
    raw: client,
    withAuth,
    close: () => client.close()
  };
}
```

---

### 底层实现的高效封装（gRPC）

- **长连接与复用**：每个 `address` 维持单一 `ScintireteServiceClient` 实例，进程内复用 HTTP/2 连接。
- **超时/截止时间**：SDK 在每个调用中支持可选 `deadline`（`defaultDeadlineMs`），防止悬挂请求。
- **压缩**：可选启用 gzip（`enableGzip`），对大向量批量插入/检索显著节省带宽。
- **大报文**：默认放宽 `grpc.max_receive_message_length`、`grpc.max_send_message_length`，支持百万级元素批量。
- **错误归一化**：将常见 gRPC 错误码（如 `UNAVAILABLE`、`DEADLINE_EXCEEDED`）映射为统一错误对象，附带重试建议。
- **Auth 自动注入**：由于 proto 将 `AuthInfo` 放在请求体，无法用 Metadata 统一注入，故提供 `withAuth(req)` 助手或在薄封装 API 中自动填充。

建议默认 ChannelOptions：

```ts
const channelOptions: ChannelOptions = {
  'grpc.keepalive_time_ms': 30_000,
  'grpc.keepalive_timeout_ms': 10_000,
  'grpc.max_receive_message_length': 64 * 1024 * 1024,
  'grpc.max_send_message_length': 64 * 1024 * 1024
};
```

---

### SDK 暴露的薄封装 API（示例）

```ts
// sdk/api.ts
import { Metadata, CallOptions } from '@grpc/grpc-js';
import type { Client } from './client';
import {
  CreateDatabaseRequest,
  CreateCollectionRequest,
  InsertVectorsRequest,
  SearchRequest,
  EmbedAndInsertRequest,
  SaveRequest
} from '../../gen/ts/scintirete/v1/scintirete';

export class Scintirete {
  constructor(private readonly c: Client) {}

  createDatabase(req: Omit<CreateDatabaseRequest, 'auth'>, options?: CallOptions) {
    return new Promise((resolve, reject) =>
      this.c.raw.CreateDatabase(this.c.withAuth(req), (err, res) => (err ? reject(err) : resolve(res)))
    );
  }

  createCollection(req: Omit<CreateCollectionRequest, 'auth'>, options?: CallOptions) {
    return new Promise((resolve, reject) =>
      this.c.raw.CreateCollection(this.c.withAuth(req), (err, res) => (err ? reject(err) : resolve(res)))
    );
  }

  insertVectors(req: Omit<InsertVectorsRequest, 'auth'>, options?: CallOptions) {
    return new Promise((resolve, reject) =>
      this.c.raw.InsertVectors(this.c.withAuth(req), (err, res) => (err ? reject(err) : resolve(res)))
    );
  }

  search(req: Omit<SearchRequest, 'auth'>, options?: CallOptions) {
    return new Promise((resolve, reject) =>
      this.c.raw.Search(this.c.withAuth(req), (err, res) => (err ? reject(err) : resolve(res)))
    );
  }

  embedAndInsert(req: Omit<EmbedAndInsertRequest, 'auth'>, options?: CallOptions) {
    return new Promise((resolve, reject) =>
      this.c.raw.EmbedAndInsert(this.c.withAuth(req), (err, res) => (err ? reject(err) : resolve(res)))
    );
  }

  save(req: Omit<SaveRequest, 'auth'> = {}, options?: CallOptions) {
    return new Promise((resolve, reject) =>
      this.c.raw.Save(this.c.withAuth(req), (err, res) => (err ? reject(err) : resolve(res)))
    );
  }
}
```

---

### 使用示例

```ts
import { createScintireteClient, Scintirete } from 'scintirete';

const client = createScintireteClient({
  address: '127.0.0.1:50051',
  password: 'secret',
  useTLS: false
});

const api = new Scintirete(client);

await api.createDatabase({ name: 'demo' });
await api.createCollection({
  db_name: 'demo',
  collection_name: 'docs',
  metric_type: 2 // COSINE
});

await api.embedAndInsert({
  db_name: 'demo',
  collection_name: 'docs',
  texts: [{ text: 'hello world', metadata: { lang: 'en' } }]
});

const result = await api.search({
  db_name: 'demo',
  collection_name: 'docs',
  query_vector: [0.1, 0.2, 0.3],
  top_k: 5,
  include_vector: true
});

console.log(result.results);

client.close();
```

---

### 开发者如何发布（npm）

- 目录与构建：
  - 源码：`src/`（封装层）；生成：`gen/ts/`
  - 构建：使用 `tsup` 或 `tsc` 出 ESM/CJS 双产物
- `package.json` 要点：
  - `name`: `scintirete`
  - `type`: `module`
  - `exports`: 同时导出 ESM/CJS
  - `files`: 包含 `dist/` 与必要的 `gen/ts` 产物
  - `peerDependencies`: `@grpc/grpc-js`
  - `scripts`: `gen:ts`, `build`, `prepublishOnly`

发布流程：

```bash
pnpm run gen:ts
pnpm run build
npm publish --access public
```

版本语义化（SemVer），与服务端/Proto 版本对齐（例如 `v0.4.x` 对应 `proto v0.4`）。

---

### 兼容性与测试

- E2E：使用本仓库 `docker-compose` 启动服务端后进行端到端调用测试
- 负载：对大批量 `InsertVectors`、`Search` 进行压测；验证 keepalive 与压缩收益
- 断线重连：模拟服务端重启，验证客户端自动恢复


