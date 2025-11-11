/*
SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package providerenv

import (
	"context"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	corev1 "k8s.io/api/core/v1"

	clientgarden "github.com/gardener/gardenctl-v2/internal/client/garden"
	"github.com/gardener/gardenctl-v2/pkg/provider/credvalidate"
)

type AliCloudProvider struct {
	validator *credvalidate.AliCloudValidator
}

var _ Provider = &AliCloudProvider{}

// newAliCloudProvider creates an AliCloudProvider with validator initialized.
func newAliCloudProvider(ctx context.Context) *AliCloudProvider {
	validator := credvalidate.NewAliCloudValidator(ctx)
	return &AliCloudProvider{validator: validator}
}

func (p *AliCloudProvider) FromSecret(_ *options, _ *gardencorev1beta1.Shoot, secret *corev1.Secret, _ *clientgarden.CloudProfileUnion) (map[string]interface{}, error) {
	return p.validator.ValidateSecret(secret)
}
