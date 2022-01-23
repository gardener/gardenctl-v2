/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package config_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	cmdconfig "github.com/gardener/gardenctl-v2/pkg/cmd/config"
	"github.com/gardener/gardenctl-v2/pkg/config"
)

var _ = Describe("Config Command - SetGarden", func() {
	BeforeEach(func() {
		manager.EXPECT().Configuration().Return(cfg)
		factory.EXPECT().Manager().Return(manager, nil)
	})

	It("should add new garden to configuration", func() {
		Expect(len(cfg.Gardens)).To(Equal(2))

		cmd := cmdconfig.NewCmdConfigSetGarden(factory, streams)
		Expect(cmd.RunE(cmd, []string{gardenIdentity3})).To(Succeed())

		_, err := cfg.Garden(gardenIdentity3)
		Expect(err).ToNot(HaveOccurred())
		Expect(len(cfg.Gardens)).To(Equal(3))
	})

	It("should modify an existing garden configuration", func() {
		Expect(len(cfg.Gardens)).To(Equal(2))

		cmd := cmdconfig.NewCmdConfigSetGarden(factory, streams)
		pathToKubeconfig := "path/to/kubeconfig"
		Expect(cmd.Flags().Set("kubeconfig", pathToKubeconfig)).To(Succeed())
		Expect(cmd.RunE(cmd, []string{gardenIdentity1})).To(Succeed())

		g, err := cfg.Garden(gardenIdentity1)
		Expect(err).ToNot(HaveOccurred())
		Expect(len(cfg.Gardens)).To(Equal(2))
		Expect(g.Name).To(Equal(gardenIdentity1))
		Expect(g.Kubeconfig).To(Equal(pathToKubeconfig))
		// Check that existing value does not get overwritten
		Expect(g.Context).To(Equal(gardenContext1))
	})
})

var _ = Describe("Config Command - SetGarden Options", func() {
	DescribeTable("Validating options",
		func(name string, matcher types.GomegaMatcher) {
			o := cmdconfig.NewSetGardenOptions()
			o.Name = name
			Expect(o.Validate()).To(matcher)
		},
		Entry("when garden is foo", "foo", Succeed()),
		Entry("when garden empty", "", Not(Succeed())),
	)
})

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
			assertAllFlagNames(cmd.Flags(), "context", "kubeconfig", "pattern")
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
				Entry("when garden empty", "", MatchError("garden identity is required")),
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
					Patterns:   []string{},
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

func assertAllFlagNames(flags *pflag.FlagSet, expNames ...string) {
	actNames := []string{}

	flags.VisitAll(func(flag *pflag.Flag) {
		actNames = append(actNames, flag.Name)
	})

	ExpectWithOffset(1, actNames).To(Equal(expNames))
}

func assertGardenNames(cfg *config.Config, names ...string) {
	ExpectWithOffset(1, cfg.GardenNames()).To(Equal(names))
}

func assertGarden(cfg *config.Config, garden *config.Garden) {
	g, err := cfg.Garden(garden.Name)
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	ExpectWithOffset(1, g).To(BeEquivalentTo(garden))
}

func assertConfigHasBeenSaved(cfg *config.Config) {
	c, err := config.LoadFromFile(cfg.Filename)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	for i, g := range cfg.Gardens {
		if len(g.Patterns) == 0 {
			cfg.Gardens[i].Patterns = nil
		}
	}

	ExpectWithOffset(1, c).To(BeEquivalentTo(cfg))
}
