package apperrors

import (
	"fmt"
	"net/http"
)

type ErrorType string

const (
	ErrRiskReject      ErrorType = "RISK_REJECT"
	ErrAuthFailed      ErrorType = "AUTH_FAILED"
	ErrNonce           ErrorType = "NONCE_ERROR"
	ErrSystemPanic     ErrorType = "SYSTEM_PANIC"
	ErrInvalidRequest  ErrorType = "INVALID_REQUEST"
	ErrInternal        ErrorType = "INTERNAL_ERROR"
	ErrNotFound        ErrorType = "NOT_FOUND"
	ErrUpstream        ErrorType = "UPSTREAM_ERROR"
)

// AppError is the standard error struct for the application
type AppError struct {
	Type       ErrorType `json:"code"`
	Message    string    `json:"message"`
	Suggestion string    `json:"suggestion,omitempty"`
	HTTPStatus int       `json:"-"`
	Cause      error     `json:"-"`
}

func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func New(errType ErrorType, msg string, cause error) *AppError {
	return &AppError{
		Type:    errType,
		Message: msg,
		Cause:   cause,
		HTTPStatus: mapTypeToStatus(errType),
		Suggestion: mapTypeToSuggestion(errType),
	}
}

func NewRiskReject(msg string) *AppError {
	return New(ErrRiskReject, msg, nil)
}

func NewInvalidRequest(msg string) *AppError {
	return New(ErrInvalidRequest, msg, nil)
}

func Wrap(err error) *AppError {
	if err == nil {
		return nil
	}
	if appErr, ok := err.(*AppError); ok {
		return appErr
	}
	return New(ErrInternal, err.Error(), err)
}

func mapTypeToStatus(t ErrorType) int {
	switch t {
	case ErrRiskReject, ErrInvalidRequest:
		return http.StatusBadRequest
	case ErrAuthFailed:
		return http.StatusUnauthorized
	case ErrNonce:
		return http.StatusConflict
	case ErrSystemPanic:
		return http.StatusServiceUnavailable
	case ErrNotFound:
		return http.StatusNotFound
	case ErrUpstream:
		return http.StatusBadGateway
	default:
		return http.StatusInternalServerError
	}
}

func mapTypeToSuggestion(t ErrorType) string {
	switch t {
	case ErrRiskReject:
		return "Check order parameters against risk limits."
	case ErrNonce:
		return "Retry the request."
	case ErrAuthFailed:
		return "Check API keys and signatures."
	case ErrSystemPanic:
		return "Wait for system recovery."
	default:
		return ""
	}
}
