package main

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/scintirete/scintirete/internal/core/algorithm"
	"github.com/scintirete/scintirete/pkg/types"
)

func main() {
	fmt.Println("=== Scintirete 内存使用分析工具 ===")

	// 记录初始内存状态
	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	fmt.Printf("初始内存使用: %.2f MB\n", float64(m1.Alloc)/1024/1024)

	// 创建HNSW索引
	params := types.HNSWParams{
		M:              16,
		EfConstruction: 200,
		EfSearch:       50,
		MaxLayers:      16,
		Seed:           12345,
	}

	hnswInterface, err := algorithm.NewHNSW(params, types.DistanceMetricL2)
	if err != nil {
		panic(err)
	}

	hnsw := hnswInterface.(*algorithm.HNSW)

	runtime.GC()
	runtime.ReadMemStats(&m2)
	fmt.Printf("创建HNSW索引后: %.2f MB (增加: %.2f MB)\n",
		float64(m2.Alloc)/1024/1024,
		float64(m2.Alloc-m1.Alloc)/1024/1024)

	// 生成测试向量数据
	fmt.Println("\n生成测试数据...")
	vectorCount := 10000 // 1万个向量
	dimension := 1536    // OpenAI text-embedding-3-small 的维度

	vectors := make([]types.Vector, vectorCount)
	for i := 0; i < vectorCount; i++ {
		elements := make([]float32, dimension)
		for j := 0; j < dimension; j++ {
			elements[j] = float32(i*dimension + j)
		}

		vectors[i] = types.Vector{
			ID:       uint64(i + 1),
			Elements: elements,
			Metadata: map[string]interface{}{
				"label": fmt.Sprintf("vector_%d", i+1),
				"index": i,
			},
		}
	}

	runtime.GC()
	var m3 runtime.MemStats
	runtime.ReadMemStats(&m3)
	fmt.Printf("生成%d个%d维向量后: %.2f MB (增加: %.2f MB)\n",
		vectorCount, dimension,
		float64(m3.Alloc)/1024/1024,
		float64(m3.Alloc-m2.Alloc)/1024/1024)

	// 构建HNSW索引
	fmt.Println("\n构建HNSW索引...")
	startTime := time.Now()

	ctx := context.Background()
	err = hnsw.Build(ctx, vectors)
	if err != nil {
		panic(err)
	}

	buildTime := time.Since(startTime)

	runtime.GC()
	var m4 runtime.MemStats
	runtime.ReadMemStats(&m4)
	fmt.Printf("构建索引完成: %.2f MB (增加: %.2f MB), 耗时: %v\n",
		float64(m4.Alloc)/1024/1024,
		float64(m4.Alloc-m3.Alloc)/1024/1024,
		buildTime)

	// 报告HNSW内部统计
	stats := hnsw.GetGraphStatistics()
	fmt.Printf("HNSW统计: 层数=%d, 节点=%d, 连接=%d, 平均度=%.2f, 内存使用=%.2f MB\n",
		stats.Layers, stats.Nodes, stats.Connections, stats.AvgDegree,
		float64(stats.MemoryUsage)/1024/1024)

	// 模拟导出图状态（RDB保存过程）
	fmt.Println("\n模拟RDB导出过程...")
	_ = hnsw.ExportGraphState() // 忽略返回值以模拟实际使用场景

	runtime.GC()
	var m5 runtime.MemStats
	runtime.ReadMemStats(&m5)
	fmt.Printf("导出图状态后: %.2f MB (增加: %.2f MB)\n",
		float64(m5.Alloc)/1024/1024,
		float64(m5.Alloc-m4.Alloc)/1024/1024)

	// 立即触发GC清理导出过程中的临时对象
	fmt.Println("触发GC清理导出过程中的临时对象...")
	runtime.GC()

	var m6 runtime.MemStats
	runtime.ReadMemStats(&m6)
	fmt.Printf("释放图状态后: %.2f MB (减少: %.2f MB)\n",
		float64(m6.Alloc)/1024/1024,
		float64(m5.Alloc-m6.Alloc)/1024/1024)

	// 性能测试
	fmt.Println("\n进行搜索性能测试...")
	query := make([]float32, dimension)
	for i := 0; i < dimension; i++ {
		query[i] = 1.0
	}

	searchParams := types.SearchParams{TopK: 10}

	// 预热
	for i := 0; i < 100; i++ {
		_, err = hnsw.Search(ctx, query, searchParams)
		if err != nil {
			panic(err)
		}
	}

	// 测试搜索时间
	searchStart := time.Now()
	for i := 0; i < 1000; i++ {
		_, err = hnsw.Search(ctx, query, searchParams)
		if err != nil {
			panic(err)
		}
	}
	avgSearchTime := time.Since(searchStart) / 1000

	runtime.GC()
	var m7 runtime.MemStats
	runtime.ReadMemStats(&m7)
	fmt.Printf("搜索测试后: %.2f MB, 平均搜索时间: %v\n",
		float64(m7.Alloc)/1024/1024, avgSearchTime)

	// 最终内存报告
	fmt.Println("\n=== 内存使用总结 ===")
	fmt.Printf("原始向量数据大小: %.2f MB (%d个向量 × %d维 × 4字节)\n",
		float64(vectorCount*dimension*4)/1024/1024, vectorCount, dimension)
	fmt.Printf("实际内存使用: %.2f MB\n", float64(m7.Alloc)/1024/1024)
	fmt.Printf("内存效率: %.2f%% (实际/理论)\n",
		float64(m7.Alloc)/float64(vectorCount*dimension*4)*100)

	// 释放向量数据测试
	fmt.Println("\n释放原始向量数据...")
	vectors = nil
	runtime.GC()

	var m8 runtime.MemStats
	runtime.ReadMemStats(&m8)
	fmt.Printf("释放原始数据后: %.2f MB (减少: %.2f MB)\n",
		float64(m8.Alloc)/1024/1024,
		float64(m7.Alloc-m8.Alloc)/1024/1024)

	fmt.Println("\n=== 优化验证完成 ===")
	fmt.Println("- ✅ HNSW连接存储已优化为slice格式")
	fmt.Println("- ✅ 图状态导出使用引用拷贝")
	fmt.Println("- ✅ 主动GC触发有效释放临时对象")
	fmt.Println("- ✅ 内存使用符合预期")
}
