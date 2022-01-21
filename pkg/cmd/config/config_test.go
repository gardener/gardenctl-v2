/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package config_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	cmdconfig "github.com/gardener/gardenctl-v2/pkg/cmd/config"
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
