/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package target_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardenctl-v2/pkg/target"
)

var _ = Describe("Target", func() {
	Describe("having an object", func() {
		It("should keep data", func() {
			t := target.NewTarget("a", "b", "c", "d")

			Expect(t.GardenName()).To(Equal("a"))
			Expect(t.ProjectName()).To(Equal("b"))
			Expect(t.SeedName()).To(Equal("c"))
			Expect(t.ShootName()).To(Equal("d"))
		})

		It("should validate", func() {
			// valid
			Expect(target.NewTarget("a", "b", "", "d").Validate()).To(Succeed())
			Expect(target.NewTarget("a", "", "c", "d").Validate()).To(Succeed())
			Expect(target.NewTarget("a", "", "", "d").Validate()).To(Succeed())

			// invalid because both project and seed are defined
			Expect(target.NewTarget("a", "b", "c", "d").Validate()).NotTo(Succeed())
		})
	})
})
