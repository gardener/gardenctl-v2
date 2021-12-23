/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package util_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardenctl-v2/internal/gardenclient"
	. "github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/target"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

var _ = Describe("Target Utilities", func() {
	var (
		testReadyProject   *gardencorev1beta1.Project
		testUnreadyProject *gardencorev1beta1.Project
		testSeed           *gardencorev1beta1.Seed
		testShoot          *gardencorev1beta1.Shoot
		gardenClient       gardenclient.Client
	)

	BeforeEach(func() {
		testReadyProject = &gardencorev1beta1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name: "prod1",
			},
			Spec: gardencorev1beta1.ProjectSpec{
				Namespace: pointer.String("garden-prod1"),
			},
		}

		testUnreadyProject = &gardencorev1beta1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name: "prod2",
			},
		}

		testSeedKubeconfig := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-seed-kubeconfig",
				Namespace: "garden",
			},
			Data: map[string][]byte{
				"data": []byte("not-used"),
			},
		}

		testSeed = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-seed",
			},
			Spec: gardencorev1beta1.SeedSpec{
				SecretRef: &corev1.SecretReference{
					Name:      testSeedKubeconfig.Name,
					Namespace: testSeedKubeconfig.Namespace,
				},
			},
		}

		testShoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-shoot",
				Namespace: *testReadyProject.Spec.Namespace,
			},
			Spec: gardencorev1beta1.ShootSpec{
				SeedName: pointer.String(testSeed.Name),
			},
		}

		testShootKubeconfig := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s.kubeconfig", testShoot.Name),
				Namespace: *testReadyProject.Spec.Namespace,
			},
			Data: map[string][]byte{
				"data": []byte("not-used"),
			},
		}

		gardenClient = gardenclient.NewGardenClient(fakeclient.NewClientBuilder().WithObjects(
			testReadyProject,
			testUnreadyProject,
			testSeedKubeconfig,
			testSeed,
			testShootKubeconfig,
			testShoot,
		).Build())
	})

	Describe("SeedForTarget", func() {
		It("should require a targeted seed", func() {
			t := target.NewTarget("a", "", "", "", false)
			ctx := context.Background()

			seed, err := SeedForTarget(ctx, gardenClient, t)
			Expect(seed).To(BeNil())
			Expect(err).To(HaveOccurred())
		})

		It("should return a valid seed", func() {
			t := target.NewTarget("a", "", testSeed.Name, "", false)
			ctx := context.Background()

			seed, err := SeedForTarget(ctx, gardenClient, t)
			Expect(err).NotTo(HaveOccurred())
			Expect(seed).NotTo(BeNil())
			Expect(seed.Name).To(Equal(testSeed.Name))
		})

		It("should require a targeted project", func() {
			t := target.NewTarget("a", "", "", "", false)
			ctx := context.Background()

			project, err := ProjectForTarget(ctx, gardenClient, t)
			Expect(project).To(BeNil())
			Expect(err).To(HaveOccurred())
		})

		It("should return a valid project", func() {
			t := target.NewTarget("a", testReadyProject.Name, "", "", false)
			ctx := context.Background()

			project, err := ProjectForTarget(ctx, gardenClient, t)
			Expect(err).NotTo(HaveOccurred())
			Expect(project).NotTo(BeNil())
			Expect(project.Name).To(Equal(testReadyProject.Name))
		})

		It("should return an unready project", func() {
			t := target.NewTarget("a", testUnreadyProject.Name, "", "", false)
			ctx := context.Background()

			project, err := ProjectForTarget(ctx, gardenClient, t)
			Expect(err).NotTo(HaveOccurred())
			Expect(project).NotTo(BeNil())
			Expect(project.Name).To(Equal(testUnreadyProject.Name))
		})

		It("should return a valid shoot when not using a project or seed", func() {
			t := target.NewTarget("a", "", "", testShoot.Name, false)
			ctx := context.Background()

			shoot, err := ShootForTarget(ctx, gardenClient, t)
			Expect(err).NotTo(HaveOccurred())
			Expect(shoot).NotTo(BeNil())
			Expect(shoot.Name).To(Equal(testShoot.Name))
		})

		It("should return a valid shoot when using a project", func() {
			t := target.NewTarget("a", testReadyProject.Name, "", testShoot.Name, false)
			ctx := context.Background()

			shoot, err := ShootForTarget(ctx, gardenClient, t)
			Expect(err).NotTo(HaveOccurred())
			Expect(shoot).NotTo(BeNil())
			Expect(shoot.Name).To(Equal(testShoot.Name))
		})

		It("should return an error when using an unready project", func() {
			t := target.NewTarget("a", testUnreadyProject.Name, "", testShoot.Name, false)
			ctx := context.Background()

			shoot, err := ShootForTarget(ctx, gardenClient, t)
			Expect(shoot).To(BeNil())
			Expect(err).To(HaveOccurred())
		})

		It("should return a valid shoot when using a seed", func() {
			t := target.NewTarget("a", "", testSeed.Name, testShoot.Name, false)
			ctx := context.Background()

			shoot, err := ShootForTarget(ctx, gardenClient, t)
			Expect(err).NotTo(HaveOccurred())
			Expect(shoot).NotTo(BeNil())
			Expect(shoot.Name).To(Equal(testShoot.Name))
		})
	})
})
