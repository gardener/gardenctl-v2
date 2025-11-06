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

			// valid shoot names (DNS label compliant)
			Expect(target.NewTarget("a", "b", "", "my-shoot").Validate()).To(Succeed())
			Expect(target.NewTarget("a", "b", "", "shoot-123").Validate()).To(Succeed())
			Expect(target.NewTarget("a", "b", "", "a").Validate()).To(Succeed())

			// invalid shoot names (not DNS label compliant)
			Expect(target.NewTarget("a", "b", "", "My-Shoot").Validate()).NotTo(Succeed())   // uppercase
			Expect(target.NewTarget("a", "b", "", "shoot_name").Validate()).NotTo(Succeed()) // underscore
			Expect(target.NewTarget("a", "b", "", "-shoot").Validate()).NotTo(Succeed())     // starts with dash
			Expect(target.NewTarget("a", "b", "", "shoot-").Validate()).NotTo(Succeed())     // ends with dash
			Expect(target.NewTarget("a", "b", "", "shoot.name").Validate()).NotTo(Succeed()) // contains dot
			Expect(target.NewTarget("a", "b", "", "shoot name").Validate()).NotTo(Succeed()) // contains space

			// valid project names (DNS subdomain compliant)
			Expect(target.NewTarget("a", "my-project", "", "").Validate()).To(Succeed())
			Expect(target.NewTarget("a", "project-123", "", "").Validate()).To(Succeed())
			Expect(target.NewTarget("a", "project.subdomain", "", "").Validate()).To(Succeed()) // dots allowed in subdomain
			Expect(target.NewTarget("a", "a", "", "").Validate()).To(Succeed())

			// invalid project names (not DNS subdomain compliant)
			Expect(target.NewTarget("a", "My-Project", "", "").Validate()).NotTo(Succeed())   // uppercase
			Expect(target.NewTarget("a", "project_name", "", "").Validate()).NotTo(Succeed()) // underscore
			Expect(target.NewTarget("a", "-project", "", "").Validate()).NotTo(Succeed())     // starts with dash
			Expect(target.NewTarget("a", "project-", "", "").Validate()).NotTo(Succeed())     // ends with dash
			Expect(target.NewTarget("a", "project name", "", "").Validate()).NotTo(Succeed()) // contains space

			// valid seed names (DNS label compliant)
			Expect(target.NewTarget("a", "", "my-seed", "").Validate()).To(Succeed())
			Expect(target.NewTarget("a", "", "seed-123", "").Validate()).To(Succeed())
			Expect(target.NewTarget("a", "", "a", "").Validate()).To(Succeed())

			// invalid seed names (not DNS label compliant)
			Expect(target.NewTarget("a", "", "My-Seed", "").Validate()).NotTo(Succeed())   // uppercase
			Expect(target.NewTarget("a", "", "seed_name", "").Validate()).NotTo(Succeed()) // underscore
			Expect(target.NewTarget("a", "", "-seed", "").Validate()).NotTo(Succeed())     // starts with dash
			Expect(target.NewTarget("a", "", "seed-", "").Validate()).NotTo(Succeed())     // ends with dash
			Expect(target.NewTarget("a", "", "seed.name", "").Validate()).NotTo(Succeed()) // contains dot
			Expect(target.NewTarget("a", "", "seed name", "").Validate()).NotTo(Succeed()) // contains space
		})
	})
})
