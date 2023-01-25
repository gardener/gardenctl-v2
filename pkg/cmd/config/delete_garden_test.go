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
)

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
			assertAllFlagNames(cmd.Flags())
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
			})

			It("should delete garden from configuration", func() {
				options.Name = gardenIdentity1
				Expect(options.Run(nil)).To(Succeed())

				assertGardenNames(cfg, gardenIdentity2)
				assertConfigHasBeenSaved(cfg)
				Expect(out.String()).To(MatchRegexp("^Successfully deleted garden"))
			})

			It("should fail when the garden does not exist", func() {
				options.Name = gardenIdentity3
				Expect(options.Run(nil)).To(MatchError(MatchRegexp(`^garden ".*" is not defined`)))
			})

			It("should fail when the filename is invalid", func() {
				options.Name = gardenIdentity1
				options.Configuration.Filename = string([]byte{0})
				Expect(options.Run(nil)).To(MatchError(MatchRegexp("^failed to delete garden")))
			})
		})
	})
})
