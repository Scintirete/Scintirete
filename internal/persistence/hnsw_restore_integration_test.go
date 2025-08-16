package persistence

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/scintirete/scintirete/internal/core/database"
	"github.com/scintirete/scintirete/internal/persistence/rdb"
	"github.com/scintirete/scintirete/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHNSWGraphRestoreIntegration(t *testing.T) {
	// 创建临时目录
	tempDir := t.TempDir()
	rdbPath := filepath.Join(tempDir, "test.rdb")

	// 步骤1: 创建数据库引擎并添加数据
	engine := database.NewEngine()
	ctx := context.Background()

	// 创建数据库
	err := engine.CreateDatabase(ctx, "testdb")
	require.NoError(t, err)

	db, err := engine.GetDatabase(ctx, "testdb")
	require.NoError(t, err)

	// 创建集合
	config := types.CollectionConfig{
		Name:   "testcoll",
		Metric: types.DistanceMetricL2,
		HNSWParams: types.HNSWParams{
			M:              8,
			EfConstruction: 100,
			EfSearch:       50,
			MaxLayers:      16,
			Seed:           12345,
		},
	}

	err = db.CreateCollection(ctx, config)
	require.NoError(t, err)

	collection, err := db.GetCollection(ctx, "testcoll")
	require.NoError(t, err)

	// 添加测试向量
	vectors := []types.Vector{
		{ID: 1, Elements: []float32{1.0, 2.0, 3.0}, Metadata: map[string]interface{}{"category": "A"}},
		{ID: 2, Elements: []float32{4.0, 5.0, 6.0}, Metadata: map[string]interface{}{"category": "B"}},
		{ID: 3, Elements: []float32{7.0, 8.0, 9.0}, Metadata: map[string]interface{}{"category": "A"}},
		{ID: 4, Elements: []float32{10.0, 11.0, 12.0}, Metadata: map[string]interface{}{"category": "C"}},
		{ID: 5, Elements: []float32{13.0, 14.0, 15.0}, Metadata: map[string]interface{}{"category": "B"}},
	}

	err = collection.Insert(ctx, vectors)
	require.NoError(t, err)

	// 验证数据已正确插入
	count, err := collection.Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(5), count)

	// 执行搜索以验证索引工作正常
	query := []float32{1.1, 2.1, 3.1}
	searchParams := types.SearchParams{TopK: 3}
	originalResults, err := collection.Search(ctx, query, searchParams)
	require.NoError(t, err)
	require.Len(t, originalResults, 3)

	// 获取HNSW统计信息（用于后续比较）
	t.Logf("Original data inserted successfully with %d vectors", len(vectors))

	// 步骤2: 保存RDB快照
	rdbManager, err := rdb.NewRDBManager(rdbPath)
	require.NoError(t, err)

	// 获取数据库状态
	databaseStates, err := engine.GetDatabaseState(ctx)
	require.NoError(t, err)

	// 转换并保存
	snapshot := rdbManager.CreateSnapshot(databaseStates)
	err = rdbManager.Save(ctx, snapshot)
	require.NoError(t, err)

	// 验证RDB文件已创建且包含HNSW图数据
	info, err := rdbManager.GetInfo()
	require.NoError(t, err)
	assert.True(t, info.Exists)
	assert.Greater(t, info.Size, int64(0))

	// 步骤3: 模拟服务器重启 - 创建新的引擎
	newEngine := database.NewEngine()

	// 加载RDB快照
	loadedSnapshot, err := rdbManager.Load(ctx)
	require.NoError(t, err)
	require.NotNil(t, loadedSnapshot)

	// 验证快照包含HNSW图数据
	dbSnapshot, exists := loadedSnapshot.Databases["testdb"]
	require.True(t, exists)
	collSnapshot, exists := dbSnapshot.Collections["testcoll"]
	require.True(t, exists)
	require.NotNil(t, collSnapshot.HNSWGraph, "HNSW graph should be included in snapshot")

	t.Logf("Loaded HNSW graph: EntryPoint=%s, MaxLayer=%d, Size=%d, Nodes=%d",
		collSnapshot.HNSWGraph.EntryPointID, collSnapshot.HNSWGraph.MaxLayer,
		collSnapshot.HNSWGraph.Size, len(collSnapshot.HNSWGraph.Nodes))

	// 步骤4: 恢复数据库状态
	err = newEngine.RestoreFromSnapshot(ctx, loadedSnapshot)
	require.NoError(t, err)

	// 步骤5: 验证恢复后的数据库状态
	newDb, err := newEngine.GetDatabase(ctx, "testdb")
	require.NoError(t, err)

	newCollection, err := newDb.GetCollection(ctx, "testcoll")
	require.NoError(t, err)

	// 验证向量数量
	newCount, err := newCollection.Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(5), newCount)

	// 验证搜索功能（关键测试：应该不需要重建索引）
	newResults, err := newCollection.Search(ctx, query, searchParams)
	require.NoError(t, err)
	require.Len(t, newResults, 3)

	// 验证搜索结果一致性
	assert.Equal(t, len(originalResults), len(newResults))
	for i, originalResult := range originalResults {
		assert.Equal(t, originalResult.Vector.ID, newResults[i].Vector.ID)
		assert.InDelta(t, originalResult.Distance, newResults[i].Distance, 0.001)
	}

	// 验证向量检索功能
	for _, originalVector := range vectors {
		retrievedVector, err := newCollection.Get(ctx, string(rune(originalVector.ID+'0')))
		require.NoError(t, err)
		assert.Equal(t, originalVector.ID, retrievedVector.ID)
		assert.Equal(t, originalVector.Elements, retrievedVector.Elements)
		assert.Equal(t, originalVector.Metadata, retrievedVector.Metadata)
	}

	// 验证HNSW图结构一致性
	t.Logf("HNSW graph restored successfully - search functionality verified")

	// 步骤6: 测试新增向量功能（验证恢复后的索引可以继续工作）
	newVector := types.Vector{
		ID:       6,
		Elements: []float32{16.0, 17.0, 18.0},
		Metadata: map[string]interface{}{"category": "D"},
	}

	err = newCollection.Insert(ctx, []types.Vector{newVector})
	require.NoError(t, err)

	finalCount, err := newCollection.Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(6), finalCount)

	// 验证新向量可以被搜索到
	newVectorResults, err := newCollection.Search(ctx, []float32{16.1, 17.1, 18.1}, types.SearchParams{TopK: 1})
	require.NoError(t, err)
	require.Len(t, newVectorResults, 1)
	assert.Equal(t, uint64(6), newVectorResults[0].Vector.ID)
}

func TestHNSWGraphRestorePerformance(t *testing.T) {
	// 这个测试用于比较传统重建和图状态导入的性能差异
	tempDir := t.TempDir()
	rdbPath := filepath.Join(tempDir, "perf_test.rdb")

	// 创建更大的数据集
	engine := database.NewEngine()
	ctx := context.Background()

	err := engine.CreateDatabase(ctx, "perfdb")
	require.NoError(t, err)

	db, err := engine.GetDatabase(ctx, "perfdb")
	require.NoError(t, err)

	config := types.CollectionConfig{
		Name:   "perfcoll",
		Metric: types.DistanceMetricL2,
		HNSWParams: types.HNSWParams{
			M:              16,
			EfConstruction: 200,
			EfSearch:       50,
			MaxLayers:      16,
			Seed:           12345,
		},
	}

	err = db.CreateCollection(ctx, config)
	require.NoError(t, err)

	collection, err := db.GetCollection(ctx, "perfcoll")
	require.NoError(t, err)

	// 生成较大的向量数据集
	vectorCount := 100 // 在测试中使用适中的数量
	vectors := make([]types.Vector, vectorCount)
	for i := 0; i < vectorCount; i++ {
		vectors[i] = types.Vector{
			ID:       uint64(i + 1),
			Elements: []float32{float32(i), float32(i + 1), float32(i + 2)},
			Metadata: map[string]interface{}{"index": i},
		}
	}

	// 测量插入时间
	insertStart := time.Now()
	err = collection.Insert(ctx, vectors)
	require.NoError(t, err)
	insertDuration := time.Since(insertStart)
	t.Logf("Initial insert took: %v", insertDuration)

	// 保存快照
	rdbManager, err := rdb.NewRDBManager(rdbPath)
	require.NoError(t, err)

	databaseStates, err := engine.GetDatabaseState(ctx)
	require.NoError(t, err)

	snapshot := rdbManager.CreateSnapshot(databaseStates)

	saveStart := time.Now()
	err = rdbManager.Save(ctx, snapshot)
	require.NoError(t, err)
	saveDuration := time.Since(saveStart)
	t.Logf("Save snapshot took: %v", saveDuration)

	// 测量恢复时间（使用图状态导入）
	newEngine := database.NewEngine()

	loadStart := time.Now()
	loadedSnapshot, err := rdbManager.Load(ctx)
	require.NoError(t, err)
	loadDuration := time.Since(loadStart)
	t.Logf("Load snapshot took: %v", loadDuration)

	restoreStart := time.Now()
	err = newEngine.RestoreFromSnapshot(ctx, loadedSnapshot)
	require.NoError(t, err)
	restoreDuration := time.Since(restoreStart)
	t.Logf("Restore with graph import took: %v", restoreDuration)

	// 验证恢复正确性
	newDb, err := newEngine.GetDatabase(ctx, "perfdb")
	require.NoError(t, err)

	newCollection, err := newDb.GetCollection(ctx, "perfcoll")
	require.NoError(t, err)

	count, err := newCollection.Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(vectorCount), count)

	// 测试搜索性能
	query := []float32{50.0, 51.0, 52.0}
	searchStart := time.Now()
	results, err := newCollection.Search(ctx, query, types.SearchParams{TopK: 10})
	require.NoError(t, err)
	searchDuration := time.Since(searchStart)
	t.Logf("Search took: %v", searchDuration)

	assert.Len(t, results, 10)

	// 记录总体性能指标
	totalRestoreTime := loadDuration + restoreDuration
	t.Logf("Total restore time (load + restore): %v", totalRestoreTime)

	// 在实际应用中，这应该比重建索引快得多
	// 这里我们只记录时间，不做具体断言，因为在小数据集上差异可能不明显
	t.Logf("Performance baseline established - restore should be faster than rebuild for larger datasets")
}

func TestNoHNSWGraphStateError(t *testing.T) {
	// 测试当RDB中没有HNSW图数据时应该返回错误
	tempDir := t.TempDir()
	rdbPath := filepath.Join(tempDir, "no_graph_test.rdb")

	// 创建一个没有HNSW图数据的快照
	snapshot := rdb.RDBSnapshot{
		Version:   "1.0",
		Timestamp: time.Now(),
		Databases: map[string]rdb.DatabaseSnapshot{
			"testdb": {
				Name:      "testdb",
				CreatedAt: time.Now(),
				Collections: map[string]rdb.CollectionSnapshot{
					"testcoll": {
						Name: "testcoll",
						Config: types.CollectionConfig{
							Name:   "testcoll",
							Metric: types.DistanceMetricL2,
							HNSWParams: types.HNSWParams{
								M:              16,
								EfConstruction: 200,
								EfSearch:       50,
								MaxLayers:      16,
								Seed:           12345,
							},
						},
						Vectors: []types.Vector{
							{ID: 1, Elements: []float32{1.0, 2.0, 3.0}},
							{ID: 2, Elements: []float32{4.0, 5.0, 6.0}},
						},
						HNSWGraph:    nil, // 没有图数据
						VectorCount:  2,
						DeletedCount: 0,
						CreatedAt:    time.Now(),
						UpdatedAt:    time.Now(),
					},
				},
			},
		},
	}

	// 保存快照
	rdbManager, err := rdb.NewRDBManager(rdbPath)
	require.NoError(t, err)

	err = rdbManager.Save(context.Background(), snapshot)
	require.NoError(t, err)

	// 创建新引擎并尝试恢复
	engine := database.NewEngine()
	ctx := context.Background()

	loadedSnapshot, err := rdbManager.Load(ctx)
	require.NoError(t, err)

	// 现在应该返回错误，因为没有HNSW图数据
	err = engine.RestoreFromSnapshot(ctx, loadedSnapshot)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HNSW graph state missing")
}

// 注意：这个测试依赖于Collection的内部实现
// 在实际实现中，可以考虑为Collection添加测试友好的方法
