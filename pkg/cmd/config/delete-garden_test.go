/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
SPDX-License-Identifier: Apache-2.0
*/

package config_test

import (
	"os"
	"path/filepath"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	targetmocks "github.com/gardener/gardenctl-v2/pkg/target/mocks"

	utilmocks "github.com/gardener/gardenctl-v2/internal/util/mocks"

	"github.com/gardener/gardenctl-v2/internal/util"
	cmdconfig "github.com/gardener/gardenctl-v2/pkg/cmd/config"
	"github.com/gardener/gardenctl-v2/pkg/config"
)

var _ = Describe("Command", func() {
	var (
		cfg             *config.Config
		gardenIdentity1 string
		gardenIdentity2 string
		factory         *utilmocks.MockFactory
		ctrl            *gomock.Controller
		manager         *targetmocks.MockManager
	)

	BeforeEach(func() {
		gardenIdentity1 = "fooGarden"
		gardenIdentity2 = "barGarden"
		cfg = &config.Config{
			Gardens: []config.Garden{
				{
					Identity: gardenIdentity1,
				},
				{
					Identity: gardenIdentity2,
				}},
		}

		dir, _ := os.MkdirTemp(os.TempDir(), "gctlv2-*")
		configFile := filepath.Join(dir, "gardenctl-testconfig"+".yaml")

		ctrl = gomock.NewController(GinkgoT())
		factory = utilmocks.NewMockFactory(ctrl)
		manager = targetmocks.NewMockManager(ctrl)
		manager.EXPECT().Configuration().Return(cfg)
		factory.EXPECT().Manager().Return(manager, nil)
		factory.EXPECT().ConfigFile().Return(configFile)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("should delete garden from configuration", func() {
		// setup command
		streams, _, _, _ := util.NewTestIOStreams()
		_, err := cfg.Garden(gardenIdentity1)
		Expect(err).ToNot(HaveOccurred())
		Expect(len(cfg.AllGardens())).To(Equal(2))

		cmd := cmdconfig.NewCmdConfigDeleteGarden(factory, streams)
		Expect(cmd.RunE(cmd, []string{gardenIdentity1})).To(Succeed())

		_, err = cfg.Garden(gardenIdentity1)
		Expect(err).To(HaveOccurred())
		Expect(len(cfg.AllGardens())).To(Equal(1))
	})
})

var _ = Describe("DeleteGardenOptions", func() {
	It("should validate", func() {
		streams, _, _, _ := util.NewTestIOStreams()
		o := cmdconfig.NewDeleteGardenOptions(streams)
		o.Identity = "foo"
		Expect(o.Validate()).To(Succeed())
	})

	It("should reject if no identity is set", func() {
		streams, _, _, _ := util.NewTestIOStreams()
		o := cmdconfig.NewDeleteGardenOptions(streams)

		Expect(o.Validate()).NotTo(Succeed())
	})
})
