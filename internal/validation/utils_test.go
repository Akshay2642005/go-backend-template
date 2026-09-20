package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"backend/internal/errs"
)

type testRequest struct {
	Name  string `json:"name" validate:"required,min=2,max=50"`
	Email string `json:"email" validate:"required,email"`
	Age   int    `json:"age" validate:"min=0,max=150"`
}

func (t testRequest) Validate() error {
	return nil
}

func TestExtractValidatorErrors_Required(t *testing.T) {
	req := testRequest{Name: "", Email: "", Age: 0}
	fieldErrors := validateStructTags(req)

	require.Len(t, fieldErrors, 2) // name + email (age=0 passes min=0)

	var foundName, foundEmail bool
	for _, fe := range fieldErrors {
		switch fe.Field {
		case "name":
			assert.Equal(t, "is required", fe.Error)
			foundName = true
		case "email":
			assert.Equal(t, "is required", fe.Error)
			foundEmail = true
		}
	}
	assert.True(t, foundName)
	assert.True(t, foundEmail)
}

func TestExtractValidatorErrors_MinLength(t *testing.T) {
	req := testRequest{Name: "A", Email: "test@example.com", Age: 25}
	fieldErrors := validateStructTags(req)

	require.Len(t, fieldErrors, 1)
	assert.Equal(t, "name", fieldErrors[0].Field)
	assert.Equal(t, "must be at least 2 characters", fieldErrors[0].Error)
}

func TestExtractValidatorErrors_Email(t *testing.T) {
	req := testRequest{Name: "Alice", Email: "not-an-email", Age: 25}
	fieldErrors := validateStructTags(req)

	require.Len(t, fieldErrors, 1)
	assert.Equal(t, "email", fieldErrors[0].Field)
	assert.Equal(t, "must be a valid email address", fieldErrors[0].Error)
}

func TestExtractValidatorErrors_Multiple(t *testing.T) {
	req := testRequest{Name: "", Email: "bad", Age: 200}
	fieldErrors := validateStructTags(req)

	assert.GreaterOrEqual(t, len(fieldErrors), 2)
}

func TestExtractValidatorErrors_Valid(t *testing.T) {
	req := testRequest{Name: "Alice", Email: "alice@example.com", Age: 30}
	fieldErrors := validateStructTags(req)

	assert.Empty(t, fieldErrors)
}

type customValidRequest struct {
	Name string `json:"name"`
}

func (c customValidRequest) Validate() error {
	if c.Name == "admin" {
		return CustomValidationErrors{
			{Field: "name", Message: "admin is a reserved name"},
		}
	}
	return nil
}

func TestExtractCustomErrors(t *testing.T) {
	req := customValidRequest{Name: "admin"}
	fieldErrors := validatePayload(req)

	require.Len(t, fieldErrors, 1)
	assert.Equal(t, "name", fieldErrors[0].Field)
	assert.Equal(t, "admin is a reserved name", fieldErrors[0].Error)
}

func TestExtractCustomErrors_NoError(t *testing.T) {
	req := customValidRequest{Name: "alice"}
	fieldErrors := validatePayload(req)

	assert.Empty(t, fieldErrors)
}

func TestIsValidUUID(t *testing.T) {
	assert.True(t, IsValidUUID("550e8400-e29b-41d4-a716-446655440000"))
	assert.True(t, IsValidUUID("550E8400-E29B-41D4-A716-446655440000"))
	assert.False(t, IsValidUUID("not-a-uuid"))
	assert.False(t, IsValidUUID(""))
	assert.False(t, IsValidUUID("550e8400-e29b-41d4-a716"))
}

func TestCustomValidationErrors_Error(t *testing.T) {
	errs := CustomValidationErrors{
		{Field: "name", Message: "is required"},
	}
	assert.Equal(t, "Validation failed", errs.Error())
}

func TestBindJSON_ValidationError(t *testing.T) {
	// Test that validateStructTags returns proper FieldErrors
	type payload struct {
		Name string `json:"name" validate:"required"`
	}
	p := payload{Name: ""}
	fieldErrors := validateStructTags(&p)

	require.Len(t, fieldErrors, 1)
	assert.Equal(t, errs.FieldError{Field: "name", Error: "is required"}, fieldErrors[0])
}

func TestExtractValidatorErrors_OneOf(t *testing.T) {
	type statusReq struct {
		Status string `json:"status" validate:"oneof=active inactive pending"`
	}
	req := statusReq{Status: "deleted"}
	fieldErrors := validateStructTags(req)

	require.Len(t, fieldErrors, 1)
	assert.Equal(t, "must be one of: active inactive pending", fieldErrors[0].Error)
}
