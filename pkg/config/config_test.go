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
		clusterIdentity1 = "testgarden"
		shortName1       = "test.garden"
		clusterIdentity2 = "foogarden"
		project          = "fooproject"
		shoot            = "fooshoot"
		namespace        = "foonamespace"
		cfg              config.Config
	)

	BeforeEach(func() {
		cfg = &config.ConfigImpl{
			Gardens: []config.Garden{{
				Identity: clusterIdentity1,
				Short:    shortName1,
				MatchPatterns: []string{
					fmt.Sprintf("^(%s/)?shoot--(?P<project>.+)--(?P<shoot>.+)$", shortName1),
					"^namespace:(?P<namespace>[^/]+)$",
				},
			},
				{
					Identity: clusterIdentity2,
					MatchPatterns: []string{
						fmt.Sprintf("^(%s)?shoot--(?P<project>.+)--(?P<shoot>.+)$", clusterIdentity2),
						"^namespace:(?P<namespace>[^/]+)$",
					},
				}},
		}
	})

	It("should find garden by identity", func() {
		garden, err := cfg.Garden(clusterIdentity1)
		Expect(err).NotTo(HaveOccurred())
		Expect(garden.Identity).Should(Equal(clusterIdentity1))

	})

	It("should find garden by name", func() {
		garden, err := cfg.Garden(shortName1)
		Expect(err).NotTo(HaveOccurred())
		Expect(garden.Short).Should(Equal(shortName1))

	})

	It("TargetName() should return short name if defined", func() {
		garden, err := cfg.Garden(clusterIdentity1)
		Expect(err).NotTo(HaveOccurred())
		Expect(garden.ShortOrIdentity()).Should(Equal(shortName1))

		garden, err = cfg.Garden(clusterIdentity2)
		Expect(err).NotTo(HaveOccurred())
		Expect(garden.ShortOrIdentity()).Should(Equal(clusterIdentity2))

	})

	It("should throw an error if garden not found", func() {
		_, err := cfg.Garden("foobar")
		Expect(err).To(HaveOccurred())
	})

	It("should return a PatternMatch for a given value", func() {
		tm, err := cfg.MatchPattern(fmt.Sprintf("%s/shoot--%s--%s", shortName1, project, shoot), shortName1)
		Expect(err).NotTo(HaveOccurred())
		Expect(tm.Garden).Should(Equal(clusterIdentity1))
		Expect(tm.Project).Should(Equal(project))
		Expect(tm.Shoot).Should(Equal(shoot))
	})

	It("should prefer PatternMatch for current garden", func() {
		tm, err := cfg.MatchPattern(fmt.Sprintf("namespace:%s", namespace), clusterIdentity1)
		Expect(err).NotTo(HaveOccurred())
		Expect(tm.Garden).Should(Equal(clusterIdentity1))
		Expect(tm.Namespace).Should(Equal(namespace))
	})
})
