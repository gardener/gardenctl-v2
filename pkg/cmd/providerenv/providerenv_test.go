/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package providerenv_test

import (
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"

	"github.com/gardener/gardenctl-v2/internal/util"
	utilmocks "github.com/gardener/gardenctl-v2/internal/util/mocks"
	"github.com/gardener/gardenctl-v2/pkg/cmd/providerenv"
	"github.com/gardener/gardenctl-v2/pkg/env"
	"github.com/gardener/gardenctl-v2/pkg/target"
	targetmocks "github.com/gardener/gardenctl-v2/pkg/target/mocks"
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

		manager := targetmocks.NewMockManager(ctrl)
		factory.EXPECT().Manager().Return(manager, nil).AnyTimes()

		targetFlags := target.NewTargetFlags("", "", "", "", false)
		factory.EXPECT().TargetFlags().Return(targetFlags).AnyTimes()

		streams = util.IOStreams{}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("given a ProviderEnv instance", func() {
		BeforeEach(func() {
			cmd = providerenv.NewCmdProviderEnv(factory, streams)
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
				Expect(c.Flag("unset")).To(BeIdenticalTo(flag))
				s := env.Shell(c.Name())
				Expect(s).To(BeElementOf(env.ValidShells()))
			}
		})
	})
})
