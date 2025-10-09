/*
SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package credvalidate

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"unicode/utf8"

	"k8s.io/klog/v2"

	"github.com/gardener/gardenctl-v2/pkg/provider/common/allowpattern"
)

// BaseValidator provides common validation functionality for all providers.
type BaseValidator struct {
	logger          klog.Logger
	allowedPatterns []allowpattern.Pattern
}

// NewBaseValidator creates a new base validator for the specified provider.
func NewBaseValidator(ctx context.Context, allowedPatterns []allowpattern.Pattern) *BaseValidator {
	return &BaseValidator{
		logger:          klog.FromContext(ctx),
		allowedPatterns: allowedPatterns,
	}
}

// Logger returns the logger instance.
func (v *BaseValidator) Logger() klog.Logger {
	return v.logger
}

// AllowedPatterns returns the list of allowed patterns.
func (v *BaseValidator) AllowedPatterns() []allowpattern.Pattern {
	return v.allowedPatterns
}

// ValidateNestedFieldsStrict validates nested fields using a registry-based approach with strict mode.
// This method is intended for use within FieldValidator implementations to validate nested objects.
// Strict mode is always enforced, ensuring only known fields are present.
// Returns only an error; the parent validator automatically collects the entire field value in its validated fields map.
func (v *BaseValidator) ValidateNestedFieldsStrict(fields map[string]interface{}, registry map[string]FieldRule) error {
	_, err := v.ValidateWithRegistry(fields, registry, Strict)
	return err
}

// ValidateWithRegistry validates fields using a registry-based approach.
// This enforces the registry as an explicit allowlist and provides centralized validation.
// The mode determines whether unknown fields are allowed (Permissive) or not (Strict).
// Returns the validated fields, consisting of only the registry-matched fields present in the input.
func (v *BaseValidator) ValidateWithRegistry(fields map[string]interface{}, registry map[string]FieldRule, mode ValidationMode) (map[string]interface{}, error) {
	validatedFields := make(map[string]interface{})

	if mode == Strict {
		for field := range fields {
			if _, allowed := registry[field]; !allowed {
				return nil, NewFieldError(field, "field is not allowed", nil, false)
			}
		}
	}

	for field, rule := range registry {
		raw, exists := fields[field]
		if !exists {
			if rule.Required {
				return nil, NewFieldError(field, "required field is missing", nil, rule.NonSensitive)
			}

			continue // optional & absent → nothing to validate
		}

		if rule.Required {
			if isEmpty := isFieldEmpty(raw); isEmpty {
				return nil, NewFieldError(field, "required field cannot be empty", nil, rule.NonSensitive)
			}
		}

		if rule.Validator != nil {
			if err := rule.Validator(v, field, raw, fields, rule.NonSensitive); err != nil {
				return nil, err
			}
		}

		validatedFields[field] = raw
	}

	return validatedFields, nil
}

// ValidateFieldExactLength validates that a field has an exact length.
func ValidateFieldExactLength(field, value string, expectedLen int, nonSensitive bool) error {
	if len(value) != expectedLen {
		return NewFieldErrorWithValue(field, fmt.Sprintf("field value must be exactly %d characters, got %d", expectedLen, len(value)), value, nil, nonSensitive)
	}

	return nil
}

// ValidateFieldMinLength validates that a field has a minimum length.
func ValidateFieldMinLength(field, value string, minLen int, nonSensitive bool) error {
	if len(value) < minLen {
		return NewFieldErrorWithValue(field, fmt.Sprintf("field value must be at least %d characters, got %d", minLen, len(value)), value, nil, nonSensitive)
	}

	return nil
}

// ValidateFieldMaxLength validates that a field has a maximum length.
func ValidateFieldMaxLength(field, value string, maxLen int, nonSensitive bool) error {
	if len(value) > maxLen {
		return NewFieldErrorWithValue(field, fmt.Sprintf("field value must be at most %d characters, got %d", maxLen, len(value)), value, nil, nonSensitive)
	}

	return nil
}

// ValidateStringWithPattern creates a validator that validates a string field against patterns using the provided matcher.
func ValidateStringWithPattern(matcher PatternMatcher) FieldValidator {
	return func(v *BaseValidator, field string, val any, allFields map[string]any, nonSensitive bool) error {
		str, ok := val.(string)
		if !ok {
			return NewFieldError(field, "field value must be a string", nil, nonSensitive)
		}

		return v.ValidateFieldPattern(field, str, allFields, matcher, nonSensitive)
	}
}

// ValidateFieldPattern validates a field value against allowed patterns using the provided matcher.
func (v *BaseValidator) ValidateFieldPattern(field, value string, credentials map[string]interface{}, matcher PatternMatcher, nonSensitive bool) error {
	for _, pattern := range v.allowedPatterns {
		if pattern.Field != field {
			continue
		}

		logPatternMatch := func(level int, message string, reason string) {
			includeValue := nonSensitive || UnsafeDebugEnabled()

			args := []any{
				"field", field,
				"patternType", pattern.String(),
			}
			if includeValue {
				args = append(args, "value", value)
			}

			if reason != "" {
				args = append(args, "reason", reason)
			}

			v.logger.V(level).Info(message, args...)
		}

		logPatternMatch(6, "Pattern match attempt", "")

		normalized, err := pattern.ToNormalizedPattern()
		if err != nil {
			return NewFieldError(field, "failed to normalize pattern", err, nonSensitive)
		}

		matchErr := matcher(value, *normalized, credentials, nonSensitive)
		if matchErr == nil {
			logPatternMatch(6, "Pattern match succeeded", "")
			return nil // Found a matching pattern
		}

		logPatternMatch(4, "Pattern match failed", matchErr.Error())

		// Check if this is a pattern mismatch (expected during pattern iteration)
		if errors.Is(matchErr, ErrPatternMismatch) {
			continue
		}

		return matchErr
	}

	return NewPatternMismatchErrorWithValues(field, "does not match any allowed patterns", value, "", nonSensitive)
}

// isFieldEmpty checks "emptiness" using your rules:
// - string: empty if ""
// - []byte: empty if len==0
// - map:    empty if len==0
// - numbers/bools: never empty (even if zero/false)
// - nil (or typed nil pointers/interfaces): empty
// - everything else: treat zero values as empty (safe default).
func isFieldEmpty(v any) bool {
	if v == nil {
		return true
	}

	rv := reflect.ValueOf(v)
	// Unwrap interfaces and pointers to inspect the underlying value.
	for rv.Kind() == reflect.Interface || rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return true
		}

		rv = rv.Elem()
	}

	switch rv.Kind() {
	case reflect.String:
		return rv.Len() == 0
	case reflect.Slice:
		// Mirror your special-case for []byte; if you ONLY want []byte, check the elem type:
		// if rv.Type().Elem().Kind() == reflect.Uint8 { ... }
		return rv.Len() == 0
	case reflect.Map:
		return rv.Len() == 0
	case reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64,
		reflect.Complex64, reflect.Complex128:
		// primitives other than string are never "empty"
		return false
	default:
		// For structs/arrays/etc., "safe default": consider zero value as empty.
		return rv.IsZero()
	}
}

// FlatCoerceBytesToStringsMap converts []byte values in a flat map to UTF-8 strings if valid, otherwise keeps original type.
func FlatCoerceBytesToStringsMap[V any](src map[string]V) map[string]interface{} {
	out := make(map[string]interface{}, len(src))
	for k, v := range src {
		if b, ok := any(v).([]byte); ok && utf8.Valid(b) {
			out[k] = string(b)
		} else {
			out[k] = v
		}
	}

	return out
}
