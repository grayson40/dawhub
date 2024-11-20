package common

import (
	"errors"
	"fmt"
	"net/http"
)

var (
	// Root error types
	ErrNotFound        = errors.New("resource not found")
	ErrInvalidInput    = errors.New("invalid input")
	ErrUnauthorized    = errors.New("unauthorized")
	ErrForbidden       = errors.New("forbidden")
	ErrInternal        = errors.New("internal error")
	ErrConflict        = errors.New("resource conflict")
	ErrValidation      = errors.New("validation error")
	ErrResourceExists  = errors.New("resource already exists")
	ErrDependencyError = errors.New("dependency error")

	// DB error types
	ErrCreateFailed = errors.New("create failed")
	ErrUpdateFailed = errors.New("update failed")
	ErrDeleteFailed = errors.New("delete failed")
	ErrInvalidID    = errors.New("invalid ID")
)

// AppError represents an application-specific error
type AppError struct {
	Err        error
	Message    string
	StatusCode int
	Code       string
	Op         string
	Details    map[string]interface{}
}

// Error satisfies the error interface
func (e *AppError) Error() string {
	if e.Op != "" {
		return fmt.Sprintf("%s: %v", e.Op, e.Err)
	}
	return e.Err.Error()
}

// Unwrap provides compatibility for errors.Is/As
func (e *AppError) Unwrap() error {
	return e.Err
}

// NewError creates a new AppError
func NewError(err error, opts ...ErrorOption) *AppError {
	appErr := &AppError{
		Err:        err,
		StatusCode: http.StatusInternalServerError,
		Details:    make(map[string]interface{}),
	}

	for _, opt := range opts {
		opt(appErr)
	}

	// Set default message if not provided
	if appErr.Message == "" {
		appErr.Message = err.Error()
	}

	// Set default error code if not provided
	if appErr.Code == "" {
		appErr.Code = ErrorCode(err)
	}

	return appErr
}

// ErrorOption represents an option for creating an AppError
type ErrorOption func(*AppError)

// WithMessage sets the human-readable message
func WithMessage(msg string) ErrorOption {
	return func(e *AppError) {
		e.Message = msg
	}
}

// WithStatusCode sets the HTTP status code
func WithStatusCode(code int) ErrorOption {
	return func(e *AppError) {
		e.StatusCode = code
	}
}

// WithCode sets the error code
func WithCode(code string) ErrorOption {
	return func(e *AppError) {
		e.Code = code
	}
}

// WithOperation sets the operation where the error occurred
func WithOperation(op string) ErrorOption {
	return func(e *AppError) {
		e.Op = op
	}
}

// WithDetails adds additional error details
func WithDetails(details map[string]interface{}) ErrorOption {
	return func(e *AppError) {
		for k, v := range details {
			e.Details[k] = v
		}
	}
}

// Helper functions

// IsNotFound checks if the error is a not found error
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsInvalidInput checks if the error is an invalid input error
func IsInvalidInput(err error) bool {
	return errors.Is(err, ErrInvalidInput)
}

// IsUnauthorized checks if the error is an unauthorized error
func IsUnauthorized(err error) bool {
	return errors.Is(err, ErrUnauthorized)
}

// IsForbidden checks if the error is a forbidden error
func IsForbidden(err error) bool {
	return errors.Is(err, ErrForbidden)
}

// ErrorCode returns a string code for the error type
func ErrorCode(err error) string {
	switch {
	case errors.Is(err, ErrNotFound):
		return "NOT_FOUND"
	case errors.Is(err, ErrInvalidInput):
		return "INVALID_INPUT"
	case errors.Is(err, ErrUnauthorized):
		return "UNAUTHORIZED"
	case errors.Is(err, ErrForbidden):
		return "FORBIDDEN"
	case errors.Is(err, ErrInternal):
		return "INTERNAL_ERROR"
	case errors.Is(err, ErrConflict):
		return "CONFLICT"
	case errors.Is(err, ErrValidation):
		return "VALIDATION_ERROR"
	case errors.Is(err, ErrResourceExists):
		return "RESOURCE_EXISTS"
	case errors.Is(err, ErrDependencyError):
		return "DEPENDENCY_ERROR"
	default:
		return "UNKNOWN_ERROR"
	}
}

// ToHTTPStatus maps an error to an HTTP status code
func ToHTTPStatus(err error) int {
	if appErr, ok := err.(*AppError); ok {
		return appErr.StatusCode
	}

	switch {
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrInvalidInput):
		return http.StatusBadRequest
	case errors.Is(err, ErrUnauthorized):
		return http.StatusUnauthorized
	case errors.Is(err, ErrForbidden):
		return http.StatusForbidden
	case errors.Is(err, ErrConflict):
		return http.StatusConflict
	case errors.Is(err, ErrValidation):
		return http.StatusUnprocessableEntity
	default:
		return http.StatusInternalServerError
	}
}

// ValidationError represents a validation error with field-specific details
type ValidationError struct {
	Field   string
	Message string
}

// NewValidationError creates a validation error for a specific field
func NewValidationError(field, message string) error {
	return NewError(ErrValidation,
		WithMessage(message),
		WithStatusCode(http.StatusUnprocessableEntity),
		WithDetails(map[string]interface{}{
			"field": field,
		}),
	)
}

// Example usage
/*
// In your repository
func (r *Repository) FindByID(id string) (*Model, error) {
    if id == "" {
        return nil, NewError(ErrInvalidInput,
            WithMessage("Invalid ID provided"),
            WithOperation("Repository.FindByID"),
            WithDetails(map[string]interface{}{
                "id": id,
            }),
        )
    }
    // ... implementation
}

// In your handler
func (h *Handler) GetItem(c *gin.Context) {
    item, err := h.repo.FindByID(id)
    if err != nil {
        var appErr *AppError
        if errors.As(err, &appErr) {
            c.JSON(appErr.StatusCode, gin.H{
                "error": appErr.Message,
                "code":  appErr.Code,
                "details": appErr.Details,
            })
            return
        }
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "Internal server error",
        })
        return
    }
    // ... success handling
}
*/
