/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
SPDX-License-Identifier: Apache-2.0
*/

package config_test

import (
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

var _ = Describe("Config Commands", func() {
	const (
		gardenIdentity1 = "fooGarden"
		gardenIdentity2 = "barGarden"
		gardenIdentity3 = "bazGarden"
		gardenContext1  = "my-context"
		kubeconfig      = "not/a/file"
	)

	var (
		cfg      *config.Config
		ctrl     *gomock.Controller
		factory  *utilmocks.MockFactory
		manager  *targetmocks.MockManager
		streams  util.IOStreams
		out      *util.SafeBytesBuffer
		patterns []string
	)

	BeforeEach(func() {
		patterns = []string{
			"^shoot--(?P<project>.+)--(?P<shoot>.+)$",
			"^namespace:(?P<namespace>[^/]+)$",
		}
		cfg = &config.Config{
			Filename: filepath.Join(gardenHomeDir, "gardenctl-testconfig.yaml"),
			Gardens: []config.Garden{
				{
					Name:       gardenIdentity1,
					Kubeconfig: kubeconfig,
					Context:    gardenContext1,
				},
				{
					Name:       gardenIdentity2,
					Kubeconfig: kubeconfig,
					Patterns:   patterns,
				}},
		}

		streams, _, out, _ = util.NewTestIOStreams()
		ctrl = gomock.NewController(GinkgoT())
		factory = utilmocks.NewMockFactory(ctrl)
		manager = targetmocks.NewMockManager(ctrl)
		manager.EXPECT().Configuration().Return(cfg)
		factory.EXPECT().Manager().Return(manager, nil)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("should print configuration", func() {
		cmd := cmdconfig.NewCmdConfigView(factory, streams)
		Expect(cmd.RunE(cmd, nil)).To(Succeed())

		Expect(out.String()).To(ContainSubstring("gardens"))
		Expect(out.String()).To(ContainSubstring(gardenIdentity1))
		Expect(out.String()).To(ContainSubstring(gardenIdentity2))
		Expect(out.String()).To(ContainSubstring("matchPatterns"))
		Expect(out.String()).To(ContainSubstring(patterns[1]))
	})

	It("should delete garden from configuration", func() {
		_, err := cfg.Garden(gardenIdentity1)
		Expect(err).ToNot(HaveOccurred())
		Expect(len(cfg.Gardens)).To(Equal(2))

		cmd := cmdconfig.NewCmdConfigDeleteGarden(factory, streams)
		Expect(cmd.RunE(cmd, []string{gardenIdentity1})).To(Succeed())

		_, err = cfg.Garden(gardenIdentity1)
		Expect(err).To(HaveOccurred())
		Expect(len(cfg.Gardens)).To(Equal(1))
	})

	It("should add new garden to configuration", func() {
		Expect(len(cfg.Gardens)).To(Equal(2))

		cmd := cmdconfig.NewCmdConfigSetGarden(factory, streams)
		Expect(cmd.RunE(cmd, []string{gardenIdentity3})).To(Succeed())

		_, err := cfg.Garden(gardenIdentity3)
		Expect(err).ToNot(HaveOccurred())
		Expect(len(cfg.Gardens)).To(Equal(3))
	})

	It("should modify an existing garden configuration", func() {
		Expect(len(cfg.Gardens)).To(Equal(2))

		cmd := cmdconfig.NewCmdConfigSetGarden(factory, streams)
		pathToKubeconfig := "path/to/kubeconfig"
		Expect(cmd.Flags().Set("kubeconfig", pathToKubeconfig)).To(Succeed())
		Expect(cmd.RunE(cmd, []string{gardenIdentity1})).To(Succeed())

		g, err := cfg.Garden(gardenIdentity1)
		Expect(err).ToNot(HaveOccurred())
		Expect(len(cfg.Gardens)).To(Equal(2))
		Expect(g.Name).To(Equal(gardenIdentity1))
		Expect(g.Kubeconfig).To(Equal(pathToKubeconfig))
		// Check that existing value does not get overwritten
		Expect(g.Context).To(Equal(gardenContext1))
	})
})
