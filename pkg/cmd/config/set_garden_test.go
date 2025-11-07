/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package config_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"github.com/spf13/cobra"

	cmdconfig "github.com/gardener/gardenctl-v2/pkg/cmd/config"
	"github.com/gardener/gardenctl-v2/pkg/config"
)

var _ = Describe("Config Subcommand SetGarden", func() {
	Describe("Instance", func() {
		var cmd *cobra.Command

		BeforeEach(func() {
			cmd = cmdconfig.NewCmdConfigSetGarden(factory, streams)
		})

		It("should have Use, ValidArgsFunction and Flags", func() {
			Expect(cmd.Use).To(Equal("set-garden"))
			Expect(cmd.ValidArgsFunction).NotTo(BeNil())
			Expect(cmd.ValidArgs).To(BeNil())
			assertAllFlagNames(cmd.Flags(), "alias", "context", "kubeconfig", "pattern")
		})
	})

	Describe("Options", func() {
		var options *cmdconfig.SetGardenOptions

		BeforeEach(func() {
			options = cmdconfig.NewSetGardenOptions()
			options.IOStreams = streams
		})

		Describe("Complete", func() {
			Context("when getting configuration fails", func() {
				It("should fail", func() {
					factory.EXPECT().Manager().Return(manager, nil)
					manager.EXPECT().Configuration().Return(nil)
					Expect(options.Complete(factory, nil, nil)).To(MatchError("failed to get configuration"))
				})
			})

			Context("when getting configuration succeeds", func() {
				It("should succeed", func() {
					factory.EXPECT().Manager().Return(manager, nil)
					manager.EXPECT().Configuration().Return(cfg)
					Expect(options.Complete(factory, nil, []string{" garden "})).To(Succeed())
					Expect(options.Configuration).To(BeIdenticalTo(cfg))
					Expect(options.Name).To(Equal("garden"))
				})
			})
		})

		Describe("Validate", func() {
			DescribeTable("Validating Name Argument",
				func(name string, matcher types.GomegaMatcher) {
					o := cmdconfig.NewSetGardenOptions()
					o.Name = name
					Expect(o.Validate()).To(matcher)
				},
				Entry("when garden is foo", "foo", Succeed()),
				Entry("when garden is my-garden", "my-garden", Succeed()),
				Entry("when garden empty", "", MatchError("garden identity is required")),
				Entry("when garden starts with hyphen", "-garden", MatchError("invalid garden name \"-garden\": garden name must start and end with an alphanumeric character")),
			)

			DescribeTable("Validating Pattern Flag",
				func(patterns []string, matcher types.GomegaMatcher) {
					o := cmdconfig.NewSetGardenOptions()
					o.Name = "foo"
					o.Patterns = patterns
					Expect(o.Validate()).To(matcher)
				},
				Entry("when patterns is nil", nil, Succeed()),
				Entry("when 1st pattern is empty", []string{""}, Succeed()),
				Entry("when 1st pattern is empty and 2nd pattern is not empty", []string{"", "foo"}, MatchError("pattern[0] must not be empty")),
				Entry("when all patterns are valid", []string{"^shoot--(?P<project>.+)--(?P<shoot>.+)$`"}, Succeed()),
				Entry("when a pattern is not a valid regular expression", []string{"("}, MatchError(MatchRegexp(`^pattern\[0\] is not a valid regular expression`))),
				Entry("when a pattern has an invalid subexpression name", []string{"^shoot--(?P<cluster>.+)$`"}, MatchError("pattern[0] contains an invalid subexpression \"cluster\"")),
			)

			DescribeTable("Validating Alias Flag",
				func(alias string, shouldSetAlias bool, matcher types.GomegaMatcher) {
					o := cmdconfig.NewSetGardenOptions()
					o.Name = "foo"
					if shouldSetAlias {
						Expect(o.Alias.Set(alias)).To(Succeed())
					}
					Expect(o.Validate()).To(matcher)
				},
				Entry("when alias is not set", "", false, Succeed()),
				Entry("when alias is bar", "bar", true, Succeed()),
				Entry("when alias is my-alias", "my-alias", true, Succeed()),
				Entry("when alias is set but empty", "", true, MatchError("invalid garden alias \"\": garden name must contain only alphanumeric characters, underscore or hyphen")),
				Entry("when alias starts with hyphen", "-alias", true, MatchError("invalid garden alias \"-alias\": garden name must start and end with an alphanumeric character")),
			)
		})

		Describe("Run", func() {
			const pathToKubeconfig = "/path/to/kubeconfig"
			const testContext = "test-context"
			const testPattern = "test-pattern"

			BeforeEach(func() {
				options.Configuration = cfg
			})

			It("should add new garden to configuration", func() {
				options.Name = gardenIdentity3
				Expect(options.KubeconfigFlag.Set(pathToKubeconfig)).To(Succeed())
				Expect(options.Run(nil)).To(Succeed())

				assertGardenNames(cfg, gardenIdentity1, gardenIdentity2, gardenIdentity3)
				assertGarden(cfg, &config.Garden{
					Name:       gardenIdentity3,
					Kubeconfig: pathToKubeconfig,
				})
				assertConfigHasBeenSaved(cfg)
				Expect(out.String()).To(MatchRegexp("^Successfully configured garden"))
			})

			It("should modify an existing garden configuration", func() {
				options.Name = gardenIdentity1
				Expect(options.ContextFlag.Set(testContext)).To(Succeed())
				options.Patterns = []string{testPattern}
				Expect(options.Run(nil)).To(Succeed())

				assertGardenNames(cfg, gardenIdentity1, gardenIdentity2)
				assertGarden(cfg, &config.Garden{
					Name:       gardenIdentity1,
					Kubeconfig: kubeconfig,
					Context:    testContext,
					Patterns:   []string{testPattern},
				})
				assertConfigHasBeenSaved(cfg)
				Expect(out.String()).To(MatchRegexp("^Successfully configured garden"))
			})

			It("should remove all patterns from an existing configuration", func() {
				options.Name = gardenIdentity2
				Expect(options.KubeconfigFlag.Set(pathToKubeconfig)).To(Succeed())
				options.Patterns = []string{""}
				Expect(options.Run(nil)).To(Succeed())

				assertGardenNames(cfg, gardenIdentity1, gardenIdentity2)
				assertGarden(cfg, &config.Garden{
					Name:       gardenIdentity2,
					Kubeconfig: pathToKubeconfig,
				})
				assertConfigHasBeenSaved(cfg)
				Expect(out.String()).To(MatchRegexp("^Successfully configured garden"))
			})

			It("should fail when the filename is invalid", func() {
				options.Configuration.Filename = string([]byte{0})
				Expect(options.Run(nil)).To(MatchError(MatchRegexp("^failed to configure garden")))
			})
		})
	})
})
