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
		gardenName1      = "test.garden"
		clusterIdentity2 = "foogarden"
		project          = "fooproject"
		shoot            = "fooshoot"
		namespace        = "foonamespace"
		cfg              *config.Config
	)

	BeforeEach(func() {
		cfg = &config.Config{
			Gardens: []config.Garden{{
				ClusterIdentity: clusterIdentity1,
				Name:            gardenName1,
			},
				{
					ClusterIdentity: clusterIdentity2,
				}},
			MatchPatterns: []string{
				"^((?P<garden>[^/]+)/)?shoot--(?P<project>.+)--(?P<shoot>.+)$",
				"^namespace:(?P<namespace>[^/]+)$",
			},
		}
	})

	It("should find garden by cluster identity", func() {
		garden, err := cfg.Garden(clusterIdentity1)
		Expect(err).NotTo(HaveOccurred())
		Expect(garden.ClusterIdentity).Should(Equal(clusterIdentity1))

	})

	It("should find garden by name", func() {
		garden, err := cfg.Garden(gardenName1)
		Expect(err).NotTo(HaveOccurred())
		Expect(garden.Name).Should(Equal(gardenName1))

	})

	It("TargetName() should return name if defined", func() {
		garden, err := cfg.Garden(clusterIdentity1)
		Expect(err).NotTo(HaveOccurred())
		Expect(garden.TargetName()).Should(Equal(gardenName1))

		garden, err = cfg.Garden(clusterIdentity2)
		Expect(err).NotTo(HaveOccurred())
		Expect(garden.TargetName()).Should(Equal(clusterIdentity2))

	})

	It("should throw an error if garden not found", func() {
		_, err := cfg.Garden("foobar")
		Expect(err).To(HaveOccurred())
	})

	It("should return a PatternMatch for a given value", func() {
		tm, err := cfg.MatchPattern(fmt.Sprintf("%s/shoot--%s--%s", gardenName1, project, shoot))
		Expect(err).NotTo(HaveOccurred())
		Expect(tm.Garden).Should(Equal(clusterIdentity1))
		Expect(tm.Project).Should(Equal(project))
		Expect(tm.Shoot).Should(Equal(shoot))

		tm, err = cfg.MatchPattern(fmt.Sprintf("namespace:%s", namespace))
		Expect(err).NotTo(HaveOccurred())
		Expect(tm.Namespace).Should(Equal(namespace))
	})
})
