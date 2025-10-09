/*
SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package providerenv

import (
	"context"
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	corev1 "k8s.io/api/core/v1"

	clientgarden "github.com/gardener/gardenctl-v2/internal/client/garden"
	allowpattern "github.com/gardener/gardenctl-v2/pkg/provider/common/allowpattern"
	"github.com/gardener/gardenctl-v2/pkg/provider/credvalidate"
)

type OpenStackProvider struct {
	validator *credvalidate.OpenStackValidator
}

var _ Provider = &OpenStackProvider{}

// newOpenStackProvider creates an OpenStackProvider with validator initialized.
func newOpenStackProvider(ctx context.Context, allowedPatterns []allowpattern.Pattern) *OpenStackProvider {
	validator := credvalidate.NewOpenStackValidator(ctx, allowedPatterns)
	return &OpenStackProvider{validator: validator}
}

func (p *OpenStackProvider) FromSecret(o *options, shoot *gardencorev1beta1.Shoot, secret *corev1.Secret, cp *clientgarden.CloudProfileUnion, configDir string) (map[string]interface{}, error) {
	validatedFields, err := p.validator.ValidateSecret(secret)
	if err != nil {
		return nil, err
	}

	// choose auth strategy based on presence of application credential secret
	if v := validatedFields["applicationCredentialSecret"]; v != nil && v.(string) != "" {
		validatedFields["authType"] = "v3applicationcredential"
		validatedFields["authStrategy"] = ""
		validatedFields["tenantName"] = ""
		validatedFields["password"] = ""
	} else {
		validatedFields["authStrategy"] = "keystone"
		validatedFields["authType"] = ""
		validatedFields["applicationCredentialID"] = ""
		validatedFields["applicationCredentialName"] = ""
		validatedFields["applicationCredentialSecret"] = ""
	}

	authURL, err := getKeyStoneURL(cp, shoot.Spec.Region)
	if err != nil {
		return nil, err
	}

	// Validate the authURL from cloud profile / namespaced cloud profile
	if err := p.validator.ValidateAuthURL(authURL); err != nil {
		return nil, fmt.Errorf("invalid authURL from cloud profile: %w", err)
	}

	validatedFields["authURL"] = authURL

	return validatedFields, nil
}
