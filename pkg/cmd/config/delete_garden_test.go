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

var _ = Describe("Config Command - DeleteGarden", func() {
	BeforeEach(func() {
		manager.EXPECT().Configuration().Return(cfg)
		factory.EXPECT().Manager().Return(manager, nil)
	})

	It("should delete garden from configuration", func() {
		_, err := cfg.Garden(gardenIdentity1)
		Expect(err).ToNot(HaveOccurred())
		Expect(len(cfg.Gardens)).To(Equal(2))

		cmd := cmdconfig.NewCmdConfigDeleteGarden(factory, streams)
		Expect(cmd.RunE(cmd, []string{gardenIdentity1})).To(Succeed())

		_, err = cfg.Garden(gardenIdentity1)
		Expect(err).To(HaveOccurred())
		Expect(len(cfg.Gardens)).To(Equal(1))
	})
})

var _ = Describe("Config Command - DeleteGarden Options", func() {
	DescribeTable("Validating options",
		func(name string, matcher types.GomegaMatcher) {
			o := cmdconfig.NewDeleteGardenOptions()
			o.Name = name
			Expect(o.Validate()).To(matcher)
		},
		Entry("when garden is foo", "foo", Succeed()),
		Entry("when garden empty", "", Not(Succeed())),
	)
})

var _ = Describe("Config Subcommand DeleteGarden", func() {
	Describe("Instance", func() {
		var cmd *cobra.Command

		BeforeEach(func() {
			cmd = cmdconfig.NewCmdConfigDeleteGarden(factory, streams)
		})

		It("should have Use, ValidArgsFunction and no Flags", func() {
			Expect(cmd.Use).To(Equal("delete-garden"))
			Expect(cmd.ValidArgsFunction).NotTo(BeNil())
			Expect(cmd.ValidArgs).To(BeNil())
			hasFlags := false
			cmd.Flags().VisitAll(func(flag *pflag.Flag) { hasFlags = true })
			Expect(hasFlags).To(BeFalse())
		})
	})

	Describe("Options", func() {
		var options *cmdconfig.DeleteGardenOptions

		BeforeEach(func() {
			options = cmdconfig.NewDeleteGardenOptions()
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
					o := cmdconfig.NewDeleteGardenOptions()
					o.Name = name
					Expect(o.Validate()).To(matcher)
				},
				Entry("when garden is foo", "foo", Succeed()),
				Entry("when garden empty", "", MatchError("garden identity is required")),
			)
		})

		Describe("Run", func() {
			BeforeEach(func() {
				options.Configuration = cfg
				options.Name = gardenIdentity1
			})

			It("should delete garden from configuration", func() {
				Expect(options.Run(nil)).To(Succeed())

				Expect(cfg.GardenNames()).To(Equal([]string{
					gardenIdentity2,
				}))
				c, err := config.LoadFromFile(cfg.Filename)
				Expect(err).NotTo(HaveOccurred())
				Expect(c).To(BeEquivalentTo(cfg))
				Expect(out.String()).To(MatchRegexp("^Successfully deleted garden"))
			})

			It("should fail when the garden does not exist", func() {
				options.Name = gardenIdentity3
				Expect(options.Run(nil)).To(MatchError(MatchRegexp(`^garden ".*" is not defined`)))
			})

			It("should fail when the filename is invalid", func() {
				options.Configuration.Filename = string([]byte{0})
				Expect(options.Run(nil)).To(MatchError(MatchRegexp("^failed to delete garden")))
			})
		})
	})
})
