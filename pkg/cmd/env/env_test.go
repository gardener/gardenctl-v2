/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package env_test

import (
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"

	"github.com/gardener/gardenctl-v2/internal/util"
	utilmocks "github.com/gardener/gardenctl-v2/internal/util/mocks"
	"github.com/gardener/gardenctl-v2/pkg/cmd/env"
)

var _ = Describe("Env Commands", func() {
	var (
		ctrl    *gomock.Controller
		factory *utilmocks.MockFactory
		streams util.IOStreams
		cmd     *cobra.Command
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		factory = utilmocks.NewMockFactory(ctrl)
		streams = util.IOStreams{}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("given a ProviderEnv instance", func() {

		BeforeEach(func() {
			cmd = env.NewCmdProviderEnv(factory, streams)
		})

		It("should have Use, Flags and SubCommands", func() {
			Expect(cmd.Use).To(Equal("provider-env"))
			Expect(cmd.Aliases).To(HaveLen(2))
			Expect(cmd.Aliases).To(Equal([]string{"p-env", "cloud-env"}))
			Expect(cmd.Flag("output")).To(BeNil())
			flag := cmd.Flag("unset")
			Expect(flag).NotTo(BeNil())
			Expect(flag.Shorthand).To(Equal("u"))
			subCmds := cmd.Commands()
			Expect(len(subCmds)).To(Equal(4))
			for _, c := range subCmds {
				Expect(cmd.Flag("unset")).To(BeIdenticalTo(flag))
				s := env.Shell(c.Name())
				Expect(s).To(BeElementOf(env.ValidShells))
			}
		})

	})

	Describe("given a KubectlEnv instance", func() {
		BeforeEach(func() {
			cmd = env.NewCmdKubectlEnv(factory, streams)
		})

		It("should have Use, Flags and SubCommands", func() {
			Expect(cmd.Use).To(Equal("kubectl-env"))
			Expect(cmd.Aliases).To(HaveLen(2))
			Expect(cmd.Aliases).To(Equal([]string{"k-env", "cluster-env"}))
			Expect(cmd.Flag("output")).To(BeNil())
			flag := cmd.Flag("unset")
			Expect(flag).NotTo(BeNil())
			Expect(flag.Shorthand).To(Equal("u"))
			subCmds := cmd.Commands()
			Expect(len(subCmds)).To(Equal(4))
			for _, c := range subCmds {
				Expect(cmd.Flag("unset")).To(BeIdenticalTo(flag))
				s := env.Shell(c.Name())
				Expect(s).To(BeElementOf(env.ValidShells))
			}
		})
	})
})
