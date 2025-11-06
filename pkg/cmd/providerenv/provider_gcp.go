/*
SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package providerenv

import (
	"context"
	"encoding/json"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	corev1 "k8s.io/api/core/v1"

	clientgarden "github.com/gardener/gardenctl-v2/internal/client/garden"
	allowpattern "github.com/gardener/gardenctl-v2/pkg/provider/common/allowpattern"
	"github.com/gardener/gardenctl-v2/pkg/provider/credvalidate"
)

type GCPProvider struct {
	validator *credvalidate.GCPValidator
}

var _ Provider = &GCPProvider{}

// newGCPProvider creates a GCPProvider with validator initialized.
func newGCPProvider(ctx context.Context, allowedPatterns []allowpattern.Pattern) *GCPProvider {
	validator := credvalidate.NewGCPValidator(ctx, allowedPatterns)
	return &GCPProvider{validator: validator}
}

func (p *GCPProvider) FromSecret(o *options, shoot *gardencorev1beta1.Shoot, secret *corev1.Secret, cp *clientgarden.CloudProfileUnion, configDir string) (map[string]interface{}, error) {
	validatedFields, err := p.validator.ValidateSecret(secret)
	if err != nil {
		return nil, err
	}

	serviceaccountJSON := validatedFields["serviceaccount.json"].(string)

	var credentialsMap map[string]interface{}
	if err := json.Unmarshal([]byte(serviceaccountJSON), &credentialsMap); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"credentials":  serviceaccountJSON,
		"client_email": credentialsMap["client_email"],
		"project_id":   credentialsMap["project_id"],
	}, nil
}
