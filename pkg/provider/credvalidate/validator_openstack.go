/*
SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package credvalidate

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"github.com/gardener/gardenctl-v2/pkg/provider/common/allowpattern"
	"github.com/gardener/gardenctl-v2/pkg/provider/common/credvalidate"
)

const (
	tenantNameMaxLen                  = 64
	domainNameMaxLen                  = 64
	usernameMaxLen                    = 255
	passwordMaxLen                    = 4096
	applicationCredentialIDMaxLen     = 255
	applicationCredentialNameMaxLen   = 255
	applicationCredentialSecretMaxLen = 4096
)

// OpenStackValidator implements the common Validator interface for OpenStack.
type OpenStackValidator struct {
	*credvalidate.BaseValidator
}

var _ credvalidate.Validator = &OpenStackValidator{}

// NewOpenStackValidator creates a new OpenStack validator.
func NewOpenStackValidator(ctx context.Context, allowedPatterns []allowpattern.Pattern) *OpenStackValidator {
	return &OpenStackValidator{
		BaseValidator: credvalidate.NewBaseValidator(ctx, allowedPatterns),
	}
}

// authMethod represents the authentication method detected from secret fields.
type authMethod int

const (
	InvalidAuth authMethod = iota
	PasswordAuth
	ApplicationCredentialAuth
)

// ValidateSecret validates OpenStack credentials from a Kubernetes secret.
func (v *OpenStackValidator) ValidateSecret(secret *corev1.Secret) (map[string]interface{}, error) {
	// Check for conflicting authentication methods first
	hasPassword := len(secret.Data["password"]) > 0
	hasAppSecret := len(secret.Data["applicationCredentialSecret"]) > 0

	if hasPassword && hasAppSecret {
		return nil, fmt.Errorf("cannot specify both 'password' and 'applicationCredentialSecret'")
	}

	// Detect authentication method and route to appropriate sub-validator
	authMethod := detectAuthMethod(secret)

	switch authMethod {
	case PasswordAuth:
		return v.validatePasswordAuth(secret)
	case ApplicationCredentialAuth:
		return v.validateAppCredentialAuth(secret)
	default:
		return nil, fmt.Errorf("must either specify 'password' or 'applicationCredentialSecret'")
	}
}

// validatePasswordAuth validates OpenStack password-based authentication.
func (v *OpenStackValidator) validatePasswordAuth(secret *corev1.Secret) (map[string]interface{}, error) {
	fields := credvalidate.FlatCoerceBytesToStringsMap(secret.Data)

	registry := map[string]credvalidate.FieldRule{
		"domainName": {
			Required:     true,
			Validator:    validateDomainName,
			NonSensitive: true,
		},
		"tenantName": {
			Required:     true,
			Validator:    validateTenantName,
			NonSensitive: true,
		},
		"username": {
			Required:     true,
			Validator:    validateUsername,
			NonSensitive: true,
		},
		"password": {
			Required:     true,
			Validator:    validatePassword,
			NonSensitive: false,
		},
	}

	return v.ValidateWithRegistry(fields, registry, credvalidate.Permissive)
}

// validateAppCredentialAuth validates OpenStack application credential authentication.
func (v *OpenStackValidator) validateAppCredentialAuth(secret *corev1.Secret) (map[string]interface{}, error) {
	fields := credvalidate.FlatCoerceBytesToStringsMap(secret.Data)

	registry := map[string]credvalidate.FieldRule{
		"domainName": {
			Required:     false,
			Validator:    validateDomainName,
			NonSensitive: true,
		},
		"applicationCredentialID": {
			Required:     false, // either ID or Name must be provided
			Validator:    validateApplicationCredentialID,
			NonSensitive: true,
		},
		"applicationCredentialName": {
			Required:     false, // either ID or Name must be provided
			Validator:    validateApplicationCredentialName,
			NonSensitive: true,
		},
		"applicationCredentialSecret": {
			Required:     true,
			Validator:    validateApplicationCredentialSecret,
			NonSensitive: false,
		},
	}

	// First validate with registry
	validatedFields, err := v.ValidateWithRegistry(fields, registry, credvalidate.Permissive)
	if err != nil {
		return nil, err
	}

	// Then apply application credential specific validation
	applicationCredentialID, err := stringValue(fields, "applicationCredentialID")
	if err != nil {
		return nil, err
	}

	applicationCredentialName, err := stringValue(fields, "applicationCredentialName")
	if err != nil {
		return nil, err
	}

	if applicationCredentialID == "" && applicationCredentialName == "" {
		return nil, fmt.Errorf("either 'applicationCredentialID' or 'applicationCredentialName' must be provided")
	}

	if applicationCredentialName != "" {
		domainName, err := stringValue(fields, "domainName")
		if err != nil {
			return nil, err
		}

		if domainName == "" {
			return nil, fmt.Errorf("'domainName' must be provided when using 'applicationCredentialName'")
		}
	}

	return validatedFields, nil
}

// ValidateAuthURL validates the authURL field format.
func (v *OpenStackValidator) ValidateAuthURL(authURL string) error {
	fields := map[string]interface{}{
		"authURL": authURL,
	}

	registry := map[string]credvalidate.FieldRule{
		"authURL": {
			Required:     true,
			Validator:    credvalidate.ValidateStringWithPattern(credvalidate.MatchURIPattern),
			NonSensitive: true,
		},
	}

	return v.ValidateNestedFieldsStrict(fields, registry)
}

// GetOpenStackValidationContext returns the validation context for OpenStack patterns.
func GetOpenStackValidationContext() *allowpattern.ValidationContext {
	return &allowpattern.ValidationContext{
		AllowedRegexFields: map[string]bool{},
		StrictHTTPS:        false, // controlled by patterns
		AllowedUserConfigurableFields: map[string]bool{
			"authURL": true,
		},
	}
}

// DefaultOpenStackAllowedPatterns returns the default allowed patterns for OpenStack credential fields.
// Note: Only the 'authURL' field is supported for pattern validation in OpenStack.
// This returns an empty slice as defaults because OpenStack auth endpoints are installation-specific (no built-in defaults).
func DefaultOpenStackAllowedPatterns() []allowpattern.Pattern {
	return []allowpattern.Pattern{}
}

// stringValue extracts a string value from a map, returning empty string if not found.
func stringValue(fields map[string]interface{}, field string) (string, error) {
	if value, ok := fields[field]; ok {
		str, ok := value.(string)
		if !ok {
			return "", credvalidate.NewFieldError(field, "field value must be a string", nil, true)
		}

		return str, nil
	}

	return "", nil
}

// detectAuthMethod determines which authentication method is being used based on secret fields.
func detectAuthMethod(secret *corev1.Secret) authMethod {
	hasPassword := len(secret.Data["password"]) > 0
	hasAppSecret := len(secret.Data["applicationCredentialSecret"]) > 0

	switch {
	case hasPassword && !hasAppSecret:
		return PasswordAuth
	case hasAppSecret && !hasPassword:
		return ApplicationCredentialAuth
	default:
		return InvalidAuth
	}
}

// Field-specific validator functions that implement the credvalidate.FieldValidator interface.

// validateDomainName validates the domainName field.
func validateDomainName(v *credvalidate.BaseValidator, field string, val any, all map[string]any, nonSensitive bool) error {
	str, err := credvalidate.AssertStringWithPrintableCheck(field, val, nonSensitive)
	if err != nil {
		return err
	}

	if err := credvalidate.ValidateFieldMaxLength(field, str, domainNameMaxLen, nonSensitive); err != nil {
		return err
	}

	return nil
}

var _ credvalidate.FieldValidator = validateDomainName

// validateTenantName validates the tenantName field.
func validateTenantName(v *credvalidate.BaseValidator, field string, val any, all map[string]any, nonSensitive bool) error {
	str, err := credvalidate.AssertStringWithPrintableCheck(field, val, nonSensitive)
	if err != nil {
		return err
	}

	if err := credvalidate.ValidateFieldMaxLength(field, str, tenantNameMaxLen, nonSensitive); err != nil {
		return err
	}

	return nil
}

var _ credvalidate.FieldValidator = validateTenantName

// validateUsername validates the username field.
func validateUsername(v *credvalidate.BaseValidator, field string, val any, all map[string]any, nonSensitive bool) error {
	str, err := credvalidate.AssertStringWithPrintableCheck(field, val, nonSensitive)
	if err != nil {
		return err
	}

	if err := credvalidate.ValidateFieldMaxLength(field, str, usernameMaxLen, nonSensitive); err != nil {
		return err
	}

	return nil
}

var _ credvalidate.FieldValidator = validateUsername

// validatePassword validates the password field.
func validatePassword(v *credvalidate.BaseValidator, field string, val any, all map[string]any, nonSensitive bool) error {
	str, err := credvalidate.AssertStringWithPrintableCheck(field, val, nonSensitive)
	if err != nil {
		return err
	}

	if err := credvalidate.ValidateFieldMaxLength(field, str, passwordMaxLen, nonSensitive); err != nil {
		return err
	}

	return nil
}

var _ credvalidate.FieldValidator = validatePassword

// validateApplicationCredentialID validates the applicationCredentialID field.
func validateApplicationCredentialID(v *credvalidate.BaseValidator, field string, val any, all map[string]any, nonSensitive bool) error {
	str, err := credvalidate.AssertStringWithPrintableCheck(field, val, nonSensitive)
	if err != nil {
		return err
	}

	if err := credvalidate.ValidateFieldMaxLength(field, str, applicationCredentialIDMaxLen, nonSensitive); err != nil {
		return err
	}

	return nil
}

var _ credvalidate.FieldValidator = validateApplicationCredentialID

// validateApplicationCredentialName validates the applicationCredentialName field.
func validateApplicationCredentialName(v *credvalidate.BaseValidator, field string, val any, all map[string]any, nonSensitive bool) error {
	str, err := credvalidate.AssertStringWithPrintableCheck(field, val, nonSensitive)
	if err != nil {
		return err
	}

	if err := credvalidate.ValidateFieldMaxLength(field, str, applicationCredentialNameMaxLen, nonSensitive); err != nil {
		return err
	}

	return nil
}

var _ credvalidate.FieldValidator = validateApplicationCredentialName

// validateApplicationCredentialSecret validates the applicationCredentialSecret field.
func validateApplicationCredentialSecret(v *credvalidate.BaseValidator, field string, val any, all map[string]any, nonSensitive bool) error {
	str, err := credvalidate.AssertStringWithPrintableCheck(field, val, nonSensitive)
	if err != nil {
		return err
	}

	if err := credvalidate.ValidateFieldMaxLength(field, str, applicationCredentialSecretMaxLen, nonSensitive); err != nil {
		return err
	}

	return nil
}

var _ credvalidate.FieldValidator = validateApplicationCredentialSecret
