/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package config_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardenctl-v2/pkg/config"
)

var _ = Describe("Config", func() {
	var (
		clusterIdentity1 = "testgarden"
		clusterIdentity2 = "foogarden"
		project          = "fooproject"
		shoot            = "fooshoot"
		cfg              *config.Config
	)

	BeforeEach(func() {
		cfg = &config.Config{
			Gardens: []config.Garden{
				{
					Name: clusterIdentity1,
					Patterns: []string{
						fmt.Sprintf("^(%s/)?shoot--(?P<project>.+)--(?P<shoot>.+)$", clusterIdentity1),
						"^uniquePattern/?shoot--(?P<project>.+)--(?P<shoot>.+)$",
					},
				},
				{
					Name: clusterIdentity2,
					Patterns: []string{
						fmt.Sprintf("^(%s)?shoot--(?P<project>.+)--(?P<shoot>.+)$", clusterIdentity2),
					},
				}},
		}
	})

	Context("when MatchPattern succeeds", func() {
		DescribeTable("Validation of target pattern",
			func(currentGardenName string, value string, expectedPM *config.PatternMatch) {
				Expect(cfg.MatchPattern(currentGardenName, value)).To(Equal(expectedPM))
			},
			Entry(
				"should succeed for unique pattern if garden is not set in current target (set garden)",
				"",
				fmt.Sprintf("uniquePattern/shoot--%s--%s", project, shoot),
				&config.PatternMatch{Garden: clusterIdentity1, Project: project, Shoot: shoot}),
			Entry(
				"should succeed for unique pattern if matching garden is set in current target (keep garden)",
				clusterIdentity1,
				fmt.Sprintf("uniquePattern/shoot--%s--%s", project, shoot),
				&config.PatternMatch{Garden: clusterIdentity1, Project: project, Shoot: shoot}),
			Entry(
				"should succeed for unique pattern if other garden is not set in current target (switch garden)",
				clusterIdentity2,
				fmt.Sprintf("uniquePattern/shoot--%s--%s", project, shoot),
				&config.PatternMatch{Garden: clusterIdentity1, Project: project, Shoot: shoot}),
			Entry(
				"should succeed for (potential) non unique pattern if garden is set in current target (unique case / keep garden)",
				clusterIdentity1,
				fmt.Sprintf("%s/shoot--%s--%s", clusterIdentity1, project, shoot),
				&config.PatternMatch{Garden: clusterIdentity1, Project: project, Shoot: shoot}),
			Entry(
				"should succeed for (potential) non unique pattern if garden is not set in current target (unique case / set garden)",
				"",
				fmt.Sprintf("%s/shoot--%s--%s", clusterIdentity1, project, shoot),
				&config.PatternMatch{Garden: clusterIdentity1, Project: project, Shoot: shoot}),
			Entry(
				"should succeed for (potential) non unique pattern if other garden is set in current target (unique case / switch garden)",
				clusterIdentity2,
				fmt.Sprintf("%s/shoot--%s--%s", clusterIdentity1, project, shoot),
				&config.PatternMatch{Garden: clusterIdentity1, Project: project, Shoot: shoot}),
			Entry(
				"should succeed for (potential) non unique pattern if clusterIdentity1 is set in current target (non unique case / keep garden)",
				clusterIdentity1,
				fmt.Sprintf("shoot--%s--%s", project, shoot),
				&config.PatternMatch{Garden: clusterIdentity1, Project: project, Shoot: shoot}),
			Entry(
				"should succeed for (potential) non unique pattern if clusterIdentity2 is set in current target (non unique case / keep garden)",
				clusterIdentity2,
				fmt.Sprintf("shoot--%s--%s", project, shoot),
				&config.PatternMatch{Garden: clusterIdentity2, Project: project, Shoot: shoot}),
		)
	})

	Context("when MatchPattern fails", func() {
		DescribeTable("Validation of target pattern",
			func(currentGardenName string, value string, expectedErrorString string) {
				_, err := cfg.MatchPattern(currentGardenName, value)
				Expect(err).To(MatchError(ContainSubstring(expectedErrorString)))
			},
			Entry(
				"should fail for (potential) non unique pattern if garden is not set in current target (non unique case / ambiguous match / fail to determine garden)",
				"",
				fmt.Sprintf("shoot--%s--%s", project, shoot),
				"the provided value resulted in an ambiguous match"),
			Entry(
				"should fail if pattern is not found and garden is set",
				clusterIdentity1,
				"invalid--pattern",
				"the provided value does not match any pattern"),
			Entry(
				"should fail if pattern is not found and garden is not set",
				"",
				"invalid--pattern",
				"the provided value does not match any pattern"),
		)
	})

	It("should find garden by identity", func() {
		garden, err := cfg.Garden(clusterIdentity1)
		Expect(err).NotTo(HaveOccurred())
		Expect(garden.Name).Should(Equal(clusterIdentity1))

	})

	It("should throw an error if garden not found", func() {
		_, err := cfg.Garden("foobar")
		Expect(err).To(HaveOccurred())
	})
})
