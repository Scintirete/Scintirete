// Package utils provides utility functions and error handling for Scintirete.
package utils

import (
	"fmt"
	"strings"
)

// ErrorCode represents different types of errors in the system
type ErrorCode int

const (
	// System errors (1000-1999)
	ErrorCodeInternal ErrorCode = 1000
	ErrorCodeConfig   ErrorCode = 1001
	ErrorCodeTimeout  ErrorCode = 1002
	ErrorCodeResource ErrorCode = 1003

	// Authentication errors (2000-2999)
	ErrorCodeUnauthorized ErrorCode = 2000
	ErrorCodeForbidden    ErrorCode = 2001
	ErrorCodeRateLimited  ErrorCode = 2002

	// Business errors (3000-3999)
	ErrorCodeDatabaseNotFound      ErrorCode = 3000
	ErrorCodeDatabaseAlreadyExists ErrorCode = 3001
	ErrorCodeCollectionNotFound    ErrorCode = 3002
	ErrorCodeCollectionAlreadyExists ErrorCode = 3003
	ErrorCodeVectorNotFound        ErrorCode = 3004
	ErrorCodeDimensionMismatch     ErrorCode = 3005
	ErrorCodeInvalidVectorID       ErrorCode = 3006
	ErrorCodeInvalidParameters     ErrorCode = 3007
	ErrorCodeEmptyCollection       ErrorCode = 3008

	// Persistence errors (4000-4999)
	ErrorCodePersistenceFailed ErrorCode = 4000
	ErrorCodeRecoveryFailed    ErrorCode = 4001
	ErrorCodeCorruptedData     ErrorCode = 4002
	ErrorCodeDiskSpace         ErrorCode = 4003

	// Algorithm errors (5000-5999)
	ErrorCodeIndexBuildFailed ErrorCode = 5000
	ErrorCodeSearchFailed     ErrorCode = 5001
	ErrorCodeInsertFailed     ErrorCode = 5002
	ErrorCodeDeleteFailed     ErrorCode = 5003

	// External service errors (6000-6999)
	ErrorCodeEmbeddingApiFailed ErrorCode = 6000
	ErrorCodeEmbeddingTimeout   ErrorCode = 6001
	ErrorCodeEmbeddingQuotaExceeded ErrorCode = 6002
)

// String returns the string representation of ErrorCode
func (ec ErrorCode) String() string {
	switch ec {
	// System errors
	case ErrorCodeInternal:
		return "INTERNAL"
	case ErrorCodeConfig:
		return "CONFIG"
	case ErrorCodeTimeout:
		return "TIMEOUT"
	case ErrorCodeResource:
		return "RESOURCE"

	// Authentication errors
	case ErrorCodeUnauthorized:
		return "UNAUTHORIZED"
	case ErrorCodeForbidden:
		return "FORBIDDEN"
	case ErrorCodeRateLimited:
		return "RATE_LIMITED"

	// Business errors
	case ErrorCodeDatabaseNotFound:
		return "DATABASE_NOT_FOUND"
	case ErrorCodeDatabaseAlreadyExists:
		return "DATABASE_ALREADY_EXISTS"
	case ErrorCodeCollectionNotFound:
		return "COLLECTION_NOT_FOUND"
	case ErrorCodeCollectionAlreadyExists:
		return "COLLECTION_ALREADY_EXISTS"
	case ErrorCodeVectorNotFound:
		return "VECTOR_NOT_FOUND"
	case ErrorCodeDimensionMismatch:
		return "DIMENSION_MISMATCH"
	case ErrorCodeInvalidVectorID:
		return "INVALID_VECTOR_ID"
	case ErrorCodeInvalidParameters:
		return "INVALID_PARAMETERS"
	case ErrorCodeEmptyCollection:
		return "EMPTY_COLLECTION"

	// Persistence errors
	case ErrorCodePersistenceFailed:
		return "PERSISTENCE_FAILED"
	case ErrorCodeRecoveryFailed:
		return "RECOVERY_FAILED"
	case ErrorCodeCorruptedData:
		return "CORRUPTED_DATA"
	case ErrorCodeDiskSpace:
		return "DISK_SPACE"

	// Algorithm errors
	case ErrorCodeIndexBuildFailed:
		return "INDEX_BUILD_FAILED"
	case ErrorCodeSearchFailed:
		return "SEARCH_FAILED"
	case ErrorCodeInsertFailed:
		return "INSERT_FAILED"
	case ErrorCodeDeleteFailed:
		return "DELETE_FAILED"

	// External service errors
	case ErrorCodeEmbeddingApiFailed:
		return "EMBEDDING_API_FAILED"
	case ErrorCodeEmbeddingTimeout:
		return "EMBEDDING_TIMEOUT"
	case ErrorCodeEmbeddingQuotaExceeded:
		return "EMBEDDING_QUOTA_EXCEEDED"

	default:
		return "UNKNOWN"
	}
}

// ScintireteError represents a structured error in the Scintirete system
type ScintireteError struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
	Cause   error     `json:"cause,omitempty"`
	Context map[string]interface{} `json:"context,omitempty"`
}

// Error implements the error interface
func (e *ScintireteError) Error() string {
	var parts []string
	parts = append(parts, fmt.Sprintf("[%s]", e.Code.String()))
	parts = append(parts, e.Message)
	
	if e.Cause != nil {
		parts = append(parts, fmt.Sprintf("caused by: %v", e.Cause))
	}
	
	return strings.Join(parts, " ")
}

// Unwrap returns the underlying cause error
func (e *ScintireteError) Unwrap() error {
	return e.Cause
}

// WithContext adds context information to the error
func (e *ScintireteError) WithContext(key string, value interface{}) *ScintireteError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// NewError creates a new ScintireteError
func NewError(code ErrorCode, message string) *ScintireteError {
	return &ScintireteError{
		Code:    code,
		Message: message,
	}
}

// NewErrorWithCause creates a new ScintireteError with a cause
func NewErrorWithCause(code ErrorCode, message string, cause error) *ScintireteError {
	return &ScintireteError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

// Error factory functions for common errors

// System errors
func ErrInternal(message string) *ScintireteError {
	return NewError(ErrorCodeInternal, message)
}

func ErrInternalWithCause(message string, cause error) *ScintireteError {
	return NewErrorWithCause(ErrorCodeInternal, message, cause)
}

func ErrConfig(message string) *ScintireteError {
	return NewError(ErrorCodeConfig, message)
}

func ErrTimeout(message string) *ScintireteError {
	return NewError(ErrorCodeTimeout, message)
}

// Authentication errors
func ErrUnauthorized(message string) *ScintireteError {
	return NewError(ErrorCodeUnauthorized, message)
}

func ErrForbidden(message string) *ScintireteError {
	return NewError(ErrorCodeForbidden, message)
}

func ErrRateLimited(message string) *ScintireteError {
	return NewError(ErrorCodeRateLimited, message)
}

// Business errors
func ErrDatabaseNotFound(dbName string) *ScintireteError {
	return NewError(ErrorCodeDatabaseNotFound, fmt.Sprintf("database '%s' not found", dbName))
}

func ErrDatabaseAlreadyExists(dbName string) *ScintireteError {
	return NewError(ErrorCodeDatabaseAlreadyExists, fmt.Sprintf("database '%s' already exists", dbName))
}

func ErrCollectionNotFound(dbName, collName string) *ScintireteError {
	return NewError(ErrorCodeCollectionNotFound, 
		fmt.Sprintf("collection '%s' not found in database '%s'", collName, dbName))
}

func ErrCollectionAlreadyExists(dbName, collName string) *ScintireteError {
	return NewError(ErrorCodeCollectionAlreadyExists, 
		fmt.Sprintf("collection '%s' already exists in database '%s'", collName, dbName))
}

func ErrVectorNotFound(id string) *ScintireteError {
	return NewError(ErrorCodeVectorNotFound, fmt.Sprintf("vector with id '%s' not found", id))
}

func ErrDimensionMismatch(expected, actual int) *ScintireteError {
	return NewError(ErrorCodeDimensionMismatch, 
		fmt.Sprintf("dimension mismatch: expected %d, got %d", expected, actual))
}

func ErrInvalidVectorID(id string) *ScintireteError {
	return NewError(ErrorCodeInvalidVectorID, fmt.Sprintf("invalid vector id: '%s'", id))
}

func ErrInvalidParameters(message string) *ScintireteError {
	return NewError(ErrorCodeInvalidParameters, message)
}

func ErrEmptyCollection(dbName, collName string) *ScintireteError {
	return NewError(ErrorCodeEmptyCollection, 
		fmt.Sprintf("collection '%s' in database '%s' is empty", collName, dbName))
}

// Persistence errors
func ErrPersistenceFailed(message string) *ScintireteError {
	return NewError(ErrorCodePersistenceFailed, message)
}

func ErrPersistenceFailedWithCause(message string, cause error) *ScintireteError {
	return NewErrorWithCause(ErrorCodePersistenceFailed, message, cause)
}

func ErrRecoveryFailed(message string) *ScintireteError {
	return NewError(ErrorCodeRecoveryFailed, message)
}

func ErrCorruptedData(message string) *ScintireteError {
	return NewError(ErrorCodeCorruptedData, message)
}

// Algorithm errors
func ErrIndexBuildFailed(message string) *ScintireteError {
	return NewError(ErrorCodeIndexBuildFailed, message)
}

func ErrSearchFailed(message string) *ScintireteError {
	return NewError(ErrorCodeSearchFailed, message)
}

func ErrInsertFailed(message string) *ScintireteError {
	return NewError(ErrorCodeInsertFailed, message)
}

// External service errors
func ErrEmbeddingApiFailed(message string) *ScintireteError {
	return NewError(ErrorCodeEmbeddingApiFailed, message)
}

func ErrEmbeddingTimeout() *ScintireteError {
	return NewError(ErrorCodeEmbeddingTimeout, "embedding API request timeout")
}

func ErrEmbeddingQuotaExceeded() *ScintireteError {
	return NewError(ErrorCodeEmbeddingQuotaExceeded, "embedding API quota exceeded")
}

// Additional business logic errors
func ErrInvalidInput(message string) *ScintireteError {
	return NewError(ErrorCodeInvalidParameters, message)
}

func ErrCollectionOperationFailed(message string) *ScintireteError {
	return NewError(ErrorCodeCollectionAlreadyExists, message) // Reusing existing code
}

func ErrInvalidVectorDimension(message string) *ScintireteError {
	return NewError(ErrorCodeDimensionMismatch, message)
}

func ErrIndexOperationFailed(message string) *ScintireteError {
	return NewError(ErrorCodeIndexBuildFailed, message)
}

func ErrCollectionEmpty(message string) *ScintireteError {
	return NewError(ErrorCodeEmptyCollection, message)
}



// IsScintireteError checks if an error is a ScintireteError
func IsScintireteError(err error) bool {
	_, ok := err.(*ScintireteError)
	return ok
}

// GetErrorCode extracts the error code from a ScintireteError
func GetErrorCode(err error) ErrorCode {
	if scintireteErr, ok := err.(*ScintireteError); ok {
		return scintireteErr.Code
	}
	return ErrorCodeInternal
}

// Additional database-specific errors
func ErrDatabaseExists(name string) *ScintireteError {
	return NewError(ErrorCodeDatabaseAlreadyExists, fmt.Sprintf("database '%s' already exists", name))
}

func ErrDatabaseOperationFailed(message string) *ScintireteError {
	return NewError(ErrorCodeInternal, message)
}

func ErrCollectionExists(name string) *ScintireteError {
	return NewError(ErrorCodeCollectionAlreadyExists, fmt.Sprintf("collection '%s' already exists", name))
}

func ErrCollectionCreationFailed(message string) *ScintireteError {
	return NewError(ErrorCodeInternal, message)
} 