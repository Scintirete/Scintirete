package algorithm

import (
	"math"
	"testing"

	"github.com/scintirete/scintirete/pkg/types"
)

func TestL2Distance_Distance(t *testing.T) {
	calc := NewL2Distance()

	tests := []struct {
		name     string
		a, b     []float32
		expected float32
		delta    float32
	}{
		{
			name:     "identical vectors",
			a:        []float32{1, 2, 3},
			b:        []float32{1, 2, 3},
			expected: 0,
			delta:    1e-6,
		},
		{
			name:     "orthogonal unit vectors",
			a:        []float32{1, 0},
			b:        []float32{0, 1},
			expected: float32(math.Sqrt(2)),
			delta:    1e-6,
		},
		{
			name:     "simple case",
			a:        []float32{1, 1},
			b:        []float32{4, 5},
			expected: 5, // sqrt((4-1)^2 + (5-1)^2) = sqrt(9+16) = 5
			delta:    1e-6,
		},
		{
			name:     "negative values",
			a:        []float32{-1, -2},
			b:        []float32{1, 2},
			expected: float32(math.Sqrt(20)), // sqrt(4+16) = sqrt(20)
			delta:    1e-6,
		},
		{
			name:     "zero vectors",
			a:        []float32{0, 0, 0},
			b:        []float32{0, 0, 0},
			expected: 0,
			delta:    1e-6,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := calc.Distance(test.a, test.b)
			if math.Abs(float64(result-test.expected)) > float64(test.delta) {
				t.Errorf("L2Distance(%v, %v) = %f, want %f", test.a, test.b, result, test.expected)
			}
		})
	}
}

func TestL2Distance_MismatchedDimensions(t *testing.T) {
	calc := NewL2Distance()
	result := calc.Distance([]float32{1, 2}, []float32{1, 2, 3})
	if !math.IsInf(float64(result), 1) {
		t.Errorf("L2Distance with mismatched dimensions should return +Inf, got %f", result)
	}
}

func TestCosineDistance_Distance(t *testing.T) {
	calc := NewCosineDistance()

	tests := []struct {
		name     string
		a, b     []float32
		expected float32
		delta    float32
	}{
		{
			name:     "identical vectors",
			a:        []float32{1, 2, 3},
			b:        []float32{1, 2, 3},
			expected: 0, // cosine distance = 1 - 1 = 0
			delta:    1e-6,
		},
		{
			name:     "orthogonal vectors",
			a:        []float32{1, 0},
			b:        []float32{0, 1},
			expected: 1, // cosine distance = 1 - 0 = 1
			delta:    1e-6,
		},
		{
			name:     "opposite vectors",
			a:        []float32{1, 0},
			b:        []float32{-1, 0},
			expected: 2, // cosine distance = 1 - (-1) = 2
			delta:    1e-6,
		},
		{
			name:     "parallel vectors with different magnitudes",
			a:        []float32{1, 2},
			b:        []float32{2, 4},
			expected: 0, // same direction, cosine similarity = 1
			delta:    1e-6,
		},
		{
			name:     "unit vectors at 60 degrees",
			a:        []float32{1, 0},
			b:        []float32{0.5, float32(math.Sqrt(3) / 2)},
			expected: 0.5, // cosine(60°) = 0.5, distance = 1 - 0.5 = 0.5
			delta:    1e-5,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := calc.Distance(test.a, test.b)
			if math.Abs(float64(result-test.expected)) > float64(test.delta) {
				t.Errorf("CosineDistance(%v, %v) = %f, want %f", test.a, test.b, result, test.expected)
			}
		})
	}
}

func TestCosineDistance_ZeroVectors(t *testing.T) {
	calc := NewCosineDistance()

	tests := []struct {
		name     string
		a, b     []float32
		expected float32
	}{
		{
			name:     "both zero vectors",
			a:        []float32{0, 0},
			b:        []float32{0, 0},
			expected: 1, // maximum distance
		},
		{
			name:     "one zero vector",
			a:        []float32{1, 2},
			b:        []float32{0, 0},
			expected: 1, // maximum distance
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := calc.Distance(test.a, test.b)
			if result != test.expected {
				t.Errorf("CosineDistance(%v, %v) = %f, want %f", test.a, test.b, result, test.expected)
			}
		})
	}
}

func TestInnerProductDistance_Distance(t *testing.T) {
	calc := NewInnerProductDistance()

	tests := []struct {
		name     string
		a, b     []float32
		expected float32
		delta    float32
	}{
		{
			name:     "positive dot product",
			a:        []float32{1, 2, 3},
			b:        []float32{1, 1, 1},
			expected: -6, // -(1*1 + 2*1 + 3*1) = -6
			delta:    1e-6,
		},
		{
			name:     "negative dot product",
			a:        []float32{1, 2},
			b:        []float32{-1, -1},
			expected: 3, // -(1*(-1) + 2*(-1)) = -(-3) = 3
			delta:    1e-6,
		},
		{
			name:     "zero dot product",
			a:        []float32{1, 0},
			b:        []float32{0, 1},
			expected: 0, // -(1*0 + 0*1) = 0
			delta:    1e-6,
		},
		{
			name:     "identical vectors",
			a:        []float32{2, 3},
			b:        []float32{2, 3},
			expected: -13, // -(2*2 + 3*3) = -(4+9) = -13
			delta:    1e-6,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := calc.Distance(test.a, test.b)
			if math.Abs(float64(result-test.expected)) > float64(test.delta) {
				t.Errorf("InnerProductDistance(%v, %v) = %f, want %f", test.a, test.b, result, test.expected)
			}
		})
	}
}

func TestDistanceCalculators_Properties(t *testing.T) {
	calculators := map[string]struct {
		calc         func() any
		expectedType types.DistanceMetric
		isSimilarity bool
	}{
		"L2Distance": {
			calc:         func() any { return NewL2Distance() },
			expectedType: types.DistanceMetricL2,
			isSimilarity: false,
		},
		"CosineDistance": {
			calc:         func() any { return NewCosineDistance() },
			expectedType: types.DistanceMetricCosine,
			isSimilarity: false,
		},
		"InnerProductDistance": {
			calc:         func() any { return NewInnerProductDistance() },
			expectedType: types.DistanceMetricInnerProduct,
			isSimilarity: false,
		},
	}

	for name, test := range calculators {
		t.Run(name, func(t *testing.T) {
			calc := test.calc().(interface {
				DistanceType() types.DistanceMetric
				IsSimilarity() bool
			})

			if calc.DistanceType() != test.expectedType {
				t.Errorf("%s.DistanceType() = %v, want %v", name, calc.DistanceType(), test.expectedType)
			}

			if calc.IsSimilarity() != test.isSimilarity {
				t.Errorf("%s.IsSimilarity() = %v, want %v", name, calc.IsSimilarity(), test.isSimilarity)
			}
		})
	}
}

func TestNewDistanceCalculator(t *testing.T) {
	tests := []struct {
		metric      types.DistanceMetric
		expectError bool
	}{
		{types.DistanceMetricL2, false},
		{types.DistanceMetricCosine, false},
		{types.DistanceMetricInnerProduct, false},
		{types.DistanceMetricUnspecified, true},
		{types.DistanceMetric(999), true},
	}

	for _, test := range tests {
		t.Run(test.metric.String(), func(t *testing.T) {
			calc, err := NewDistanceCalculator(test.metric)

			if test.expectError {
				if err == nil {
					t.Errorf("NewDistanceCalculator(%v) should return error", test.metric)
				}
			} else {
				if err != nil {
					t.Errorf("NewDistanceCalculator(%v) should not return error: %v", test.metric, err)
				}
				if calc == nil {
					t.Errorf("NewDistanceCalculator(%v) should return calculator", test.metric)
				}
				if calc.DistanceType() != test.metric {
					t.Errorf("NewDistanceCalculator(%v).DistanceType() = %v", test.metric, calc.DistanceType())
				}
			}
		})
	}
}

func TestBatchDistance(t *testing.T) {
	calc := NewL2Distance()
	query := []float32{0, 0}
	targets := [][]float32{
		{1, 0},
		{0, 1},
		{1, 1},
		{2, 2},
	}

	distances := BatchDistance(calc, query, targets)

	expected := []float32{1, 1, float32(math.Sqrt(2)), float32(math.Sqrt(8))}

	if len(distances) != len(expected) {
		t.Fatalf("BatchDistance returned %d distances, want %d", len(distances), len(expected))
	}

	for i, distance := range distances {
		if math.Abs(float64(distance-expected[i])) > 1e-6 {
			t.Errorf("distances[%d] = %f, want %f", i, distance, expected[i])
		}
	}
}

func TestNormalizeVector(t *testing.T) {
	tests := []struct {
		name     string
		input    []float32
		expected []float32
		delta    float32
	}{
		{
			name:     "unit vector",
			input:    []float32{1, 0},
			expected: []float32{1, 0},
			delta:    1e-6,
		},
		{
			name:     "simple vector",
			input:    []float32{3, 4},
			expected: []float32{0.6, 0.8}, // 3/5, 4/5
			delta:    1e-6,
		},
		{
			name:     "zero vector",
			input:    []float32{0, 0},
			expected: []float32{0, 0}, // should return original
			delta:    1e-6,
		},
		{
			name:     "negative values",
			input:    []float32{-1, 1},
			expected: []float32{-1 / float32(math.Sqrt(2)), 1 / float32(math.Sqrt(2))},
			delta:    1e-6,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := NormalizeVector(test.input)

			if len(result) != len(test.expected) {
				t.Fatalf("NormalizeVector returned length %d, want %d", len(result), len(test.expected))
			}

			for i, v := range result {
				if math.Abs(float64(v-test.expected[i])) > float64(test.delta) {
					t.Errorf("NormalizeVector(%v)[%d] = %f, want %f", test.input, i, v, test.expected[i])
				}
			}

			// Check that the result is unit length (except for zero vector)
			magnitude := VectorMagnitude(result)
			if test.name != "zero vector" && math.Abs(float64(magnitude-1.0)) > 1e-6 {
				t.Errorf("Normalized vector magnitude = %f, want 1.0", magnitude)
			}
		})
	}
}

func TestVectorMagnitude(t *testing.T) {
	tests := []struct {
		name     string
		input    []float32
		expected float32
		delta    float32
	}{
		{
			name:     "unit vector",
			input:    []float32{1, 0},
			expected: 1,
			delta:    1e-6,
		},
		{
			name:     "3-4-5 triangle",
			input:    []float32{3, 4},
			expected: 5,
			delta:    1e-6,
		},
		{
			name:     "zero vector",
			input:    []float32{0, 0, 0},
			expected: 0,
			delta:    1e-6,
		},
		{
			name:     "negative values",
			input:    []float32{-1, -1},
			expected: float32(math.Sqrt(2)),
			delta:    1e-6,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := VectorMagnitude(test.input)
			if math.Abs(float64(result-test.expected)) > float64(test.delta) {
				t.Errorf("VectorMagnitude(%v) = %f, want %f", test.input, result, test.expected)
			}
		})
	}
}

func TestDotProduct(t *testing.T) {
	tests := []struct {
		name     string
		a, b     []float32
		expected float32
	}{
		{
			name:     "orthogonal vectors",
			a:        []float32{1, 0},
			b:        []float32{0, 1},
			expected: 0,
		},
		{
			name:     "parallel vectors",
			a:        []float32{1, 2, 3},
			b:        []float32{1, 2, 3},
			expected: 14, // 1 + 4 + 9
		},
		{
			name:     "mixed values",
			a:        []float32{1, -2, 3},
			b:        []float32{4, 5, 6},
			expected: 12, // 4 - 10 + 18
		},
		{
			name:     "mismatched dimensions",
			a:        []float32{1, 2},
			b:        []float32{1, 2, 3},
			expected: 0, // should return 0 for mismatched dimensions
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := DotProduct(test.a, test.b)
			if result != test.expected {
				t.Errorf("DotProduct(%v, %v) = %f, want %f", test.a, test.b, result, test.expected)
			}
		})
	}
}

// Benchmark tests
func BenchmarkL2Distance(b *testing.B) {
	calc := NewL2Distance()
	a := make([]float32, 128)
	vec := make([]float32, 128)
	for i := range a {
		a[i] = float32(i)
		vec[i] = float32(i + 1)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calc.Distance(a, vec)
	}
}

func BenchmarkCosineDistance(b *testing.B) {
	calc := NewCosineDistance()
	a := make([]float32, 128)
	vec := make([]float32, 128)
	for i := range a {
		a[i] = float32(i)
		vec[i] = float32(i + 1)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calc.Distance(a, vec)
	}
}

func BenchmarkInnerProductDistance(b *testing.B) {
	calc := NewInnerProductDistance()
	a := make([]float32, 128)
	vec := make([]float32, 128)
	for i := range a {
		a[i] = float32(i)
		vec[i] = float32(i + 1)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calc.Distance(a, vec)
	}
}
