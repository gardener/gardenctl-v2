/*
SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package providerenv

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardensecurityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
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

func (p *GCPProvider) FromSecret(o *options, shoot *gardencorev1beta1.Shoot, secret *corev1.Secret, cp *clientgarden.CloudProfileUnion) (map[string]interface{}, error) {
	validatedFields, err := p.validator.ValidateSecret(secret)
	if err != nil {
		return nil, err
	}

	serviceaccountJSON := validatedFields["serviceaccount.json"].(string)

	var credentialsMap map[string]interface{}
	if err := json.Unmarshal([]byte(serviceaccountJSON), &credentialsMap); err != nil {
		return nil, err
	}

	templateFields := map[string]interface{}{
		"credentials": serviceaccountJSON,
		"project_id":  credentialsMap["project_id"],
	}

	return templateFields, nil
}

func (p *GCPProvider) FromWorkloadIdentity(o *options, wi *gardensecurityv1alpha1.WorkloadIdentity, token, configDir string) (map[string]interface{}, error) {
	validatedConfig, err := p.validator.ValidateWorkloadIdentityConfig(wi)
	if err != nil {
		return nil, err
	}

	// Only delete the token file when the unset flag is set; gcloud does not keep a separate copy of this token file.
	tokenFileName := fmt.Sprintf(".%s-web-identity-token", computeWorkloadIdentityFilePrefix(o.SessionID, wi))

	tokenFilePath := filepath.Join(configDir, tokenFileName)
	if err := writeOrRemoveFile(o.Unset, tokenFilePath, []byte(token)); err != nil {
		return nil, err
	}

	credentialsConfig := validatedConfig["credentialsConfig"].(map[string]interface{})

	credentialsConfig["credential_source"] = map[string]interface{}{
		"file":   tokenFilePath,
		"format": map[string]interface{}{"type": "text"},
	}

	credentialsJSON, err := json.Marshal(credentialsConfig)
	if err != nil {
		return nil, err
	}

	templateFields := map[string]interface{}{
		"credentials": string(credentialsJSON),
		"project_id":  validatedConfig["projectID"].(string),
	}

	return templateFields, nil
}
