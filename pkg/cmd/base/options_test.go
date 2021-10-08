/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package base_test

import (
	"fmt"
	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Options", func() {
	It("should validate", func() {
		streams, _, _, _ := util.NewTestIOStreams()
		o := base.NewOptions(streams)
		err := o.Validate()
		Expect(err).ToNot(HaveOccurred())

	})

	It("should validate valid output formats", func() {
		streams, _, _, _ := util.NewTestIOStreams()
		o := base.NewOptions(streams)
		o.Output = "yaml"
		err := o.Validate()
		Expect(err).ToNot(HaveOccurred())
		o.Output = "json"
		err = o.Validate()
		Expect(err).ToNot(HaveOccurred())
	})

	It("should not validate invalid output format", func() {
		streams, _, _, _ := util.NewTestIOStreams()
		o := base.NewOptions(streams)
		o.Output = "foo"
		err := o.Validate()
		Expect(err).To(MatchError(ContainSubstring("--output must be either 'yaml' or 'json")))
	})

	type fooType struct {
		Foo string
		Bar string
	}

	foo := fooType{
		Foo: "foo",
		Bar: "bar",
	}

	It("should print with no output set", func() {
		streams, _, out, _ := util.NewTestIOStreams()
		o := base.NewOptions(streams)
		o.PrintObject(foo)
		Expect(out.String()).To(ContainSubstring(foo.Foo))
		Expect(out.String()).To(ContainSubstring(foo.Bar))
	})

	It("should print yaml with output yaml", func() {
		streams, _, out, _ := util.NewTestIOStreams()
		o := base.NewOptions(streams)
		o.Output = "yaml"
		o.PrintObject(foo)
		Expect(out.String()).To(Equal(fmt.Sprintf("foo: %s\nbar: %s\n\n", foo.Foo, foo.Bar)))
	})

	It("should print json with output json", func() {
		streams, _, out, _ := util.NewTestIOStreams()
		o := base.NewOptions(streams)
		o.Output = "json"
		o.PrintObject(foo)
		Expect(out.String()).To(Equal(fmt.Sprintf("{\n  \"Foo\": \"%s\",\n  \"Bar\": \"%s\"\n}\n", foo.Foo, foo.Bar)))
	})
})
