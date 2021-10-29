/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package config_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardenctl-v2/pkg/config"
)

var _ = Describe("Config", func() {
	var (
		gardenName1  = "testgarden"
		gardenAlias1 = []string{"test.garden"}
		gardenName2  = "foogarden"
		gardenAlias2 = []string{"foo", "bar"}
		project      = "fooproject"
		shoot        = "fooshoot"
		namespace    = "foonamespace"
		cfg          *config.Config
	)

	BeforeEach(func() {
		cfg = &config.Config{
			Gardens: []config.Garden{{
				Name:    gardenName1,
				Aliases: gardenAlias1,
			},
				{
					Name:    gardenName2,
					Aliases: gardenAlias2,
				}},
			MatchPatterns: []string{
				"^((?P<garden>[^/]+)/)?shoot--(?P<project>.+)--(?P<shoot>.+)$",
				"^namespace:(?P<namespace>[^/]+)$",
			},
		}
	})

	It("should find garden by name", func() {
		gardenName, err := cfg.GardenName(gardenName1)
		Expect(err).NotTo(HaveOccurred())
		Expect(gardenName).Should(Equal(gardenName1))

	})

	It("should find garden by alias", func() {
		gardenName, err := cfg.GardenName(gardenAlias1[0])
		Expect(err).NotTo(HaveOccurred())
		Expect(gardenName).Should(Equal(gardenName1))

		gardenName, err = cfg.GardenName(gardenAlias2[1])
		Expect(err).NotTo(HaveOccurred())
		Expect(gardenName).Should(Equal(gardenName2))

	})

	It("should throw an error if garden not found", func() {
		_, err := cfg.GardenName("foobar")
		Expect(err).To(HaveOccurred())
	})

	It("should return a PatternMatch for a given value", func() {
		tm, err := cfg.MatchPattern(fmt.Sprintf("%s/shoot--%s--%s", gardenAlias2[1], project, shoot))
		Expect(err).NotTo(HaveOccurred())
		Expect(tm.Garden).Should(Equal(gardenAlias2[1]))
		Expect(tm.Project).Should(Equal(project))
		Expect(tm.Shoot).Should(Equal(shoot))

		tm, err = cfg.MatchPattern(fmt.Sprintf("namespace:%s", namespace))
		Expect(err).NotTo(HaveOccurred())
		Expect(tm.Namespace).Should(Equal(namespace))
	})
})
