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
						fmt.Sprintf("^%s/shoot--(?P<project>.+)--(?P<shoot>.+)$", clusterIdentity1),
						"^shoot--(?P<project>.+)--(?P<shoot>.+)$",
					},
				},
				{
					Name: clusterIdentity2,
					Patterns: []string{
						fmt.Sprintf("^(%s/)?shoot--(?P<project>.+)--(?P<shoot>.+)$", clusterIdentity2),
					},
				}},
		}
	})

	var patternValue = func(prefix string) string {
		value := fmt.Sprintf("shoot--%s--%s", project, shoot)

		if prefix != "" {
			value = fmt.Sprintf("%s/%s", prefix, value)
		}
		return value
	}

	DescribeTable("Validation of valid target values",
		func(currentGardenName string, patternPrefix string, expectedGarden string) {
			value := patternValue(patternPrefix)
			expectedPM := &config.PatternMatch{Garden: expectedGarden, Project: project, Shoot: shoot}
			match, err := cfg.MatchPattern(currentGardenName, value)
			Expect(err).NotTo(HaveOccurred())
			Expect(match).To(Equal(expectedPM))
		},
		Entry(
			"should succeed if pattern for value is (potential) non unique pattern and garden is set in current target (unique case / keep garden)",
			clusterIdentity1,
			clusterIdentity1,
			clusterIdentity1),
		Entry(
			"should succeed if pattern for value is (potential) non unique pattern and garden is not set in current target (unique case / set garden)",
			"",
			clusterIdentity1,
			clusterIdentity1),
		Entry(
			"should succeed if pattern for value is (potential) non unique pattern and other garden is set in current target (unique case / switch garden)",
			clusterIdentity2,
			clusterIdentity1,
			clusterIdentity1),
		Entry(
			"should succeed if pattern for value is (potential) non unique pattern and clusterIdentity1 is set in current target (non unique case / keep garden)",
			clusterIdentity1,
			"",
			clusterIdentity1),
		Entry(
			"should succeed if pattern for value is (potential) non unique pattern and clusterIdentity2 is set in current target (non unique case / keep garden)",
			clusterIdentity2,
			"",
			clusterIdentity2),
	)

	DescribeTable("Validation of invalid target values",
		func(currentGardenName string, patternPrefix string, expectedErrorString string) {
			value := patternValue(patternPrefix)
			_, err := cfg.MatchPattern(currentGardenName, value)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring(expectedErrorString)))
		},
		Entry(
			"should fail if pattern for value is (potential) non unique pattern and garden is not set in current target (non unique case / ambiguous match / fail to determine garden)",
			"",
			"",
			"the provided value resulted in an ambiguous match"),
		Entry(
			"should fail if pattern for value is not found and garden is set",
			clusterIdentity1,
			"invalidPrefix",
			"the provided value does not match any pattern"),
		Entry(
			"should fail if pattern for value is not found and garden is not set",
			"",
			"invalidPrefix",
			"the provided value does not match any pattern"),
	)

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
