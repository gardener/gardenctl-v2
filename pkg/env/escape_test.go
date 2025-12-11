/*
SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package env_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardenctl-v2/pkg/env"
)

var _ = Describe("Shell Escaping Functions", func() {
	Describe("ShellEscapePOSIX", func() {
		It("returns empty string for no args", func() {
			Expect(env.ShellEscapePOSIX()).To(Equal(""))
		})

		DescribeTable("single string argument",
			func(input string, expected string) {
				Expect(env.ShellEscapePOSIX(input)).To(Equal(expected))
			},
			Entry("empty string", "", "''"),
			Entry("simple string", "foo", "'foo'"),
			Entry("string with space", "foo bar", "'foo bar'"),
			Entry("single apostrophe", "'", `''"'"''`),
			Entry("apostrophe", "O'Reilly", "'O'\"'\"'Reilly'"),
			Entry("double quote", `foo"bar`, "'foo\"bar'"),
			Entry("backslash", `foo\bar`, "'foo\\bar'"),
			Entry("dollar", "foo$bar", "'foo$bar'"),
			Entry("unicode", "föö", "'föö'"),
		)

		It("handles integer", func() {
			Expect(env.ShellEscapePOSIX(123)).To(Equal("'123'"))
		})

		It("handles multiple arguments", func() {
			Expect(env.ShellEscapePOSIX("foo", "bar", 123)).To(Equal("'foo' 'bar' '123'"))
		})
	})

	Describe("ShellEscapeFish", func() {
		It("returns empty string for no args", func() {
			Expect(env.ShellEscapeFish()).To(Equal(""))
		})

		DescribeTable("single string argument",
			func(input string, expected string) {
				Expect(env.ShellEscapeFish(input)).To(Equal(expected))
			},
			Entry("empty string", "", "''"),
			Entry("simple string", "foo", "'foo'"),
			Entry("string with space", "foo bar", "'foo bar'"),
			Entry("single apostrophe", "'", `''\'''`),
			Entry("apostrophe", "O'Reilly", "'O'\\''Reilly'"),
			Entry("double quote", `foo"bar`, "'foo\"bar'"),
			Entry("backslash", `foo\bar`, `'foo\\bar'`),
			Entry("dollar", "foo$bar", "'foo$bar'"),
			Entry("unicode", "föö", "'föö'"),
		)

		It("handles integer", func() {
			Expect(env.ShellEscapeFish(123)).To(Equal("'123'"))
		})

		It("handles multiple arguments", func() {
			Expect(env.ShellEscapeFish("foo", "bar", 123)).To(Equal("'foo' 'bar' '123'"))
		})
	})

	Describe("ShellEscapePowerShell", func() {
		It("returns empty string for no args", func() {
			Expect(env.ShellEscapePowerShell()).To(Equal(""))
		})

		DescribeTable("single string argument",
			func(input string, expected string) {
				Expect(env.ShellEscapePowerShell(input)).To(Equal(expected))
			},
			Entry("empty string", "", "''"),
			Entry("simple string", "foo", "'foo'"),
			Entry("string with space", "foo bar", "'foo bar'"),
			Entry("single apostrophe", "'", `''''`),
			Entry("apostrophe", "O'Reilly", "'O''Reilly'"),
			Entry("left single quotation mark", "O\u2018Reilly", "'O\u2018\u2018Reilly'"),
			Entry("right single quotation mark", "O\u2019Reilly", "'O\u2019\u2019Reilly'"),
			Entry("single low-9 quotation mark", "O\u201AReilly", "'O\u201A\u201AReilly'"),
			Entry("single high-reversed-9 quotation mark", "O\u201BReilly", "'O\u201B\u201BReilly'"),
			Entry("double quote", `foo"bar`, "'foo\"bar'"),
			Entry("backslash", `foo\bar`, "'foo\\bar'"),
			Entry("dollar", "foo$bar", "'foo$bar'"),
			Entry("unicode", "föö", "'föö'"),
		)

		It("handles integer", func() {
			Expect(env.ShellEscapePowerShell(123)).To(Equal("'123'"))
		})

		It("handles multiple arguments", func() {
			Expect(env.ShellEscapePowerShell("foo", "bar", 123)).To(Equal("'foo' 'bar' '123'"))
		})
	})
})
