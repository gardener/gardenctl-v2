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

var _ = Describe("Config Command - View Options", func() {
	Describe("Validating options", func() {
		It("should succeed", func() {
			o := cmdconfig.NewViewOptions()
			Expect(o.Validate()).To(Succeed())
		})
	})
})
