/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package version_test

import (
	"github.com/gardener/gardenctl-v2/internal/util"
	. "github.com/gardener/gardenctl-v2/pkg/cmd/version"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Command", func() {
	It("should print version", func() {
		streams, _, out, _ := util.NewTestIOStreams()
		o := NewVersionOptions(streams)
		cmd := NewCmdVersion(&util.FactoryImpl{}, o)

		Expect(cmd.RunE(cmd, nil)).To(Succeed())
		Expect(out.String()).To(ContainSubstring("GitVersion"))
	})
})

var _ = Describe("VersionOptions", func() {
	It("should validate", func() {
		streams, _, _, _ := util.NewTestIOStreams()
		o := NewVersionOptions(streams)
		err := o.Validate()
		Expect(err).ToNot(HaveOccurred())
	})
})
