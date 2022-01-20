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

var _ = Describe("Config Commands - Options", func() {
	DescribeTable("Validation of config subcommand",
		func(cmd string, name string, matcher types.GomegaMatcher) {
			o := cmdconfig.NewOptions(cmd)
			o.Name = name
			Expect(o.Validate()).To(matcher)
		},
		Entry("for command view and garden foo", "view", "foo", Succeed()),
		Entry("for command view and no garden", "view", "", Succeed()),
		Entry("for command set-garden and garden foo", "set-garden", "foo", Succeed()),
		Entry("for command set-garden and no garden", "set-garden", "", Not(Succeed())),
		Entry("for command delete-garden and garden foo", "delete-garden", "foo", Succeed()),
		Entry("for command delete-garden and no garden", "delete-garden", "", Not(Succeed())),
	)
})
