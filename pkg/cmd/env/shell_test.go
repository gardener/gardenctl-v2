/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package env_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardenctl-v2/pkg/cmd/env"
)

var _ = Describe("Env Commands - Shell", func() {
	Describe("validation", func() {
		It("should succeed for all valid shells", func() {
			Expect(env.Shell("bash").Validate()).To(Succeed())
			Expect(env.Shell("zsh").Validate()).To(Succeed())
			Expect(env.Shell("fish").Validate()).To(Succeed())
			Expect(env.Shell("powershell").Validate()).To(Succeed())
		})

		It("should fail for a currently unsupported shell", func() {
			Expect(env.Shell("cmd").Validate()).To(MatchError(fmt.Sprintf("invalid shell given, must be one of %v", env.ValidShells)))
		})
	})

	Describe("getting the prompt", func() {
		It("should return the typical prompt for the given shell and goos", func() {
			Expect(env.Shell("bash").Prompt("linux")).To(Equal("$ "))
			Expect(env.Shell("powershell").Prompt("darwin")).To(Equal("PS /> "))
			Expect(env.Shell("powershell").Prompt("windows")).To(Equal("PS C:\\> "))
		})
	})

	Describe("getting the eval command", func() {
		It("should return the script to eval a command", func() {
			cmd := "test"
			Expect(env.Shell("bash").EvalCommand(cmd)).To(Equal(fmt.Sprintf("eval \"$(%s)\"", cmd)))
			Expect(env.Shell("fish").EvalCommand(cmd)).To(Equal(fmt.Sprintf("eval (%s)", cmd)))
			Expect(env.Shell("powershell").EvalCommand(cmd)).To(Equal(fmt.Sprintf("& %s | Invoke-Expression", cmd)))
		})
	})
})
