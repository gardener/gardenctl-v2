/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
SPDX-License-Identifier: Apache-2.0
*/

package config_test

import (
	internalfake "github.com/gardener/gardenctl-v2/internal/fake"
	"github.com/gardener/gardenctl-v2/internal/util"
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

		// setup command
		factory := internalfake.NewFakeFactory(cfg, nil, nil, nil, nil)

		streams, _, out, _ := util.NewTestIOStreams()
		o := cmdconfig.NewViewOptions(streams)
		cmd := cmdconfig.NewCmdConfigView(factory, o)

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
