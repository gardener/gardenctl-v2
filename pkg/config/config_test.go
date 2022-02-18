/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package config_test

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardenctl-v2/pkg/config"
)

var _ = Describe("Config", func() {
	var (
		clusterIdentity1 = "garden1"
		clusterIdentity2 = "garden2"
		fooIdentity      = "fooGarden"
		project          = "fooProject"
		shoot            = "fooShoot"
		cfg              *config.Config
	)

	BeforeEach(func() {
		cfg = &config.Config{
			LinkKubeconfig: pointer.Bool(false),
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

	DescribeTable("MatchPattern returns a match",
		func(currentGardenName string, patternPrefix string, expectedGarden string) {
			value := patternValue(patternPrefix)
			expectedPM := &config.PatternMatch{Garden: expectedGarden, Project: project, Shoot: shoot}
			match, err := cfg.MatchPattern(currentGardenName, value)
			Expect(err).NotTo(HaveOccurred())
			Expect(match).To(Equal(expectedPM))
		},
		Entry(
			"when the targetValue contains a garden prefix and preferred gardenName is equal (keep garden) - return extracted garden",
			clusterIdentity1,
			clusterIdentity1,
			clusterIdentity1),
		Entry(
			"when the targetValue contains a garden prefix and no preferred gardenName is set (set garden) - return extracted garden",
			"",
			clusterIdentity1,
			clusterIdentity1),
		Entry(
			"when the targetValue contains a garden prefix and preferred gardenName is set to other garden (switch garden) - return extracted garden",
			clusterIdentity2,
			clusterIdentity1,
			clusterIdentity1),
		Entry(
			"when the targetValue does not contain a garden prefix the preferred gardenName is unchanged (keep garden) - return preferred garden",
			clusterIdentity1,
			"",
			clusterIdentity1),
		Entry(
			"when the targetValue does not contain a garden prefix the preferred gardenName is unchanged (keep garden) - return preferred garden",
			clusterIdentity2,
			"",
			clusterIdentity2),
	)

	DescribeTable("MatchPattern returns an error",
		func(currentGardenName string, patternPrefix string, expectedErrorString string) {
			value := patternValue(patternPrefix)
			_, err := cfg.MatchPattern(currentGardenName, value)
			Expect(err).To(MatchError(ContainSubstring(expectedErrorString)))
		},
		Entry(
			"when no garden is preferred and the value matches multiple gardens (no prefix)",
			"",
			"",
			"the provided value resulted in an ambiguous match"),
		Entry(
			"when value does not match any pattern and preferred garden is given",
			clusterIdentity1,
			"invalidPrefix",
			"the provided value does not match any pattern"),
		Entry(
			"when value does not match any pattern and no preferred garden is given",
			"",
			"invalidPrefix",
			"the provided value does not match any pattern"),
		Entry(
			"when preferred garden cannot be fetched from configuration (does not exist)",
			fooIdentity,
			"",
			fmt.Sprintf("garden %q is not defined in gardenctl configuration", fooIdentity)),
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

	DescribeTable("saving and loading the linkKubeconfig configuration", func(actVal *bool, envVal string, expVal *bool) {
		envKey := "GCTL_LINK_KUBECONFIG"
		filename := filepath.Join(gardenHomeDir, "gardenctl-v2.yaml")
		cfg = &config.Config{
			Filename:       filename,
			LinkKubeconfig: actVal,
		}
		Expect(cfg.Save()).NotTo(HaveOccurred())
		if envVal == "" {
			os.Unsetenv(envKey)
		} else {
			os.Setenv(envKey, envVal)
			defer os.Unsetenv(envKey)
		}
		cfg, err := config.LoadFromFile(filename)
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg.Filename).To(Equal(filename))
		Expect(cfg.LinkKubeconfig).To(Equal(expVal))
	},
		Entry("when LinkKubeconfig is nil and envVar is unset", nil, "", nil),
		Entry("when LinkKubeconfig is nil and envVar is True", nil, "True", pointer.Bool(true)),
		Entry("when LinkKubeconfig is nil and envVar is False", nil, "False", pointer.Bool(false)),
		Entry("when LinkKubeconfig is true and envVar is unset", pointer.Bool(true), "", pointer.Bool(true)),
		Entry("when LinkKubeconfig is true and envVar is True", pointer.Bool(true), "True", pointer.Bool(true)),
		Entry("when LinkKubeconfig is true and envVar is False", pointer.Bool(true), "False", pointer.Bool(false)),
		Entry("when LinkKubeconfig is false and envVar is unset", pointer.Bool(false), "", pointer.Bool(false)),
		Entry("when LinkKubeconfig is false and envVar is True", pointer.Bool(false), "True", pointer.Bool(true)),
		Entry("when LinkKubeconfig is false and envVar is False", pointer.Bool(false), "False", pointer.Bool(false)),
	)
})
