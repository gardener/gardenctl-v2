/*
SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package credvalidate

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardenctl-v2/pkg/provider/common/allowpattern"
	"github.com/gardener/gardenctl-v2/pkg/provider/common/credvalidate"
)

const (
	// AWS Access Key IDs are always exactly 20 characters.
	accessKeyLen = 20
	// AWS Secret Access Keys are always exactly 40 characters.
	secretAccessKeyLen = 40
	// AWS Role ARN minimum length as per AWS documentation.
	roleARNMinLen = 20
	// AWS Role ARN maximum length as per AWS documentation.
	roleARNMaxLen = 2048
)

// AWSValidator implements the common Validator interface for AWS.
type AWSValidator struct {
	*credvalidate.BaseValidator
}

var _ credvalidate.Validator = &AWSValidator{}

// NewAWSValidator creates a new AWS validator.
func NewAWSValidator(ctx context.Context) *AWSValidator {
	allowedPatterns := []allowpattern.Pattern{
		{
			Field:      "accessKeyID",
			RegexValue: ptr.To(`^(AKIA|ASIA)[A-Z0-9]{16}$`),
		},
		{
			Field:      "secretAccessKey",
			RegexValue: ptr.To(`^[A-Za-z0-9/+=]{40}$`),
		},
		{
			Field:      "roleARN",
			RegexValue: ptr.To(`^arn:(?:aws|aws-us-gov|aws-cn|aws-iso|aws-iso-b):iam::\d{12}:role(?:/[\w+=,.@-]+)+$`),
		},
	}

	return &AWSValidator{
		BaseValidator: credvalidate.NewBaseValidator(ctx, allowedPatterns),
	}
}

// ValidateSecret validates AWS credentials from a Kubernetes secret using the.
func (v *AWSValidator) ValidateSecret(secret *corev1.Secret) (map[string]interface{}, error) {
	fields := credvalidate.FlatCoerceBytesToStringsMap(secret.Data)

	registry := map[string]credvalidate.FieldRule{
		"accessKeyID":     {Required: true, Validator: validateAccessKeyID, NonSensitive: true},
		"secretAccessKey": {Required: true, Validator: validateSecretAccessKey, NonSensitive: false},
	}

	return v.ValidateWithRegistry(fields, registry, credvalidate.Permissive)
}

// Field-specific validator functions that implement the credvalidate.FieldValidator interface.

// validateAccessKeyID validates the accessKeyID field.
func validateAccessKeyID(v *credvalidate.BaseValidator, field string, val any, allFields map[string]any, nonSensitive bool) error {
	str, ok := val.(string)
	if !ok {
		return credvalidate.NewFieldError(field, "field value must be a string", nil, nonSensitive)
	}

	if err := credvalidate.ValidateFieldExactLength(field, str, accessKeyLen, nonSensitive); err != nil {
		return err
	}

	return v.ValidateFieldPattern(field, str, allFields, credvalidate.MatchRegexValuePattern, nonSensitive)
}

var _ credvalidate.FieldValidator = validateAccessKeyID

// validateSecretAccessKey validates the secretAccessKey field.
func validateSecretAccessKey(v *credvalidate.BaseValidator, field string, val any, allFields map[string]any, nonSensitive bool) error {
	str, ok := val.(string)
	if !ok {
		return credvalidate.NewFieldError(field, "field value must be a string", nil, nonSensitive)
	}

	if err := credvalidate.ValidateFieldExactLength(field, str, secretAccessKeyLen, nonSensitive); err != nil {
		return err
	}

	return v.ValidateFieldPattern(field, str, allFields, credvalidate.MatchRegexValuePattern, nonSensitive)
}

var _ credvalidate.FieldValidator = validateSecretAccessKey

// validateRoleARN validates the roleARN field using hardcoded regex validation.
func validateRoleARN(v *credvalidate.BaseValidator, field string, val any, allFields map[string]any, nonSensitive bool) error {
	str, ok := val.(string)
	if !ok {
		return credvalidate.NewFieldError(field, "field value must be a string", nil, nonSensitive)
	}

	if err := credvalidate.ValidateFieldMinLength(field, str, roleARNMinLen, nonSensitive); err != nil {
		return err
	}

	if err := credvalidate.ValidateFieldMaxLength(field, str, roleARNMaxLen, nonSensitive); err != nil {
		return err
	}

	return v.ValidateFieldPattern(field, str, allFields, credvalidate.MatchRegexValuePattern, nonSensitive)
}

var _ credvalidate.FieldValidator = validateRoleARN
