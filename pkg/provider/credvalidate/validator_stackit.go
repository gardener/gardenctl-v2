/*
SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package credvalidate

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardenctl-v2/pkg/provider/common/allowpattern"
	"github.com/gardener/gardenctl-v2/pkg/provider/common/credvalidate"
)

// STACKITValidator implements the common validator interface for STACKIT.
type STACKITValidator struct {
	openstack *OpenStackValidator
	stackit   *credvalidate.BaseValidator
}

var _ credvalidate.Validator = &STACKITValidator{}

// NewSTACKITValidator creates a new STACKIT validator.
func NewSTACKITValidator(ctx context.Context) *STACKITValidator {
	allowedPatterns := []allowpattern.Pattern{
		{
			Field:      "project-id",
			RegexValue: ptr.To(`^[0-9a-fA-F-]{36}$`),
		},
	}

	return &STACKITValidator{
		openstack: NewOpenStackValidator(ctx, []allowpattern.Pattern{}),
		stackit:   credvalidate.NewBaseValidator(ctx, allowedPatterns),
	}
}

// ValidateSecret validates OpenStack credentials from a Kubernetes secret.
func (v *STACKITValidator) ValidateSecret(secret *corev1.Secret) (map[string]interface{}, error) {
	fields := credvalidate.FlatCoerceBytesToStringsMap(secret.Data)
	registry := map[string]credvalidate.FieldRule{
		"serviceaccount.json": {Required: true, Validator: validateSTACKITServiceaccount, NonSensitive: false},
		"project-id":          {Required: true, Validator: validateSTACKITProjectID, NonSensitive: true},
	}

	validatedValues, err := v.stackit.ValidateWithRegistry(fields, registry, credvalidate.Permissive)
	if err != nil {
		return nil, err
	}

	// The secret is validated against the OpenStack validation in order to get unknown fields removed.
	// But the error is ignored as OpensStack is optional in provider STACKIT and will be removed in the future.
	validatedValuesOpenstack, _ := v.openstack.ValidateSecret(secret)
	maps.Copy(validatedValues, validatedValuesOpenstack)

	return validatedValues, nil
}

func validateSTACKITProjectID(v *credvalidate.BaseValidator, field string, val any, all map[string]any, nonSensitive bool) error {
	str, err := credvalidate.AssertStringWithPrintableCheck(field, val, nonSensitive)
	if err != nil {
		return err
	}

	return v.ValidateFieldPattern(field, str, all, credvalidate.MatchRegexValuePattern, nonSensitive)
}

func validateSTACKITServiceaccount(v *credvalidate.BaseValidator, field string, val any, all map[string]any, nonSensitive bool) error {
	str, err := credvalidate.AssertStringWithPrintableCheck(field, val, nonSensitive)
	if err != nil {
		return err
	}
	// check if it is a valid json
	err = json.Unmarshal([]byte(str), &map[string]interface{}{})
	if err != nil {
		return credvalidate.NewFieldError(field, fmt.Sprintf("no valid json %v", err), nil, nonSensitive)
	}

	return nil
}
