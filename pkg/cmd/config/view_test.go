/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package config_test

import (
	"encoding/json"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	cmdconfig "github.com/gardener/gardenctl-v2/pkg/cmd/config"
	"github.com/gardener/gardenctl-v2/pkg/config"
)

var _ = Describe("Config Command - View", func() {
	BeforeEach(func() {
		manager.EXPECT().Configuration().Return(cfg)
		factory.EXPECT().Manager().Return(manager, nil)
	})

	It("should print configuration", func() {
		cmd := cmdconfig.NewCmdConfigView(factory, streams)
		Expect(cmd.RunE(cmd, nil)).To(Succeed())

		Expect(out.String()).To(ContainSubstring("gardens"))
		Expect(out.String()).To(ContainSubstring(gardenIdentity1))
		Expect(out.String()).To(ContainSubstring(gardenIdentity2))
		Expect(out.String()).To(ContainSubstring("matchPatterns"))
		Expect(out.String()).To(ContainSubstring(patterns[1]))
	})
})

var _ = Describe("Config Command - View Options", func() {
	Describe("Validating options", func() {
		It("should succeed", func() {
			o := cmdconfig.NewViewOptions()
			Expect(o.Validate()).To(Succeed())
		})
	})
})

var _ = Describe("Config Subcommand View", func() {
	Describe("Instance", func() {
		var cmd *cobra.Command

		BeforeEach(func() {
			cmd = cmdconfig.NewCmdConfigView(factory, streams)
		})

		It("should have ", func() {
			Expect(cmd.Use).To(Equal("view"))
			Expect(cmd.Flag("output")).NotTo(BeNil())
		})
	})

	Describe("Options", func() {
		var options *cmdconfig.ViewOptions

		BeforeEach(func() {
			options = cmdconfig.NewViewOptions()
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
					Expect(options.Complete(factory, nil, nil)).To(Succeed())
					Expect(options.Configuration).To(BeIdenticalTo(cfg))
					Expect(options.Output).To(Equal("yaml"))
				})
			})
		})

		Describe("Validate", func() {
			DescribeTable("Output Flag",
				func(output string, matcher types.GomegaMatcher) {
					options.Output = output
					Expect(options.Validate()).To(matcher)
				},
				Entry("when output is yaml", "yaml", Succeed()),
				Entry("when output is json", "json", Succeed()),
				Entry("when output is empty", "", Succeed()),
				Entry("when output is invalid", "invalid", Not(Succeed())),
			)
		})

		Describe("Run", func() {
			It("should print configuration in json format", func() {
				options.Configuration = cfg
				options.Output = "json"
				Expect(options.Run(nil)).To(Succeed())

				c := &config.Config{Filename: cfg.Filename}
				Expect(json.Unmarshal([]byte(out.String()), c)).To(Succeed())
				Expect(c).To(BeEquivalentTo(cfg))
			})
		})

		Describe("AddFlags", func() {
			It("should add the output flag", func() {
				flags := &pflag.FlagSet{}
				options.AddFlags(flags)
				Expect(flags.Lookup("output")).NotTo(BeNil())
			})
		})
	})
})
