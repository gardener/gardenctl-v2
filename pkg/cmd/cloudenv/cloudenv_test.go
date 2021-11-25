/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package cloudenv_test

import (
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
				s := cloudenv.Shell(c.Name())
				Expect(s).To(BeElementOf(cloudenv.ValidShells))
			}
		})
	})
})
