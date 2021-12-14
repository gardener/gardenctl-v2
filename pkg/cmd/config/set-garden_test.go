/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
SPDX-License-Identifier: Apache-2.0
*/

package config_test

import (
	"k8s.io/component-base/cli/flag"

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
		cfg             config.Config
		gardenIdentity1 string
		gardenIdentity2 string
		gardenContext1  string
		targetProvider  target.TargetProvider
		factory         util.Factory
	)

	BeforeEach(func() {
		gardenIdentity1 = "fooGarden"
		gardenIdentity2 = "barGarden"
		gardenContext1 = "my-context"
		cfg = &config.ConfigImpl{
			Gardens: []config.Garden{{
				Identity: gardenIdentity1,
				Context:  gardenContext1,
			}},
		}

		// setup fakes
		targetProvider = internalfake.NewFakeTargetProvider(target.NewTarget("", "", "", ""))
		factory = internalfake.NewFakeFactory(cfg, nil, nil, nil, targetProvider)
	})

	It("should add new garden to configuration", func() {
		// setup command
		streams, _, _, _ := util.NewTestIOStreams()
		o := cmdconfig.NewSetGardenOptions(streams)

		Expect(len(cfg.AllGardens())).To(Equal(1))

		cmd := cmdconfig.NewCmdConfigSetGarden(factory, o)
		Expect(cmd.RunE(cmd, []string{gardenIdentity2})).To(Succeed())

		_, err := cfg.Garden(gardenIdentity2)
		Expect(err).ToNot(HaveOccurred())
		Expect(len(cfg.AllGardens())).To(Equal(2))
	})

	It("should modify an existing garden configuration", func() {
		// setup command
		streams, _, _, _ := util.NewTestIOStreams()
		o := cmdconfig.NewSetGardenOptions(streams)

		Expect(len(cfg.AllGardens())).To(Equal(1))

		shortName := flag.StringFlag{}
		err := shortName.Set("custom")
		Expect(err).ToNot(HaveOccurred())

		o.Short = shortName
		cmd := cmdconfig.NewCmdConfigSetGarden(factory, o)
		Expect(cmd.RunE(cmd, []string{gardenIdentity1})).To(Succeed())

		g, err := cfg.Garden(gardenIdentity1)
		Expect(err).ToNot(HaveOccurred())
		Expect(len(cfg.AllGardens())).To(Equal(1))
		Expect(g.Identity).To(Equal(gardenIdentity1))
		Expect(g.Short).To(Equal(shortName.Value()))
		// Check that existing value does not get overwritten
		Expect(g.Context).To(Equal(gardenContext1))
	})
})

var _ = Describe("SetGardenOptions", func() {
	It("should validate", func() {
		streams, _, _, _ := util.NewTestIOStreams()
		o := cmdconfig.NewSetGardenOptions(streams)
		o.Identity = "foo"
		err := o.Validate()
		Expect(err).ToNot(HaveOccurred())
	})

	It("should reject if no identity is set", func() {
		streams, _, _, _ := util.NewTestIOStreams()
		o := cmdconfig.NewSetGardenOptions(streams)

		Expect(o.Validate()).NotTo(Succeed())
	})
})
