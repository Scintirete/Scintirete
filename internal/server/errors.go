// Package server provides server error definitions.
package server

import "errors"

// Common server errors
var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUnauthenticated    = errors.New("authentication required")
	ErrForbidden          = errors.New("operation forbidden")
)
