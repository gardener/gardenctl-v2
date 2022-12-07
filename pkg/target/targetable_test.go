/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package target_test

import (
	// gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/onsi/ginkgo"
	// . "github.com/onsi/gomega"
)

var _ = Describe("Target Utilities", func() {
	// var (
	// 	testReadyProject   *gardencorev1beta1.Project
	// 	testUnreadyProject *gardencorev1beta1.Project
	// 	testSeed           *gardencorev1beta1.Seed
	// 	testShoot          *gardencorev1beta1.Shoot
	// 	gardenClient       gardenclient.Client
	// )

	BeforeEach(func() {
		// testReadyProject = &gardencorev1beta1.Project{
		// 	ObjectMeta: metav1.ObjectMeta{
		// 		Name: "prod1",
		// 	},
		// 	Spec: gardencorev1beta1.ProjectSpec{
		// 		Namespace: pointer.String("garden-prod1"),
		// 	},
		// }

		// testUnreadyProject = &gardencorev1beta1.Project{
		// 	ObjectMeta: metav1.ObjectMeta{
		// 		Name: "prod2",
		// 	},
		// }

		// testSeed = &gardencorev1beta1.Seed{
		// 	ObjectMeta: metav1.ObjectMeta{
		// 		Name: "test-seed",
		// 	},
		// }

		// testShoot = &gardencorev1beta1.Shoot{
		// 	ObjectMeta: metav1.ObjectMeta{
		// 		Name:      "test-shoot",
		// 		Namespace: *testReadyProject.Spec.Namespace,
		// 	},
		// 	Spec: gardencorev1beta1.ShootSpec{
		// 		SeedName: pointer.String(testSeed.Name),
		// 	},
		// }

		// gardenClient = gardenclient.NewGardenClient(
		// 	fake.NewClientWithObjects(
		// 		testReadyProject,
		// 		testUnreadyProject,
		// 		testSeed,
		// 		testShoot,
		// 	),
		// 	"my-garden",
		// )
	})

	Describe("GardenNames", func() {
	})

	Describe("ProjectNamesForTarget", func() {
	})

	Describe("SeedNamesForTarget", func() {
	})

	Describe("ShootNamesForTarget", func() {
	})
})
