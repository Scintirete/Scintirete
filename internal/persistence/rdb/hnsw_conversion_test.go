package rdb

import (
	"testing"

	"github.com/scintirete/scintirete/internal/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertHNSWGraphState(t *testing.T) {
	// 创建测试的core.HNSWGraphState
	graphState := &core.HNSWGraphState{
		Nodes: map[uint64]*core.HNSWNodeState{
			1: {
				ID:       1,
				Vector:   []float32{1.0, 2.0, 3.0},
				Metadata: map[string]interface{}{"label": "node1"},
				Deleted:  false,
				Connections: [][]uint64{
					{2}, // Layer 0
					{},  // Layer 1
				},
			},
			2: {
				ID:       2,
				Vector:   []float32{4.0, 5.0, 6.0},
				Metadata: map[string]interface{}{"label": "node2"},
				Deleted:  false,
				Connections: [][]uint64{
					{1, 3}, // Layer 0
				},
			},
			3: {
				ID:       3,
				Vector:   []float32{7.0, 8.0, 9.0},
				Metadata: map[string]interface{}{"label": "node3"},
				Deleted:  false,
				Connections: [][]uint64{
					{2}, // Layer 0
				},
			},
		},
		EntryPoint: 1,
		MaxLayer:   1,
		Size:       3,
	}

	// 转换为RDB格式
	hnswSnapshot := ConvertHNSWGraphState(graphState)
	require.NotNil(t, hnswSnapshot)

	// 验证基本信息
	assert.Equal(t, "1", hnswSnapshot.EntryPointID)
	assert.Equal(t, 1, hnswSnapshot.MaxLayer)
	assert.Equal(t, 3, hnswSnapshot.Size)
	assert.Len(t, hnswSnapshot.Nodes, 3)

	// 验证节点数据
	nodeMap := make(map[string]HNSWNodeSnapshot)
	for _, node := range hnswSnapshot.Nodes {
		nodeMap[node.ID] = node
	}

	// 验证节点1
	node1, exists := nodeMap["1"]
	require.True(t, exists)
	assert.Equal(t, []float32{1.0, 2.0, 3.0}, node1.Elements)
	assert.Equal(t, map[string]interface{}{"label": "node1"}, node1.Metadata)
	assert.False(t, node1.Deleted)
	assert.Equal(t, 1, node1.MaxLayer)
	assert.Len(t, node1.LayerConnections, 1) // 只有一个有连接的层

	// 验证层连接
	layer0 := node1.LayerConnections[0]
	assert.Equal(t, 0, layer0.Layer)
	assert.Equal(t, []string{"2"}, layer0.ConnectedNodeIDs)

	// 验证节点2
	node2, exists := nodeMap["2"]
	require.True(t, exists)
	assert.Equal(t, []float32{4.0, 5.0, 6.0}, node2.Elements)
	assert.Len(t, node2.LayerConnections, 1)
	layer0 = node2.LayerConnections[0]
	assert.Equal(t, 0, layer0.Layer)
	assert.ElementsMatch(t, []string{"1", "3"}, layer0.ConnectedNodeIDs)

	// 验证节点3
	node3, exists := nodeMap["3"]
	require.True(t, exists)
	assert.Equal(t, []float32{7.0, 8.0, 9.0}, node3.Elements)
	assert.Len(t, node3.LayerConnections, 1)
	layer0 = node3.LayerConnections[0]
	assert.Equal(t, 0, layer0.Layer)
	assert.Equal(t, []string{"2"}, layer0.ConnectedNodeIDs)
}

func TestConvertHNSWGraphSnapshot(t *testing.T) {
	// 创建测试的HNSWGraphSnapshot
	hnswSnapshot := &HNSWGraphSnapshot{
		Nodes: []HNSWNodeSnapshot{
			{
				ID:       "1",
				Elements: []float32{1.0, 2.0, 3.0},
				Metadata: map[string]interface{}{"label": "node1"},
				Deleted:  false,
				MaxLayer: 1,
				LayerConnections: []LayerConnectionsSnapshot{
					{
						Layer:            0,
						ConnectedNodeIDs: []string{"2"},
					},
				},
			},
			{
				ID:       "2",
				Elements: []float32{4.0, 5.0, 6.0},
				Metadata: map[string]interface{}{"label": "node2"},
				Deleted:  false,
				MaxLayer: 0,
				LayerConnections: []LayerConnectionsSnapshot{
					{
						Layer:            0,
						ConnectedNodeIDs: []string{"1", "3"},
					},
				},
			},
			{
				ID:       "3",
				Elements: []float32{7.0, 8.0, 9.0},
				Metadata: map[string]interface{}{"label": "node3"},
				Deleted:  false,
				MaxLayer: 0,
				LayerConnections: []LayerConnectionsSnapshot{
					{
						Layer:            0,
						ConnectedNodeIDs: []string{"2"},
					},
				},
			},
		},
		EntryPointID: "1",
		MaxLayer:     1,
		Size:         3,
	}

	// 转换为core格式
	graphState, err := ConvertHNSWGraphSnapshot(hnswSnapshot)
	require.NoError(t, err)
	require.NotNil(t, graphState)

	// 验证基本信息
	assert.Equal(t, uint64(1), graphState.EntryPoint)
	assert.Equal(t, 1, graphState.MaxLayer)
	assert.Equal(t, 3, graphState.Size)
	assert.Len(t, graphState.Nodes, 3)

	// 验证节点数据
	node1, exists := graphState.Nodes[1]
	require.True(t, exists)
	assert.Equal(t, uint64(1), node1.ID)
	assert.Equal(t, []float32{1.0, 2.0, 3.0}, node1.Vector)
	assert.Equal(t, map[string]interface{}{"label": "node1"}, node1.Metadata)
	assert.False(t, node1.Deleted)

	// 验证连接结构
	assert.Len(t, node1.Connections, 2)    // MaxLayer + 1
	assert.Len(t, node1.Connections[0], 1) // Layer 0 有一个连接
	assert.Len(t, node1.Connections[1], 0) // Layer 1 没有连接
	// 验证layer 0的连接包含节点2
	assert.Contains(t, node1.Connections[0], uint64(2))

	// 验证节点2
	node2, exists := graphState.Nodes[2]
	require.True(t, exists)
	assert.Equal(t, uint64(2), node2.ID)
	assert.Len(t, node2.Connections[0], 2) // Layer 0 有两个连接
	// 验证layer 0的连接包含节点1和3
	assert.Contains(t, node2.Connections[0], uint64(1))
	assert.Contains(t, node2.Connections[0], uint64(3))

	// 验证节点3
	node3, exists := graphState.Nodes[3]
	require.True(t, exists)
	assert.Equal(t, uint64(3), node3.ID)
	assert.Len(t, node3.Connections[0], 1) // Layer 0 有一个连接
	// 验证layer 0的连接包含节点2
	assert.Contains(t, node3.Connections[0], uint64(2))
}

func TestConvertHNSWGraphState_Nil(t *testing.T) {
	result := ConvertHNSWGraphState(nil)
	assert.Nil(t, result)
}

func TestConvertHNSWGraphSnapshot_Nil(t *testing.T) {
	result, err := ConvertHNSWGraphSnapshot(nil)
	assert.NoError(t, err)
	assert.Nil(t, result)
}

func TestConvertHNSWGraphState_EmptyGraph(t *testing.T) {
	graphState := &core.HNSWGraphState{
		Nodes:      make(map[uint64]*core.HNSWNodeState),
		EntryPoint: 0,
		MaxLayer:   -1,
		Size:       0,
	}

	hnswSnapshot := ConvertHNSWGraphState(graphState)
	require.NotNil(t, hnswSnapshot)

	assert.Equal(t, "0", hnswSnapshot.EntryPointID)
	assert.Equal(t, -1, hnswSnapshot.MaxLayer)
	assert.Equal(t, 0, hnswSnapshot.Size)
	assert.Len(t, hnswSnapshot.Nodes, 0)
}

func TestConvertHNSWGraphSnapshot_EmptyGraph(t *testing.T) {
	hnswSnapshot := &HNSWGraphSnapshot{
		Nodes:        []HNSWNodeSnapshot{},
		EntryPointID: "0",
		MaxLayer:     -1,
		Size:         0,
	}

	graphState, err := ConvertHNSWGraphSnapshot(hnswSnapshot)
	require.NoError(t, err)
	require.NotNil(t, graphState)

	assert.Equal(t, uint64(0), graphState.EntryPoint)
	assert.Equal(t, -1, graphState.MaxLayer)
	assert.Equal(t, 0, graphState.Size)
	assert.Len(t, graphState.Nodes, 0)
}

func TestConvertHNSWGraphSnapshot_InvalidNodeID(t *testing.T) {
	hnswSnapshot := &HNSWGraphSnapshot{
		Nodes: []HNSWNodeSnapshot{
			{
				ID:               "invalid_id",
				Elements:         []float32{1.0, 2.0, 3.0},
				Metadata:         map[string]interface{}{},
				Deleted:          false,
				MaxLayer:         0,
				LayerConnections: []LayerConnectionsSnapshot{},
			},
		},
		EntryPointID: "1",
		MaxLayer:     0,
		Size:         1,
	}

	_, err := ConvertHNSWGraphSnapshot(hnswSnapshot)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse node ID")
}

func TestConvertHNSWGraphSnapshot_InvalidEntryPointID(t *testing.T) {
	hnswSnapshot := &HNSWGraphSnapshot{
		Nodes:        []HNSWNodeSnapshot{},
		EntryPointID: "invalid_entry_point",
		MaxLayer:     0,
		Size:         0,
	}

	_, err := ConvertHNSWGraphSnapshot(hnswSnapshot)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse entry point ID")
}

func TestRoundTripConversion(t *testing.T) {
	// 创建原始图状态
	originalState := &core.HNSWGraphState{
		Nodes: map[uint64]*core.HNSWNodeState{
			10: {
				ID:       10,
				Vector:   []float32{10.0, 20.0, 30.0},
				Metadata: map[string]interface{}{"type": "test", "value": 42},
				Deleted:  false,
				Connections: [][]uint64{
					{20, 30}, // Layer 0
					{20},     // Layer 1
					{},       // Layer 2
				},
			},
			20: {
				ID:       20,
				Vector:   []float32{40.0, 50.0, 60.0},
				Metadata: map[string]interface{}{"type": "test", "value": 84},
				Deleted:  false,
				Connections: [][]uint64{
					{10, 30}, // Layer 0
					{10},     // Layer 1
				},
			},
			30: {
				ID:       30,
				Vector:   []float32{70.0, 80.0, 90.0},
				Metadata: map[string]interface{}{"type": "test", "value": 126},
				Deleted:  false,
				Connections: [][]uint64{
					{10, 20}, // Layer 0
				},
			},
		},
		EntryPoint: 10,
		MaxLayer:   2,
		Size:       3,
	}

	// 转换到RDB格式
	hnswSnapshot := ConvertHNSWGraphState(originalState)
	require.NotNil(t, hnswSnapshot)

	// 转换回core格式
	recoveredState, err := ConvertHNSWGraphSnapshot(hnswSnapshot)
	require.NoError(t, err)
	require.NotNil(t, recoveredState)

	// 验证恢复的状态与原始状态一致
	assert.Equal(t, originalState.EntryPoint, recoveredState.EntryPoint)
	assert.Equal(t, originalState.MaxLayer, recoveredState.MaxLayer)
	assert.Equal(t, originalState.Size, recoveredState.Size)
	assert.Equal(t, len(originalState.Nodes), len(recoveredState.Nodes))

	// 验证每个节点
	for id, originalNode := range originalState.Nodes {
		recoveredNode, exists := recoveredState.Nodes[id]
		require.True(t, exists, "Node %d should exist", id)

		assert.Equal(t, originalNode.ID, recoveredNode.ID)
		assert.Equal(t, originalNode.Vector, recoveredNode.Vector)
		assert.Equal(t, originalNode.Metadata, recoveredNode.Metadata)
		assert.Equal(t, originalNode.Deleted, recoveredNode.Deleted)

		// 验证连接结构
		assert.Equal(t, len(originalNode.Connections), len(recoveredNode.Connections))
		for layer, originalConns := range originalNode.Connections {
			recoveredConns := recoveredNode.Connections[layer]
			assert.Equal(t, len(originalConns), len(recoveredConns),
				"Layer %d of node %d should have same number of connections", layer, id)

			// 验证所有原始连接都在恢复的连接中
			for _, connID := range originalConns {
				assert.Contains(t, recoveredConns, connID, "Connection to node %d should exist in layer %d of node %d",
					connID, layer, id)
			}
		}
	}
}
