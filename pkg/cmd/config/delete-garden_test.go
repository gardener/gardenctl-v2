/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
SPDX-License-Identifier: Apache-2.0
*/

package config_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	internalfake "github.com/gardener/gardenctl-v2/internal/fake"
	"github.com/gardener/gardenctl-v2/pkg/config"
	"github.com/gardener/gardenctl-v2/pkg/target"

	"github.com/gardener/gardenctl-v2/internal/util"
	cmdconfig "github.com/gardener/gardenctl-v2/pkg/cmd/config"
)

var _ = Describe("Command", func() {
	var (
		cfg             *config.Config
		gardenIdentity1 string
		gardenIdentity2 string
		targetProvider  target.TargetProvider
		factory         util.Factory
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

		// setup fakes
		targetProvider = internalfake.NewFakeTargetProvider(target.NewTarget("", "", "", ""))
		factory = internalfake.NewFakeFactory(cfg, nil, nil, nil, targetProvider)
	})

	It("should delete garden from configuration", func() {
		// setup command
		streams, _, _, _ := util.NewTestIOStreams()
		o := cmdconfig.NewDeleteGardenOptions(streams)

		_, err := cfg.Garden(gardenIdentity1)
		Expect(err).ToNot(HaveOccurred())
		Expect(len(cfg.AllGardens())).To(Equal(2))

		cmd := cmdconfig.NewCmdConfigDeleteGarden(factory, o)
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
