package response

import (
	"encoding/json"
	"errors"
	"net/http"
)

const contentTypeJSON = "application/json; charset=utf-8"

// Problem is a lightweight, problem-style error response.
// It intentionally matches the shape described in `personal-recipe-app-spec.md`.
type Problem struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

// FieldError represents a single field-level validation error.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationErrors accumulates field-level errors for request validation.
type ValidationErrors struct {
	FieldErrors []FieldError
}

// Add appends a field-level validation error.
func (v *ValidationErrors) Add(field, message string) {
	if v == nil {
		return
	}
	v.FieldErrors = append(v.FieldErrors, FieldError{Field: field, Message: message})
}

// Any reports whether any validation errors were recorded.
func (v *ValidationErrors) Any() bool {
	return v != nil && len(v.FieldErrors) > 0
}

// WriteJSON writes a JSON response with a stable content type.
func WriteJSON(w http.ResponseWriter, status int, value any) error {
	if w == nil {
		return errors.New("response writer is required")
	}

	w.Header().Set("Content-Type", contentTypeJSON)
	w.WriteHeader(status)

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)
	return enc.Encode(value)
}

// WriteProblem writes a problem-style JSON error response.
func WriteProblem(w http.ResponseWriter, status int, code, message string, details any) error {
	return WriteJSON(w, status, Problem{
		Code:    code,
		Message: message,
		Details: details,
	})
}

// WriteValidationProblem writes a 400 response with a consistent validation error shape.
func WriteValidationProblem(w http.ResponseWriter, errs ValidationErrors) error {
	var details any
	if len(errs.FieldErrors) > 0 {
		details = errs.FieldErrors
	}

	return WriteProblem(w, http.StatusBadRequest, "validation_error", "validation failed", details)
}
