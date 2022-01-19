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

	"github.com/gardener/gardenctl-v2/internal/util"
	utilmocks "github.com/gardener/gardenctl-v2/internal/util/mocks"
	cmdconfig "github.com/gardener/gardenctl-v2/pkg/cmd/config"
	"github.com/gardener/gardenctl-v2/pkg/config"
	targetmocks "github.com/gardener/gardenctl-v2/pkg/target/mocks"
)

var _ = Describe("Command", func() {
	var (
		cfg             *config.Config
		gardenIdentity1 string
		gardenIdentity2 string
		gardenContext1  string
		factory         *utilmocks.MockFactory
		ctrl            *gomock.Controller
		manager         *targetmocks.MockManager
	)

	BeforeEach(func() {
		gardenIdentity1 = "fooGarden"
		gardenIdentity2 = "barGarden"
		gardenContext1 = "my-context"
		cfg = &config.Config{
			Gardens: []config.Garden{{
				Identity: gardenIdentity1,
				Context:  gardenContext1,
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

	It("should add new garden to configuration", func() {
		// setup command
		streams, _, _, _ := util.NewTestIOStreams()

		Expect(len(cfg.AllGardens())).To(Equal(1))

		cmd := cmdconfig.NewCmdConfigSetGarden(factory, streams)
		Expect(cmd.RunE(cmd, []string{gardenIdentity2})).To(Succeed())

		_, err := cfg.Garden(gardenIdentity2)
		Expect(err).ToNot(HaveOccurred())
		Expect(len(cfg.AllGardens())).To(Equal(2))
	})

	It("should modify an existing garden configuration", func() {
		// setup command
		streams, _, _, _ := util.NewTestIOStreams()

		Expect(len(cfg.AllGardens())).To(Equal(1))

		kubeconfig := "path/to/kubeconfig"
		cmd := cmdconfig.NewCmdConfigSetGarden(factory, streams)
		Expect(cmd.Flags().Set("kubeconfig", kubeconfig)).To(Succeed())
		Expect(cmd.RunE(cmd, []string{gardenIdentity1})).To(Succeed())

		g, err := cfg.Garden(gardenIdentity1)
		Expect(err).ToNot(HaveOccurred())
		Expect(len(cfg.AllGardens())).To(Equal(1))
		Expect(g.Identity).To(Equal(gardenIdentity1))
		Expect(g.Kubeconfig).To(Equal(kubeconfig))
		// Check that existing value does not get overwritten
		Expect(g.Context).To(Equal(gardenContext1))
	})
})

var _ = Describe("SetGardenOptions", func() {
	It("should validate", func() {
		streams, _, _, _ := util.NewTestIOStreams()
		o := cmdconfig.NewSetGardenOptions(streams)
		o.Identity = "foo"
		Expect(o.Validate()).To(Succeed())
	})

	It("should reject if no identity is set", func() {
		streams, _, _, _ := util.NewTestIOStreams()
		o := cmdconfig.NewSetGardenOptions(streams)

		Expect(o.Validate()).NotTo(Succeed())
	})
})
