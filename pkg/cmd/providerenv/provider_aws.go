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

type AWSProvider struct {
	validator *credvalidate.AWSValidator
}

var _ Provider = &AWSProvider{}

// newAWSProvider creates an AWSProvider with validator initialized.
func newAWSProvider(ctx context.Context) *AWSProvider {
	validator := credvalidate.NewAWSValidator(ctx)
	return &AWSProvider{validator: validator}
}

func (p *AWSProvider) FromSecret(o *options, shoot *gardencorev1beta1.Shoot, secret *corev1.Secret, cp *clientgarden.CloudProfileUnion) (map[string]interface{}, error) {
	return p.validator.ValidateSecret(secret)
}
