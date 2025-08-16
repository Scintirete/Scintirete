# 性能分析工具集成

## 概述

为 Scintirete 项目集成了完整的性能分析工具链，包括 pprof 和 go tool trace，以便进行深入的性能分析和优化。

## 集成的工具

### 1. pprof 集成
- **位置**: `cmd/scintirete-server/main.go`
- **功能**: CPU、内存、goroutine、mutex、block 等性能分析
- **启用方式**: 
  ```bash
  ./bin/scintirete-server -pprof=true -pprof-port=6060
  ```
- **访问地址**: `http://localhost:6060/debug/pprof/`

### 2. go tool trace 集成
- **功能**: 系统调用、goroutine 调度、GC 等跟踪分析
- **启用方式**:
  ```bash
  ./bin/scintirete-server -trace=trace.out
  ```
- **分析命令**: `go tool trace trace.out`

### 3. 自动化分析脚本
- **脚本路径**: `scripts/performance_analysis.sh`
- **功能**: 
  - 自动构建项目
  - 启动服务器并收集性能数据
  - 生成分析报告
  - 清理旧文件

## 命令行参数

新增的服务器命令行参数：

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `-pprof` | bool | false | 启用 pprof 服务器 |
| `-pprof-port` | int | 6060 | pprof 服务器端口 |
| `-trace` | string | "" | 启用跟踪并写入文件 |

## 使用方法

### 快速性能分析
```bash
# 运行 30 秒分析
./scripts/performance_analysis.sh -d 30

# 运行 2 分钟分析
./scripts/performance_analysis.sh -d 120

# 同时运行负载测试
./scripts/performance_analysis.sh -l
```

### 手动性能分析
```bash
# 1. 启动服务器
./bin/scintirete-server -pprof=true -trace=trace.out &

# 2. 收集 CPU profile
curl http://localhost:6060/debug/pprof/profile?seconds=30 -o cpu.prof

# 3. 收集内存 profile
curl http://localhost:6060/debug/pprof/heap -o heap.prof

# 4. 分析数据
go tool pprof cpu.prof
go tool trace trace.out
```

### Web 界面分析
```bash
# CPU 分析 Web 界面
go tool pprof -http=:8081 cpu.prof

# 内存分析 Web 界面
go tool pprof -http=:8082 heap.prof
```

## 分析报告输出

脚本会在 `performance_analysis/` 目录生成：

1. **Profile 文件**:
   - `cpu_*.prof` - CPU 性能数据
   - `heap_*.prof` - 内存使用数据
   - `goroutine_*.prof` - Goroutine 状态
   - `mutex_*.prof` - 互斥锁竞争
   - `block_*.prof` - 阻塞操作

2. **Trace 文件**:
   - `trace_*.out` - 系统跟踪数据

3. **分析报告**:
   - `performance_report_*.md` - 自动生成的报告
   - `detailed_analysis_report.md` - 详细分析结果

## 性能分析最佳实践

### 1. 数据收集
- **生产环境**: 使用较短的采样时间（10-30秒）
- **测试环境**: 可以使用较长的采样时间（60-300秒）
- **负载测试**: 在有代表性负载下收集数据

### 2. 分析重点
- **CPU**: 查找热点函数和不必要的计算
- **内存**: 识别内存泄漏和高分配点
- **Goroutine**: 检查并发控制和泄漏
- **I/O**: 分析磁盘和网络操作效率

### 3. 优化策略
- **算法优化**: 针对 CPU 热点进行算法改进
- **内存优化**: 减少分配和提高重用
- **并发优化**: 调整 goroutine 数量和同步机制
- **I/O 优化**: 批量操作和异步处理

## 当前分析结果摘要

基于 `configs/scintirete.toml` 的分析结果：

### ✅ 良好表现
- 无 CPU 空转情况
- Goroutine 使用健康（8个）
- 配置参数适合 2C2G 环境
- AOF "no" 策略有效减少 I/O 阻塞

### ⚠️ 需要关注
- 向量距离计算占用 71% CPU 时间
- RDB 恢复时内存峰值约 158MB
- ef_construction=200 可能偏高

### 💡 优化建议
- 调整 ef_construction 从 200 到 150
- 考虑 SIMD 优化向量计算
- 实现流式 RDB 加载
- 持续监控内存使用

## 文件清理

脚本会自动清理 7 天前的分析文件：
```bash
./scripts/performance_analysis.sh --cleanup
```

## 故障排除

### 常见问题

1. **pprof 端口被占用**:
   ```bash
   # 更换端口
   ./bin/scintirete-server -pprof-port=6061
   ```

2. **trace 文件过大**:
   ```bash
   # 减少跟踪时间
   ./scripts/performance_analysis.sh -d 15
   ```

3. **内存不足**:
   ```bash
   # 设置内存限制
   export GOMEMLIMIT=1GB
   ```

## 集成历史

- **2025-08-16**: 初始集成 pprof 和 trace 支持
- **2025-08-16**: 创建自动化分析脚本
- **2025-08-16**: 完成当前配置性能基线分析
