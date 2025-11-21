/*
SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package credvalidate

import (
	"context"
	"errors"

	gardensecurityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardenctl-v2/pkg/provider/common/allowpattern"
	"github.com/gardener/gardenctl-v2/pkg/provider/common/credvalidate"
)

const (
	// AliCloud Access Key IDs are always exactly 30 characters.
	accessKeyIDLen = 24
	// AliCloud Access Key Secrets are always exactly 30 characters.
	accessKeySecretLen = 30
)

// AliCloudValidator implements the common Validator interface for AliCloud.
type AliCloudValidator struct {
	*credvalidate.BaseValidator
}

var _ credvalidate.Validator = &AliCloudValidator{}

// NewAliCloudValidator creates a new AliCloud validator.
func NewAliCloudValidator(ctx context.Context) *AliCloudValidator {
	allowedPatterns := []allowpattern.Pattern{
		{
			Field:      "accessKeyID",
			RegexValue: ptr.To(`^LTAI[A-Za-z0-9]{20}$`),
		},
		{
			Field:      "accessKeySecret",
			RegexValue: ptr.To(`^[A-Za-z0-9]{30}$`),
		},
	}

	return &AliCloudValidator{
		BaseValidator: credvalidate.NewBaseValidator(ctx, allowedPatterns),
	}
}

// ValidateSecret validates AliCloud credentials from a Kubernetes secret.
func (v *AliCloudValidator) ValidateSecret(secret *corev1.Secret) (map[string]interface{}, error) {
	fields := credvalidate.FlatCoerceBytesToStringsMap(secret.Data)

	registry := map[string]credvalidate.FieldRule{
		"accessKeyID":     {Required: true, Validator: validateAliCloudAccessKeyID, NonSensitive: true},
		"accessKeySecret": {Required: true, Validator: validateAliCloudAccessKeySecret, NonSensitive: false},
	}

	return v.ValidateWithRegistry(fields, registry, credvalidate.Permissive)
}

// ValidateWorkloadIdentityConfig validates AliCloud workload identity configuration.
// AliCloud does not support workload identity, so this always returns an error.
func (v *AliCloudValidator) ValidateWorkloadIdentityConfig(wi *gardensecurityv1alpha1.WorkloadIdentity) (map[string]interface{}, error) {
	return nil, errors.New("workload identity not supported for alicloud")
}

// Field-specific validator functions that implement the credvalidate.FieldValidator interface.

// validateAliCloudAccessKeyID validates the accessKeyID field.
func validateAliCloudAccessKeyID(v *credvalidate.BaseValidator, field string, val any, allFields map[string]any, nonSensitive bool) error {
	str, err := credvalidate.AssertStringWithPrintableCheck(field, val, nonSensitive)
	if err != nil {
		return err
	}

	if err := credvalidate.ValidateFieldExactLength(field, str, accessKeyIDLen, nonSensitive); err != nil {
		return err
	}

	return v.ValidateFieldPattern(field, str, allFields, credvalidate.MatchRegexValuePattern, nonSensitive)
}

var _ credvalidate.FieldValidator = validateAliCloudAccessKeyID

// validateAliCloudAccessKeySecret validates the accessKeySecret field.
func validateAliCloudAccessKeySecret(v *credvalidate.BaseValidator, field string, val any, allFields map[string]any, nonSensitive bool) error {
	str, err := credvalidate.AssertStringWithPrintableCheck(field, val, nonSensitive)
	if err != nil {
		return err
	}

	if err := credvalidate.ValidateFieldExactLength(field, str, accessKeySecretLen, nonSensitive); err != nil {
		return err
	}

	return v.ValidateFieldPattern(field, str, allFields, credvalidate.MatchRegexValuePattern, nonSensitive)
}

var _ credvalidate.FieldValidator = validateAliCloudAccessKeySecret
