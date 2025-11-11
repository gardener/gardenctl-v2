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
	"github.com/gardener/gardenctl-v2/pkg/provider/common/allowpattern"
	"github.com/gardener/gardenctl-v2/pkg/provider/credvalidate"
)

type Provider interface {
	// FromSecret validates and extracts credential data from a Kubernetes Secret.
	FromSecret(o *options, shoot *gardencorev1beta1.Shoot, secret *corev1.Secret, cp *clientgarden.CloudProfileUnion) (map[string]interface{}, error)
	// FromWorkloadIdentity validates and extracts credential data from a WorkloadIdentity.
	FromWorkloadIdentity(o *options, wi *gardensecurityv1alpha1.WorkloadIdentity, dataWriter DataWriter) (map[string]interface{}, error)
}

func providerFor(typ string, ctx context.Context, mergedPatterns *MergedProviderPatterns) Provider {
	switch typ {
	case "gcp":
		return newGCPProvider(ctx, credvalidate.DefaultGCPAllowedPatterns())
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
