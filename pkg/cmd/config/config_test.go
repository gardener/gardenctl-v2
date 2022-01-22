/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package config_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	cmdconfig "github.com/gardener/gardenctl-v2/pkg/cmd/config"
	"github.com/gardener/gardenctl-v2/pkg/config"
)

var _ = Describe("Config Command", func() {
	Describe("Instance", func() {
		var cmd *cobra.Command

		BeforeEach(func() {
			cmd = cmdconfig.NewCmdConfig(factory, streams)
		})

		It("should have 3 subcommands", func() {
			Expect(cmd.Use).To(Equal("config"))
			subCommands := []string{}
			for _, c := range cmd.Commands() {
				subCommands = append(subCommands, c.Name())
			}
			Expect(subCommands).To(Equal([]string{"delete-garden", "set-garden", "view"}))
		})

		Describe("Execute Subcommands", func() {
			BeforeEach(func() {
				factory.EXPECT().Manager().Return(manager, nil)
				manager.EXPECT().Configuration().Return(cfg)
			})

			It("should successfully run subcommand view", func() {
				cmd.SetArgs([]string{
					"view",
				})
				Expect(cmd.Execute()).To(Succeed())

				c := &config.Config{Filename: cfg.Filename}
				Expect(yaml.Unmarshal([]byte(out.String()), c)).To(Succeed())
				Expect(c).To(BeEquivalentTo(cfg))
			})

			It("should successfully run subcommand set-garden", func() {
				garden := &config.Garden{
					Name:       gardenIdentity3,
					Kubeconfig: "/path/to/kubeconfig",
					Context:    "default",
				}
				cmd.SetArgs([]string{
					"set-garden",
					gardenIdentity3,
					"--kubeconfig=" + garden.Kubeconfig,
					"--context=" + garden.Context,
				})
				Expect(cmd.Execute()).To(Succeed())

				Expect(cfg.GardenNames()).To(Equal([]string{
					gardenIdentity1,
					gardenIdentity2,
					gardenIdentity3,
				}))
				g, err := cfg.Garden(gardenIdentity3)
				Expect(err).NotTo(HaveOccurred())
				Expect(g).To(BeEquivalentTo(garden))
				c, err := config.LoadFromFile(cfg.Filename)
				Expect(err).NotTo(HaveOccurred())
				Expect(c).To(BeEquivalentTo(cfg))
			})

			It("should successfully run subcommand delete-garden", func() {
				cmd.SetArgs([]string{
					"delete-garden",
					gardenIdentity2,
				})
				Expect(cmd.Execute()).To(Succeed())

				Expect(cfg.GardenNames()).To(Equal([]string{
					gardenIdentity1,
				}))
				c, err := config.LoadFromFile(cfg.Filename)
				Expect(err).NotTo(HaveOccurred())
				Expect(c).To(BeEquivalentTo(cfg))
			})
		})
	})

	Describe("Helper Functions", func() {
		Describe("Completing the configured gardens", func() {
			var validGardenArgsFunction cmdconfig.CobraValidArgsFunction

			BeforeEach(func() {
				validGardenArgsFunction = cmdconfig.ValidGardenArgsFunctionWrapper(factory, streams)
			})

			Context("when args are empty and no error occurs", func() {
				BeforeEach(func() {
					factory.EXPECT().Manager().Return(manager, nil)
					manager.EXPECT().Configuration().Return(cfg)
				})

				It(`should return one garden for ""`, func() {
					values, directive := validGardenArgsFunction(nil, nil, "")
					Expect(values).To(Equal([]string{gardenIdentity1, gardenIdentity2}))
					Expect(directive).To(Equal(cobra.ShellCompDirectiveNoFileComp))
				})

				It(`should return one garden for "foo"`, func() {
					values, directive := validGardenArgsFunction(nil, nil, "foo")
					Expect(values).To(Equal([]string{gardenIdentity1}))
					Expect(directive).To(Equal(cobra.ShellCompDirectiveNoFileComp))
				})
			})

			Context("when args are empty and config does not exist", func() {
				It("should return nothing", func() {
					factory.EXPECT().Manager().Return(manager, nil)
					manager.EXPECT().Configuration().Return(nil)

					values, directive := validGardenArgsFunction(nil, []string{}, "")
					Expect(values).To(BeNil())
					Expect(directive).To(Equal(cobra.ShellCompDirectiveNoFileComp))
					Expect(errOut.String()).To(Equal("failed to get configuration\n"))
				})
			})

			Context("when args are empty and an error occurs", func() {
				It("should return nothing", func() {
					factory.EXPECT().Manager().Return(nil, errors.New("error"))

					values, directive := validGardenArgsFunction(nil, []string{}, "")
					Expect(values).To(BeNil())
					Expect(directive).To(Equal(cobra.ShellCompDirectiveNoFileComp))
					Expect(errOut.String()).To(MatchRegexp("^failed to get target manager:"))
				})
			})

			Context("when args are not empty", func() {
				It("should return nothing", func() {
					values, directive := validGardenArgsFunction(nil, []string{gardenIdentity1}, "")
					Expect(values).To(BeNil())
					Expect(directive).To(Equal(cobra.ShellCompDirectiveNoFileComp))
				})
			})
		})
	})
})
