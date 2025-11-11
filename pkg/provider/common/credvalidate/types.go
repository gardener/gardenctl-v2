/*
SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package credvalidate

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	gardensecurityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	corev1 "k8s.io/api/core/v1"

	"github.com/gardener/gardenctl-v2/pkg/provider/common/allowpattern"
)

// ErrPatternMismatch indicates a non-fatal pattern mismatch during credential validation.
var ErrPatternMismatch = errors.New("pattern mismatch")

// UnsafeDebugEnabled returns true if unsafe debug mode is enabled via environment variable.
func UnsafeDebugEnabled() bool {
	value := os.Getenv("GCTL_UNSAFE_DEBUG")
	parsed, err := strconv.ParseBool(value)

	return err == nil && parsed
}

// PatternMismatchError represents a pattern mismatch error during credential validation.
// This type is used to distinguish between pattern mismatches and validation failures.
// Pattern mismatches are expected during pattern iteration, while validation errors
// should be returned immediately.
type PatternMismatchError struct {
	Field         string
	Message       string
	ActualValue   string // Stored separately for debugging, not included in Error() by default
	ExpectedValue string // Optional: what was expected
	NonSensitive  bool   // Whether this field's value is considered non-sensitive for logging/error messages
}

// NewPatternMismatchError creates a new pattern mismatch error.
// Precondition: field and message must be non-empty.
func NewPatternMismatchError(field, message string) *PatternMismatchError {
	return &PatternMismatchError{
		Field:        field,
		Message:      message,
		NonSensitive: false,
	}
}

// NewPatternMismatchErrorWithValues creates a new pattern mismatch error with actual/expected values.
// The values are stored but not included in the error message unless unsafe debug is enabled.
func NewPatternMismatchErrorWithValues(field, message, actualValue, expectedValue string, nonSensitive bool) *PatternMismatchError {
	return &PatternMismatchError{
		Field:         field,
		Message:       message,
		ActualValue:   actualValue,
		ExpectedValue: expectedValue,
		NonSensitive:  nonSensitive,
	}
}

// Error implements the error interface.
func (e *PatternMismatchError) Error() string {
	// Only show values if field is marked non-sensitive in context or if unsafe debug is enabled
	if e.ActualValue != "" && (e.NonSensitive || UnsafeDebugEnabled()) {
		if e.ExpectedValue != "" {
			return fmt.Sprintf("pattern mismatch in field %q: %s (actual: %q, expected: %q)", e.Field, e.Message, e.ActualValue, e.ExpectedValue)
		}

		return fmt.Sprintf("pattern mismatch in field %q: %s (actual: %q)", e.Field, e.Message, e.ActualValue)
	}
	// Default: redacted message
	return fmt.Sprintf("pattern mismatch in field %q: %s", e.Field, e.Message)
}

// Unwrap exposes the sentinel so errors.Is can detect the classification.
func (e *PatternMismatchError) Unwrap() error {
	return ErrPatternMismatch
}

// Validator defines the interface that all provider validators must implement.
type Validator interface {
	// ValidateSecret validates credentials from a Kubernetes secret.
	// Returns a map containing only the validated key-value pairs.
	ValidateSecret(secret *corev1.Secret) (map[string]interface{}, error)

	// ValidateWorkloadIdentityConfig validates workload identity configuration.
	// Returns a map containing only the validated configuration fields.
	ValidateWorkloadIdentityConfig(wi *gardensecurityv1alpha1.WorkloadIdentity) (map[string]interface{}, error)
}

// FieldValidator validates individual fields. The last parameter indicates
// whether the field is non-sensitive for logging/error redaction.
//
// When a FieldValidator needs to validate nested objects (e.g., validating a JSON field),
// it should use ValidateNestedFieldsStrict which enforces strict mode and only returns an error.
// The parent validator automatically collects the entire field value in its validated fields map.
type FieldValidator func(v *BaseValidator, field string, value any, allFields map[string]any, nonSensitive bool) error

// FieldRule defines validation rules for a specific field.
type FieldRule struct {
	Required     bool           // Whether the field is required
	Validator    FieldValidator // Function to validate the field value (preferred)
	NonSensitive bool           // Whether this field's value is safe to log or include in errors
}

// FieldError represents a validation error with support for sensitive value redaction.
type FieldError struct {
	Field        string
	Message      string
	Cause        error
	ActualValue  string // Stored separately, only shown in unsafe debug mode
	NonSensitive bool   // Whether this field's value is considered non-sensitive for logging/error messages
}

// NewFieldError returns a validation error for the given field and message.
// The returned error redacts the underlying cause to avoid leaking sensitive information.
// The nonSensitive parameter controls whether the field is considered non-sensitive for message redaction.
func NewFieldError(field, message string, cause error, nonSensitive bool) error {
	return &FieldError{
		Field:        field,
		Message:      message,
		Cause:        cause,
		ActualValue:  "",
		NonSensitive: nonSensitive,
	}
}

// NewFieldErrorWithValue creates a field error that stores the actual value separately.
func NewFieldErrorWithValue(field, message, actualValue string, cause error, nonSensitive bool) error {
	return &FieldError{
		Field:        field,
		Message:      message,
		Cause:        cause,
		ActualValue:  actualValue,
		NonSensitive: nonSensitive,
	}
}

// Error implements the error interface with automatic redaction.
func (e *FieldError) Error() string {
	var msg string

	if e.ActualValue != "" && (e.NonSensitive || UnsafeDebugEnabled()) {
		msg = fmt.Sprintf("validation error in field %q: %s (value: %q)", e.Field, e.Message, e.ActualValue)
	} else {
		msg = fmt.Sprintf("validation error in field %q: %s", e.Field, e.Message)
	}

	if e.Cause != nil && (e.NonSensitive || UnsafeDebugEnabled()) {
		return fmt.Sprintf("%s: %v", msg, e.Cause)
	}

	return msg
}

// Unwrap returns the underlying cause.
func (e *FieldError) Unwrap() error {
	return e.Cause
}

// PatternMatcher is a function type for matching field values against patterns.
type PatternMatcher func(value string, pattern allowpattern.Pattern, credentials map[string]interface{}, nonSensitive bool) error

// ValidationMode defines the modes for field validation.
type ValidationMode int

const (
	// Strict mode: unknown fields cause an error.
	Strict ValidationMode = iota
	// Permissive mode: unknown fields are ignored.
	Permissive
)
