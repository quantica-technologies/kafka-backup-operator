package errors

import (
	"errors"
	"fmt"
)

// Error codes
const (
	ErrCodeInternal         = "INTERNAL_ERROR"
	ErrCodeNotFound         = "NOT_FOUND"
	ErrCodeAlreadyExists    = "ALREADY_EXISTS"
	ErrCodeInvalidArgument  = "INVALID_ARGUMENT"
	ErrCodeUnavailable      = "UNAVAILABLE"
	ErrCodePermissionDenied = "PERMISSION_DENIED"
)

// AppError represents an application error
type AppError struct {
	Code    string
	Message string
	Err     error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error {
	return e.Err
}

// New creates a new error
func New(code, message string) error {
	return &AppError{
		Code:    code,
		Message: message,
	}
}

// Wrap wraps an error with a message
func Wrap(err error, code, message string) error {
	if err == nil {
		return nil
	}
	return &AppError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// Is checks if an error is of a specific type
func Is(err error, target error) bool {
	return errors.Is(err, target)
}

// As finds the first error in err's chain that matches target
func As(err error, target interface{}) bool {
	return errors.As(err, target)
}

// Common errors
var (
	ErrNotFound         = New(ErrCodeNotFound, "resource not found")
	ErrAlreadyExists    = New(ErrCodeAlreadyExists, "resource already exists")
	ErrInvalidArgument  = New(ErrCodeInvalidArgument, "invalid argument")
	ErrUnavailable      = New(ErrCodeUnavailable, "service unavailable")
	ErrPermissionDenied = New(ErrCodePermissionDenied, "permission denied")
)
