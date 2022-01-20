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
		clusterIdentity2 = "foogarden"
		project          = "fooproject"
		shoot            = "fooshoot"
		namespace        = "foonamespace"
		cfg              *config.Config
	)

	BeforeEach(func() {
		cfg = &config.Config{
			Gardens: []config.Garden{
				{
					Name: clusterIdentity1,
					Patterns: []string{
						fmt.Sprintf("^(%s/)?shoot--(?P<project>.+)--(?P<shoot>.+)$", clusterIdentity1),
						"^namespace:(?P<namespace>[^/]+)$",
					},
				},
				{
					Name: clusterIdentity2,
					Patterns: []string{
						fmt.Sprintf("^(%s)?shoot--(?P<project>.+)--(?P<shoot>.+)$", clusterIdentity2),
						"^namespace:(?P<namespace>[^/]+)$",
					},
				}},
		}
	})

	It("should find garden by identity", func() {
		garden, err := cfg.Garden(clusterIdentity1)
		Expect(err).NotTo(HaveOccurred())
		Expect(garden.Name).Should(Equal(clusterIdentity1))

	})

	It("should throw an error if garden not found", func() {
		_, err := cfg.Garden("foobar")
		Expect(err).To(HaveOccurred())
	})

	It("should return a PatternMatch for a given value", func() {
		tm, err := cfg.MatchPattern(fmt.Sprintf("%s/shoot--%s--%s", clusterIdentity1, project, shoot), clusterIdentity1)
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
