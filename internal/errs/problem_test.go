package errs

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPErrorIsRFC7807(t *testing.T) {
	err := NewBadRequestError("invalid name", false, nil, nil, nil)

	assert.Equal(t, http.StatusBadRequest, err.Status)
	assert.Contains(t, err.Type, "/problems/")
	assert.Equal(t, "Bad Request", err.Title)
	assert.Equal(t, "invalid name", err.Detail)
	assert.Equal(t, "invalid name", err.Message)
}

func TestProblemNotFound(t *testing.T) {
	err := ProblemNotFound("User")

	assert.Equal(t, http.StatusNotFound, err.Status)
	assert.Equal(t, "/problems/not-found", err.Type)
	assert.Equal(t, "Not Found", err.Title)
	assert.Equal(t, "User not found", err.Detail)
}

func TestProblemValidation(t *testing.T) {
	fields := []FieldError{
		{Field: "email", Error: "is required"},
		{Field: "name", Error: "must be at least 2 characters"},
	}
	err := ProblemValidation("Validation failed", fields)

	assert.Equal(t, http.StatusBadRequest, err.Status)
	assert.Equal(t, "/problems/validation-error", err.Type)
	assert.Equal(t, "Validation Error", err.Title)
	assert.Len(t, err.Errors, 2)
	assert.Equal(t, "email", err.Errors[0].Field)
}

func TestProblemFrom(t *testing.T) {
	orig := errors.New("something broke")
	err := ProblemFrom(orig, http.StatusBadGateway)

	assert.Equal(t, http.StatusBadGateway, err.Status)
	assert.Equal(t, "Bad Gateway", err.Title)
	assert.Equal(t, "something broke", err.Detail)
}

func TestProblemSerialization(t *testing.T) {
	err := NewBadRequestError("field missing", false, nil, []FieldError{
		{Field: "email", Error: "is required"},
	}, nil)
	err.Instance = "req-abc-123"

	data, err2 := json.Marshal(err)
	require.NoError(t, err2)

	var parsed map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &parsed))

	assert.Equal(t, float64(400), parsed["status"])
	assert.Equal(t, "field missing", parsed["detail"])
	assert.Equal(t, "field missing", parsed["message"])
	assert.Equal(t, "req-abc-123", parsed["instance"])
	assert.Contains(t, parsed["type"], "/problems/")
	assert.NotNil(t, parsed["errors"])
}

func TestMakeUpperCaseWithUnderscores(t *testing.T) {
	assert.Equal(t, "BAD_REQUEST", MakeUpperCaseWithUnderscores("Bad Request"))
	assert.Equal(t, "NOT_FOUND", MakeUpperCaseWithUnderscores("Not Found"))
	assert.Equal(t, "INTERNAL_SERVER_ERROR", MakeUpperCaseWithUnderscores("Internal Server Error"))
}

func TestProblemURI(t *testing.T) {
	assert.Equal(t, "/problems/bad-request", ProblemURI("BAD_REQUEST"))
	assert.Equal(t, "/problems/not-found", ProblemURI("NOT_FOUND"))
	assert.Equal(t, "/problems/validation-error", ProblemURI("VALIDATION_ERROR"))
}

func TestWithMessagePreservesRFC7807(t *testing.T) {
	err := NewBadRequestError("original", false, nil, nil, nil)
	clone := err.WithMessage("updated")

	assert.Equal(t, "updated", clone.Detail)
	assert.Equal(t, "updated", clone.Message)
	assert.Equal(t, err.Status, clone.Status)
	assert.Equal(t, err.Type, clone.Type)
	assert.Equal(t, err.Title, clone.Title)
}

func TestNewInternalServerError(t *testing.T) {
	err := NewInternalServerError()

	assert.Equal(t, http.StatusInternalServerError, err.Status)
	assert.Equal(t, "Internal Server Error", err.Title)
	assert.Equal(t, "An internal server error occurred", err.Detail)
	assert.Equal(t, "/problems/internal-server-error", err.Type)
	assert.False(t, err.Override)
}

func TestIsHTTPError(t *testing.T) {
	err := NewBadRequestError("test", false, nil, nil, nil)
	assert.True(t, errors.Is(err, &HTTPError{}))

	other := errors.New("not an HTTPError")
	assert.False(t, errors.Is(other, &HTTPError{}))
}
