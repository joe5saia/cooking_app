package httpapi

import (
	"errors"
	"net/http"

	"github.com/saiaj/cooking_app/backend/internal/httpapi/response"
)

type apiErrorKind string

const (
	apiErrorBadRequest       apiErrorKind = "bad_request"
	apiErrorValidation       apiErrorKind = "validation_error"
	apiErrorUnauthorized     apiErrorKind = "unauthorized"
	apiErrorForbidden        apiErrorKind = "forbidden"
	apiErrorNotFound         apiErrorKind = "not_found"
	apiErrorConflict         apiErrorKind = "conflict"
	apiErrorRateLimited      apiErrorKind = "rate_limited"
	apiErrorRequestTooLarge  apiErrorKind = "request_too_large"
	apiErrorMethodNotAllowed apiErrorKind = "method_not_allowed"
	apiErrorInternalError    apiErrorKind = "internal_error"
)

type apiError struct {
	kind    apiErrorKind
	message string
	details any
	cause   error
}

func (e *apiError) Error() string {
	if e == nil {
		return ""
	}
	if e.cause == nil {
		return e.message
	}
	return e.message + ": " + e.cause.Error()
}

func (e *apiError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func newAPIError(kind apiErrorKind, message string, details any, cause error) *apiError {
	return &apiError{
		kind:    kind,
		message: message,
		details: details,
		cause:   cause,
	}
}

func errBadRequest(message string) error {
	return newAPIError(apiErrorBadRequest, message, nil, nil)
}

func errUnauthorized(message string) error {
	return newAPIError(apiErrorUnauthorized, message, nil, nil)
}

func errForbidden(message string) error {
	return newAPIError(apiErrorForbidden, message, nil, nil)
}

func errNotFound() error {
	return newAPIError(apiErrorNotFound, "not found", nil, nil)
}

func errConflict(message string) error {
	return newAPIError(apiErrorConflict, message, nil, nil)
}

func errRateLimited() error {
	return newAPIError(apiErrorRateLimited, "rate limit exceeded", nil, nil)
}

func errRequestTooLarge() error {
	return newAPIError(apiErrorRequestTooLarge, "request body too large", nil, nil)
}

func errMethodNotAllowed() error {
	return newAPIError(apiErrorMethodNotAllowed, "method not allowed", nil, nil)
}

func errInternal(cause error) error {
	return newAPIError(apiErrorInternalError, "internal error", nil, cause)
}

func errValidation(fieldErrors []response.FieldError) error {
	var details any
	if len(fieldErrors) > 0 {
		details = fieldErrors
	}
	return newAPIError(apiErrorValidation, "validation failed", details, nil)
}

func errValidationField(field, message string) error {
	return errValidation([]response.FieldError{{Field: field, Message: message}})
}

func statusForAPIErrorKind(kind apiErrorKind) int {
	switch kind {
	case apiErrorBadRequest, apiErrorValidation:
		return http.StatusBadRequest
	case apiErrorUnauthorized:
		return http.StatusUnauthorized
	case apiErrorForbidden:
		return http.StatusForbidden
	case apiErrorNotFound:
		return http.StatusNotFound
	case apiErrorConflict:
		return http.StatusConflict
	case apiErrorRateLimited:
		return http.StatusTooManyRequests
	case apiErrorRequestTooLarge:
		return http.StatusRequestEntityTooLarge
	case apiErrorMethodNotAllowed:
		return http.StatusMethodNotAllowed
	case apiErrorInternalError:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

func asAPIError(err error) (*apiError, bool) {
	var e *apiError
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}
