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

const guidPattern = `^[0-9A-Fa-f]{8}-([0-9A-Fa-f]{4}-){3}[0-9A-Fa-f]{12}$`
const (
	// Azure client secret allowed length range.
	clientSecretMinLen = 32
	clientSecretMaxLen = 44
)

// AzureValidator implements the common Validator interface for Azure.
type AzureValidator struct {
	*credvalidate.BaseValidator
}

var _ credvalidate.Validator = &AzureValidator{}

// NewAzureValidator creates a new Azure validator.
func NewAzureValidator(ctx context.Context) *AzureValidator {
	allowedPatterns := []allowpattern.Pattern{
		{
			Field:      "subscriptionID",
			RegexValue: ptr.To(guidPattern),
		},
		{
			Field:      "tenantID",
			RegexValue: ptr.To(guidPattern),
		},
		{
			Field:      "clientID",
			RegexValue: ptr.To(guidPattern),
		},
		{
			Field: "clientSecret",
			// Allow all printable ASCII characters from '!' (U+0021) to '~' (U+007E),
			// This range covers letters (A–Z, a–z), digits (0–9), and punctuation/symbols.
			// Space (U+0020) and all control characters are not allowed.
			RegexValue: ptr.To(`^[!-~]+$`),
		},
	}

	return &AzureValidator{
		BaseValidator: credvalidate.NewBaseValidator(ctx, allowedPatterns),
	}
}

// validateAzureClientSecret validates the clientSecret with explicit min/max length
// and the allowed character pattern to produce clearer error messages.
func validateAzureClientSecret(v *credvalidate.BaseValidator, field string, val any, allFields map[string]any, nonSensitive bool) error {
	str, err := credvalidate.AssertStringWithPrintableCheck(field, val, nonSensitive)
	if err != nil {
		return err
	}

	if err := credvalidate.ValidateFieldMinLength(field, str, clientSecretMinLen, nonSensitive); err != nil {
		return err
	}

	if err := credvalidate.ValidateFieldMaxLength(field, str, clientSecretMaxLen, nonSensitive); err != nil {
		return err
	}

	return v.ValidateFieldPattern(field, str, allFields, credvalidate.MatchRegexValuePattern, nonSensitive)
}

// ValidateSecret validates Azure credentials from a Kubernetes secret.
func (v *AzureValidator) ValidateSecret(secret *corev1.Secret) (map[string]interface{}, error) {
	fields := credvalidate.FlatCoerceBytesToStringsMap(secret.Data)

	registry := map[string]credvalidate.FieldRule{
		"subscriptionID": {Required: true, Validator: credvalidate.ValidateStringWithPattern(credvalidate.MatchRegexValuePattern), NonSensitive: true},
		"tenantID":       {Required: true, Validator: credvalidate.ValidateStringWithPattern(credvalidate.MatchRegexValuePattern), NonSensitive: true},
		"clientID":       {Required: true, Validator: credvalidate.ValidateStringWithPattern(credvalidate.MatchRegexValuePattern), NonSensitive: true},
		"clientSecret":   {Required: true, Validator: validateAzureClientSecret, NonSensitive: false},
	}

	return v.ValidateWithRegistry(fields, registry, credvalidate.Permissive)
}
