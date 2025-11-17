/*
SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package providerenv

import (
	"context"
	"fmt"
	"path/filepath"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardensecurityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
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

func (p *AWSProvider) FromWorkloadIdentity(o *options, wi *gardensecurityv1alpha1.WorkloadIdentity, token, configDir string) (map[string]interface{}, error) {
	validatedConfig, err := p.validator.ValidateWorkloadIdentityConfig(wi)
	if err != nil {
		return nil, err
	}

	// Only delete the token file when the unset flag is set
	tokenFileName := fmt.Sprintf(".%s-web-identity-token", computeWorkloadIdentityFilePrefix(o.SessionID, wi))

	tokenFilePath := filepath.Join(configDir, tokenFileName)
	if err := writeOrRemoveToken(o.Unset, tokenFilePath, []byte(token)); err != nil {
		return nil, err
	}

	templateFields := map[string]interface{}{
		"webIdentityTokenFile": tokenFilePath,
		"roleARN":              validatedConfig["roleARN"],
	}

	return templateFields, nil
}
