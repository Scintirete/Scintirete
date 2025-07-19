// Package algorithm provides vector indexing algorithms for Scintirete.
package algorithm

import (
	"math"

	"github.com/scintirete/scintirete/internal/core"
	"github.com/scintirete/scintirete/internal/utils"
	"github.com/scintirete/scintirete/pkg/types"
)

// L2Distance implements Euclidean distance calculation.
type L2Distance struct{}

// NewL2Distance creates a new L2 distance calculator.
func NewL2Distance() core.DistanceCalculator {
	return &L2Distance{}
}

// Distance calculates the Euclidean distance between two vectors.
func (d *L2Distance) Distance(a, b []float32) float32 {
	if len(a) != len(b) {
		return float32(math.Inf(1)) // Return infinity for mismatched dimensions
	}

	var sum float32
	for i := range a {
		diff := a[i] - b[i]
		sum += diff * diff
	}
	return float32(math.Sqrt(float64(sum)))
}

// DistanceType returns the distance metric type.
func (d *L2Distance) DistanceType() types.DistanceMetric {
	return types.DistanceMetricL2
}

// IsSimilarity returns false because L2 distance is a distance metric (lower is better).
func (d *L2Distance) IsSimilarity() bool {
	return false
}

// CosineDistance implements cosine similarity calculation.
type CosineDistance struct{}

// NewCosineDistance creates a new cosine distance calculator.
func NewCosineDistance() core.DistanceCalculator {
	return &CosineDistance{}
}

// Distance calculates the cosine distance (1 - cosine similarity) between two vectors.
func (d *CosineDistance) Distance(a, b []float32) float32 {
	if len(a) != len(b) {
		return float32(math.Inf(1)) // Return infinity for mismatched dimensions
	}

	var dotProduct, normA, normB float32
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	normA = float32(math.Sqrt(float64(normA)))
	normB = float32(math.Sqrt(float64(normB)))

	if normA == 0 || normB == 0 {
		return 1.0 // Maximum distance for zero vectors
	}

	cosineSimilarity := dotProduct / (normA * normB)
	// Clamp to [-1, 1] to handle floating-point precision issues
	if cosineSimilarity > 1.0 {
		cosineSimilarity = 1.0
	} else if cosineSimilarity < -1.0 {
		cosineSimilarity = -1.0
	}

	// Return cosine distance (1 - cosine similarity)
	return 1.0 - cosineSimilarity
}

// DistanceType returns the distance metric type.
func (d *CosineDistance) DistanceType() types.DistanceMetric {
	return types.DistanceMetricCosine
}

// IsSimilarity returns false because we return cosine distance (lower is better).
func (d *CosineDistance) IsSimilarity() bool {
	return false
}

// InnerProductDistance implements inner product similarity calculation.
type InnerProductDistance struct{}

// NewInnerProductDistance creates a new inner product distance calculator.
func NewInnerProductDistance() core.DistanceCalculator {
	return &InnerProductDistance{}
}

// Distance calculates the negative inner product between two vectors.
// We return negative inner product so that higher similarity becomes lower distance.
func (d *InnerProductDistance) Distance(a, b []float32) float32 {
	if len(a) != len(b) {
		return float32(math.Inf(1)) // Return infinity for mismatched dimensions
	}

	var dotProduct float32
	for i := range a {
		dotProduct += a[i] * b[i]
	}

	// Return negative inner product so that higher similarity becomes lower distance
	return -dotProduct
}

// DistanceType returns the distance metric type.
func (d *InnerProductDistance) DistanceType() types.DistanceMetric {
	return types.DistanceMetricInnerProduct
}

// IsSimilarity returns false because we return negative inner product (lower is better).
func (d *InnerProductDistance) IsSimilarity() bool {
	return false
}

// NewDistanceCalculator creates a distance calculator based on the metric type.
func NewDistanceCalculator(metric types.DistanceMetric) (core.DistanceCalculator, error) {
	switch metric {
	case types.DistanceMetricL2:
		return NewL2Distance(), nil
	case types.DistanceMetricCosine:
		return NewCosineDistance(), nil
	case types.DistanceMetricInnerProduct:
		return NewInnerProductDistance(), nil
	default:
		return nil, utils.ErrInvalidParameters("unsupported distance metric")
	}
}

// BatchDistance calculates distances between a query vector and multiple target vectors.
// This can be optimized for better performance in batch operations.
func BatchDistance(calc core.DistanceCalculator, query []float32, targets [][]float32) []float32 {
	distances := make([]float32, len(targets))
	for i, target := range targets {
		distances[i] = calc.Distance(query, target)
	}
	return distances
}

// NormalizeVector normalizes a vector to unit length (L2 norm = 1).
// This is useful for cosine distance calculations.
func NormalizeVector(vector []float32) []float32 {
	var norm float32
	for _, v := range vector {
		norm += v * v
	}
	norm = float32(math.Sqrt(float64(norm)))

	if norm == 0 {
		return vector // Return original vector if it's zero
	}

	normalized := make([]float32, len(vector))
	for i, v := range vector {
		normalized[i] = v / norm
	}
	return normalized
}

// VectorMagnitude calculates the L2 norm (magnitude) of a vector.
func VectorMagnitude(vector []float32) float32 {
	var sum float32
	for _, v := range vector {
		sum += v * v
	}
	return float32(math.Sqrt(float64(sum)))
}

// DotProduct calculates the dot product between two vectors.
func DotProduct(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}

	var product float32
	for i := range a {
		product += a[i] * b[i]
	}
	return product
} 