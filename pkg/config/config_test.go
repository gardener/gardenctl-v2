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

	Describe("OpenStackConfig", func() {
		Describe("Validate", func() {
			It("should validate empty config", func() {
				openstackConfig := &config.OpenStackConfig{}
				err := openstackConfig.Validate()
				Expect(err).NotTo(HaveOccurred())
			})

			It("should validate config with valid patterns (URI)", func() {
				openstackConfig := &config.OpenStackConfig{
					AllowedPatterns: []allowpattern.Pattern{
						{
							Field: "authURL",
							URI:   "https://keystone.example.com:5000/v3",
						},
					},
				}
				err := openstackConfig.Validate()
				Expect(err).NotTo(HaveOccurred())
			})

			It("should reject config with invalid pattern (unsupported scheme)", func() {
				openstackConfig := &config.OpenStackConfig{
					AllowedPatterns: []allowpattern.Pattern{
						{
							Field: "authURL",
							URI:   "ftp://keystone.example.com/v3", // Invalid: unsupported scheme
						},
					},
				}
				err := openstackConfig.Validate()
				Expect(err).To(MatchError(ContainSubstring("invalid allowed pattern at index 0")))
				Expect(err).To(MatchError(ContainSubstring("invalid value for field authURL: scheme must be one of {https, http}, got \"ftp\"")))
			})
		})

		Describe("IsUserProvided flag", func() {
			It("should set IsUserProvided=true when loading patterns from config file", func() {
				filename := filepath.Join(gardenHomeDir, "gardenctl-v2.yaml")
				cfgData := `gardens:
- identity: test-garden
  kubeconfig: /path/to/kubeconfig
provider:
  openstack:
    allowedPatterns:
    - field: authURL
      uri: https://keystone.example.com:5000/v3
    - field: authURL
      host: keystone2.example.com
      path: /v3
`
				Expect(os.WriteFile(filename, []byte(cfgData), 0o600)).To(Succeed())

				cfg, err := config.LoadFromFile(filename)
				Expect(err).NotTo(HaveOccurred())
				Expect(cfg.Provider).NotTo(BeNil())
				Expect(cfg.Provider.OpenStack).NotTo(BeNil())
				Expect(cfg.Provider.OpenStack.AllowedPatterns).To(HaveLen(2))

				// Verify all patterns loaded from config have IsUserProvided=true
				for i, pattern := range cfg.Provider.OpenStack.AllowedPatterns {
					Expect(pattern.IsUserProvided).To(BeTrue(), "pattern at index %d should have IsUserProvided=true", i)
				}
			})

			It("should not serialize IsUserProvided field when saving config", func() {
				filename := filepath.Join(gardenHomeDir, "gardenctl-v2.yaml")
				cfg := &config.Config{
					Filename: filename,
					Gardens: []config.Garden{
						{
							Name:       "test-garden",
							Kubeconfig: "/path/to/kubeconfig",
						},
					},
					Provider: &config.ProviderConfig{
						OpenStack: &config.OpenStackConfig{
							AllowedPatterns: []allowpattern.Pattern{
								{
									Field:          "authURL",
									URI:            "https://keystone.example.com:5000/v3",
									IsUserProvided: true, // This should not be serialized
								},
							},
						},
					},
				}

				Expect(cfg.Save()).To(Succeed())

				// Read the file and verify IsUserProvided is not in the YAML
				content, err := os.ReadFile(filename)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).NotTo(ContainSubstring("isUserProvided"))
				Expect(string(content)).NotTo(ContainSubstring("IsUserProvided"))
			})

			It("should ignore IsUserProvided field in config file if present", func() {
				filename := filepath.Join(gardenHomeDir, "gardenctl-v2.yaml")
				// Manually craft YAML with IsUserProvided field (which should be ignored)
				cfgData := `gardens:
- identity: test-garden
  kubeconfig: /path/to/kubeconfig
provider:
  openstack:
    allowedPatterns:
    - field: authURL
      uri: https://keystone.example.com:5000/v3
      isUserProvided: false
`
				Expect(os.WriteFile(filename, []byte(cfgData), 0o600)).To(Succeed())

				cfg, err := config.LoadFromFile(filename)
				Expect(err).NotTo(HaveOccurred())
				Expect(cfg.Provider).NotTo(BeNil())
				Expect(cfg.Provider.OpenStack).NotTo(BeNil())
				Expect(cfg.Provider.OpenStack.AllowedPatterns).To(HaveLen(1))

				// Even if isUserProvided: false is in the YAML, it should be set to true
				// because the custom UnmarshalJSON always sets it to true
				Expect(cfg.Provider.OpenStack.AllowedPatterns[0].IsUserProvided).To(BeTrue())
			})
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

		It("should reject config with invalid garden name ($)", func() {
			filename := filepath.Join(gardenHomeDir, "gardenctl-v2.yaml")
			cfgData := `gardens:
- identity: garden$test
  kubeconfig: /path/to/kubeconfig
`
			Expect(os.WriteFile(filename, []byte(cfgData), 0o600)).To(Succeed())

			_, err := config.LoadFromFile(filename)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("invalid garden name \"garden$test\"")))
		})

		It("should reject config with garden name starting with hyphen", func() {
			filename := filepath.Join(gardenHomeDir, "gardenctl-v2.yaml")
			cfgData := `gardens:
- identity: -garden
  kubeconfig: /path/to/kubeconfig
`
			Expect(os.WriteFile(filename, []byte(cfgData), 0o600)).To(Succeed())

			_, err := config.LoadFromFile(filename)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("invalid garden name \"-garden\"")))
			Expect(err).To(MatchError(ContainSubstring("must start and end with an alphanumeric character")))
		})

		It("should reject config with invalid garden alias ($)", func() {
			filename := filepath.Join(gardenHomeDir, "gardenctl-v2.yaml")
			cfgData := `gardens:
- identity: my-garden
  name: Invalid$Alias
  kubeconfig: /path/to/kubeconfig
`
			Expect(os.WriteFile(filename, []byte(cfgData), 0o600)).To(Succeed())

			_, err := config.LoadFromFile(filename)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("invalid garden alias \"Invalid$Alias\"")))
			Expect(err).To(MatchError(ContainSubstring("must contain only alphanumeric characters, underscore or hyphen")))
		})

		It("should accept config with valid garden names", func() {
			filename := filepath.Join(gardenHomeDir, "gardenctl-v2.yaml")
			cfgData := `gardens:
- identity: my-garden
  name: my_alias
  kubeconfig: /path/to/kubeconfig
- identity: garden123
  kubeconfig: /path/to/kubeconfig2
`
			Expect(os.WriteFile(filename, []byte(cfgData), 0o600)).To(Succeed())

			cfg, err := config.LoadFromFile(filename)
			Expect(err).NotTo(HaveOccurred())
			Expect(cfg.Gardens).To(HaveLen(2))
		})
	})

	Describe("validating garden name", func() {
		It("should accept valid garden names", func() {
			Expect(config.ValidateGardenName("foo")).To(Succeed())
			Expect(config.ValidateGardenName("my-garden")).To(Succeed())
			Expect(config.ValidateGardenName("my_garden")).To(Succeed())
			Expect(config.ValidateGardenName("MyGarden")).To(Succeed())
			Expect(config.ValidateGardenName("garden123")).To(Succeed())
			Expect(config.ValidateGardenName("test-garden-123")).To(Succeed())
			Expect(config.ValidateGardenName("test_garden_123")).To(Succeed())
			Expect(config.ValidateGardenName("test-garden_123")).To(Succeed())
			Expect(config.ValidateGardenName("a")).To(Succeed())
			Expect(config.ValidateGardenName("1")).To(Succeed())

			// Underscore alone does not start/end with alphanumeric, so it should fail
			// Expect(config.ValidateGardenName("_")).To(Succeed())
		})

		It("should reject garden names starting with hyphen", func() {
			Expect(config.ValidateGardenName("-garden")).To(MatchError("garden name must start and end with an alphanumeric character"))
			Expect(config.ValidateGardenName("-test")).To(MatchError("garden name must start and end with an alphanumeric character"))
		})

		It("should reject garden names ending with hyphen", func() {
			Expect(config.ValidateGardenName("garden-")).To(MatchError("garden name must start and end with an alphanumeric character"))
			Expect(config.ValidateGardenName("test-")).To(MatchError("garden name must start and end with an alphanumeric character"))
		})

		It("should reject garden names with invalid characters", func() {
			Expect(config.ValidateGardenName("my.garden")).To(MatchError("garden name must contain only alphanumeric characters, underscore or hyphen"))
			Expect(config.ValidateGardenName("my garden")).To(MatchError("garden name must contain only alphanumeric characters, underscore or hyphen"))
			Expect(config.ValidateGardenName("my@garden")).To(MatchError("garden name must contain only alphanumeric characters, underscore or hyphen"))
		})

		It("should reject garden names starting or ending with underscore", func() {
			Expect(config.ValidateGardenName("_garden")).To(MatchError("garden name must start and end with an alphanumeric character"))
			Expect(config.ValidateGardenName("garden_")).To(MatchError("garden name must start and end with an alphanumeric character"))
			Expect(config.ValidateGardenName("_")).To(MatchError("garden name must start and end with an alphanumeric character"))
		})
	})
})
