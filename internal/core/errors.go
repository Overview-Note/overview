package core

import (
	"errors"
	"fmt"
)

// Sentinel errors that adapters map to transport-level responses.
var (
	// ErrNotFound indicates the requested resource does not exist.
	ErrNotFound = errors.New("not found")
	// ErrConflict indicates a version or existence conflict.
	ErrConflict = errors.New("conflict")
	// ErrInvalid indicates malformed or rejected input.
	ErrInvalid = errors.New("invalid argument")
	// ErrUnauthorized indicates missing or invalid credentials.
	ErrUnauthorized = errors.New("unauthorized")
	// ErrForbidden indicates the authenticated user lacks permission.
	ErrForbidden = errors.New("forbidden")
)

// Invalidf wraps ErrInvalid with a formatted message.
func Invalidf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, args...))
}

// Conflictf wraps ErrConflict with a formatted message.
func Conflictf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrConflict, fmt.Sprintf(format, args...))
}

// Forbiddenf wraps ErrForbidden with a formatted message.
func Forbiddenf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrForbidden, fmt.Sprintf(format, args...))
}
