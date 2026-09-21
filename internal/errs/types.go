package errs

import (
	"net/http"
)

func NewUnauthorizedError(message string, override bool) *HTTPError {
	code := MakeUpperCaseWithUnderscores(http.StatusText(http.StatusUnauthorized))
	return &HTTPError{
		Type:     ProblemURI(code),
		Title:    http.StatusText(http.StatusUnauthorized),
		Status:   http.StatusUnauthorized,
		Detail:   message,
		Message:  message,
		Code:     code,
		Override: override,
	}
}

func NewForbiddenError(message string, override bool) *HTTPError {
	code := MakeUpperCaseWithUnderscores(http.StatusText(http.StatusForbidden))
	return &HTTPError{
		Type:     ProblemURI(code),
		Title:    http.StatusText(http.StatusForbidden),
		Status:   http.StatusForbidden,
		Detail:   message,
		Message:  message,
		Code:     code,
		Override: override,
	}
}

func NewBadRequestError(message string, override bool, code *string, errors []FieldError, action *Action) *HTTPError {
	formattedCode := MakeUpperCaseWithUnderscores(http.StatusText(http.StatusBadRequest))

	if code != nil {
		formattedCode = *code
	}

	return &HTTPError{
		Type:     ProblemURI(formattedCode),
		Title:    http.StatusText(http.StatusBadRequest),
		Status:   http.StatusBadRequest,
		Detail:   message,
		Message:  message,
		Code:     formattedCode,
		Override: override,
		Errors:   errors,
		Action:   action,
	}
}

func NewNotFoundError(message string, override bool, code *string) *HTTPError {
	formattedCode := MakeUpperCaseWithUnderscores(http.StatusText(http.StatusNotFound))

	if code != nil {
		formattedCode = *code
	}

	return &HTTPError{
		Type:     ProblemURI(formattedCode),
		Title:    http.StatusText(http.StatusNotFound),
		Status:   http.StatusNotFound,
		Detail:   message,
		Message:  message,
		Code:     formattedCode,
		Override: override,
	}
}

func NewInternalServerError() *HTTPError {
	code := MakeUpperCaseWithUnderscores(http.StatusText(http.StatusInternalServerError))
	return &HTTPError{
		Type:    ProblemURI(code),
		Title:   http.StatusText(http.StatusInternalServerError),
		Status:  http.StatusInternalServerError,
		Detail:  "An internal server error occurred",
		Message: http.StatusText(http.StatusInternalServerError),
		Code:    code,
	}
}

func ValidationError(err error) *HTTPError {
	e := NewBadRequestError("Validation failed: "+err.Error(), false, nil, nil, nil)
	e.Title = "Validation Error"
	e.Type = ProblemURI("VALIDATION_ERROR")
	return e
}

// NewConflictError creates a 409 Conflict error for resource conflicts
func NewConflictError(message string, override bool) *HTTPError {
	code := MakeUpperCaseWithUnderscores(http.StatusText(http.StatusConflict))
	return &HTTPError{
		Type:     ProblemURI(code),
		Title:    http.StatusText(http.StatusConflict),
		Status:   http.StatusConflict,
		Detail:   message,
		Message:  message,
		Code:     code,
		Override: override,
	}
}

// NewUnprocessableEntityError creates a 422 Unprocessable Entity error
func NewUnprocessableEntityError(message string, override bool) *HTTPError {
	code := MakeUpperCaseWithUnderscores(http.StatusText(http.StatusUnprocessableEntity))
	return &HTTPError{
		Type:     ProblemURI(code),
		Title:    http.StatusText(http.StatusUnprocessableEntity),
		Status:   http.StatusUnprocessableEntity,
		Detail:   message,
		Message:  message,
		Code:     code,
		Override: override,
	}
}

// NewRateLimitError creates a 429 Too Many Requests error
func NewRateLimitError(message string, override bool) *HTTPError {
	code := MakeUpperCaseWithUnderscores(http.StatusText(http.StatusTooManyRequests))
	return &HTTPError{
		Type:     ProblemURI(code),
		Title:    http.StatusText(http.StatusTooManyRequests),
		Status:   http.StatusTooManyRequests,
		Detail:   message,
		Message:  message,
		Code:     code,
		Override: override,
	}
}
