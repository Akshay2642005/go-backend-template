package errs

import (
	"fmt"
	"net/http"
	"strings"
)

type FieldError struct {
	Field string `json:"field"`
	Error string `json:"error"`
}

type ActionType string

const (
	ActionTypeRedirect ActionType = "redirect"
)

type Action struct {
	Type    ActionType `json:"type"`
	Message string     `json:"message"`
	Value   string     `json:"value"`
}

// HTTPError represents an API error response conforming to RFC 7807 Problem Details.
type HTTPError struct {
	// RFC 7807 fields
	Type     string `json:"type"`               // URI reference for the error type
	Title    string `json:"title"`              // Short human-readable summary
	Status   int    `json:"status"`             // HTTP status code
	Detail   string `json:"detail,omitempty"`   // Human-readable explanation
	Instance string `json:"instance,omitempty"` // URI identifying the occurrence

	// Extended fields (backward compatible)
	Code     string       `json:"code"`               // Application-specific error code
	Message  string       `json:"message"`            // Alias for detail (backward compat)
	Override bool         `json:"override,omitempty"` // Whether to expose message to client
	Errors   []FieldError `json:"errors,omitempty"`   // Field-level validation errors
	Action   *Action      `json:"action,omitempty"`   // Action to be taken
}

func (e *HTTPError) Error() string {
	return e.Detail
}

func (e *HTTPError) Is(target error) bool {
	_, ok := target.(*HTTPError)
	return ok
}

func (e *HTTPError) WithMessage(message string) *HTTPError {
	clone := *e
	clone.Message = message
	clone.Detail = message
	return &clone
}

// ProblemURI returns the RFC 7807 type URI for a given error code.
func ProblemURI(code string) string {
	return "/problems/" + strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(code, " ", "_"), "_", "-"))
}

// MakeUpperCaseWithUnderscores converts a string to UPPER_SNAKE_CASE.
func MakeUpperCaseWithUnderscores(str string) string {
	return strings.ToUpper(strings.ReplaceAll(str, " ", "_"))
}

// NewProblem creates an RFC 7807 Problem Details error.
func NewProblem(status int, title, detail string) *HTTPError {
	code := MakeUpperCaseWithUnderscores(title)
	return &HTTPError{
		Type:    ProblemURI(code),
		Title:   title,
		Status:  status,
		Detail:  detail,
		Message: detail,
		Code:    code,
	}
}

// NewProblemWithCode creates an RFC 7807 error with a custom application code.
func NewProblemWithCode(status int, code, title, detail string) *HTTPError {
	return &HTTPError{
		Type:    ProblemURI(code),
		Title:   title,
		Status:  status,
		Detail:  detail,
		Message: detail,
		Code:    code,
	}
}

// ProblemFrom wraps an unknown error as an RFC 7807 problem.
func ProblemFrom(err error, status int) *HTTPError {
	title := http.StatusText(status)
	return &HTTPError{
		Type:    ProblemURI(MakeUpperCaseWithUnderscores(title)),
		Title:   title,
		Status:  status,
		Detail:  err.Error(),
		Message: err.Error(),
		Code:    MakeUpperCaseWithUnderscores(title),
	}
}

// ProblemNotFound creates a 404 Problem Details error.
func ProblemNotFound(resource string) *HTTPError {
	return NewProblemWithCode(
		http.StatusNotFound,
		"NOT_FOUND",
		"Not Found",
		fmt.Sprintf("%s not found", resource),
	)
}

// ProblemValidation creates a 400 Problem Details error for validation failures.
func ProblemValidation(detail string, fieldErrors []FieldError) *HTTPError {
	e := NewProblemWithCode(
		http.StatusBadRequest,
		"VALIDATION_ERROR",
		"Validation Error",
		detail,
	)
	e.Errors = fieldErrors
	return e
}
