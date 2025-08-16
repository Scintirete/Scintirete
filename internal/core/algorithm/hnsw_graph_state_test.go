package algorithm

import (
	"context"
	"testing"

	"github.com/scintirete/scintirete/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHNSW_ExportImportGraphState(t *testing.T) {
	// 创建测试HNSW索引
	params := types.HNSWParams{
		M:              16,
		EfConstruction: 200,
		EfSearch:       50,
		MaxLayers:      16,
		Seed:           12345,
	}

	hnswInterface, err := NewHNSW(params, types.DistanceMetricL2)
	require.NoError(t, err)

	hnsw := hnswInterface.(*HNSW)

	ctx := context.Background()

	// 准备测试向量
	vectors := []types.Vector{
		{ID: 1, Elements: []float32{1.0, 2.0, 3.0}, Metadata: map[string]interface{}{"label": "vector1"}},
		{ID: 2, Elements: []float32{4.0, 5.0, 6.0}, Metadata: map[string]interface{}{"label": "vector2"}},
		{ID: 3, Elements: []float32{7.0, 8.0, 9.0}, Metadata: map[string]interface{}{"label": "vector3"}},
		{ID: 4, Elements: []float32{10.0, 11.0, 12.0}, Metadata: map[string]interface{}{"label": "vector4"}},
		{ID: 5, Elements: []float32{13.0, 14.0, 15.0}, Metadata: map[string]interface{}{"label": "vector5"}},
	}

	// 构建索引
	err = hnsw.Build(ctx, vectors)
	require.NoError(t, err)

	// 验证索引基本状态
	assert.Equal(t, 5, hnsw.Size())
	assert.Greater(t, hnsw.MemoryUsage(), int64(0))

	// 获取原始统计信息
	originalStats := hnsw.GetGraphStatistics()

	// 导出图状态
	graphState := hnsw.ExportGraphState()

	// 验证导出状态的完整性
	assert.Equal(t, 5, graphState.Size)
	assert.Equal(t, 5, len(graphState.Nodes))
	assert.Greater(t, graphState.MaxLayer, -1)
	assert.NotEqual(t, uint64(0), graphState.EntryPoint)

	// 验证节点数据
	for id, nodeState := range graphState.Nodes {
		assert.Equal(t, id, nodeState.ID)
		assert.NotNil(t, nodeState.Vector)
		assert.NotNil(t, nodeState.Connections)
		assert.False(t, nodeState.Deleted)

		// 验证向量数据
		originalVector := findVectorByID(vectors, id)
		require.NotNil(t, originalVector)
		assert.Equal(t, originalVector.Elements, nodeState.Vector)
		assert.Equal(t, originalVector.Metadata, nodeState.Metadata)
	}

	// 创建新的HNSW索引并导入状态
	newHnswInterface, err := NewHNSW(params, types.DistanceMetricL2)
	require.NoError(t, err)

	newHnsw := newHnswInterface.(*HNSW)

	// 导入图状态
	err = newHnsw.ImportGraphState(graphState)
	require.NoError(t, err)

	// 验证导入后的索引状态
	assert.Equal(t, hnsw.Size(), newHnsw.Size())
	assert.Equal(t, hnsw.GetLayers(), newHnsw.GetLayers())

	// 验证统计信息一致
	newStats := newHnsw.GetGraphStatistics()
	assert.Equal(t, originalStats.Nodes, newStats.Nodes)
	assert.Equal(t, originalStats.Layers, newStats.Layers)
	assert.Equal(t, originalStats.Connections, newStats.Connections)

	// 验证搜索功能正常
	query := []float32{1.1, 2.1, 3.1}
	searchParams := types.SearchParams{TopK: 3}

	originalResults, err := hnsw.Search(ctx, query, searchParams)
	require.NoError(t, err)

	newResults, err := newHnsw.Search(ctx, query, searchParams)
	require.NoError(t, err)

	// 验证搜索结果一致
	assert.Equal(t, len(originalResults), len(newResults))
	for i := range originalResults {
		assert.Equal(t, originalResults[i].Vector.ID, newResults[i].Vector.ID)
		assert.InDelta(t, originalResults[i].Distance, newResults[i].Distance, 0.001)
	}

	// 验证向量检索功能
	for _, vector := range vectors {
		originalVec, err := hnsw.Get(ctx, string(rune(vector.ID+'0')))
		require.NoError(t, err)

		newVec, err := newHnsw.Get(ctx, string(rune(vector.ID+'0')))
		require.NoError(t, err)

		assert.Equal(t, originalVec.ID, newVec.ID)
		assert.Equal(t, originalVec.Elements, newVec.Elements)
		assert.Equal(t, originalVec.Metadata, newVec.Metadata)
	}
}

func TestHNSW_EmptyGraphState(t *testing.T) {
	params := types.HNSWParams{
		M:              16,
		EfConstruction: 200,
		EfSearch:       50,
		MaxLayers:      16,
		Seed:           12345,
	}

	hnswInterface, err := NewHNSW(params, types.DistanceMetricL2)
	require.NoError(t, err)

	hnsw := hnswInterface.(*HNSW)

	// 导出空图状态
	graphState := hnsw.ExportGraphState()

	// 验证空状态
	assert.Equal(t, 0, graphState.Size)
	assert.Equal(t, 0, len(graphState.Nodes))
	assert.Equal(t, -1, graphState.MaxLayer)
	assert.Equal(t, uint64(0), graphState.EntryPoint)

	// 创建新索引并导入空状态
	newHnswInterface, err := NewHNSW(params, types.DistanceMetricL2)
	require.NoError(t, err)

	newHnsw := newHnswInterface.(*HNSW)

	err = newHnsw.ImportGraphState(graphState)
	require.NoError(t, err)

	// 验证导入后仍为空
	assert.Equal(t, 0, newHnsw.Size())
	assert.Equal(t, 0, newHnsw.GetLayers())
}

func TestHNSW_GraphStateDeepCopy(t *testing.T) {
	params := types.HNSWParams{
		M:              8,
		EfConstruction: 100,
		EfSearch:       50,
		MaxLayers:      16,
		Seed:           12345,
	}

	hnswInterface, err := NewHNSW(params, types.DistanceMetricL2)
	require.NoError(t, err)

	hnsw := hnswInterface.(*HNSW)

	ctx := context.Background()

	// 添加一个向量
	vector := types.Vector{
		ID:       1,
		Elements: []float32{1.0, 2.0, 3.0},
		Metadata: map[string]interface{}{"test": "value"},
	}

	err = hnsw.Insert(ctx, vector)
	require.NoError(t, err)

	// 导出状态
	graphState := hnsw.ExportGraphState()

	// 修改导出状态的数据
	for _, nodeState := range graphState.Nodes {
		nodeState.Vector[0] = 999.0             // 修改向量数据
		nodeState.Metadata["test"] = "modified" // 修改元数据
	}

	// 验证原始索引未被影响
	originalVec, err := hnsw.Get(ctx, "1")
	require.NoError(t, err)
	assert.Equal(t, float32(1.0), originalVec.Elements[0])
	assert.Equal(t, "value", originalVec.Metadata["test"])
}

func TestHNSW_SingleNodeGraphState(t *testing.T) {
	params := types.HNSWParams{
		M:              16,
		EfConstruction: 200,
		EfSearch:       50,
		MaxLayers:      16,
		Seed:           12345,
	}

	hnswInterface, err := NewHNSW(params, types.DistanceMetricL2)
	require.NoError(t, err)

	hnsw := hnswInterface.(*HNSW)

	ctx := context.Background()

	// 添加单个向量
	vector := types.Vector{
		ID:       1,
		Elements: []float32{1.0, 2.0, 3.0},
		Metadata: map[string]interface{}{"single": "node"},
	}

	err = hnsw.Insert(ctx, vector)
	require.NoError(t, err)

	// 导出并导入
	graphState := hnsw.ExportGraphState()

	newHnswInterface, err := NewHNSW(params, types.DistanceMetricL2)
	require.NoError(t, err)

	newHnsw := newHnswInterface.(*HNSW)

	err = newHnsw.ImportGraphState(graphState)
	require.NoError(t, err)

	// 验证单节点情况
	assert.Equal(t, 1, newHnsw.Size())

	vec, err := newHnsw.Get(ctx, "1")
	require.NoError(t, err)
	assert.Equal(t, vector.Elements, vec.Elements)
	assert.Equal(t, vector.Metadata, vec.Metadata)

	// 验证搜索功能
	results, err := newHnsw.Search(ctx, []float32{1.1, 2.1, 3.1}, types.SearchParams{TopK: 1})
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, uint64(1), results[0].Vector.ID)
}

// 辅助函数
func findVectorByID(vectors []types.Vector, id uint64) *types.Vector {
	for _, v := range vectors {
		if v.ID == id {
			return &v
		}
	}
	return nil
}
