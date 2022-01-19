/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
SPDX-License-Identifier: Apache-2.0
*/

package config_test

import (
	"github.com/golang/mock/gomock"

	targetmocks "github.com/gardener/gardenctl-v2/pkg/target/mocks"

	"github.com/gardener/gardenctl-v2/internal/util"
	utilmocks "github.com/gardener/gardenctl-v2/internal/util/mocks"
	cmdconfig "github.com/gardener/gardenctl-v2/pkg/cmd/config"
	"github.com/gardener/gardenctl-v2/pkg/config"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Command", func() {
	It("should print configuration", func() {
		gardenIdentity1 := "fooGarden"
		gardenIdentity2 := "barGarden"
		kubeconfig := "not/a/file"
		matchPatterns := []string{
			"^shoot--(?P<project>.+)--(?P<shoot>.+)$",
			"^namespace:(?P<namespace>[^/]+)$",
		}
		cfg := &config.Config{
			Gardens: []config.Garden{
				{
					Identity:   gardenIdentity1,
					Kubeconfig: kubeconfig,
				},
				{
					Identity:   gardenIdentity2,
					Kubeconfig: kubeconfig,
					Patterns:   matchPatterns,
				}},
		}

		ctrl := gomock.NewController(GinkgoT())
		factory := utilmocks.NewMockFactory(ctrl)
		manager := targetmocks.NewMockManager(ctrl)
		manager.EXPECT().Configuration().Return(cfg)
		factory.EXPECT().Manager().Return(manager, nil)

		streams, _, out, _ := util.NewTestIOStreams()
		cmd := cmdconfig.NewCmdConfigView(factory, streams)

		Expect(cmd.RunE(cmd, nil)).To(Succeed())
		Expect(out.String()).To(ContainSubstring("gardens"))
		Expect(out.String()).To(ContainSubstring(gardenIdentity1))
		Expect(out.String()).To(ContainSubstring(gardenIdentity2))
		Expect(out.String()).To(ContainSubstring("matchPatterns"))
		Expect(out.String()).To(ContainSubstring(matchPatterns[1]))
	})
})

var _ = Describe("ViewOptions", func() {
	It("should validate", func() {
		streams, _, _, _ := util.NewTestIOStreams()
		o := cmdconfig.NewViewOptions(streams)
		Expect(o.Validate()).To(Succeed())
	})
})
