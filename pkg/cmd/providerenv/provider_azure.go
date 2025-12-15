/*
SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package providerenv

import (
	"context"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardensecurityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	corev1 "k8s.io/api/core/v1"

	clientgarden "github.com/gardener/gardenctl-v2/internal/client/garden"
	"github.com/gardener/gardenctl-v2/pkg/provider/credvalidate"
)

type AzureProvider struct {
	validator *credvalidate.AzureValidator
}

var _ Provider = &AzureProvider{}

// newAzureProvider creates an AzureProvider with validator initialized.
func newAzureProvider(ctx context.Context) *AzureProvider {
	validator := credvalidate.NewAzureValidator(ctx)
	return &AzureProvider{validator: validator}
}

func (p *AzureProvider) FromSecret(o *options, shoot *gardencorev1beta1.Shoot, secret *corev1.Secret, cp *clientgarden.CloudProfileUnion) (map[string]interface{}, error) {
	return p.validator.ValidateSecret(secret)
}

func (p *AzureProvider) FromWorkloadIdentity(o *options, wi *gardensecurityv1alpha1.WorkloadIdentity, token, configDir string) (map[string]interface{}, error) {
	return p.validator.ValidateWorkloadIdentityConfig(wi)
}
