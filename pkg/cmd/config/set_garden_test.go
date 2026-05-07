/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package config_test

import (
	"errors"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"github.com/spf13/cobra"

	cmdconfig "github.com/gardener/gardenctl-v2/pkg/cmd/config"
	"github.com/gardener/gardenctl-v2/pkg/config"
	"github.com/gardener/gardenctl-v2/pkg/target"
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
			assertAllFlagNames(cmd.Flags(), "alias", "context", "default-managed-seed-access-level", "default-shoot-access-level", "kubeconfig", "pattern")
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
					Expect(options.Manager).To(BeIdenticalTo(manager))
					Expect(options.Name).To(Equal("garden"))
				})
			})

			Context("when getting the manager fails", func() {
				It("should propagate the error", func() {
					factory.EXPECT().Manager().Return(nil, errors.New("boom"))
					Expect(options.Complete(factory, nil, nil)).To(MatchError(MatchRegexp("^failed to get target manager")))
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
			const (
				pathToKubeconfig = "/path/to/kubeconfig"
				testContext      = "test-context"
				testPattern      = "test-pattern"
			)

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

			Describe("default kubeconfig access level flags", func() {
				It("sets shoots and managedSeeds on a new garden", func() {
					options.Name = gardenIdentity3
					Expect(options.KubeconfigFlag.Set(pathToKubeconfig)).To(Succeed())
					Expect(options.DefaultShootAccessLevelFlag.Set(string(config.KubeconfigAccessLevelViewer))).To(Succeed())
					Expect(options.DefaultManagedSeedAccessLevelFlag.Set(string(config.KubeconfigAccessLevelAdmin))).To(Succeed())
					Expect(options.Run(nil)).To(Succeed())

					assertGarden(cfg, &config.Garden{
						Name:       gardenIdentity3,
						Kubeconfig: pathToKubeconfig,
						DefaultKubeconfigAccessLevel: &config.KubeconfigAccessLevels{
							Shoots:       config.KubeconfigAccessLevelViewer,
							ManagedSeeds: config.KubeconfigAccessLevelAdmin,
						},
					})
				})

				It("sets only the provided sub-field on an existing garden, leaving the other untouched", func() {
					options.Name = gardenIdentity1
					Expect(options.DefaultShootAccessLevelFlag.Set(string(config.KubeconfigAccessLevelViewer))).To(Succeed())
					Expect(options.Run(nil)).To(Succeed())

					garden, err := cfg.Garden(gardenIdentity1)
					Expect(err).NotTo(HaveOccurred())
					Expect(garden.DefaultKubeconfigAccessLevel).NotTo(BeNil())
					Expect(garden.DefaultKubeconfigAccessLevel.Shoots).To(Equal(config.KubeconfigAccessLevelViewer))
					Expect(garden.DefaultKubeconfigAccessLevel.ManagedSeeds).To(BeEmpty())
				})

				It("clears the struct when both fields end up empty", func() {
					options.Name = gardenIdentity1
					// Pre-seed the garden with a non-nil struct so we can prove it gets cleared.
					garden, err := cfg.Garden(gardenIdentity1)
					Expect(err).NotTo(HaveOccurred())

					garden.DefaultKubeconfigAccessLevel = &config.KubeconfigAccessLevels{
						Shoots: config.KubeconfigAccessLevelViewer,
					}

					Expect(options.DefaultShootAccessLevelFlag.Set("")).To(Succeed())
					Expect(options.Run(nil)).To(Succeed())

					garden, err = cfg.Garden(gardenIdentity1)
					Expect(err).NotTo(HaveOccurred())
					Expect(garden.DefaultKubeconfigAccessLevel).To(BeNil())
				})

				It("rejects an invalid value at flag-parse time", func() {
					Expect(options.DefaultShootAccessLevelFlag.Set("guest")).
						To(MatchError(ContainSubstring(`invalid kubeconfig access level "guest"`)))
				})
			})

			Describe("--default-*-access-level flags via real cobra flag parsing", func() {
				It("clears the per-scope field when an empty value is parsed", func() {
					// Pre-seed the existing garden with a non-nil struct.
					existing, err := cfg.Garden(gardenIdentity1)

					Expect(err).NotTo(HaveOccurred())

					existing.DefaultKubeconfigAccessLevel = &config.KubeconfigAccessLevels{
						Shoots: config.KubeconfigAccessLevelViewer,
					}

					factory.EXPECT().Manager().Return(manager, nil)
					manager.EXPECT().Configuration().Return(cfg)
					// Modified garden is not the current target -> refresh path is skipped.
					manager.EXPECT().CurrentTarget().Return(target.NewTarget("", "", "", ""), nil)
					factory.EXPECT().Context().Return(nil).AnyTimes()

					cmd := cmdconfig.NewCmdConfigSetGarden(factory, streams)
					cmd.SetArgs([]string{
						gardenIdentity1,
						"--default-shoot-access-level=",
					})
					Expect(cmd.Execute()).To(Succeed())

					updated, err := cfg.Garden(gardenIdentity1)
					Expect(err).NotTo(HaveOccurred())
					Expect(updated.DefaultKubeconfigAccessLevel).To(BeNil())
				})

				It("rejects an invalid value during cobra parse, never reaching Run", func() {
					// No factory/manager EXPECTs: cobra rejects the flag value before
					// PreRun/Run fires, so none of our code is invoked.
					cmd := cmdconfig.NewCmdConfigSetGarden(factory, streams)
					cmd.SetArgs([]string{
						gardenIdentity1,
						"--default-shoot-access-level=guest",
					})
					Expect(cmd.Execute()).To(MatchError(ContainSubstring(`invalid kubeconfig access level "guest"`)))
				})
			})

			Describe("automatic kubeconfig refresh after a config change", func() {
				BeforeEach(func() {
					// The Run path needs a Factory only for f.Context() in the refresh
					// soft-fail branch, so a mock that returns a real ctx is enough.
					factory.EXPECT().Context().Return(nil).AnyTimes()

					options.Manager = manager
				})

				It("refreshes the kubeconfig when the modified garden is the current target", func() {
					options.Name = gardenIdentity1
					Expect(options.DefaultShootAccessLevelFlag.Set(string(config.KubeconfigAccessLevelViewer))).To(Succeed())

					manager.EXPECT().CurrentTarget().Return(target.NewTarget(gardenIdentity1, "", "", ""), nil)
					manager.EXPECT().RefreshKubeconfig(gomock.Any()).Return(nil)

					Expect(options.Run(factory)).To(Succeed())
				})

				It("does not refresh when the modified garden is not the current target", func() {
					options.Name = gardenIdentity1
					Expect(options.DefaultShootAccessLevelFlag.Set(string(config.KubeconfigAccessLevelViewer))).To(Succeed())

					manager.EXPECT().CurrentTarget().Return(target.NewTarget(gardenIdentity2, "", "", ""), nil)
					// No RefreshKubeconfig expectation - Mock would FAIL on an unexpected call.

					Expect(options.Run(factory)).To(Succeed())
				})

				It("soft-fails (warns, does not error) when refresh fails", func() {
					options.Name = gardenIdentity1
					Expect(options.DefaultShootAccessLevelFlag.Set(string(config.KubeconfigAccessLevelViewer))).To(Succeed())

					manager.EXPECT().CurrentTarget().Return(target.NewTarget(gardenIdentity1, "", "", ""), nil)
					manager.EXPECT().RefreshKubeconfig(gomock.Any()).Return(errors.New("refresh boom"))

					Expect(options.Run(factory)).To(Succeed())
					Expect(errOut.String()).To(ContainSubstring("Warning: failed to refresh kubeconfig"))
				})
			})
		})
	})
})
