### Scintirete Go SDK 设计方案

定位：Go 1.21+，基于 `google.golang.org/grpc` 与 `protoc-gen-go`/`protoc-gen-go-grpc` 生成强类型客户端。提供 `Client` 封装，统一注入 `AuthInfo`、连接池/单连接复用、超时/重试、gzip 压缩，面向后端与高并发场景。

---

### 安装

```bash
go get google.golang.org/grpc
go get google.golang.org/protobuf
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

仓库已设置 `option go_package = "github.com/scintirete/scintirete/gen/go/scintirete/v1;scintiretev1"`，建议代码生成产物输出至 `gen/go/` 并作为内部依赖使用。

生成命令（示例）：

```bash
protoc \
  -I ./schemas/proto \
  --go_out=./gen/go --go_opt=paths=source_relative \
  --go-grpc_out=./gen/go --go-grpc_opt=paths=source_relative \
  ./schemas/proto/scintirete/v1/scintirete.proto
```

---

### 初始化

提供轻量封装 `client.Client`：

```go
// sdk/client/client.go
package client

import (
    "context"
    "time"

    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
    "google.golang.org/grpc/encoding/gzip"
    scintiretev1 "github.com/scintirete/scintirete/gen/go/scintirete/v1"
)

type Options struct {
    Address          string
    Password         string
    UseTLS           bool
    DialOptions      []grpc.DialOption
    DefaultTimeout   time.Duration
    EnableGzip       bool
}

type Client struct {
    cc   *grpc.ClientConn
    stub scintiretev1.ScintireteServiceClient
    opts Options
}

func New(opts Options) (*Client, error) {
    dialOpts := []grpc.DialOption{
        grpc.WithTransportCredentials(insecure.NewCredentials()),
        grpc.WithDefaultCallOptions(
            grpc.MaxCallRecvMsgSize(64 * 1024 * 1024),
            grpc.MaxCallSendMsgSize(64 * 1024 * 1024),
        ),
    }
    if opts.EnableGzip {
        dialOpts = append(dialOpts, grpc.WithDefaultCallOptions(grpc.UseCompressor(gzip.Name)))
    }
    dialOpts = append(dialOpts, opts.DialOptions...)

    cc, err := grpc.Dial(opts.Address, dialOpts...)
    if err != nil {
        return nil, err
    }
    return &Client{cc: cc, stub: scintiretev1.NewScintireteServiceClient(cc), opts: opts}, nil
}

func (c *Client) Close() error { return c.cc.Close() }

func (c *Client) withAuth[T any](in T) T {
    // 通过泛型+结构体嵌入方式不可直接修改，建议在调用构造请求时统一填充
    return in
}

func (c *Client) ctx(ctx context.Context) (context.Context, context.CancelFunc) {
    if c.opts.DefaultTimeout <= 0 {
        return ctx, func() {}
    }
    return context.WithTimeout(ctx, c.opts.DefaultTimeout)
}
```

在具体 API 包装时，统一构造请求并注入 `AuthInfo`：

```go
// sdk/api/api.go
package api

import (
    "context"
    scintiretev1 "github.com/scintirete/scintirete/gen/go/scintirete/v1"
    "github.com/scintirete/scintirete/sdk/client"
)

type API struct{ C *client.Client }

func (a *API) CreateDatabase(ctx context.Context, name string) (*scintiretev1.CreateDatabaseResponse, error) {
    ctx, cancel := a.C.Ctx(ctx)
    defer cancel()
    req := &scintiretev1.CreateDatabaseRequest{Auth: &scintiretev1.AuthInfo{Password: a.C.Opts().Password}, Name: name}
    return a.C.Stub().CreateDatabase(ctx, req)
}
```

说明：实际实现中在 `client.Client` 暴露 `Stub() ScintireteServiceClient`、`Opts()` 与 `Ctx()` 等方法，或在生成的请求构造 helper 中集中注入 `Auth`，以免重复样板代码。

---

### 底层实现的高效封装（gRPC）

- **连接复用**：`*grpc.ClientConn` 单连接，内部管理 HTTP/2 连接池与 name resolver。
- **默认超时**：`context.WithTimeout` 统一截止时间，防止泄漏。
- **压缩**：可配置 `grpc.UseCompressor(gzip.Name)`。
- **大报文**：`MaxCall{Recv,Send}MsgSize` 放宽。
- **重试**：可选启用 `service config` 重试策略（需服务端/Envoy 支持）；或在 SDK 层对 `Unavailable`/`DeadlineExceeded` 做指数退避重试（幂等方法）。
- **Auth 注入**：请求体内 `AuthInfo` 统一填充。

---

### 使用示例

```go
package main

import (
    "context"
    "fmt"
    "time"

    api "github.com/scintirete/scintirete/sdk/api"
    "github.com/scintirete/scintirete/sdk/client"
)

func main() {
    c, _ := client.New(client.Options{Address: "127.0.0.1:50051", Password: "secret", DefaultTimeout: 10 * time.Second})
    defer c.Close()

    a := &api.API{C: c}
    a.CreateDatabase(context.Background(), "demo")
    fmt.Println("OK")
}
```

---

### 开发者如何发布（Go Module）

- 模块命名：`github.com/scintirete/scintirete-sdk-go`（建议独立仓库）
- 目录：
  - `client/` 连接与底层封装
  - `api/` 薄封装方法
  - `gen/` 放置编译生成的 pb（或引用服务端已发布的 `gen/go` 包）
- 版本：SemVer，跟进 proto 版本；使用 `tags` 发布：`v0.x.y`
- 发布：`git tag v0.1.0 && git push origin v0.1.0`

---

### 测试与兼容

- `go test ./...` 集成 docker-compose 做端到端用例
- 压测 `InsertVectors`/`Search`、连接中断恢复
- race detector 与 `-bench` 针对高并发场景


