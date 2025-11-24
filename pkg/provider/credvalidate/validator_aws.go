/*
SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package credvalidate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	gardensecurityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
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
	// AWS Role ARN minimum length.
	roleARNMinLen = 20
	// AWS Role ARN maximum length.
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

// ValidateWorkloadIdentityConfig validates AWS workload identity configuration using registry-based validation.
// It follows the same pattern as GCP's ValidateWorkloadIdentityConfig for architectural alignment.
func (v *AWSValidator) ValidateWorkloadIdentityConfig(wi *gardensecurityv1alpha1.WorkloadIdentity) (map[string]interface{}, error) {
	if wi.Spec.TargetSystem.ProviderConfig == nil || wi.Spec.TargetSystem.ProviderConfig.Raw == nil {
		return nil, errors.New("providerConfig is missing")
	}

	fields := make(map[string]interface{})
	if err := json.Unmarshal(wi.Spec.TargetSystem.ProviderConfig.Raw, &fields); err != nil {
		return nil, fmt.Errorf("failed to unmarshal AWS workload identity config: %w", err)
	}

	registry := map[string]credvalidate.FieldRule{
		"roleARN": {Required: true, Validator: validateRoleARN, NonSensitive: true},
	}

	return v.ValidateWithRegistry(fields, registry, credvalidate.Permissive)
}

// Field-specific validator functions that implement the credvalidate.FieldValidator interface.

// validateAccessKeyID validates the accessKeyID field.
func validateAccessKeyID(v *credvalidate.BaseValidator, field string, val any, allFields map[string]any, nonSensitive bool) error {
	str, err := credvalidate.AssertStringWithPrintableCheck(field, val, nonSensitive)
	if err != nil {
		return err
	}

	if err := credvalidate.ValidateFieldExactLength(field, str, accessKeyLen, nonSensitive); err != nil {
		return err
	}

	return v.ValidateFieldPattern(field, str, allFields, credvalidate.MatchRegexValuePattern, nonSensitive)
}

var _ credvalidate.FieldValidator = validateAccessKeyID

// validateSecretAccessKey validates the secretAccessKey field.
func validateSecretAccessKey(v *credvalidate.BaseValidator, field string, val any, allFields map[string]any, nonSensitive bool) error {
	str, err := credvalidate.AssertStringWithPrintableCheck(field, val, nonSensitive)
	if err != nil {
		return err
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
