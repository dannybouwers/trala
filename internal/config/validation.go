package config

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

// Validate checks the configuration struct against struct-tag rules.
func Validate(c *TralaConfiguration) error {
	if c == nil {
		return fmt.Errorf("configuration validation failed: nil config")
	}

	validate := validator.New()

	// Return yaml field names in error messages
	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("yaml"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})

	if err := validate.Struct(c); err != nil {
		if verrs, ok := err.(validator.ValidationErrors); ok {
			return fmt.Errorf("configuration validation failed: %s", formatValidationErrors(verrs))
		}
		return fmt.Errorf("configuration validation failed: %w", err)
	}
	return nil
}

func formatValidationErrors(verrs validator.ValidationErrors) string {
	var msgs []string
	for _, verr := range verrs {
		field := verr.Field()
		tag := verr.Tag()
		param := verr.Param()

		var msg string
		switch tag {
		case "required":
			msg = fmt.Sprintf("field '%s' is required", field)
		case "url":
			msg = fmt.Sprintf("field '%s' must be a valid URL", field)
		case "gte":
			msg = fmt.Sprintf("field '%s' must be >= %s (got %v)", field, param, verr.Value())
		case "lte":
			msg = fmt.Sprintf("field '%s' must be <= %s (got %v)", field, param, verr.Value())
		case "gt":
			msg = fmt.Sprintf("field '%s' must be > %s (got %v)", field, param, verr.Value())
		case "oneof":
			msg = fmt.Sprintf("field '%s' must be one of [%s] (got %v)", field, param, verr.Value())
		default:
			msg = fmt.Sprintf("field '%s' failed '%s' validation", field, tag)
		}
		msgs = append(msgs, msg)
	}
	return strings.Join(msgs, "; ")
}
