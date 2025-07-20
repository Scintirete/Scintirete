package utils

import (
	"errors"
	"testing"
)

func TestErrorCode_String(t *testing.T) {
	tests := []struct {
		code     ErrorCode
		expected string
	}{
		{ErrorCodeInternal, "INTERNAL"},
		{ErrorCodeUnauthorized, "UNAUTHORIZED"},
		{ErrorCodeDatabaseNotFound, "DATABASE_NOT_FOUND"},
		{ErrorCodePersistenceFailed, "PERSISTENCE_FAILED"},
		{ErrorCodeIndexBuildFailed, "INDEX_BUILD_FAILED"},
		{ErrorCodeEmbeddingApiFailed, "EMBEDDING_API_FAILED"},
		{ErrorCode(999999), "UNKNOWN"},
	}

	for _, test := range tests {
		if got := test.code.String(); got != test.expected {
			t.Errorf("ErrorCode(%d).String() = %q, want %q", test.code, got, test.expected)
		}
	}
}

func TestScintireteError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *ScintireteError
		expected string
	}{
		{
			name: "simple error",
			err: &ScintireteError{
				Code:    ErrorCodeDatabaseNotFound,
				Message: "test database not found",
			},
			expected: "[DATABASE_NOT_FOUND] test database not found",
		},
		{
			name: "error with cause",
			err: &ScintireteError{
				Code:    ErrorCodePersistenceFailed,
				Message: "failed to save data",
				Cause:   errors.New("disk full"),
			},
			expected: "[PERSISTENCE_FAILED] failed to save data caused by: disk full",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.err.Error(); got != test.expected {
				t.Errorf("ScintireteError.Error() = %q, want %q", got, test.expected)
			}
		})
	}
}

func TestScintireteError_Unwrap(t *testing.T) {
	cause := errors.New("original error")
	err := &ScintireteError{
		Code:    ErrorCodeInternal,
		Message: "wrapped error",
		Cause:   cause,
	}

	if unwrapped := err.Unwrap(); unwrapped != cause {
		t.Errorf("ScintireteError.Unwrap() = %v, want %v", unwrapped, cause)
	}

	// Test error without cause
	errNoCause := &ScintireteError{
		Code:    ErrorCodeInternal,
		Message: "no cause",
	}

	if unwrapped := errNoCause.Unwrap(); unwrapped != nil {
		t.Errorf("ScintireteError.Unwrap() = %v, want nil", unwrapped)
	}
}

func TestScintireteError_WithContext(t *testing.T) {
	err := &ScintireteError{
		Code:    ErrorCodeDatabaseNotFound,
		Message: "test error",
	}

	err.WithContext("database", "test_db")
	err.WithContext("operation", "search")

	if err.Context == nil {
		t.Fatal("Expected context to be set, got nil")
	}

	if db, ok := err.Context["database"]; !ok || db != "test_db" {
		t.Errorf("Expected context['database'] = 'test_db', got %v", db)
	}

	if op, ok := err.Context["operation"]; !ok || op != "search" {
		t.Errorf("Expected context['operation'] = 'search', got %v", op)
	}
}

func TestNewError(t *testing.T) {
	err := NewError(ErrorCodeDatabaseNotFound, "test message")

	if err.Code != ErrorCodeDatabaseNotFound {
		t.Errorf("NewError().Code = %v, want %v", err.Code, ErrorCodeDatabaseNotFound)
	}
	if err.Message != "test message" {
		t.Errorf("NewError().Message = %q, want %q", err.Message, "test message")
	}
	if err.Cause != nil {
		t.Errorf("NewError().Cause = %v, want nil", err.Cause)
	}
}

func TestNewErrorWithCause(t *testing.T) {
	cause := errors.New("original error")
	err := NewErrorWithCause(ErrorCodePersistenceFailed, "test message", cause)

	if err.Code != ErrorCodePersistenceFailed {
		t.Errorf("NewErrorWithCause().Code = %v, want %v", err.Code, ErrorCodePersistenceFailed)
	}
	if err.Message != "test message" {
		t.Errorf("NewErrorWithCause().Message = %q, want %q", err.Message, "test message")
	}
	if err.Cause != cause {
		t.Errorf("NewErrorWithCause().Cause = %v, want %v", err.Cause, cause)
	}
}

func TestErrorFactoryFunctions(t *testing.T) {
	tests := []struct {
		name     string
		errFunc  func() *ScintireteError
		wantCode ErrorCode
	}{
		{"ErrInternal", func() *ScintireteError { return ErrInternal("test") }, ErrorCodeInternal},
		{"ErrUnauthorized", func() *ScintireteError { return ErrUnauthorized("test") }, ErrorCodeUnauthorized},
		{"ErrDatabaseNotFound", func() *ScintireteError { return ErrDatabaseNotFound("test_db") }, ErrorCodeDatabaseNotFound},
		{"ErrCollectionNotFound", func() *ScintireteError { return ErrCollectionNotFound("db", "coll") }, ErrorCodeCollectionNotFound},
		{"ErrVectorNotFound", func() *ScintireteError { return ErrVectorNotFound("id123") }, ErrorCodeVectorNotFound},
		{"ErrDimensionMismatch", func() *ScintireteError { return ErrDimensionMismatch(128, 256) }, ErrorCodeDimensionMismatch},
		{"ErrPersistenceFailed", func() *ScintireteError { return ErrPersistenceFailed("test") }, ErrorCodePersistenceFailed},
		{"ErrIndexBuildFailed", func() *ScintireteError { return ErrIndexBuildFailed("test") }, ErrorCodeIndexBuildFailed},
		{"ErrEmbeddingTimeout", func() *ScintireteError { return ErrEmbeddingTimeout() }, ErrorCodeEmbeddingTimeout},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.errFunc()
			if err.Code != test.wantCode {
				t.Errorf("%s().Code = %v, want %v", test.name, err.Code, test.wantCode)
			}
		})
	}
}

func TestErrInternalWithCause(t *testing.T) {
	cause := errors.New("original error")
	err := ErrInternalWithCause("wrapped message", cause)

	if err.Code != ErrorCodeInternal {
		t.Errorf("ErrInternalWithCause().Code = %v, want %v", err.Code, ErrorCodeInternal)
	}
	if err.Cause != cause {
		t.Errorf("ErrInternalWithCause().Cause = %v, want %v", err.Cause, cause)
	}
}

func TestErrPersistenceFailedWithCause(t *testing.T) {
	cause := errors.New("disk error")
	err := ErrPersistenceFailedWithCause("failed to write", cause)

	if err.Code != ErrorCodePersistenceFailed {
		t.Errorf("ErrPersistenceFailedWithCause().Code = %v, want %v", err.Code, ErrorCodePersistenceFailed)
	}
	if err.Cause != cause {
		t.Errorf("ErrPersistenceFailedWithCause().Cause = %v, want %v", err.Cause, cause)
	}
}

func TestIsScintireteError(t *testing.T) {
	scintireteErr := NewError(ErrorCodeInternal, "test")
	regularErr := errors.New("regular error")

	if !IsScintireteError(scintireteErr) {
		t.Error("IsScintireteError should return true for ScintireteError")
	}

	if IsScintireteError(regularErr) {
		t.Error("IsScintireteError should return false for regular error")
	}

	if IsScintireteError(nil) {
		t.Error("IsScintireteError should return false for nil")
	}
}

func TestGetErrorCode(t *testing.T) {
	scintireteErr := NewError(ErrorCodeDatabaseNotFound, "test")
	regularErr := errors.New("regular error")

	if code := GetErrorCode(scintireteErr); code != ErrorCodeDatabaseNotFound {
		t.Errorf("GetErrorCode(ScintireteError) = %v, want %v", code, ErrorCodeDatabaseNotFound)
	}

	if code := GetErrorCode(regularErr); code != ErrorCodeInternal {
		t.Errorf("GetErrorCode(regularError) = %v, want %v", code, ErrorCodeInternal)
	}

	if code := GetErrorCode(nil); code != ErrorCodeInternal {
		t.Errorf("GetErrorCode(nil) = %v, want %v", code, ErrorCodeInternal)
	}
}
