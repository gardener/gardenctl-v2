/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package config_test

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardenctl-v2/pkg/config"
	"github.com/gardener/gardenctl-v2/pkg/provider/common/allowpattern"
)

var _ = Describe("Config", func() {
	var (
		gardenHomeDir    string
		clusterIdentity1 = "garden1"
		clusterIdentity2 = "garden2"
		clusterAlias1    = "gardenalias1"
		clusterAlias2    = "gardenalias2"
		fooIdentity      = "fooGarden"
		project          = "fooProject"
		shoot            = "fooShoot"
		cfg              *config.Config
	)

	BeforeEach(func() {
		dir, err := os.MkdirTemp("", "garden-*")
		Expect(err).NotTo(HaveOccurred())
		gardenHomeDir = dir

		cfg = &config.Config{
			LinkKubeconfig: ptr.To(false),
			Gardens: []config.Garden{
				{
					Name:  clusterIdentity1,
					Alias: clusterAlias1,
					Patterns: []string{
						fmt.Sprintf("^%s/shoot--(?P<project>.+)--(?P<shoot>.+)$", clusterIdentity1),
						"^shoot--(?P<project>.+)--(?P<shoot>.+)$",
					},
				},
				{
					Name:  clusterIdentity2,
					Alias: clusterAlias2,
					Patterns: []string{
						fmt.Sprintf("^(%s/)?shoot--(?P<project>.+)--(?P<shoot>.+)$", clusterIdentity2),
					},
				},
			},
		}
	})

	AfterEach(func() {
		Expect(os.RemoveAll(gardenHomeDir)).To(Succeed())
	})

	patternValue := func(prefix string) string {
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

	DescribeTable("Should find garden by identity and alias", func(name, identityOrAlias string) {
		garden, err := cfg.Garden(name)
		Expect(err).NotTo(HaveOccurred())
		Expect(garden.Name).Should(Equal(identityOrAlias))
	},
		Entry("should find garden by identity", clusterIdentity1, clusterIdentity1),
		Entry("should find garden by alias", clusterAlias2, clusterIdentity2),
	)

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
		Entry("when LinkKubeconfig is nil and envVar is True", nil, "True", ptr.To(true)),
		Entry("when LinkKubeconfig is nil and envVar is False", nil, "False", ptr.To(false)),
		Entry("when LinkKubeconfig is true and envVar is unset", ptr.To(true), "", ptr.To(true)),
		Entry("when LinkKubeconfig is true and envVar is True", ptr.To(true), "True", ptr.To(true)),
		Entry("when LinkKubeconfig is true and envVar is False", ptr.To(true), "False", ptr.To(false)),
		Entry("when LinkKubeconfig is false and envVar is unset", ptr.To(false), "", ptr.To(false)),
		Entry("when LinkKubeconfig is false and envVar is True", ptr.To(false), "True", ptr.To(true)),
		Entry("when LinkKubeconfig is false and envVar is False", ptr.To(false), "False", ptr.To(false)),
	)

	It("should create configuration directories and file", func() {
		dir, err := os.MkdirTemp("", "home-*")
		Expect(err).NotTo(HaveOccurred())
		defer os.RemoveAll(dir)

		filename := filepath.Join(dir, ".garden", "gardenctl-v2.yaml")

		cfg = &config.Config{
			Filename: filename,
		}
		Expect(cfg.Save()).NotTo(HaveOccurred())
	})

	Describe("GCPConfig", func() {
		Describe("Validate", func() {
			It("should validate empty config", func() {
				gcpConfig := &config.GCPConfig{}
				err := gcpConfig.Validate()
				Expect(err).NotTo(HaveOccurred())
			})

			It("should validate config with valid patterns", func() {
				gcpConfig := &config.GCPConfig{
					AllowedPatterns: []allowpattern.Pattern{
						{
							Field: "token_uri",
							URI:   "https://oauth2.googleapis.com/token",
						},
						{
							Field: "universe_domain",
							Host:  ptr.To("googleapis.com"),
							Path:  ptr.To(""),
						},
					},
				}
				err := gcpConfig.Validate()
				Expect(err).NotTo(HaveOccurred())
			})

			It("should reject config with invalid pattern", func() {
				gcpConfig := &config.GCPConfig{
					AllowedPatterns: []allowpattern.Pattern{
						{
							Field: "token_uri",
							URI:   "http://oauth2.googleapis.com/token", // Invalid: HTTP instead of HTTPS
						},
					},
				}
				err := gcpConfig.Validate()
				Expect(err).To(MatchError(ContainSubstring("invalid allowed pattern at index 0")))
				Expect(err).To(MatchError(ContainSubstring("invalid value for field token_uri: scheme must be one of {https}, got \"http\"")))
			})
		})
	})

	Describe("Config with GCP provider", func() {
		It("should validate config with valid GCP provider configuration", func() {
			cfg := &config.Config{
				Provider: &config.ProviderConfig{
					GCP: &config.GCPConfig{
						AllowedPatterns: []allowpattern.Pattern{
							{
								Field: "token_uri",
								URI:   "https://oauth2.googleapis.com/token",
							},
						},
					},
				},
			}
			err := cfg.Validate()
			Expect(err).NotTo(HaveOccurred())
		})

		It("should reject config with invalid GCP provider configuration", func() {
			cfg := &config.Config{
				Provider: &config.ProviderConfig{
					GCP: &config.GCPConfig{
						AllowedPatterns: []allowpattern.Pattern{
							{
								Field: "token_uri",
								URI:   "http://oauth2.googleapis.com/token", // Invalid: HTTP instead of HTTPS
							},
						},
					},
				},
			}
			err := cfg.Validate()
			Expect(err).To(MatchError(ContainSubstring("invalid GCP provider configuration")))
		})
	})

	Describe("#LoadFromFile", func() {
		It("should succeed when file does not exist", func() {
			filename := filepath.Join(gardenHomeDir, "gardenctl-v2.yaml")
			cfg = &config.Config{
				Filename: filename,
			}

			cfg, err := config.LoadFromFile(filename)
			Expect(err).NotTo(HaveOccurred())
			Expect(cfg.Filename).To(Equal(filename))
			Expect(cfg.Gardens).To(BeNil())
		})

		It("should succeed when file is empty", func() {
			filename := filepath.Join(gardenHomeDir, "gardenctl-v2.yaml")
			_, err := os.Create(filename)
			Expect(err).NotTo(HaveOccurred())

			cfg = &config.Config{
				Filename: filename,
			}

			cfg, err := config.LoadFromFile(filename)
			Expect(err).NotTo(HaveOccurred())
			Expect(cfg.Filename).To(Equal(filename))
			Expect(cfg.Gardens).To(BeNil())
		})
	})
})
