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
		fullPath := buildYAMLPath(verr)
		envVar := envVarForField(fullPath)
		msg := formatValidationMessage(verr, fullPath, envVar)
		msgs = append(msgs, msg)
	}
	return strings.Join(msgs, "; ")
}

// buildYAMLPath constructs the full dot-separated YAML path (e.g. "environment.traefik.api_host")
// from the struct namespace returned by the validator library.
func buildYAMLPath(verr validator.FieldError) string {
	ns := verr.Namespace()
	if ns == "" {
		return verr.Field()
	}

	// Strip leading "TralaConfiguration." prefix
	ns = strings.TrimPrefix(ns, "TralaConfiguration.")

	parts := strings.Split(ns, ".")
	var pathParts []string
	for _, part := range parts {
		if tag, ok := yamlTagForPath[part]; ok {
			pathParts = append(pathParts, tag)
		} else {
			pathParts = append(pathParts, part)
		}
	}

	return strings.Join(pathParts, ".")
}

// envVarForField returns the corresponding environment variable name when the
// given YAML path identifies an Environment field, or "" otherwise.  It is a
// thin delegation to the single authoritative implementation in models.go.
func envVarForField(path string) string {
	return EnvironmentEnvVar(path)
}

// formatValidationMessage builds a human-readable error message for a single validation error.
// When envVar is non-empty the message is prefixed with the config-field / env-var hint;
// for a "required" violation the hint itself is sufficient (it already conveys that the
// field is absent).  For all other violation types the specific validation message is
// appended after the hint.
func formatValidationMessage(verr validator.FieldError, fullPath, envVar string) string {
	tag := verr.Tag()
	param := verr.Param()

	switch tag {
	case "required":
		if envVar != "" {
			return fmt.Sprintf("config field %s or env var %s is required", fullPath, envVar)
		}
		return fmt.Sprintf("field '%s' is required", fullPath)
	case "url":
		detail := fmt.Sprintf("must be a valid URL")
		return enrichmentMsg(envVar, fullPath, detail)
	case "gte":
		detail := fmt.Sprintf("must be >= %s (got %v)", param, verr.Value())
		return enrichmentMsg(envVar, fullPath, detail)
	case "lte":
		detail := fmt.Sprintf("must be <= %s (got %v)", param, verr.Value())
		return enrichmentMsg(envVar, fullPath, detail)
	case "gt":
		detail := fmt.Sprintf("must be > %s (got %v)", param, verr.Value())
		return enrichmentMsg(envVar, fullPath, detail)
	case "oneof":
		detail := fmt.Sprintf("must be one of [%s] (got %v)", param, verr.Value())
		return enrichmentMsg(envVar, fullPath, detail)
	default:
		detail := fmt.Sprintf("failed '%s' validation", tag)
		return enrichmentMsg(envVar, fullPath, detail)
	}
}

// enrichmentMsg prepends the env-var hint to the detail string when envVar is set.
func enrichmentMsg(envVar, fullPath, detail string) string {
	if envVar == "" {
		return detail
	}
	return fmt.Sprintf("config field %s or env var %s: %s", fullPath, envVar, detail)
}
