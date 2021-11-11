/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package cloudenv_test

import (
	"fmt"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"

	"github.com/gardener/gardenctl-v2/internal/util"
	utilmocks "github.com/gardener/gardenctl-v2/internal/util/mocks"
	"github.com/gardener/gardenctl-v2/pkg/cmd/cloudenv"
)

var _ = Describe("CloudEnv Command", func() {
	Describe("given an instance", func() {
		var (
			ctrl    *gomock.Controller
			factory *utilmocks.MockFactory
			cmd     *cobra.Command
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			factory = utilmocks.NewMockFactory(ctrl)
			cmd = cloudenv.NewCmdCloudEnv(factory, util.IOStreams{})
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		It("should have Use, Aliases, ValidArgs and Flags ", func() {
			Expect(cmd.Use).To(Equal("cloud-env [bash | fish | powershell | zsh]"))
			Expect(cmd.Aliases).To(Equal(cloudenv.Aliases))
			Expect(cmd.ValidArgs).To(Equal(cloudenv.ValidShells))
			flag := cmd.Flag("unset")
			Expect(flag).NotTo(BeNil())
			Expect(flag.Shorthand).To(Equal("u"))
			Expect(cmd.Flag("output")).To(BeNil())
		})
	})

	Describe("matching all positional args checks", func() {
		var (
			use       = "test"
			validArgs = []string{"foo", "bar"}
			max       = 1
			cmd       *cobra.Command
		)
		BeforeEach(func() {
			cmd = &cobra.Command{
				Use:       use,
				ValidArgs: validArgs,
				Args:      cloudenv.MatchAll(cobra.MaximumNArgs(max), cobra.OnlyValidArgs),
			}
		})

		It("should succeed if all check succeed", func() {
			Expect(cmd.ValidateArgs([]string{})).To(Succeed())
			Expect(cmd.ValidateArgs([]string{"foo"})).To(Succeed())
			Expect(cmd.ValidateArgs([]string{"bar"})).To(Succeed())
		})

		It("should fail if the first check fails", func() {
			args := []string{"foo", "bar"}
			Expect(cmd.ValidateArgs(args)).To(MatchError(fmt.Sprintf("accepts at most %d arg(s), received %d", max, len(args))))
		})

		It("should fail if the first check fails", func() {
			args := []string{"baz"}
			Expect(cmd.ValidateArgs(args)).To(MatchError(fmt.Sprintf("invalid argument %q for %q", args[0], use)))
		})
	})
})
