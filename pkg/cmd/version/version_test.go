/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package version_test

import (
	"encoding/json"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardenctl-v2/internal/util"
	. "github.com/gardener/gardenctl-v2/pkg/cmd/version"
)

var _ = Describe("Version Command", func() {
	var (
		o       *VersionOptions
		factory util.Factory
		streams util.IOStreams
		buf     *util.SafeBytesBuffer
		args    []string
	)

	BeforeEach(func() {
		streams, _, buf, _ = util.NewTestIOStreams()
		o = NewVersionOptions(streams)
		factory = &util.FactoryImpl{}
		args = make([]string, 0, 5)
	})

	It("should run the command without any flags", func() {
		cmd := NewCmdVersion(factory, o)
		cmd.SetArgs(args)
		Expect(cmd.Execute()).To(Succeed())
		Expect(o.Short).To(BeFalse())
		Expect(o.Output).To(BeEmpty())
		Expect(buf.String()).To(MatchRegexp("^Version: version.Info{.*GitVersion:\"v0.0.0-master.*\".*}\n?$"))
	})

	It("should run the command with flags --short", func() {
		cmd := NewCmdVersion(factory, o)
		cmd.SetArgs(append(args, "--short"))
		Expect(cmd.Execute()).To(Succeed())
		Expect(o.Short).To(BeTrue())
		Expect(o.Output).To(BeEmpty())
		Expect(buf.String()).To(MatchRegexp("^Version: v0.0.0-master.*\n?$"))
	})

	It("should run the command with flags --output json", func() {
		cmd := NewCmdVersion(factory, o)
		cmd.SetArgs(append(args, "--output", "json"))
		Expect(o.Short).To(BeFalse())
		Expect(cmd.Execute()).To(Succeed())
		Expect(o.Output).To(Equal("json"))
		var anyJSON map[string]interface{}
		Expect(json.Unmarshal([]byte(buf.String()), &anyJSON)).To(Succeed())
		Expect(anyJSON["gitVersion"]).To(HavePrefix("v0.0.0-master"))
	})

	It("should run the command with flags --short --output json", func() {
		cmd := NewCmdVersion(factory, o)
		cmd.SetArgs(append(args, "--short", "--output", "json"))
		Expect(cmd.Execute()).To(Succeed())
		Expect(o.Short).To(BeTrue())
		Expect(o.Output).To(Equal("json"))
		var anyJSON map[string]interface{}
		Expect(json.Unmarshal([]byte(buf.String()), &anyJSON)).To(Succeed())
		Expect(anyJSON["gitVersion"]).To(HavePrefix("v0.0.0-master"))
	})

	It("should validate the options", func() {
		Expect(o.Validate()).ToNot(HaveOccurred())
	})
})
