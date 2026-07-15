// Package apierrors provides structured API errors for extension runtimes.
package apierrors

import (
	"fmt"
	"net/http"
)

type ErrorType string

const (
	ErrorTypeValidation     ErrorType = "validation"
	ErrorTypeAuthentication ErrorType = "authentication"
	ErrorTypeAuthorization  ErrorType = "authorization"
	ErrorTypeNotFound       ErrorType = "not_found"
	ErrorTypeConflict       ErrorType = "conflict"
	ErrorTypeInternal       ErrorType = "internal"
	ErrorTypeExternal       ErrorType = "external"
	ErrorTypeRateLimit      ErrorType = "rate_limit"
	ErrorTypeBadRequest     ErrorType = "bad_request"
	ErrorTypeTimeout        ErrorType = "timeout"
	ErrorTypeUnavailable    ErrorType = "unavailable"
)

type APIError struct {
	Type       ErrorType              `json:"type"`
	Message    string                 `json:"message"`
	Details    map[string]interface{} `json:"details,omitempty"`
	Code       string                 `json:"code,omitempty"`
	StatusCode int                    `json:"-"`
	Cause      error                  `json:"-"`
}

func (e *APIError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

func (e *APIError) Unwrap() error { return e.Cause }

func (e *APIError) Is(target error) bool {
	t, ok := target.(*APIError)
	return ok && e.Type == t.Type
}

func New(errorType ErrorType, message string) *APIError {
	return &APIError{Type: errorType, Message: message, StatusCode: statusCode(errorType)}
}

func Newf(errorType ErrorType, format string, args ...interface{}) *APIError {
	return New(errorType, fmt.Sprintf(format, args...))
}

func Wrap(err error, errorType ErrorType, message string) *APIError {
	return &APIError{Type: errorType, Message: message, Cause: err, StatusCode: statusCode(errorType)}
}

func Wrapf(err error, errorType ErrorType, format string, args ...interface{}) *APIError {
	return Wrap(err, errorType, fmt.Sprintf(format, args...))
}

func (e *APIError) WithDetails(details map[string]interface{}) *APIError {
	e.Details = details
	return e
}

func (e *APIError) WithCode(code string) *APIError {
	e.Code = code
	return e
}

func (e *APIError) WithStatusCode(code int) *APIError {
	e.StatusCode = code
	return e
}

func statusCode(errorType ErrorType) int {
	switch errorType {
	case ErrorTypeValidation, ErrorTypeBadRequest:
		return http.StatusBadRequest
	case ErrorTypeAuthentication:
		return http.StatusUnauthorized
	case ErrorTypeAuthorization:
		return http.StatusForbidden
	case ErrorTypeNotFound:
		return http.StatusNotFound
	case ErrorTypeConflict:
		return http.StatusConflict
	case ErrorTypeRateLimit:
		return http.StatusTooManyRequests
	case ErrorTypeTimeout:
		return http.StatusRequestTimeout
	case ErrorTypeUnavailable:
		return http.StatusServiceUnavailable
	case ErrorTypeExternal:
		return http.StatusBadGateway
	default:
		return http.StatusInternalServerError
	}
}

var (
	ErrNotFound           = New(ErrorTypeNotFound, "Resource not found")
	ErrUnauthorized       = New(ErrorTypeAuthentication, "Authentication required")
	ErrForbidden          = New(ErrorTypeAuthorization, "Access forbidden")
	ErrValidationFailed   = New(ErrorTypeValidation, "Validation failed")
	ErrInternalServer     = New(ErrorTypeInternal, "Internal server error")
	ErrBadRequest         = New(ErrorTypeBadRequest, "Bad request")
	ErrConflict           = New(ErrorTypeConflict, "Resource conflict")
	ErrRateLimit          = New(ErrorTypeRateLimit, "Rate limit exceeded")
	ErrTimeout            = New(ErrorTypeTimeout, "Request timeout")
	ErrServiceUnavailable = New(ErrorTypeUnavailable, "Service unavailable")
)

type ValidationError struct {
	Field   string      `json:"field"`
	Value   interface{} `json:"value,omitempty"`
	Message string      `json:"message"`
	Code    string      `json:"code,omitempty"`
}

type ValidationErrors struct {
	Errors []ValidationError `json:"errors"`
}

func NewValidationError(field, message string) ValidationError {
	return ValidationError{Field: field, Message: message}
}

func NewValidationErrors(errors ...ValidationError) *APIError {
	return &APIError{
		Type:       ErrorTypeValidation,
		Message:    "Validation failed",
		StatusCode: http.StatusBadRequest,
		Details: map[string]interface{}{
			"validation_errors": errors,
		},
	}
}

func NotFoundError(resource, id string) *APIError {
	return Newf(ErrorTypeNotFound, "%s not found", resource).
		WithCode("RESOURCE_NOT_FOUND").
		WithDetails(map[string]interface{}{"resource": resource, "id": id})
}

func DatabaseError(operation string, err error) *APIError {
	return Wrapf(err, ErrorTypeInternal, "Database %s failed", operation).
		WithCode("DATABASE_ERROR").
		WithDetails(map[string]interface{}{"operation": operation})
}
