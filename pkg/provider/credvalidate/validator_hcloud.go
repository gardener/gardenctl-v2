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
	// HCloud API tokens are always exactly 64 characters.
	hcloudTokenLen = 64
)

// HCloudValidator implements the common Validator interface for HCloud.
type HCloudValidator struct {
	*credvalidate.BaseValidator
}

var _ credvalidate.Validator = &HCloudValidator{}

// NewHCloudValidator creates a new HCloud validator.
func NewHCloudValidator(ctx context.Context) *HCloudValidator {
	allowedPatterns := []allowpattern.Pattern{
		{
			Field:      "hcloudToken",
			RegexValue: ptr.To(`^[A-Za-z0-9]{64}$`),
		},
	}

	return &HCloudValidator{
		BaseValidator: credvalidate.NewBaseValidator(ctx, allowedPatterns),
	}
}

// ValidateSecret validates HCloud credentials from a Kubernetes secret.
func (v *HCloudValidator) ValidateSecret(secret *corev1.Secret) (map[string]interface{}, error) {
	fields := credvalidate.FlatCoerceBytesToStringsMap(secret.Data)

	registry := map[string]credvalidate.FieldRule{
		"hcloudToken": {Required: true, Validator: validateHCloudToken, NonSensitive: false},
	}

	return v.ValidateWithRegistry(fields, registry, credvalidate.Permissive)
}

// Field-specific validator functions that implement the credvalidate.FieldValidator interface.

// validateHCloudToken validates the hcloudToken field.
func validateHCloudToken(v *credvalidate.BaseValidator, field string, val any, allFields map[string]any, nonSensitive bool) error {
	str, err := credvalidate.AssertStringWithPrintableCheck(field, val, nonSensitive)
	if err != nil {
		return err
	}

	if err := credvalidate.ValidateFieldExactLength(field, str, hcloudTokenLen, nonSensitive); err != nil {
		return err
	}

	return v.ValidateFieldPattern(field, str, allFields, credvalidate.MatchRegexValuePattern, nonSensitive)
}

var _ credvalidate.FieldValidator = validateHCloudToken
