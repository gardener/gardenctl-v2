/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package config_test

import (
	"fmt"
	"github.com/gardener/gardenctl-v2/pkg/config"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {
	var (
		gardenName_1  = "testgarden"
		gardenAlias_1 = []string{"test.garden"}
		gardenName_2  = "foogarden"
		gardenAlias_2 = []string{"foo", "bar"}
		project       = "fooproject"
		shoot         = "fooshoot"
		namespace     = "foonamespace"
		cfg           *config.Config
	)

	BeforeEach(func() {
		cfg = &config.Config{
			Gardens: []config.Garden{{
				Name:    gardenName_1,
				Aliases: gardenAlias_1,
			},
				{
					Name:    gardenName_2,
					Aliases: gardenAlias_2,
				}},
			MatchPatterns: []string{
				"^((?P<garden>[^/]+)/)?shoot--(?P<project>.+)--(?P<shoot>.+)$",
				"^namespace:(?P<namespace>[^/]+)$",
			},
		}
	})

	It("should find garden by name", func() {
		gardenName := cfg.FindGarden(gardenName_1)
		Expect(gardenName).Should(Equal(gardenName_1))

	})

	It("should find garden by alias", func() {
		gardenName := cfg.FindGarden(gardenAlias_1[0])
		Expect(gardenName).Should(Equal(gardenName_1))

		gardenName = cfg.FindGarden(gardenAlias_2[1])
		Expect(gardenName).Should(Equal(gardenName_2))

	})

	It("should return a TargetMatch for a given value", func() {
		tm, err := cfg.MatchPattern(fmt.Sprintf("%s/shoot--%s--%s", gardenAlias_2[1], project, shoot))
		Expect(err).NotTo(HaveOccurred())
		Expect(tm.Garden).Should(Equal(gardenAlias_2[1]))
		Expect(tm.Project).Should(Equal(project))
		Expect(tm.Shoot).Should(Equal(shoot))

		tm, err = cfg.MatchPattern(fmt.Sprintf("namespace:%s", namespace))
		Expect(err).NotTo(HaveOccurred())
		Expect(tm.Namespace).Should(Equal(namespace))
	})
})
