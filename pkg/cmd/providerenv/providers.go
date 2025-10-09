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
	"github.com/gardener/gardenctl-v2/pkg/provider/common/allowpattern"
)

type Provider interface {
	FromSecret(o *options, shoot *gardencorev1beta1.Shoot, secret *corev1.Secret, cp *clientgarden.CloudProfileUnion, configDir string) (map[string]interface{}, error)
}

func providerFor(typ string, ctx context.Context, mergedPatterns *MergedProviderPatterns) Provider {
	switch typ {
	case "gcp":
		var patterns []allowpattern.Pattern
		if mergedPatterns != nil {
			patterns = mergedPatterns.GCP
		}

		return newGCPProvider(ctx, patterns)
	case "openstack":
		var patterns []allowpattern.Pattern
		if mergedPatterns != nil {
			patterns = mergedPatterns.OpenStack
		}

		return newOpenStackProvider(ctx, patterns)
	case "stackit":
		var patterns []allowpattern.Pattern
		if mergedPatterns != nil {
			patterns = mergedPatterns.OpenStack
		}

		return newOpenStackProvider(ctx, patterns)
	case "aws":
		return newAWSProvider(ctx)
	case "azure":
		return newAzureProvider(ctx)
	case "alicloud":
		return newAliCloudProvider(ctx)
	case "hcloud":
		return newHCloudProvider(ctx)
	default:
		return nil
	}
}
