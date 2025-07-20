package types

import (
	"testing"
	"time"

	pb "github.com/scintirete/scintirete/gen/go/scintirete/v1"
)

func TestDistanceMetric_String(t *testing.T) {
	tests := []struct {
		metric   DistanceMetric
		expected string
	}{
		{DistanceMetricL2, "L2"},
		{DistanceMetricCosine, "Cosine"},
		{DistanceMetricInnerProduct, "InnerProduct"},
		{DistanceMetricUnspecified, "Unspecified"},
		{DistanceMetric(999), "Unspecified"},
	}

	for _, test := range tests {
		if got := test.metric.String(); got != test.expected {
			t.Errorf("DistanceMetric(%d).String() = %q, want %q", test.metric, got, test.expected)
		}
	}
}

func TestDistanceMetric_ToProto(t *testing.T) {
	tests := []struct {
		metric   DistanceMetric
		expected pb.DistanceMetric
	}{
		{DistanceMetricL2, pb.DistanceMetric_L2},
		{DistanceMetricCosine, pb.DistanceMetric_COSINE},
		{DistanceMetricInnerProduct, pb.DistanceMetric_INNER_PRODUCT},
		{DistanceMetricUnspecified, pb.DistanceMetric_DISTANCE_METRIC_UNSPECIFIED},
	}

	for _, test := range tests {
		if got := test.metric.ToProto(); got != test.expected {
			t.Errorf("DistanceMetric(%d).ToProto() = %v, want %v", test.metric, got, test.expected)
		}
	}
}

func TestDistanceMetricFromProto(t *testing.T) {
	tests := []struct {
		protoMetric pb.DistanceMetric
		expected    DistanceMetric
	}{
		{pb.DistanceMetric_L2, DistanceMetricL2},
		{pb.DistanceMetric_COSINE, DistanceMetricCosine},
		{pb.DistanceMetric_INNER_PRODUCT, DistanceMetricInnerProduct},
		{pb.DistanceMetric_DISTANCE_METRIC_UNSPECIFIED, DistanceMetricUnspecified},
	}

	for _, test := range tests {
		if got := DistanceMetricFromProto(test.protoMetric); got != test.expected {
			t.Errorf("DistanceMetricFromProto(%v) = %v, want %v", test.protoMetric, got, test.expected)
		}
	}
}

func TestVector_Dimension(t *testing.T) {
	tests := []struct {
		vector   Vector
		expected int
	}{
		{Vector{Elements: []float32{1, 2, 3}}, 3},
		{Vector{Elements: []float32{}}, 0},
		{Vector{Elements: []float32{1.0, 2.0, 3.0, 4.0, 5.0}}, 5},
	}

	for _, test := range tests {
		if got := test.vector.Dimension(); got != test.expected {
			t.Errorf("Vector.Dimension() = %d, want %d", got, test.expected)
		}
	}
}

func TestDefaultHNSWParams(t *testing.T) {
	params := DefaultHNSWParams()

	if params.M <= 0 {
		t.Errorf("DefaultHNSWParams().M = %d, want > 0", params.M)
	}
	if params.EfConstruction <= 0 {
		t.Errorf("DefaultHNSWParams().EfConstruction = %d, want > 0", params.EfConstruction)
	}
	if params.EfSearch <= 0 {
		t.Errorf("DefaultHNSWParams().EfSearch = %d, want > 0", params.EfSearch)
	}
	if params.MaxLayers <= 0 {
		t.Errorf("DefaultHNSWParams().MaxLayers = %d, want > 0", params.MaxLayers)
	}
	if params.Seed == 0 {
		t.Errorf("DefaultHNSWParams().Seed = %d, want != 0", params.Seed)
	}
}

func TestHNSWParams_ToProto(t *testing.T) {
	params := HNSWParams{
		M:              16,
		EfConstruction: 200,
		EfSearch:       50,
		MaxLayers:      10,
		Seed:           12345,
	}

	proto := params.ToProto()

	if proto.M != int32(params.M) {
		t.Errorf("HNSWParams.ToProto().M = %d, want %d", proto.M, params.M)
	}
	if proto.EfConstruction != int32(params.EfConstruction) {
		t.Errorf("HNSWParams.ToProto().EfConstruction = %d, want %d", proto.EfConstruction, params.EfConstruction)
	}
}

func TestHNSWParamsFromProto(t *testing.T) {
	t.Run("with valid config", func(t *testing.T) {
		pbConfig := &pb.HnswConfig{
			M:              32,
			EfConstruction: 400,
		}

		params := HNSWParamsFromProto(pbConfig)

		if params.M != 32 {
			t.Errorf("HNSWParamsFromProto().M = %d, want 32", params.M)
		}
		if params.EfConstruction != 400 {
			t.Errorf("HNSWParamsFromProto().EfConstruction = %d, want 400", params.EfConstruction)
		}
		// EfSearch should keep default value
		if params.EfSearch != DefaultHNSWParams().EfSearch {
			t.Errorf("HNSWParamsFromProto().EfSearch = %d, want default %d", params.EfSearch, DefaultHNSWParams().EfSearch)
		}
	})

	t.Run("with nil config", func(t *testing.T) {
		params := HNSWParamsFromProto(nil)
		defaultParams := DefaultHNSWParams()

		if params.M != defaultParams.M {
			t.Errorf("HNSWParamsFromProto(nil).M = %d, want %d", params.M, defaultParams.M)
		}
		if params.EfConstruction != defaultParams.EfConstruction {
			t.Errorf("HNSWParamsFromProto(nil).EfConstruction = %d, want %d", params.EfConstruction, defaultParams.EfConstruction)
		}
	})

	t.Run("with zero values", func(t *testing.T) {
		pbConfig := &pb.HnswConfig{
			M:              0,
			EfConstruction: 0,
		}

		params := HNSWParamsFromProto(pbConfig)
		defaultParams := DefaultHNSWParams()

		// Zero values should be replaced with defaults
		if params.M != defaultParams.M {
			t.Errorf("HNSWParamsFromProto(zero).M = %d, want default %d", params.M, defaultParams.M)
		}
		if params.EfConstruction != defaultParams.EfConstruction {
			t.Errorf("HNSWParamsFromProto(zero).EfConstruction = %d, want default %d", params.EfConstruction, defaultParams.EfConstruction)
		}
	})
}

func TestCollectionInfo_ToProto(t *testing.T) {
	now := time.Now()
	info := CollectionInfo{
		Name:         "test_collection",
		Dimension:    128,
		VectorCount:  1000,
		DeletedCount: 10,
		MemoryBytes:  1024000,
		MetricType:   DistanceMetricCosine,
		HNSWConfig:   DefaultHNSWParams(),
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	proto := info.ToProto()

	if proto.Name != info.Name {
		t.Errorf("CollectionInfo.ToProto().Name = %q, want %q", proto.Name, info.Name)
	}
	if proto.Dimension != int32(info.Dimension) {
		t.Errorf("CollectionInfo.ToProto().Dimension = %d, want %d", proto.Dimension, info.Dimension)
	}
	if proto.VectorCount != info.VectorCount {
		t.Errorf("CollectionInfo.ToProto().VectorCount = %d, want %d", proto.VectorCount, info.VectorCount)
	}
	if proto.DeletedCount != info.DeletedCount {
		t.Errorf("CollectionInfo.ToProto().DeletedCount = %d, want %d", proto.DeletedCount, info.DeletedCount)
	}
	if proto.MemoryBytes != info.MemoryBytes {
		t.Errorf("CollectionInfo.ToProto().MemoryBytes = %d, want %d", proto.MemoryBytes, info.MemoryBytes)
	}
	if proto.MetricType != info.MetricType.ToProto() {
		t.Errorf("CollectionInfo.ToProto().MetricType = %v, want %v", proto.MetricType, info.MetricType.ToProto())
	}
}
