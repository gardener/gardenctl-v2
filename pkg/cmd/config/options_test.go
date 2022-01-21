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

var _ = Describe("Config Command - View Options", func() {
	Describe("Validating options", func() {
		It("should succeed", func() {
			o := cmdconfig.NewOptions("view")
			Expect(o.Validate()).To(Succeed())
		})
	})
})

var _ = Describe("Config Command - SetGarden Options", func() {
	DescribeTable("Validating options",
		func(name string, matcher types.GomegaMatcher) {
			o := cmdconfig.NewOptions("set-garden")
			o.Name = name
			Expect(o.Validate()).To(matcher)
		},
		Entry("when garden is foo", "foo", Succeed()),
		Entry("when garden empty", "", Not(Succeed())),
	)
})

var _ = Describe("Config Command - DeleteGarden Options", func() {
	DescribeTable("Validating options",
		func(name string, matcher types.GomegaMatcher) {
			o := cmdconfig.NewOptions("delete-garden")
			o.Name = name
			Expect(o.Validate()).To(matcher)
		},
		Entry("when garden is foo", "foo", Succeed()),
		Entry("when garden empty", "", Not(Succeed())),
	)
})
