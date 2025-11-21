/*
SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package credvalidate

import (
	"context"
	"maps"

	corev1 "k8s.io/api/core/v1"

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
	return &STACKITValidator{
		openstack: NewOpenStackValidator(ctx, []allowpattern.Pattern{}),
		stackit:   credvalidate.NewBaseValidator(ctx, []allowpattern.Pattern{}),
	}
}

// ValidateSecret validates OpenStack credentials from a Kubernetes secret.
func (v *STACKITValidator) ValidateSecret(secret *corev1.Secret) (map[string]interface{}, error) {
	fields := credvalidate.FlatCoerceBytesToStringsMap(secret.Data)
	registry := map[string]credvalidate.FieldRule{
		"serviceaccount.json": {Required: true, Validator: nil, NonSensitive: false},
		"project-id":          {Required: true, Validator: nil, NonSensitive: false},
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
