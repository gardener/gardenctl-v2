/*
SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package target_test

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	corev1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	clientgarden "github.com/gardener/gardenctl-v2/internal/client/garden"
	"github.com/gardener/gardenctl-v2/internal/fake"
	"github.com/gardener/gardenctl-v2/pkg/target"
)

var _ = Describe("Resolver", func() {
	createTestProject := func(name, namespace string) *gardencorev1beta1.Project {
		return &gardencorev1beta1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				UID:  "00000000-0000-0000-0000-000000000000",
			},
			Spec: gardencorev1beta1.ProjectSpec{
				Namespace: ptr.To(namespace),
			},
		}
	}

	createTestManagedSeed := func(name, shootName string) *seedmanagementv1alpha1.ManagedSeed {
		return &seedmanagementv1alpha1.ManagedSeed{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: corev1beta1constants.GardenNamespace,
				UID:       "00000000-0000-0000-0000-000000000000",
			},
			Spec: seedmanagementv1alpha1.ManagedSeedSpec{
				Shoot: &seedmanagementv1alpha1.Shoot{
					Name: shootName,
				},
			},
		}
	}

	Describe("#ResolveShoot", func() {
		It("fails when no shoot is targeted", func() {
			resolver := target.NewResolver(clientgarden.NewClient(nil, fake.NewClientWithObjects(), gardenName))

			shoot, err := resolver.ResolveShoot(ctx, target.NewTarget(gardenName, "prod", "", ""))

			Expect(err).To(MatchError(target.ErrNoShootTargeted))
			Expect(shoot).To(BeNil())
		})

		It("finds the shoot in the project namespace when a project is targeted", func() {
			project := createTestProject("prod", "garden-prod")
			shoot := createTestShoot("golden-shoot", *project.Spec.Namespace, nil)
			resolver := target.NewResolver(clientgarden.NewClient(nil, fake.NewClientWithObjects(project, shoot), gardenName))

			resolvedShoot, err := resolver.ResolveShoot(ctx, target.NewTarget(gardenName, project.Name, "", shoot.Name))

			Expect(err).NotTo(HaveOccurred())
			Expect(resolvedShoot.Namespace).To(Equal(shoot.Namespace))
			Expect(resolvedShoot.Name).To(Equal(shoot.Name))
		})

		It("finds the shoot cluster-wide when no project is targeted", func() {
			shoot := createTestShoot("golden-shoot", "garden-prod", nil)
			resolver := target.NewResolver(clientgarden.NewClient(nil, fake.NewClientWithObjects(shoot), gardenName))

			resolvedShoot, err := resolver.ResolveShoot(ctx, target.NewTarget(gardenName, "", "", shoot.Name))

			Expect(err).NotTo(HaveOccurred())
			Expect(resolvedShoot.Namespace).To(Equal(shoot.Namespace))
			Expect(resolvedShoot.Name).To(Equal(shoot.Name))
		})

		It("resolves a managed seed target to its backing shoot", func() {
			shoot := createTestShoot("seed-shoot", corev1beta1constants.GardenNamespace, nil)
			managedSeed := createTestManagedSeed("seed", shoot.Name)
			project := createTestProject(corev1beta1constants.GardenNamespace, corev1beta1constants.GardenNamespace)
			resolver := target.NewResolver(clientgarden.NewClient(nil, fake.NewClientWithObjects(project, managedSeed, shoot), gardenName))

			resolvedShoot, err := resolver.ResolveShoot(ctx, target.NewTarget(gardenName, "", "seed", ""))

			Expect(err).NotTo(HaveOccurred())
			Expect(resolvedShoot.Namespace).To(Equal(shoot.Namespace))
			Expect(resolvedShoot.Name).To(Equal(shoot.Name))
		})

		It("resolves a control-plane target to the backing shoot of the hosting managed seed", func() {
			project := createTestProject("prod", "garden-prod")
			gardenProject := createTestProject(corev1beta1constants.GardenNamespace, corev1beta1constants.GardenNamespace)
			workloadShoot := createTestShoot("workload-shoot", *project.Spec.Namespace, ptr.To("seed"))
			seedShoot := createTestShoot("seed-shoot", corev1beta1constants.GardenNamespace, nil)
			managedSeed := createTestManagedSeed("seed", seedShoot.Name)
			resolver := target.NewResolver(clientgarden.NewClient(nil, fake.NewClientWithObjects(project, gardenProject, workloadShoot, managedSeed, seedShoot), gardenName))

			resolvedShoot, err := resolver.ResolveShoot(ctx, target.NewTarget(gardenName, project.Name, "", workloadShoot.Name).WithControlPlane(true))

			Expect(err).NotTo(HaveOccurred())
			Expect(resolvedShoot.Namespace).To(Equal(seedShoot.Namespace))
			Expect(resolvedShoot.Name).To(Equal(seedShoot.Name))
		})

		It("fails clearly when a control-plane target's shoot has no seed assigned", func() {
			project := createTestProject("prod", "garden-prod")
			workloadShoot := createTestShoot("workload-shoot", *project.Spec.Namespace, nil)
			resolver := target.NewResolver(clientgarden.NewClient(nil, fake.NewClientWithObjects(project, workloadShoot), gardenName))

			shoot, err := resolver.ResolveShoot(ctx, target.NewTarget(gardenName, project.Name, "", workloadShoot.Name).WithControlPlane(true))

			Expect(err).To(MatchError("no seed assigned to shoot garden-prod/workload-shoot"))
			Expect(shoot).To(BeNil())
		})
	})
})
