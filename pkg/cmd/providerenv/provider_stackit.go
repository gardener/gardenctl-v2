/*
SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package providerenv

import (
	"context"
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"go.yaml.in/yaml/v3"
	corev1 "k8s.io/api/core/v1"

	clientgarden "github.com/gardener/gardenctl-v2/internal/client/garden"
	"github.com/gardener/gardenctl-v2/pkg/provider/credvalidate"
)

type STACKITProvider struct {
	validator *credvalidate.STACKITValidator
}

var _ Provider = &STACKITProvider{}

// newSTACKITProvider creates an STACKITProvider with validator initialized.
func newSTACKITProvider(ctx context.Context) *STACKITProvider {
	validator := credvalidate.NewSTACKITValidator(ctx)
	return &STACKITProvider{validator: validator}
}

func (p *STACKITProvider) FromSecret(o *options, shoot *gardencorev1beta1.Shoot, secret *corev1.Secret, cp *clientgarden.CloudProfileUnion, configDir string) (map[string]interface{}, error) {
	validatedFields, err := p.validator.ValidateSecret(secret)
	if err != nil {
		return nil, err
	}

	authURL, err := getKeyStoneURLInSTACKIT(cp)
	if err != nil {
		return nil, err
	}

	// Currently the region in the shoot in eu01 is RegionOne. We just can replace this here.
	region := shoot.Spec.Region
	if region == "RegionOne" {
		region = "eu01"
	}

	templateFields := map[string]interface{}{
		"authURL":        authURL,
		"domainName":     validatedFields["domainName"],
		"tenantName":     validatedFields["tenantName"],
		"username":       validatedFields["username"],
		"password":       validatedFields["password"],
		"authStrategy":   "keystone",
		"projectId":      validatedFields["project-id"],
		"serviceaccount": validatedFields["serviceaccount.json"],
		"stackitRegion":  region,
	}

	return templateFields, nil
}

// getKeyStoneURLInSTACKIT returns the keyStoneURL like the getKeyStoneURL for openstack. As the ProviderConfig in
// Cloudprofile is using stackit.provider.extensions.gardener.cloud as APIGroup this is needs to be done in a
// dedicated function. This is using map[string]interface{} instead of the api object because the
// gardener-extension-provider-stackit is not yet opensource (and the openstack parts will be removed in the future).
func getKeyStoneURLInSTACKIT(cloudProfile *clientgarden.CloudProfileUnion) (string, error) {
	providerConfig := cloudProfile.GetCloudProfileSpec().ProviderConfig
	if providerConfig == nil {
		return "", fmt.Errorf("providerConfig of %s is empty", cloudProfile.GetObjectMeta().Name)
	}

	var cloudProfileConfig map[string]interface{}

	if yaml.Unmarshal(providerConfig.Raw, &cloudProfileConfig) != nil {
		return "", fmt.Errorf("fail to unmarshal providerConfig in %s", cloudProfile.GetObjectMeta().Name)
	}

	keystoneURL, ok := cloudProfileConfig["keystoneURL"].(string)
	if !ok || keystoneURL == "" {
		return "", fmt.Errorf("keystoneURL of %s is invalid", cloudProfile.GetObjectMeta().Name)
	}

	return keystoneURL, nil
}
