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
