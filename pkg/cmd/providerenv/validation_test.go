/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package providerenv_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardenctl-v2/pkg/cmd/providerenv"
)

var _ = Describe("Provider Type Validation", func() {
	DescribeTable("valid provider types",
		func(providerType string) {
			err := providerenv.ValidateProviderType(providerType)
			Expect(err).NotTo(HaveOccurred())
		},
		Entry("valid aws provider", "aws"),
		Entry("valid azure provider", "azure"),
		Entry("valid gcp provider", "gcp"),
		Entry("valid alicloud provider", "alicloud"),
		Entry("valid openstack provider", "openstack"),
		Entry("valid hcloud provider", "hcloud"),
		Entry("valid stackit provider", "stackit"),
		Entry("valid provider with numbers", "provider123"),
		Entry("valid provider with hyphens", "my-provider"),
		Entry("valid provider with mixed", "provider-123"),
		Entry("single character valid", "a"),
		Entry("single number valid", "1"),
		Entry("long valid provider name", "very-long-provider-name-with-numbers-123"),
	)

	DescribeTable("invalid provider types",
		func(providerType string, expectedErrorMsg string) {
			err := providerenv.ValidateProviderType(providerType)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring(expectedErrorMsg)))
		},
		Entry("empty provider type", "", "provider type cannot be empty"),
		Entry("reserved kubernetes provider", "kubernetes", "provider type \"kubernetes\" is reserved and cannot be used"),

		// Invalid provider types - invalid characters
		Entry("provider with uppercase", "AWS", "invalid provider type"),
		Entry("provider with spaces", "my provider", "invalid provider type"),
		Entry("provider with dots", "my.provider", "invalid provider type"),
		Entry("provider with slashes", "my/provider", "invalid provider type"),
		Entry("provider with backslashes", "my\\provider", "invalid provider type"),
		Entry("provider with underscores", "my_provider", "invalid provider type"),
		Entry("provider with special chars", "my@provider", "invalid provider type"),
		Entry("provider with path traversal", "../../../etc/passwd", "invalid provider type"),
		Entry("provider with path traversal dots", "...", "invalid provider type"),
		Entry("provider with null bytes", "provider\x00", "invalid provider type"),
		Entry("provider with newlines", "provider\n", "invalid provider type"),
		Entry("provider with tabs", "provider\t", "invalid provider type"),
		Entry("provider with unicode", "provider™", "invalid provider type"),
		Entry("single hyphen valid", "-", "invalid provider type"),
	)

	Context("when validating realistic provider names", func() {
		It("should accept all known cloud provider types", func() {
			knownProviders := []string{
				"aws",
				"azure",
				"gcp",
				"alicloud",
				"openstack",
				"hcloud",
				"stackit",
			}

			for _, provider := range knownProviders {
				err := providerenv.ValidateProviderType(provider)
				Expect(err).NotTo(HaveOccurred(), "Expected validation to succeed for known provider: %s", provider)
			}
		})

		It("should accept custom provider names with valid patterns", func() {
			customProviders := []string{
				"custom-provider",
				"provider-v2",
				"my-cloud-123",
				"test-provider-2024",
				"provider1",
				"p1",
			}

			for _, provider := range customProviders {
				err := providerenv.ValidateProviderType(provider)
				Expect(err).NotTo(HaveOccurred(), "Expected validation to succeed for custom provider: %s", provider)
			}
		})
	})
})
