/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package target_test

import (
	"fmt"
	"os"
	"strings"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardenctl-v2/internal/fake"
	"github.com/gardener/gardenctl-v2/pkg/config"
	"github.com/gardener/gardenctl-v2/pkg/target"
	targetmocks "github.com/gardener/gardenctl-v2/pkg/target/mocks"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

func assertTargetProvider(tp target.TargetProvider, expected target.Target) {
	t, err := tp.Read()
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	ExpectWithOffset(1, t).NotTo(BeNil())
	ExpectWithOffset(1, t.GardenName()).To(Equal(expected.GardenName()))
	ExpectWithOffset(1, t.ProjectName()).To(Equal(expected.ProjectName()))
	ExpectWithOffset(1, t.SeedName()).To(Equal(expected.SeedName()))
	ExpectWithOffset(1, t.ShootName()).To(Equal(expected.ShootName()))
	ExpectWithOffset(1, t.ControlPlane()).To(Equal(expected.ControlPlane()))
}

func createTestShoot(name string, namespace string, seedName *string) *gardencorev1beta1.Shoot {
	shoot := &gardencorev1beta1.Shoot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: gardencorev1beta1.ShootSpec{
			SeedName: seedName,
		},
	}

	if strings.HasPrefix(namespace, "garden-") {
		project := strings.TrimPrefix(namespace, "garden-")
		shoot.Status.TechnicalID = fmt.Sprintf("shoot--%s--%s", project, name)
	}

	return shoot
}

func cloneTarget(t target.Target) target.Target {
	return target.NewTarget(t.GardenName(), t.ProjectName(), t.SeedName(), t.ShootName()).WithControlPlane(t.ControlPlane())
}

func createTestManager(t target.Target, cfg *config.Config, clientProvider target.ClientProvider) (target.Manager, target.TargetProvider) {
	targetProvider := fake.NewFakeTargetProvider(cloneTarget(t))

	sessionDir := os.TempDir()
	manager, err := target.NewManager(cfg, targetProvider, clientProvider, sessionDir)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	ExpectWithOffset(1, manager).NotTo(BeNil())

	return manager, targetProvider
}

var _ = Describe("Target Manager", func() {
	const (
		gardenName       = "testgarden"
		gardenKubeconfig = "/not/a/real/file"
	)

	var (
		ctrl                                *gomock.Controller
		prod1Project                        *gardencorev1beta1.Project
		prod2Project                        *gardencorev1beta1.Project
		unreadyProject                      *gardencorev1beta1.Project
		seed                                *gardencorev1beta1.Seed
		seedKubeconfigSecret                *corev1.Secret
		prod1GoldenShoot                    *gardencorev1beta1.Shoot
		prod1GoldenShootKubeconfigConfigMap *corev1.ConfigMap
		prod1AmbiguousShoot                 *gardencorev1beta1.Shoot
		prod2AmbiguousShoot                 *gardencorev1beta1.Shoot
		prod1PendingShoot                   *gardencorev1beta1.Shoot
		cfg                                 *config.Config
		gardenClient                        client.Client
		clientProvider                      *targetmocks.MockClientProvider
		namespace                           *corev1.Namespace
	)

	BeforeEach(func() {
		cfg = &config.Config{
			Gardens: []config.Garden{{
				Name:       gardenName,
				Kubeconfig: gardenKubeconfig,
				Patterns: []string{
					fmt.Sprintf("^(%s/)?shoot--(?P<project>.+)--(?P<shoot>.+)$", gardenName),
					"^namespace:(?P<namespace>[^/]+)$",
				},
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

		seedKubeconfigSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-seed.oidc",
				Namespace: "garden",
			},
			Data: map[string][]byte{
				"kubeconfig": createTestKubeconfig("test-seed"),
			},
		}

		seed = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-seed",
			},
		}

		namespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: *prod1Project.Spec.Namespace,
				Labels: map[string]string{
					"project.gardener.cloud/name": prod1Project.Name,
				},
			},
		}

		prod1GoldenShoot = createTestShoot("golden-shoot", *prod1Project.Spec.Namespace, pointer.String(seed.Name))
		prod1GoldenShootKubeconfigConfigMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      prod1GoldenShoot.Name + ".kubeconfig",
				Namespace: prod1GoldenShoot.Namespace,
			},
			Data: map[string]string{
				"kubeconfig": string(createTestKubeconfig(prod1GoldenShoot.Name)),
			},
		}
		prod1AmbiguousShoot = createTestShoot("ambiguous-shoot", *prod1Project.Spec.Namespace, pointer.String(seed.Name))
		prod2AmbiguousShoot = createTestShoot("ambiguous-shoot", *prod2Project.Spec.Namespace, pointer.String(seed.Name))
		prod1PendingShoot = createTestShoot("pending-shoot", *prod1Project.Spec.Namespace, nil)

		gardenClient = fake.NewClientWithObjects(
			prod1Project,
			prod2Project,
			unreadyProject,
			seed,
			seedKubeconfigSecret,
			prod1GoldenShoot,
			prod1GoldenShootKubeconfigConfigMap,
			prod1AmbiguousShoot,
			prod2AmbiguousShoot,
			prod1PendingShoot,
			namespace,
		)

		ctrl = gomock.NewController(GinkgoT())
		clientProvider = targetmocks.NewMockClientProvider(ctrl)
		clientConfig, err := cfg.ClientConfig(gardenName)
		Expect(err).NotTo(HaveOccurred())
		clientProvider.EXPECT().FromClientConfig(gomock.Eq(clientConfig)).Return(gardenClient, nil).AnyTimes()
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("should be able to target valid gardens", func() {
		t := target.NewTarget("", "", "", "")
		manager, targetProvider := createTestManager(t, cfg, clientProvider)

		Expect(manager.TargetGarden(ctx, gardenName)).To(Succeed())
		assertTargetProvider(targetProvider, target.NewTarget(gardenName, "", "", ""))
	})

	It("should fail with invalid garden name", func() {
		t := target.NewTarget("", "", "", "")
		manager, targetProvider := createTestManager(t, cfg, clientProvider)

		Expect(manager.TargetGarden(ctx, "does-not-exist")).NotTo(Succeed())
		assertTargetProvider(targetProvider, t)
	})

	It("should be able to target valid projects", func() {
		t := target.NewTarget(gardenName, "", "", "")
		manager, targetProvider := createTestManager(t, cfg, clientProvider)

		Expect(manager.TargetProject(ctx, prod1Project.Name)).To(Succeed())
		assertTargetProvider(targetProvider, target.NewTarget(gardenName, prod1Project.Name, "", ""))
	})

	It("should fail with invalid project name", func() {
		t := target.NewTarget(gardenName, "", "", "")
		manager, targetProvider := createTestManager(t, cfg, clientProvider)

		Expect(manager.TargetProject(ctx, "does-not-exist")).NotTo(Succeed())
		assertTargetProvider(targetProvider, t)
	})

	It("should fail with unready project", func() {
		t := target.NewTarget(gardenName, "", "", "")
		manager, targetProvider := createTestManager(t, cfg, clientProvider)

		Expect(manager.TargetProject(ctx, unreadyProject.Name)).NotTo(Succeed())
		assertTargetProvider(targetProvider, t)
	})

	It("should unset deeper target levels when 'going back'", func() {
		t := target.NewTarget(gardenName, "", "", "")
		manager, targetProvider := createTestManager(t, cfg, clientProvider)

		// go deep
		Expect(manager.TargetProject(ctx, prod1Project.Name)).To(Succeed())
		// go back up
		Expect(manager.TargetGarden(ctx, gardenName)).To(Succeed())

		// should have the same as before
		assertTargetProvider(targetProvider, t)
	})

	It("should be able to target valid seeds and drop project and shoot target", func() {
		t := target.NewTarget(gardenName, prod1Project.Name, "", prod1AmbiguousShoot.Name)
		manager, targetProvider := createTestManager(t, cfg, clientProvider)

		Expect(manager.TargetSeed(ctx, seed.Name)).To(Succeed())
		assertTargetProvider(targetProvider, target.NewTarget(gardenName, "", seed.Name, ""))
	})

	It("should fail with invalid seed name", func() {
		t := target.NewTarget(gardenName, "", "", "")
		manager, targetProvider := createTestManager(t, cfg, clientProvider)

		Expect(manager.TargetSeed(ctx, "does-not-exist")).NotTo(Succeed())
		assertTargetProvider(targetProvider, t)
	})

	It("should be able to target valid shoots with a project already targeted", func() {
		t := target.NewTarget(gardenName, prod1Project.Name, "", "")
		manager, targetProvider := createTestManager(t, cfg, clientProvider)

		Expect(manager.TargetShoot(ctx, prod1AmbiguousShoot.Name)).To(Succeed())
		assertTargetProvider(targetProvider, target.NewTarget(gardenName, prod1Project.Name, "", prod1AmbiguousShoot.Name))
	})

	It("should be able to target valid shoots with a seed already targeted. Should drop seed and set shoot project instead", func() {
		t := target.NewTarget(gardenName, "", seed.Name, "")
		manager, targetProvider := createTestManager(t, cfg, clientProvider)

		Expect(manager.TargetShoot(ctx, prod1GoldenShoot.Name)).To(Succeed())
		assertTargetProvider(targetProvider, target.NewTarget(gardenName, prod1Project.Name, "", prod1GoldenShoot.Name))
	})

	It("should not be able to target valid shoots with another seed already targeted", func() {
		t := target.NewTarget(gardenName, "", seed.Name, "")
		manager, targetProvider := createTestManager(t, cfg, clientProvider)

		// another seed is already targeted, so even though this shoot exists, it does not match
		Expect(manager.TargetShoot(ctx, prod1PendingShoot.Name)).NotTo(Succeed())
		assertTargetProvider(targetProvider, t)
	})

	It("should be able to target valid shoots with only garden targeted", func() {
		t := target.NewTarget(gardenName, "", "", "")
		manager, targetProvider := createTestManager(t, cfg, clientProvider)

		Expect(manager.TargetShoot(ctx, prod1GoldenShoot.Name)).To(Succeed())
		// project should be inserted into the path, as it is preferred over a seed step
		assertTargetProvider(targetProvider, target.NewTarget(gardenName, prod1Project.Name, "", prod1GoldenShoot.Name))
	})

	It("should error when multiple shoots match", func() {
		t := target.NewTarget(gardenName, "", "", "")
		manager, targetProvider := createTestManager(t, cfg, clientProvider)

		Expect(manager.TargetShoot(ctx, prod1AmbiguousShoot.Name)).NotTo(Succeed())
		assertTargetProvider(targetProvider, t)
	})

	It("should be able to target valid garden, project and shoot by matching a pattern", func() {
		t := target.NewTarget("", "", "", "")
		manager, targetProvider := createTestManager(t, cfg, clientProvider)

		Expect(manager.TargetMatchPattern(ctx, fmt.Sprintf("%s/shoot--%s--%s", gardenName, prod1Project.Name, prod1GoldenShoot.Name))).To(Succeed())
		assertTargetProvider(targetProvider, target.NewTarget(gardenName, prod1Project.Name, "", prod1GoldenShoot.Name))
	})

	It("should be able to target valid project shoot by matching a pattern if garden is set", func() {
		t := target.NewTarget(gardenName, "", "", "")
		manager, targetProvider := createTestManager(t, cfg, clientProvider)

		Expect(manager.TargetMatchPattern(ctx, fmt.Sprintf("shoot--%s--%s", prod1Project.Name, prod1GoldenShoot.Name))).To(Succeed())
		assertTargetProvider(targetProvider, target.NewTarget(gardenName, prod1Project.Name, "", prod1GoldenShoot.Name))
	})

	It("should be able to target shoot by matching a pattern if garden is not set", func() {
		t := target.NewTarget("", "", "", "")
		manager, targetProvider := createTestManager(t, cfg, clientProvider)

		Expect(manager.TargetMatchPattern(ctx, fmt.Sprintf("shoot--%s--%s", prod1Project.Name, prod1GoldenShoot.Name))).To(Succeed())
		assertTargetProvider(targetProvider, target.NewTarget(gardenName, prod1Project.Name, "", prod1GoldenShoot.Name))
	})

	It("should not target anything if target is not completely valid", func() {
		t := target.NewTarget(gardenName, "", "", "")
		manager, targetProvider := createTestManager(t, cfg, clientProvider)

		Expect(manager.TargetMatchPattern(ctx, fmt.Sprintf("shoot--%s--%s", prod1Project.Name, "invalid shoot"))).NotTo(Succeed())
		assertTargetProvider(targetProvider, target.NewTarget(gardenName, "", "", ""))
	})

	It("should be able to target valid project by matching a pattern containing a namespace", func() {
		t := target.NewTarget(gardenName, "", "", "")
		manager, targetProvider := createTestManager(t, cfg, clientProvider)

		Expect(manager.TargetMatchPattern(ctx, fmt.Sprintf("namespace:%s", *prod1Project.Spec.Namespace))).To(Succeed())
		assertTargetProvider(targetProvider, target.NewTarget(gardenName, prod1Project.Name, "", ""))
	})

	It("should be able to target control plane for a shoot", func() {
		t := target.NewTarget(gardenName, prod1Project.Name, "", prod1GoldenShoot.Name)
		manager, targetProvider := createTestManager(t, cfg, clientProvider)

		Expect(manager.TargetControlPlane(ctx)).To(Succeed())
		assertTargetProvider(targetProvider, t.WithControlPlane(true))
	})

	It("should fail to target control plane if shoot is not set", func() {
		t := target.NewTarget(gardenName, prod1Project.Name, "", "")
		manager, targetProvider := createTestManager(t, cfg, clientProvider)

		Expect(manager.TargetControlPlane(ctx)).NotTo(Succeed())
		assertTargetProvider(targetProvider, t)
	})

	It("should fail to target control plane if garden is not set", func() {
		t := target.NewTarget("", prod1Project.Name, "", prod1GoldenShoot.Name)
		manager, targetProvider := createTestManager(t, cfg, clientProvider)

		Expect(manager.TargetControlPlane(ctx)).NotTo(Succeed())
		assertTargetProvider(targetProvider, t)
	})

	It("should provide a garden client", func() {
		t := target.NewTarget(gardenName, "", "", "")
		manager, _ := createTestManager(t, cfg, clientProvider)

		newClient, err := manager.GardenClient(t.GardenName())
		Expect(err).NotTo(HaveOccurred())
		Expect(newClient).NotTo(BeNil())
	})

	It("should provide a seed client", func() {
		t := target.NewTarget(gardenName, "", seed.Name, "")
		manager, _ := createTestManager(t, cfg, clientProvider)

		seedClient := fake.NewClientWithObjects()
		clientConfig, err := clientcmd.NewClientConfigFromBytes(seedKubeconfigSecret.Data["kubeconfig"])
		Expect(err).NotTo(HaveOccurred())
		clientProvider.EXPECT().FromClientConfig(gomock.Eq(clientConfig)).Return(seedClient, nil)

		newClient, err := manager.SeedClient(ctx, t)
		Expect(err).NotTo(HaveOccurred())
		Expect(newClient).NotTo(BeNil())
	})

	It("should provide a shoot client", func() {
		t := target.NewTarget(gardenName, prod1Project.Name, "", prod1GoldenShoot.Name)
		manager, _ := createTestManager(t, cfg, clientProvider)

		shootClient := fake.NewClientWithObjects()
		clientConfig, err := clientcmd.NewClientConfigFromBytes([]byte(prod1GoldenShootKubeconfigConfigMap.Data["kubeconfig"]))
		Expect(err).NotTo(HaveOccurred())
		clientProvider.EXPECT().FromClientConfig(gomock.Eq(clientConfig)).Return(shootClient, nil)

		newClient, err := manager.ShootClient(ctx, t)
		Expect(err).NotTo(HaveOccurred())
		Expect(newClient).NotTo(BeNil())
	})

	It("should be able to unset selected garden", func() {
		t := target.NewTarget(gardenName, "", "", "")
		manager, targetProvider := createTestManager(t, cfg, clientProvider)

		Expect(manager.UnsetTargetGarden()).Should(Equal(gardenName))
		assertTargetProvider(targetProvider, target.NewTarget("", "", "", ""))
	})

	It("should fail if no garden selected", func() {
		t := target.NewTarget("", "", "", "")
		manager, targetProvider := createTestManager(t, cfg, clientProvider)

		res, unsetErr := manager.UnsetTargetGarden()
		Expect(unsetErr).To(HaveOccurred())
		Expect(res).To(BeEmpty())
		assertTargetProvider(targetProvider, t)
	})

	It("should unset deeper target levels when unsetting garden", func() {
		t := target.NewTarget(gardenName, prod1Project.Name, seed.Name, prod1AmbiguousShoot.Name)
		manager, targetProvider := createTestManager(t, cfg, clientProvider)

		// Unset Garden
		Expect(manager.UnsetTargetGarden()).Should(Equal(gardenName))

		// should also unset project, seed and shoot
		assertTargetProvider(targetProvider, target.NewTarget("", "", "", ""))
	})

	It("should be able to unset selected project", func() {
		t := target.NewTarget(gardenName, prod1Project.Name, "", "")
		manager, targetProvider := createTestManager(t, cfg, clientProvider)

		Expect(manager.UnsetTargetProject()).Should(Equal(prod1Project.Name))
		assertTargetProvider(targetProvider, target.NewTarget(gardenName, "", "", ""))
	})

	It("should fail if no project selected", func() {
		t := target.NewTarget(gardenName, "", "", "")
		manager, targetProvider := createTestManager(t, cfg, clientProvider)

		res, unsetErr := manager.UnsetTargetProject()
		Expect(unsetErr).To(HaveOccurred())
		Expect(res).To(BeEmpty())
		assertTargetProvider(targetProvider, t)
	})

	It("should unset deeper target levels when unsetting project", func() {
		t := target.NewTarget(gardenName, prod1Project.Name, "", prod1AmbiguousShoot.Name).WithControlPlane(true)
		manager, targetProvider := createTestManager(t, cfg, clientProvider)

		// Unset Project
		Expect(manager.UnsetTargetProject()).Should(Equal(prod1Project.Name))

		// should also unset shoot
		assertTargetProvider(targetProvider, target.NewTarget(gardenName, "", "", ""))
	})

	It("should be able to unset selected shoot", func() {
		t := target.NewTarget(gardenName, prod1Project.Name, "", prod1AmbiguousShoot.Name)
		manager, targetProvider := createTestManager(t, cfg, clientProvider)

		Expect(manager.UnsetTargetShoot()).Should(Equal(prod1AmbiguousShoot.Name))
		assertTargetProvider(targetProvider, target.NewTarget(gardenName, prod1Project.Name, "", ""))
	})

	It("should fail if no shoot selected", func() {
		t := target.NewTarget(gardenName, prod1Project.Name, "", "")
		manager, targetProvider := createTestManager(t, cfg, clientProvider)

		res, unsetErr := manager.UnsetTargetShoot()
		Expect(unsetErr).To(HaveOccurred())
		Expect(res).To(BeEmpty())
		assertTargetProvider(targetProvider, t)
	})

	It("should be able to unset selected control plane", func() {
		t := target.NewTarget(gardenName, prod1Project.Name, "", prod1AmbiguousShoot.Name).WithControlPlane(true)
		manager, targetProvider := createTestManager(t, cfg, clientProvider)

		Expect(manager.UnsetTargetControlPlane()).To(Succeed())
		assertTargetProvider(targetProvider, t.WithControlPlane(false))
	})

	It("should fail if no control plane targeted", func() {
		t := target.NewTarget(gardenName, prod1Project.Name, "", "")
		manager, targetProvider := createTestManager(t, cfg, clientProvider)

		unsetErr := manager.UnsetTargetControlPlane()
		Expect(unsetErr).To(HaveOccurred())
		assertTargetProvider(targetProvider, t)
	})

	It("should be able to unset selected seed", func() {
		t := target.NewTarget(gardenName, "", seed.Name, "")
		manager, targetProvider := createTestManager(t, cfg, clientProvider)

		Expect(manager.UnsetTargetSeed()).Should(Equal(seed.Name))
		assertTargetProvider(targetProvider, target.NewTarget(gardenName, "", "", ""))
	})

	It("should fail if no seed selected", func() {
		t := target.NewTarget(gardenName, prod1Project.Name, "", "")
		manager, targetProvider := createTestManager(t, cfg, clientProvider)

		res, unsetErr := manager.UnsetTargetSeed()
		Expect(unsetErr).To(HaveOccurred())
		Expect(res).To(BeEmpty())
		assertTargetProvider(targetProvider, t)
	})

	Describe("Getting Client Configurations", func() {
		var (
			manager target.Manager
			t       target.Target
		)

		JustBeforeEach(func() {
			manager, _ = createTestManager(t, cfg, clientProvider)
		})

		Context("when shoot control-plane is targeted", func() {
			BeforeEach(func() {
				t = target.NewTarget(gardenName, prod1Project.Name, "", prod1GoldenShoot.Name).WithControlPlane(true)
			})

			It("should return the client configuration", func() {
				clientConfig, err := manager.ClientConfig(ctx, t)
				Expect(err).NotTo(HaveOccurred())
				Expect(clientConfig.Namespace()).To(Equal(prod1GoldenShoot.Status.TechnicalID))
				rawConfig, err := clientConfig.RawConfig()
				Expect(err).NotTo(HaveOccurred())
				Expect(rawConfig.CurrentContext).To(Equal(*prod1GoldenShoot.Spec.SeedName))
			})
		})

		Context("when shoot is targeted", func() {
			BeforeEach(func() {
				t = target.NewTarget(gardenName, prod1Project.Name, "", prod1GoldenShoot.Name)
			})

			It("should return the client configuration", func() {
				clientConfig, err := manager.ClientConfig(ctx, t)
				Expect(err).NotTo(HaveOccurred())
				Expect(clientConfig.Namespace()).To(Equal("default"))
				rawConfig, err := clientConfig.RawConfig()
				Expect(err).NotTo(HaveOccurred())
				Expect(rawConfig.CurrentContext).To(Equal(prod1GoldenShoot.Name))
			})
		})
	})
})
