package validation

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"

	"backend/internal/errs"
)

// Validatable is implemented by request types that define custom validation
// beyond struct-tag validation.
type Validatable interface {
	Validate() error
}

// CustomValidationError represents a single field-level validation failure.
type CustomValidationError struct {
	Field   string
	Message string
}

// CustomValidationErrors is a collection of custom validation errors.
type CustomValidationErrors []CustomValidationError

func (c CustomValidationErrors) Error() string {
	return "Validation failed"
}

// BindAndValidate binds the request body to payload, runs the Validatable
// interface check, and returns an RFC 7807 ProblemValidation error on failure.
func BindAndValidate(c echo.Context, payload Validatable) error {
	if err := c.Bind(payload); err != nil {
		message := strings.Split(strings.Split(err.Error(), ",")[1], "message=")[1]
		return errs.NewBadRequestError(message, false, nil, nil, nil)
	}

	if fieldErrors := validatePayload(payload); len(fieldErrors) > 0 {
		return errs.ProblemValidation("Validation failed", fieldErrors)
	}

	return nil
}

// BindJSON binds the request body to payload and runs struct-tag validation
// via validator/v10. Returns an RFC 7807 ProblemValidation error on failure.
// Use this for types that only need struct-tag validation (no custom Validate()).
func BindJSON(c echo.Context, payload interface{}) error {
	if err := c.Bind(payload); err != nil {
		return errs.NewBadRequestError(err.Error(), false, nil, nil, nil)
	}

	if fieldErrors := validateStructTags(payload); len(fieldErrors) > 0 {
		return errs.ProblemValidation("Validation failed", fieldErrors)
	}

	return nil
}

// validatePayload runs the Validatable interface check, then struct-tag validation.
func validatePayload(v Validatable) []errs.FieldError {
	var fieldErrors []errs.FieldError

	// Custom validation via the Validatable interface
	if err := v.Validate(); err != nil {
		fieldErrors = append(fieldErrors, extractCustomErrors(err)...)
	}

	// Struct-tag validation
	fieldErrors = append(fieldErrors, validateStructTags(v)...)

	return fieldErrors
}

// validateStructTags validates struct tags using validator/v10.
func validateStructTags(v interface{}) []errs.FieldError {
	validate := validator.New()
	if err := validate.Struct(v); err != nil {
		return extractValidatorErrors(err)
	}
	return nil
}

// extractCustomErrors converts CustomValidationErrors into FieldErrors.
func extractCustomErrors(err error) []errs.FieldError {
	var fieldErrors []errs.FieldError

	switch e := err.(type) {
	case CustomValidationErrors:
		for _, ve := range e {
			fieldErrors = append(fieldErrors, errs.FieldError{
				Field: ve.Field,
				Error: ve.Message,
			})
		}
	case validator.ValidationErrors:
		fieldErrors = append(fieldErrors, extractValidatorErrors(err)...)
	}

	return fieldErrors
}

// extractValidatorErrors converts validator.ValidationErrors into FieldErrors
// with human-readable messages.
func extractValidatorErrors(err error) []errs.FieldError {
	validationErrors, ok := err.(validator.ValidationErrors)
	if !ok {
		return nil
	}

	var fieldErrors []errs.FieldError

	for _, fe := range validationErrors {
		field := strings.ToLower(fe.Field())
		var msg string

		switch fe.Tag() {
		case "required":
			msg = "is required"
		case "min":
			if fe.Type().Kind() == reflect.String {
				msg = fmt.Sprintf("must be at least %s characters", fe.Param())
			} else {
				msg = fmt.Sprintf("must be at least %s", fe.Param())
			}
		case "max":
			if fe.Type().Kind() == reflect.String {
				msg = fmt.Sprintf("must not exceed %s characters", fe.Param())
			} else {
				msg = fmt.Sprintf("must not exceed %s", fe.Param())
			}
		case "oneof":
			msg = fmt.Sprintf("must be one of: %s", fe.Param())
		case "email":
			msg = "must be a valid email address"
		case "e164":
			msg = "must be a valid phone number with country code"
		case "uuid":
			msg = "must be a valid UUID"
		case "uuidList":
			msg = "must be a comma-separated list of valid UUIDs"
		case "dive":
			msg = "some items are invalid"
		default:
			if fe.Param() != "" {
				msg = fmt.Sprintf("%s: %s:%s", field, fe.Tag(), fe.Param())
			} else {
				msg = fmt.Sprintf("%s: %s", field, fe.Tag())
			}
		}

		fieldErrors = append(fieldErrors, errs.FieldError{
			Field: field,
			Error: msg,
		})
	}

	return fieldErrors
}

var uuidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func IsValidUUID(uuid string) bool {
	return uuidRegex.MatchString(uuid)
}
