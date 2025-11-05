/*
SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package config_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"

	cmdconfig "github.com/gardener/gardenctl-v2/pkg/cmd/config"
	"github.com/gardener/gardenctl-v2/pkg/config"
	"github.com/gardener/gardenctl-v2/pkg/provider/common/allowpattern"
)

var _ = Describe("Config Subcommand SetOpenStackAuthURL", func() {
	Describe("Instance", func() {
		var cmd *cobra.Command

		BeforeEach(func() {
			cmd = cmdconfig.NewCmdConfigSetOpenStackAuthURL(factory, streams)
		})

		It("should have Use and Flags", func() {
			Expect(cmd.Use).To(Equal("set-openstack-authurl"))
			assertAllFlagNames(cmd.Flags(), "clear", "uri-pattern")
		})
	})

	Describe("Options", func() {
		var options *cmdconfig.SetOpenStackAuthURLOptions

		BeforeEach(func() {
			options = cmdconfig.NewSetOpenStackAuthURLOptions()
			options.IOStreams = streams
		})

		Describe("Complete", func() {
			It("should set configuration from factory", func() {
				factory.EXPECT().Manager().Return(manager, nil)
				manager.EXPECT().Configuration().Return(cfg)

				Expect(options.Complete(factory, nil, nil)).To(Succeed())
				Expect(options.Configuration).To(Equal(cfg))
			})

			It("should fail when factory returns error", func() {
				factory.EXPECT().Manager().Return(nil, errors.New("factory error"))

				Expect(options.Complete(factory, nil, nil)).To(MatchError("failed to get target manager: factory error"))
			})
		})

		Describe("Validate", func() {
			It("should fail when both --clear and --uri-pattern are specified", func() {
				options.Clear = true
				options.URIPatterns = []string{"https://keystone.example.com:5000/v3"}

				err := options.Validate()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("cannot specify --uri-pattern when --clear flag is set"))
			})

			It("should fail when no flags are specified", func() {
				options.Clear = false
				options.URIPatterns = []string{}

				err := options.Validate()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("at least one --uri-pattern must be specified, or use --clear to remove all patterns"))
			})

			It("should succeed with valid URI patterns", func() {
				options.URIPatterns = []string{
					"https://keystone.example.com:5000/v3",
					"https://keystone.another.com/v3",
				}

				Expect(options.Validate()).To(Succeed())
			})

			It("should succeed with --clear flag", func() {
				options.Clear = true

				Expect(options.Validate()).To(Succeed())
			})

			It("should fail with invalid URI pattern", func() {
				options.URIPatterns = []string{"invalid-uri"}

				err := options.Validate()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("validation failed for URI at index 0: invalid value for field authURL"))
			})

			It("should fail with multiple patterns where one is invalid", func() {
				options.URIPatterns = []string{
					"https://keystone.example.com:5000/v3",
					"invalid-uri",
				}

				err := options.Validate()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("validation failed for URI at index 1: invalid value for field authURL"))
			})
		})

		Describe("Run", func() {
			BeforeEach(func() {
				options.Configuration = cfg
			})

			Context("when clearing patterns", func() {
				BeforeEach(func() {
					options.Clear = true
				})

				It("should clear patterns from existing OpenStack config", func() {
					// Set up existing patterns
					cfg.Provider = &config.ProviderConfig{
						OpenStack: &config.OpenStackConfig{
							AllowedPatterns: []allowpattern.Pattern{
								{Field: "authURL", URI: "https://old.example.com:5000/v3"},
							},
						},
					}

					Expect(options.Run(nil)).To(Succeed())

					Expect(cfg.Provider.OpenStack.AllowedPatterns).To(BeNil())
					assertConfigHasBeenSaved(cfg)
					Expect(out.String()).To(ContainSubstring("Successfully cleared all OpenStack authURL patterns"))
				})

				It("should initialize provider config and clear patterns", func() {
					cfg.Provider = nil

					Expect(options.Run(nil)).To(Succeed())

					Expect(cfg.Provider).NotTo(BeNil())
					Expect(cfg.Provider.OpenStack).NotTo(BeNil())
					Expect(cfg.Provider.OpenStack.AllowedPatterns).To(BeNil())
					assertConfigHasBeenSaved(cfg)
					Expect(out.String()).To(ContainSubstring("Successfully cleared all OpenStack authURL patterns"))
				})
			})

			Context("when setting patterns", func() {
				It("should set single URI pattern", func() {
					options.URIPatterns = []string{"https://keystone.example.com:5000/v3"}

					Expect(options.Run(nil)).To(Succeed())

					Expect(cfg.Provider).NotTo(BeNil())
					Expect(cfg.Provider.OpenStack).NotTo(BeNil())
					Expect(cfg.Provider.OpenStack.AllowedPatterns).To(HaveLen(1))
					Expect(cfg.Provider.OpenStack.AllowedPatterns[0]).To(Equal(allowpattern.Pattern{
						Field: "authURL",
						URI:   "https://keystone.example.com:5000/v3",
					}))
					assertConfigHasBeenSaved(cfg)
					Expect(out.String()).To(ContainSubstring("Successfully configured 1 OpenStack authURL pattern(s)"))
				})

				It("should set multiple URI patterns", func() {
					options.URIPatterns = []string{
						"https://keystone.example.com:5000/v3",
						"https://keystone.another.com/v3",
					}

					Expect(options.Run(nil)).To(Succeed())

					Expect(cfg.Provider).NotTo(BeNil())
					Expect(cfg.Provider.OpenStack).NotTo(BeNil())
					Expect(cfg.Provider.OpenStack.AllowedPatterns).To(HaveLen(2))
					Expect(cfg.Provider.OpenStack.AllowedPatterns[0]).To(Equal(allowpattern.Pattern{
						Field: "authURL",
						URI:   "https://keystone.example.com:5000/v3",
					}))
					Expect(cfg.Provider.OpenStack.AllowedPatterns[1]).To(Equal(allowpattern.Pattern{
						Field: "authURL",
						URI:   "https://keystone.another.com/v3",
					}))
					assertConfigHasBeenSaved(cfg)
					Expect(out.String()).To(ContainSubstring("Successfully configured 2 OpenStack authURL pattern(s)"))
				})

				It("should replace existing patterns", func() {
					// Set up existing patterns
					cfg.Provider = &config.ProviderConfig{
						OpenStack: &config.OpenStackConfig{
							AllowedPatterns: []allowpattern.Pattern{
								{Field: "authURL", URI: "https://old.example.com:5000/v3"},
								{Field: "authURL", URI: "https://old2.example.com:5000/v3"},
							},
						},
					}

					options.URIPatterns = []string{"https://new.example.com:5000/v3"}

					Expect(options.Run(nil)).To(Succeed())

					Expect(cfg.Provider.OpenStack.AllowedPatterns).To(HaveLen(1))
					Expect(cfg.Provider.OpenStack.AllowedPatterns[0]).To(Equal(allowpattern.Pattern{
						Field: "authURL",
						URI:   "https://new.example.com:5000/v3",
					}))
					assertConfigHasBeenSaved(cfg)
					Expect(out.String()).To(ContainSubstring("Successfully configured 1 OpenStack authURL pattern(s)"))
				})

				It("should initialize provider config when not present", func() {
					cfg.Provider = nil
					options.URIPatterns = []string{"https://keystone.example.com:5000/v3"}

					Expect(options.Run(nil)).To(Succeed())

					Expect(cfg.Provider).NotTo(BeNil())
					Expect(cfg.Provider.OpenStack).NotTo(BeNil())
					Expect(cfg.Provider.OpenStack.AllowedPatterns).To(HaveLen(1))
					assertConfigHasBeenSaved(cfg)
				})

				It("should initialize OpenStack config when provider exists but OpenStack doesn't", func() {
					cfg.Provider = &config.ProviderConfig{}
					options.URIPatterns = []string{"https://keystone.example.com:5000/v3"}

					Expect(options.Run(nil)).To(Succeed())

					Expect(cfg.Provider.OpenStack).NotTo(BeNil())
					Expect(cfg.Provider.OpenStack.AllowedPatterns).To(HaveLen(1))
					assertConfigHasBeenSaved(cfg)
				})
			})

			It("should fail when configuration save fails", func() {
				cfg.Filename = "/invalid/path/that/cannot/be/written"
				options.URIPatterns = []string{"https://keystone.example.com:5000/v3"}

				err := options.Run(nil)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to save configuration"))
			})
		})
	})
})
