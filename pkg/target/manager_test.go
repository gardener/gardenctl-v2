/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package target_test

import (
	"context"
	"fmt"

	"github.com/gardener/gardenctl-v2/internal/fake"
	"github.com/gardener/gardenctl-v2/pkg/config"
	"github.com/gardener/gardenctl-v2/pkg/target"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func assertTarget(t target.Target, expected target.Target) {
	ExpectWithOffset(1, t.GardenName()).To(Equal(expected.GardenName()))
	ExpectWithOffset(1, t.ProjectName()).To(Equal(expected.ProjectName()))
	ExpectWithOffset(1, t.SeedName()).To(Equal(expected.SeedName()))
	ExpectWithOffset(1, t.ShootName()).To(Equal(expected.ShootName()))
}

func assertTargetProvider(tp target.TargetProvider, expected target.Target) {
	t, err := tp.Read()
	Expect(err).NotTo(HaveOccurred())
	Expect(t).NotTo(BeNil())
	assertTarget(t, expected)
}

func createFakeShoot(name string, namespace string, seedName *string) (*gardencorev1beta1.Shoot, *corev1.Secret) {
	kubeconfigSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s.kubeconfig", name),
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"data": []byte("not-used"),
		},
	}

	shoot := &gardencorev1beta1.Shoot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Status: gardencorev1beta1.ShootStatus{
			SeedName: seedName,
		},
	}

	return shoot, kubeconfigSecret
}

var _ = Describe("Manager", func() {
	const (
		gardenName       = "testgarden"
		gardenKubeconfig = "/not/a/real/file"
	)

	var (
		prod1Project        *gardencorev1beta1.Project
		prod2Project        *gardencorev1beta1.Project
		unreadyProject      *gardencorev1beta1.Project
		seed                *gardencorev1beta1.Seed
		prod1GoldenShoot    *gardencorev1beta1.Shoot
		prod1AmbiguousShoot *gardencorev1beta1.Shoot
		prod2AmbiguousShoot *gardencorev1beta1.Shoot
		prod1PendingShoot   *gardencorev1beta1.Shoot
		cfg                 *config.Config
		gardenClient        client.Client
		clientProvider      *fake.ClientProvider
		kubeconfigCache     target.KubeconfigCache
	)

	BeforeEach(func() {
		cfg = &config.Config{
			Gardens: []config.Garden{{
				Name:       gardenName,
				Kubeconfig: gardenKubeconfig,
			}},
		}

		prod1Project = &gardencorev1beta1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name: "prod1",
			},
			Spec: gardencorev1beta1.ProjectSpec{
				Namespace: pointer.String("garden-prod1"),
			},
		}

		prod2Project = &gardencorev1beta1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name: "prod2",
			},
			Spec: gardencorev1beta1.ProjectSpec{
				Namespace: pointer.String("garden-prod2"),
			},
		}

		unreadyProject = &gardencorev1beta1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name: "unready-project",
			},
		}

		seedKubeconfigSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-seed-kubeconfig",
				Namespace: "garden",
			},
			Data: map[string][]byte{
				"data": []byte("not-used"),
			},
		}

		seed = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-seed",
			},
			Spec: gardencorev1beta1.SeedSpec{
				SecretRef: &corev1.SecretReference{
					Name:      seedKubeconfigSecret.Name,
					Namespace: seedKubeconfigSecret.Namespace,
				},
			},
		}

		var (
			prod1GoldenShootKubeconfig    *corev1.Secret
			prod1AmbiguousShootKubeconfig *corev1.Secret
			prod2AmbiguousShootKubeconfig *corev1.Secret
			prod1PendingShootKubeconfig   *corev1.Secret
		)

		prod1GoldenShoot, prod1GoldenShootKubeconfig = createFakeShoot("golden-shoot", *prod1Project.Spec.Namespace, pointer.String(seed.Name))
		prod1AmbiguousShoot, prod1AmbiguousShootKubeconfig = createFakeShoot("ambiguous-shoot", *prod1Project.Spec.Namespace, pointer.String(seed.Name))
		prod2AmbiguousShoot, prod2AmbiguousShootKubeconfig = createFakeShoot("ambiguous-shoot", *prod2Project.Spec.Namespace, pointer.String(seed.Name))
		prod1PendingShoot, prod1PendingShootKubeconfig = createFakeShoot("pending-shoot", *prod1Project.Spec.Namespace, nil)

		gardenClient = fakeclient.NewClientBuilder().WithObjects(
			prod1Project,
			prod2Project,
			unreadyProject,
			seedKubeconfigSecret,
			seed,
			prod1GoldenShoot,
			prod1GoldenShootKubeconfig,
			prod1AmbiguousShoot,
			prod1AmbiguousShootKubeconfig,
			prod2AmbiguousShoot,
			prod2AmbiguousShootKubeconfig,
			prod1PendingShoot,
			prod1PendingShootKubeconfig,
		).Build()

		clientProvider = fake.NewFakeClientProvider()
		clientProvider.WithClient(gardenKubeconfig, gardenClient)

		kubeconfigCache = fake.NewFakeKubeconfigCache()
	})

	It("should be able to target valid gardens", func() {
		t := target.NewTarget("", "", "", "")
		targetProvider := fake.NewFakeTargetProvider(t)

		manager, err := target.NewManager(cfg, targetProvider, clientProvider, kubeconfigCache)
		Expect(err).NotTo(HaveOccurred())
		Expect(manager).NotTo(BeNil())

		Expect(manager.TargetGarden(gardenName)).To(Succeed())
		assertTargetProvider(targetProvider, target.NewTarget(gardenName, "", "", ""))
	})

	It("should fail with invalid garden name", func() {
		t := target.NewTarget("", "", "", "")
		targetProvider := fake.NewFakeTargetProvider(t)

		manager, err := target.NewManager(cfg, targetProvider, clientProvider, kubeconfigCache)
		Expect(err).NotTo(HaveOccurred())
		Expect(manager).NotTo(BeNil())

		Expect(manager.TargetGarden("does-not-exist")).NotTo(Succeed())
		assertTargetProvider(targetProvider, t)
	})

	It("should be able to target valid projects", func() {
		t := target.NewTarget(gardenName, "", "", "")
		targetProvider := fake.NewFakeTargetProvider(t)

		manager, err := target.NewManager(cfg, targetProvider, clientProvider, kubeconfigCache)
		Expect(err).NotTo(HaveOccurred())
		Expect(manager).NotTo(BeNil())

		Expect(manager.TargetProject(context.TODO(), prod1Project.Name)).To(Succeed())
		assertTargetProvider(targetProvider, target.NewTarget(gardenName, prod1Project.Name, "", ""))
	})

	It("should fail with invalid project name", func() {
		t := target.NewTarget(gardenName, "", "", "")
		targetProvider := fake.NewFakeTargetProvider(t)

		manager, err := target.NewManager(cfg, targetProvider, clientProvider, kubeconfigCache)
		Expect(err).NotTo(HaveOccurred())
		Expect(manager).NotTo(BeNil())

		Expect(manager.TargetProject(context.TODO(), "does-not-exist")).NotTo(Succeed())
		assertTargetProvider(targetProvider, t)
	})

	It("should fail with unready project", func() {
		t := target.NewTarget(gardenName, "", "", "")
		targetProvider := fake.NewFakeTargetProvider(t)

		manager, err := target.NewManager(cfg, targetProvider, clientProvider, kubeconfigCache)
		Expect(err).NotTo(HaveOccurred())
		Expect(manager).NotTo(BeNil())

		Expect(manager.TargetProject(context.TODO(), unreadyProject.Name)).NotTo(Succeed())
		assertTargetProvider(targetProvider, t)
	})

	It("should unset deeper target levels when 'going back'", func() {
		t := target.NewTarget(gardenName, "", "", "")
		targetProvider := fake.NewFakeTargetProvider(t)

		manager, err := target.NewManager(cfg, targetProvider, clientProvider, kubeconfigCache)
		Expect(err).NotTo(HaveOccurred())
		Expect(manager).NotTo(BeNil())

		// go deep
		Expect(manager.TargetProject(context.TODO(), prod1Project.Name)).To(Succeed())
		// go back up
		Expect(manager.TargetGarden(gardenName)).To(Succeed())

		// should have the same as before
		assertTargetProvider(targetProvider, t)
	})

	It("should be able to target valid seeds and drop project and shoot target", func() {
		t := target.NewTarget(gardenName, prod1Project.Name, "", prod1AmbiguousShoot.Name)
		targetProvider := fake.NewFakeTargetProvider(t)

		manager, err := target.NewManager(cfg, targetProvider, clientProvider, kubeconfigCache)
		Expect(err).NotTo(HaveOccurred())
		Expect(manager).NotTo(BeNil())

		Expect(manager.TargetSeed(context.TODO(), seed.Name)).To(Succeed())
		assertTargetProvider(targetProvider, target.NewTarget(gardenName, "", seed.Name, ""))
	})

	It("should fail with invalid seed name", func() {
		t := target.NewTarget(gardenName, "", "", "")
		targetProvider := fake.NewFakeTargetProvider(t)

		manager, err := target.NewManager(cfg, targetProvider, clientProvider, kubeconfigCache)
		Expect(err).NotTo(HaveOccurred())
		Expect(manager).NotTo(BeNil())

		Expect(manager.TargetSeed(context.TODO(), "does-not-exist")).NotTo(Succeed())
		assertTargetProvider(targetProvider, t)
	})

	It("should be able to target valid shoots with a project already targeted", func() {
		t := target.NewTarget(gardenName, prod1Project.Name, "", "")
		targetProvider := fake.NewFakeTargetProvider(t)

		manager, err := target.NewManager(cfg, targetProvider, clientProvider, kubeconfigCache)
		Expect(err).NotTo(HaveOccurred())
		Expect(manager).NotTo(BeNil())

		Expect(manager.TargetShoot(context.TODO(), prod1AmbiguousShoot.Name)).To(Succeed())
		assertTargetProvider(targetProvider, target.NewTarget(gardenName, prod1Project.Name, "", prod1AmbiguousShoot.Name))
	})

	It("should be able to target valid shoots with a seed already targeted. Should drop seed and set shoot project instead", func() {
		t := target.NewTarget(gardenName, "", seed.Name, "")
		targetProvider := fake.NewFakeTargetProvider(t)

		manager, err := target.NewManager(cfg, targetProvider, clientProvider, kubeconfigCache)
		Expect(err).NotTo(HaveOccurred())
		Expect(manager).NotTo(BeNil())

		Expect(manager.TargetShoot(context.TODO(), prod1GoldenShoot.Name)).To(Succeed())
		assertTargetProvider(targetProvider, target.NewTarget(gardenName, prod1Project.Name, "", prod1GoldenShoot.Name))
	})

	It("should be able to target valid shoots with another seed already targeted", func() {
		t := target.NewTarget(gardenName, "", seed.Name, "")
		targetProvider := fake.NewFakeTargetProvider(t)

		manager, err := target.NewManager(cfg, targetProvider, clientProvider, kubeconfigCache)
		Expect(err).NotTo(HaveOccurred())
		Expect(manager).NotTo(BeNil())

		// another seed is already targeted, so even though this shoot exists, it does not match
		Expect(manager.TargetShoot(context.TODO(), prod1PendingShoot.Name)).NotTo(Succeed())
		assertTargetProvider(targetProvider, t)
	})

	It("should be able to target valid shoots with only garden targeted", func() {
		t := target.NewTarget(gardenName, "", "", "")
		targetProvider := fake.NewFakeTargetProvider(t)

		manager, err := target.NewManager(cfg, targetProvider, clientProvider, kubeconfigCache)
		Expect(err).NotTo(HaveOccurred())
		Expect(manager).NotTo(BeNil())

		Expect(manager.TargetShoot(context.TODO(), prod1GoldenShoot.Name)).To(Succeed())
		// project should be inserted into the path, as it is prefered over a seed step
		assertTargetProvider(targetProvider, target.NewTarget(gardenName, prod1Project.Name, "", prod1GoldenShoot.Name))
	})

	It("should error when multiple shoots match", func() {
		t := target.NewTarget(gardenName, "", "", "")
		targetProvider := fake.NewFakeTargetProvider(t)

		manager, err := target.NewManager(cfg, targetProvider, clientProvider, kubeconfigCache)
		Expect(err).NotTo(HaveOccurred())
		Expect(manager).NotTo(BeNil())

		Expect(manager.TargetShoot(context.TODO(), prod1AmbiguousShoot.Name)).NotTo(Succeed())
		assertTargetProvider(targetProvider, t)
	})

	It("should provide a garden client", func() {
		t := target.NewTarget(gardenName, "", "", "")
		targetProvider := fake.NewFakeTargetProvider(t)

		manager, err := target.NewManager(cfg, targetProvider, clientProvider, kubeconfigCache)
		Expect(err).NotTo(HaveOccurred())
		Expect(manager).NotTo(BeNil())

		newClient, err := manager.GardenClient(t)
		Expect(err).NotTo(HaveOccurred())
		Expect(newClient).NotTo(BeNil())
	})

	It("should provide a seed client", func() {
		t := target.NewTarget(gardenName, "", seed.Name, "")
		targetProvider := fake.NewFakeTargetProvider(t)

		manager, err := target.NewManager(cfg, targetProvider, clientProvider, kubeconfigCache)
		Expect(err).NotTo(HaveOccurred())
		Expect(manager).NotTo(BeNil())

		// provide a fake cached kubeconfig
		seedKubeconfig := "seed"
		seedClient := fakeclient.NewClientBuilder().Build()
		Expect(kubeconfigCache.Write(t, []byte(seedKubeconfig))).To(Succeed())
		clientProvider.WithClient(seedKubeconfig, seedClient)

		newClient, err := manager.SeedClient(context.TODO(), t)
		Expect(err).NotTo(HaveOccurred())
		Expect(newClient).NotTo(BeNil())
	})

	It("should provide a shoot client", func() {
		t := target.NewTarget(gardenName, "", seed.Name, prod1GoldenShoot.Name)
		targetProvider := fake.NewFakeTargetProvider(t)

		manager, err := target.NewManager(cfg, targetProvider, clientProvider, kubeconfigCache)
		Expect(err).NotTo(HaveOccurred())
		Expect(manager).NotTo(BeNil())

		// provide a fake cached kubeconfig
		shootKubeconfig := "shoot"
		shootClient := fakeclient.NewClientBuilder().Build()
		Expect(kubeconfigCache.Write(t, []byte(shootKubeconfig))).To(Succeed())
		clientProvider.WithClient(shootKubeconfig, shootClient)

		newClient, err := manager.ShootClusterClient(context.TODO(), t)
		Expect(err).NotTo(HaveOccurred())
		Expect(newClient).NotTo(BeNil())
	})

	It("should be able to unset selected garden", func() {
		t := target.NewTarget(gardenName, "", "", "")
		targetProvider := fake.NewFakeTargetProvider(t)

		manager, err := target.NewManager(cfg, targetProvider, clientProvider, kubeconfigCache)
		Expect(err).NotTo(HaveOccurred())
		Expect(manager).NotTo(BeNil())

		Expect(manager.UnsetTargetGarden()).Should(Equal(gardenName))
		assertTargetProvider(targetProvider, target.NewTarget("", "", "", ""))
	})

	It("should fail if no garden selected", func() {
		t := target.NewTarget("", "", "", "")
		targetProvider := fake.NewFakeTargetProvider(t)

		manager, err := target.NewManager(cfg, targetProvider, clientProvider, kubeconfigCache)
		Expect(err).NotTo(HaveOccurred())
		Expect(manager).NotTo(BeNil())

		res, unsetErr := manager.UnsetTargetGarden()
		Expect(unsetErr).To(HaveOccurred())
		Expect(res).To(BeEmpty())
		assertTargetProvider(targetProvider, t)
	})

	It("should unset deeper target levels when unsetting garden", func() {
		t := target.NewTarget(gardenName, prod1Project.Name, seed.Name, prod1AmbiguousShoot.Name)
		targetProvider := fake.NewFakeTargetProvider(t)

		manager, err := target.NewManager(cfg, targetProvider, clientProvider, kubeconfigCache)
		Expect(err).NotTo(HaveOccurred())
		Expect(manager).NotTo(BeNil())

		// Unset Garden
		Expect(manager.UnsetTargetGarden()).Should(Equal(gardenName))

		// should also unset project, seed and shoot
		assertTargetProvider(targetProvider, target.NewTarget("", "", "", ""))
	})

	It("should be able to unset selected project", func() {
		t := target.NewTarget(gardenName, prod1Project.Name, "", "")
		targetProvider := fake.NewFakeTargetProvider(t)

		manager, err := target.NewManager(cfg, targetProvider, clientProvider, kubeconfigCache)
		Expect(err).NotTo(HaveOccurred())
		Expect(manager).NotTo(BeNil())

		Expect(manager.UnsetTargetProject()).Should(Equal(prod1Project.Name))
		assertTargetProvider(targetProvider, target.NewTarget(gardenName, "", "", ""))
	})

	It("should fail if no project selected", func() {
		t := target.NewTarget(gardenName, "", "", "")
		targetProvider := fake.NewFakeTargetProvider(t)

		manager, err := target.NewManager(cfg, targetProvider, clientProvider, kubeconfigCache)
		Expect(err).NotTo(HaveOccurred())
		Expect(manager).NotTo(BeNil())

		res, unsetErr := manager.UnsetTargetProject()
		Expect(unsetErr).To(HaveOccurred())
		Expect(res).To(BeEmpty())
		assertTargetProvider(targetProvider, t)
	})

	It("should unset deeper target levels when unsetting project", func() {
		t := target.NewTarget(gardenName, prod1Project.Name, "", prod1AmbiguousShoot.Name)
		targetProvider := fake.NewFakeTargetProvider(t)

		manager, err := target.NewManager(cfg, targetProvider, clientProvider, kubeconfigCache)
		Expect(err).NotTo(HaveOccurred())
		Expect(manager).NotTo(BeNil())

		// Unset Project
		Expect(manager.UnsetTargetProject()).Should(Equal(prod1Project.Name))

		// should also unset shoot
		assertTargetProvider(targetProvider, target.NewTarget(gardenName, "", "", ""))
	})

	It("should be able to unset selected shoot", func() {
		t := target.NewTarget(gardenName, prod1Project.Name, "", prod1AmbiguousShoot.Name)
		targetProvider := fake.NewFakeTargetProvider(t)

		manager, err := target.NewManager(cfg, targetProvider, clientProvider, kubeconfigCache)
		Expect(err).NotTo(HaveOccurred())
		Expect(manager).NotTo(BeNil())

		Expect(manager.UnsetTargetShoot()).Should(Equal(prod1AmbiguousShoot.Name))
		assertTargetProvider(targetProvider, target.NewTarget(gardenName, prod1Project.Name, "", ""))
	})

	It("should fail if no shoot selected", func() {
		t := target.NewTarget(gardenName, prod1Project.Name, "", "")
		targetProvider := fake.NewFakeTargetProvider(t)

		manager, err := target.NewManager(cfg, targetProvider, clientProvider, kubeconfigCache)
		Expect(err).NotTo(HaveOccurred())
		Expect(manager).NotTo(BeNil())

		res, unsetErr := manager.UnsetTargetShoot()
		Expect(unsetErr).To(HaveOccurred())
		Expect(res).To(BeEmpty())
		assertTargetProvider(targetProvider, t)
	})

	It("should be able to unset selected seed", func() {
		t := target.NewTarget(gardenName, "", seed.Name, "")
		targetProvider := fake.NewFakeTargetProvider(t)

		manager, err := target.NewManager(cfg, targetProvider, clientProvider, kubeconfigCache)
		Expect(err).NotTo(HaveOccurred())
		Expect(manager).NotTo(BeNil())

		Expect(manager.UnsetTargetSeed()).Should(Equal(seed.Name))
		assertTargetProvider(targetProvider, target.NewTarget(gardenName, "", "", ""))
	})

	It("should fail if no seed selected", func() {
		t := target.NewTarget(gardenName, prod1Project.Name, "", "")
		targetProvider := fake.NewFakeTargetProvider(t)

		manager, err := target.NewManager(cfg, targetProvider, clientProvider, kubeconfigCache)
		Expect(err).NotTo(HaveOccurred())
		Expect(manager).NotTo(BeNil())

		res, unsetErr := manager.UnsetTargetSeed()
		Expect(unsetErr).To(HaveOccurred())
		Expect(res).To(BeEmpty())
		assertTargetProvider(targetProvider, t)
	})
})
