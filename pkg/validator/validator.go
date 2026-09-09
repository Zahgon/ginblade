package validator

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

// validate is the process-wide validator instance used by the HTTP layer.
var validate = validator.New()

// InitValidator is the hook for registering custom validation rules.
func InitValidator() {
}

// Validator adapts the package validator to the echo.Validator interface.
type Validator struct{}

// New returns the request validator used by the HTTP layer.
func New() *Validator {
	return &Validator{}
}

// Validate validates the given struct and returns the raw validator error.
func (*Validator) Validate(i any) error {
	return validate.Struct(i)
}

// HandleValidatorError converts validation errors into a concise client message.
func HandleValidatorError(errs validator.ValidationErrors) string {
	if len(errs) == 0 {
		return "invalid request parameters"
	}

	err := errs[0]
	field := strings.ToLower(err.Field())
	switch err.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", field)
	case "min":
		return fmt.Sprintf("%s must be at least %s", field, err.Param())
	case "max":
		return fmt.Sprintf("%s must be at most %s", field, err.Param())
	default:
		return fmt.Sprintf("%s is invalid", field)
	}
}
