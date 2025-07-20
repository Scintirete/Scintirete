package algorithm

import (
	"context"
	"math"
	"math/rand"
	"sync"

	"github.com/scintirete/scintirete/internal/core"
	"github.com/scintirete/scintirete/internal/utils"
	"github.com/scintirete/scintirete/pkg/types"
)

// HNSWNode represents a node in the HNSW graph
type HNSWNode struct {
	ID       string                 // Vector ID
	Vector   []float32              // Vector data
	Metadata map[string]interface{} // Associated metadata
	Deleted  bool                   // Soft delete flag

	// Connections at each layer: layer -> set of connected node IDs
	Connections []map[string]struct{}
}

// NewHNSWNode creates a new HNSW node
func NewHNSWNode(id string, vector []float32, metadata map[string]interface{}, maxLayers int) *HNSWNode {
	connections := make([]map[string]struct{}, maxLayers)
	for i := range connections {
		connections[i] = make(map[string]struct{})
	}

	return &HNSWNode{
		ID:          id,
		Vector:      vector,
		Metadata:    metadata,
		Deleted:     false,
		Connections: connections,
	}
}

// GetConnections returns the connections at a specific layer
func (n *HNSWNode) GetConnections(layer int) map[string]struct{} {
	if layer >= len(n.Connections) {
		return make(map[string]struct{})
	}
	return n.Connections[layer]
}

// AddConnection adds a connection at a specific layer
func (n *HNSWNode) AddConnection(layer int, nodeID string) {
	if layer < len(n.Connections) {
		n.Connections[layer][nodeID] = struct{}{}
	}
}

// RemoveConnection removes a connection at a specific layer
func (n *HNSWNode) RemoveConnection(layer int, nodeID string) {
	if layer < len(n.Connections) {
		delete(n.Connections[layer], nodeID)
	}
}

// HNSW implements the Hierarchical Navigable Small World algorithm
type HNSW struct {
	// Configuration
	params   types.HNSWParams
	metric   types.DistanceMetric
	distCalc core.DistanceCalculator

	// Graph data
	mu         sync.RWMutex
	nodes      map[string]*HNSWNode // All nodes indexed by ID
	entrypoint string               // ID of the entry point node
	maxLayer   int                  // Current maximum layer

	// Statistics
	size        int
	memoryUsage int64

	// Random number generator
	rng *rand.Rand
}

// NewHNSW creates a new HNSW index
func NewHNSW(params types.HNSWParams, metric types.DistanceMetric) (core.HNSWIndex, error) {
	distCalc, err := NewDistanceCalculator(metric)
	if err != nil {
		return nil, err
	}

	return &HNSW{
		params:      params,
		metric:      metric,
		distCalc:    distCalc,
		nodes:       make(map[string]*HNSWNode),
		entrypoint:  "",
		maxLayer:    -1,
		size:        0,
		memoryUsage: 0,
		rng:         rand.New(rand.NewSource(params.Seed)),
	}, nil
}

// Build constructs the index from the given vectors
func (h *HNSW) Build(ctx context.Context, vectors []types.Vector) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Clear existing data
	h.nodes = make(map[string]*HNSWNode)
	h.entrypoint = ""
	h.maxLayer = -1
	h.size = 0

	// Insert vectors one by one
	for _, vector := range vectors {
		if err := h.insertVector(vector); err != nil {
			return utils.ErrIndexBuildFailed("failed to insert vector "+vector.ID).WithContext("cause", err.Error())
		}

		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}

	h.updateMemoryUsage()
	return nil
}

// Insert adds a single vector to the index
func (h *HNSW) Insert(ctx context.Context, vector types.Vector) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if err := h.insertVector(vector); err != nil {
		return utils.ErrInsertFailed("failed to insert vector "+vector.ID).WithContext("cause", err.Error())
	}

	h.updateMemoryUsage()
	return nil
}

// insertVector performs the actual vector insertion (must be called with lock held)
func (h *HNSW) insertVector(vector types.Vector) error {
	// Check if vector already exists
	if _, exists := h.nodes[vector.ID]; exists {
		return utils.ErrInvalidParameters("vector with ID '" + vector.ID + "' already exists")
	}

	// Determine the layer for this node
	layer := h.selectLayer()

	// Create the new node
	node := NewHNSWNode(vector.ID, vector.Elements, vector.Metadata, layer+1)
	h.nodes[vector.ID] = node
	h.size++

	// Update max layer
	if layer > h.maxLayer {
		h.maxLayer = layer
	}

	// If this is the first node, make it the entry point
	if h.entrypoint == "" {
		h.entrypoint = vector.ID
		return nil
	}

	// Find entry points for each layer and build connections
	entryPoints := []string{h.entrypoint}

	// Search from top layer down to target layer + 1
	for lc := h.maxLayer; lc > layer; lc-- {
		entryPoints = h.searchLayer(vector.Elements, entryPoints, 1, lc)
	}

	// Search and connect from target layer down to layer 0
	for lc := min(layer, h.maxLayer); lc >= 0; lc-- {
		candidates := h.searchLayer(vector.Elements, entryPoints, h.params.EfConstruction, lc)

		// Select neighbors
		maxConnections := h.params.M
		if lc == 0 {
			maxConnections = h.params.M * 2 // Layer 0 can have more connections
		}

		selectedNeighbors := h.selectNeighbors(vector.Elements, candidates, maxConnections)

		// Add connections
		for _, neighborID := range selectedNeighbors {
			node.AddConnection(lc, neighborID)

			// Add reverse connection
			if neighbor, exists := h.nodes[neighborID]; exists {
				neighbor.AddConnection(lc, vector.ID)

				// Prune connections if necessary
				h.pruneConnections(neighbor, lc)
			}
		}

		entryPoints = selectedNeighbors
	}

	// Update entry point if the new node is at a higher layer
	if layer > h.getNodeLayer(h.entrypoint) {
		h.entrypoint = vector.ID
	}

	return nil
}

// Delete marks a vector as deleted
func (h *HNSW) Delete(ctx context.Context, id string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	node, exists := h.nodes[id]
	if !exists {
		return utils.ErrVectorNotFound(id)
	}

	if node.Deleted {
		return nil // Already deleted
	}

	node.Deleted = true
	h.size--

	// If this was the entry point, find a new one
	if h.entrypoint == id {
		h.findNewEntrypoint()
	}

	h.updateMemoryUsage()
	return nil
}

// Search finds the most similar vectors to the query
func (h *HNSW) Search(ctx context.Context, query []float32, params types.SearchParams) ([]types.SearchResult, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.entrypoint == "" || h.size == 0 {
		return []types.SearchResult{}, nil
	}

	ef := h.params.EfSearch
	if params.EfSearch != nil && *params.EfSearch > 0 {
		ef = *params.EfSearch
	}

	// Start from entry point and search down
	entryPoints := []string{h.entrypoint}

	// Search from top layer down to layer 1
	for lc := h.maxLayer; lc > 0; lc-- {
		entryPoints = h.searchLayer(query, entryPoints, 1, lc)
	}

	// Search layer 0 with the specified ef
	candidates := h.searchLayer(query, entryPoints, ef, 0)

	// Convert to search results and sort by distance
	results := make([]types.SearchResult, 0, min(params.TopK, len(candidates)))

	for _, candidateID := range candidates {
		if len(results) >= params.TopK {
			break
		}

		node := h.nodes[candidateID]
		if node.Deleted {
			continue
		}

		distance := h.distCalc.Distance(query, node.Vector)
		result := types.SearchResult{
			Vector: types.Vector{
				ID:       node.ID,
				Elements: node.Vector,
				Metadata: node.Metadata,
			},
			Distance: distance,
		}
		results = append(results, result)
	}

	// Sort results by distance
	h.sortSearchResults(results)

	// Limit to top-k
	if len(results) > params.TopK {
		results = results[:params.TopK]
	}

	return results, nil
}

// Get retrieves a vector by ID
func (h *HNSW) Get(ctx context.Context, id string) (*types.Vector, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	node, exists := h.nodes[id]
	if !exists || node.Deleted {
		return nil, utils.ErrVectorNotFound(id)
	}

	return &types.Vector{
		ID:       node.ID,
		Elements: node.Vector,
		Metadata: node.Metadata,
	}, nil
}

// Size returns the number of vectors in the index
func (h *HNSW) Size() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.size
}

// MemoryUsage returns the memory usage in bytes
func (h *HNSW) MemoryUsage() int64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.memoryUsage
}

// GetStatistics returns HNSW-specific statistics
func (h *HNSW) GetStatistics() interface{} {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return h.GetGraphStatistics()
}

// GetParameters returns the HNSW configuration parameters
func (h *HNSW) GetParameters() types.HNSWParams {
	return h.params
}

// GetLayers returns the number of layers in the HNSW graph
func (h *HNSW) GetLayers() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.maxLayer < 0 {
		return 0
	}
	return h.maxLayer + 1
}

// GetGraphStatistics returns detailed graph statistics
func (h *HNSW) GetGraphStatistics() types.GraphStats {
	totalConnections := 0
	maxDegree := 0

	for _, node := range h.nodes {
		if node.Deleted {
			continue
		}

		degree := 0
		for _, connections := range node.Connections {
			degree += len(connections)
		}

		totalConnections += degree
		if degree > maxDegree {
			maxDegree = degree
		}
	}

	avgDegree := 0.0
	if h.size > 0 {
		avgDegree = float64(totalConnections) / float64(h.size)
	}

	return types.GraphStats{
		Layers:      h.maxLayer + 1,
		Nodes:       h.size,
		Connections: totalConnections,
		AvgDegree:   avgDegree,
		MaxDegree:   maxDegree,
		MemoryUsage: h.memoryUsage,
	}
}

// SetEfSearch dynamically updates the ef_search parameter for queries
func (h *HNSW) SetEfSearch(efSearch int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.params.EfSearch = efSearch
}

// Helper methods

// selectLayer determines which layer a new node should be inserted into
func (h *HNSW) selectLayer() int {
	// Use exponential decay probability distribution
	mL := 1.0 / math.Log(2.0)
	level := int(math.Floor(-math.Log(h.rng.Float64()) * mL))

	// Cap at maximum layers
	if level >= h.params.MaxLayers {
		level = h.params.MaxLayers - 1
	}

	return level
}

// getNodeLayer returns the highest layer of a node
func (h *HNSW) getNodeLayer(nodeID string) int {
	node, exists := h.nodes[nodeID]
	if !exists {
		return -1
	}

	for i := len(node.Connections) - 1; i >= 0; i-- {
		if len(node.Connections[i]) > 0 {
			return i
		}
	}
	return 0 // At least exists in layer 0
}

// searchLayer performs greedy search in a specific layer
func (h *HNSW) searchLayer(query []float32, entryPoints []string, numClosest int, layer int) []string {
	visited := make(map[string]struct{})
	candidates := make([]CandidateItem, 0)

	// Initialize with entry points
	for _, ep := range entryPoints {
		if node, exists := h.nodes[ep]; exists && !node.Deleted {
			distance := h.distCalc.Distance(query, node.Vector)
			candidates = append(candidates, CandidateItem{ID: ep, Distance: distance})
			visited[ep] = struct{}{}
		}
	}

	if len(candidates) == 0 {
		return []string{}
	}

	// Sort candidates by distance
	h.sortCandidates(candidates)

	dynamic := make([]CandidateItem, len(candidates))
	copy(dynamic, candidates)

	for len(dynamic) > 0 {
		// Get closest unvisited candidate
		current := dynamic[0]
		dynamic = dynamic[1:]

		// Check if we should continue (stopping condition)
		if len(candidates) >= numClosest && current.Distance > candidates[numClosest-1].Distance {
			break
		}

		// Explore neighbors
		node := h.nodes[current.ID]
		for neighborID := range node.GetConnections(layer) {
			if _, alreadyVisited := visited[neighborID]; alreadyVisited {
				continue
			}

			neighbor := h.nodes[neighborID]
			if neighbor.Deleted {
				continue
			}

			visited[neighborID] = struct{}{}
			distance := h.distCalc.Distance(query, neighbor.Vector)

			// Add to candidates if it's close enough
			if len(candidates) < numClosest {
				candidates = append(candidates, CandidateItem{ID: neighborID, Distance: distance})
				dynamic = append(dynamic, CandidateItem{ID: neighborID, Distance: distance})
			} else if distance < candidates[numClosest-1].Distance {
				candidates[numClosest-1] = CandidateItem{ID: neighborID, Distance: distance}
				dynamic = append(dynamic, CandidateItem{ID: neighborID, Distance: distance})
			}

			// Keep candidates sorted
			h.sortCandidates(candidates)
			h.sortCandidates(dynamic)
		}
	}

	// Extract IDs
	result := make([]string, min(numClosest, len(candidates)))
	for i := range result {
		result[i] = candidates[i].ID
	}

	return result
}

// selectNeighbors selects the best neighbors using a simple heuristic
func (h *HNSW) selectNeighbors(query []float32, candidates []string, maxConnections int) []string {
	if len(candidates) <= maxConnections {
		return candidates
	}

	// Create candidate items with distances
	items := make([]CandidateItem, len(candidates))
	for i, candidateID := range candidates {
		node := h.nodes[candidateID]
		distance := h.distCalc.Distance(query, node.Vector)
		items[i] = CandidateItem{ID: candidateID, Distance: distance}
	}

	// Sort by distance
	h.sortCandidates(items)

	// Return top maxConnections
	result := make([]string, maxConnections)
	for i := 0; i < maxConnections; i++ {
		result[i] = items[i].ID
	}

	return result
}

// pruneConnections removes excess connections if a node has too many
func (h *HNSW) pruneConnections(node *HNSWNode, layer int) {
	maxConnections := h.params.M
	if layer == 0 {
		maxConnections = h.params.M * 2
	}

	connections := node.GetConnections(layer)
	if len(connections) <= maxConnections {
		return
	}

	// Create candidate items
	candidates := make([]CandidateItem, 0, len(connections))
	for connectionID := range connections {
		if connectedNode, exists := h.nodes[connectionID]; exists && !connectedNode.Deleted {
			distance := h.distCalc.Distance(node.Vector, connectedNode.Vector)
			candidates = append(candidates, CandidateItem{ID: connectionID, Distance: distance})
		}
	}

	// Sort by distance and keep the closest ones
	h.sortCandidates(candidates)

	// Clear connections and re-add the selected ones
	node.Connections[layer] = make(map[string]struct{})
	for i := 0; i < min(maxConnections, len(candidates)); i++ {
		node.AddConnection(layer, candidates[i].ID)
	}
}

// findNewEntrypoint finds a new entry point when the current one is deleted
func (h *HNSW) findNewEntrypoint() {
	h.entrypoint = ""
	maxLayerFound := -1

	for nodeID, node := range h.nodes {
		if node.Deleted {
			continue
		}

		nodeLayer := h.getNodeLayer(nodeID)
		if nodeLayer > maxLayerFound {
			maxLayerFound = nodeLayer
			h.entrypoint = nodeID
		}
	}

	h.maxLayer = maxLayerFound
}

// updateMemoryUsage calculates and updates the memory usage estimate
func (h *HNSW) updateMemoryUsage() {
	var usage int64

	for _, node := range h.nodes {
		// Vector data
		usage += int64(len(node.Vector) * 4) // 4 bytes per float32

		// ID string
		usage += int64(len(node.ID))

		// Connections
		for _, connections := range node.Connections {
			usage += int64(len(connections) * 16) // Estimate for map overhead
		}

		// Node overhead
		usage += 64 // Estimate for struct overhead
	}

	h.memoryUsage = usage
}

// CandidateItem represents a candidate node with its distance
type CandidateItem struct {
	ID       string
	Distance float32
}

// sortCandidates sorts candidates by distance (ascending)
func (h *HNSW) sortCandidates(candidates []CandidateItem) {
	// Simple insertion sort for small arrays, efficient for the typical use case
	for i := 1; i < len(candidates); i++ {
		key := candidates[i]
		j := i - 1
		for j >= 0 && candidates[j].Distance > key.Distance {
			candidates[j+1] = candidates[j]
			j--
		}
		candidates[j+1] = key
	}
}

// sortSearchResults sorts search results by distance (ascending)
func (h *HNSW) sortSearchResults(results []types.SearchResult) {
	for i := 1; i < len(results); i++ {
		key := results[i]
		j := i - 1
		for j >= 0 && results[j].Distance > key.Distance {
			results[j+1] = results[j]
			j--
		}
		results[j+1] = key
	}
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
