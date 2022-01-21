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

	cmdconfig "github.com/gardener/gardenctl-v2/pkg/cmd/config"
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
