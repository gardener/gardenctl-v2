/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package target_test

import (
	"fmt"
	"strings"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/utils/secrets"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	internalclient "github.com/gardener/gardenctl-v2/internal/client"
	clientmocks "github.com/gardener/gardenctl-v2/internal/client/mocks"
	"github.com/gardener/gardenctl-v2/internal/fake"
	"github.com/gardener/gardenctl-v2/pkg/config"
	"github.com/gardener/gardenctl-v2/pkg/target"
)

func assertTargetProvider(tp target.TargetProvider, expected target.Target) {
	t, err := tp.Read()
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	ExpectWithOffset(1, t).To(Equal(expected))
}

func assertClientConfig(clientConfig clientcmd.ClientConfig, name, ns string) {
	namespace, overridden, err := clientConfig.Namespace()
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	ExpectWithOffset(1, overridden).To(BeFalse())
	ExpectWithOffset(1, namespace).To(Equal(ns))

	rawConfig, err := clientConfig.RawConfig()
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	currentContext := rawConfig.CurrentContext
	ExpectWithOffset(1, currentContext).To(Equal(name))
	ExpectWithOffset(1, rawConfig.Contexts[currentContext].Namespace).To(Equal(namespace))
}

func createTestShoot(name string, namespace string, seedName *string) *gardencorev1beta1.Shoot {
	shoot := &gardencorev1beta1.Shoot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: gardencorev1beta1.ShootSpec{
			SeedName: seedName,
			Kubernetes: gardencorev1beta1.Kubernetes{
				Version: "1.20.0", // >= 1.20.0 for non-legacy shoot kubeconfigs
			},
		},
		Status: gardencorev1beta1.ShootStatus{
			AdvertisedAddresses: []gardencorev1beta1.ShootAdvertisedAddress{
				{
					Name: "shoot-address1",
					URL:  "https://api.bar.baz",
				},
			},
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

func createTestManager(t target.Target, cfg *config.Config, clientProvider internalclient.Provider) (target.Manager, target.TargetProvider) {
	targetProvider := fake.NewFakeTargetProvider(cloneTarget(t))

	manager, err := target.NewManager(cfg, targetProvider, clientProvider, sessionDir)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	ExpectWithOffset(1, manager).NotTo(BeNil())

	return manager, targetProvider
}

var _ = Describe("Target Manager", func() {
	var (
		ctrl                 *gomock.Controller
		prod1Project         *gardencorev1beta1.Project
		prod2Project         *gardencorev1beta1.Project
		unreadyProject       *gardencorev1beta1.Project
		seed                 *gardencorev1beta1.Seed
		seedKubeconfigSecret *corev1.Secret
		prod1GoldenShoot     *gardencorev1beta1.Shoot
		prod1AmbiguousShoot  *gardencorev1beta1.Shoot
		prod2AmbiguousShoot  *gardencorev1beta1.Shoot
		prod1PendingShoot    *gardencorev1beta1.Shoot
		cfg                  *config.Config
		gardenClient         client.Client
		clientProvider       *clientmocks.MockProvider
		namespace            *corev1.Namespace
	)

	BeforeEach(func() {
		cfg = &config.Config{
			LinkKubeconfig: pointer.Bool(false),
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

		testSeedKubeconfig, err := fake.NewConfigData("test-seed")
		Expect(err).ToNot(HaveOccurred())

		seedKubeconfigSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-seed.login",
				Namespace: "garden",
			},
			Data: map[string][]byte{
				"kubeconfig": testSeedKubeconfig,
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
		prod1AmbiguousShoot = createTestShoot("ambiguous-shoot", *prod1Project.Spec.Namespace, pointer.String(seed.Name))
		prod2AmbiguousShoot = createTestShoot("ambiguous-shoot", *prod2Project.Spec.Namespace, pointer.String(seed.Name))
		prod1PendingShoot = createTestShoot("pending-shoot", *prod1Project.Spec.Namespace, nil)

		csc := &secrets.CertificateSecretConfig{
			Name:       "ca-test",
			CommonName: "ca-test",
			CertType:   secrets.CACert,
		}
		ca, err := csc.GenerateCertificate()
		Expect(err).NotTo(HaveOccurred())

		prod1GoldenShootCaConfigMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      prod1GoldenShoot.Name + ".ca-cluster",
				Namespace: *prod1Project.Spec.Namespace,
			},
			Data: map[string]string{
				"ca.crt": string(ca.CertificatePEM),
			},
		}

		gardenClient = fake.NewClientWithObjects(
			prod1Project,
			prod2Project,
			unreadyProject,
			seed,
			seedKubeconfigSecret,
			prod1GoldenShoot,
			prod1AmbiguousShoot,
			prod2AmbiguousShoot,
			prod1PendingShoot,
			namespace,
			prod1GoldenShootCaConfigMap,
		)

		ctrl = gomock.NewController(GinkgoT())
		clientProvider = clientmocks.NewMockProvider(ctrl)
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

	Describe("#TargetMatchPattern", func() {
		var tf target.TargetFlags
		BeforeEach(func() {
			tf = target.NewTargetFlags("", "", "", "", false)
		})

		It("should be able to target valid garden, project and shoot by matching a pattern", func() {
			t := target.NewTarget("", "", "", "")
			manager, targetProvider := createTestManager(t, cfg, clientProvider)

			Expect(manager.TargetMatchPattern(ctx, tf, fmt.Sprintf("%s/shoot--%s--%s", gardenName, prod1Project.Name, prod1GoldenShoot.Name))).To(Succeed())
			assertTargetProvider(targetProvider, target.NewTarget(gardenName, prod1Project.Name, "", prod1GoldenShoot.Name))
		})

		It("should be able to target valid project shoot by matching a pattern if garden is set", func() {
			t := target.NewTarget(gardenName, "", "", "")
			manager, targetProvider := createTestManager(t, cfg, clientProvider)

			Expect(manager.TargetMatchPattern(ctx, tf, fmt.Sprintf("shoot--%s--%s", prod1Project.Name, prod1GoldenShoot.Name))).To(Succeed())
			assertTargetProvider(targetProvider, target.NewTarget(gardenName, prod1Project.Name, "", prod1GoldenShoot.Name))
		})

		It("should be able to target shoot by matching a pattern if garden is not set", func() {
			t := target.NewTarget("", "", "", "")
			manager, targetProvider := createTestManager(t, cfg, clientProvider)

			Expect(manager.TargetMatchPattern(ctx, tf, fmt.Sprintf("shoot--%s--%s", prod1Project.Name, prod1GoldenShoot.Name))).To(Succeed())
			assertTargetProvider(targetProvider, target.NewTarget(gardenName, prod1Project.Name, "", prod1GoldenShoot.Name))
		})

		It("should not target anything if target is not completely valid", func() {
			t := target.NewTarget(gardenName, "", "", "")
			manager, targetProvider := createTestManager(t, cfg, clientProvider)

			Expect(manager.TargetMatchPattern(ctx, tf, fmt.Sprintf("shoot--%s--%s", prod1Project.Name, "invalid shoot"))).NotTo(Succeed())
			assertTargetProvider(targetProvider, target.NewTarget(gardenName, "", "", ""))
		})

		It("should be able to target valid project by matching a pattern containing a namespace", func() {
			t := target.NewTarget(gardenName, "", "", "")
			manager, targetProvider := createTestManager(t, cfg, clientProvider)

			Expect(manager.TargetMatchPattern(ctx, tf, fmt.Sprintf("namespace:%s", *prod1Project.Spec.Namespace))).To(Succeed())
			assertTargetProvider(targetProvider, target.NewTarget(gardenName, prod1Project.Name, "", ""))
		})
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
		clientProvider.EXPECT().FromClientConfig(gomock.Any()).Return(shootClient, nil)

		newClient, err := manager.ShootClient(ctx, t)
		Expect(err).NotTo(HaveOccurred())
		Expect(newClient).NotTo(BeNil())
	})

	It("should be able to unset selected garden", func() {
		t := target.NewTarget(gardenName, "", "", "")
		manager, targetProvider := createTestManager(t, cfg, clientProvider)

		Expect(manager.UnsetTargetGarden(ctx)).Should(Equal(gardenName))
		assertTargetProvider(targetProvider, target.NewTarget("", "", "", ""))
	})

	It("should fail if no garden selected", func() {
		t := target.NewTarget("", "", "", "")
		manager, targetProvider := createTestManager(t, cfg, clientProvider)

		res, unsetErr := manager.UnsetTargetGarden(ctx)
		Expect(unsetErr).To(HaveOccurred())
		Expect(res).To(BeEmpty())
		assertTargetProvider(targetProvider, t)
	})

	It("should unset deeper target levels when unsetting garden", func() {
		t := target.NewTarget(gardenName, prod1Project.Name, seed.Name, prod1AmbiguousShoot.Name)
		manager, targetProvider := createTestManager(t, cfg, clientProvider)

		// Unset Garden
		Expect(manager.UnsetTargetGarden(ctx)).Should(Equal(gardenName))

		// should also unset project, seed and shoot
		assertTargetProvider(targetProvider, target.NewTarget("", "", "", ""))
	})

	It("should be able to unset selected project", func() {
		t := target.NewTarget(gardenName, prod1Project.Name, "", "")
		manager, targetProvider := createTestManager(t, cfg, clientProvider)

		Expect(manager.UnsetTargetProject(ctx)).Should(Equal(prod1Project.Name))
		assertTargetProvider(targetProvider, target.NewTarget(gardenName, "", "", ""))
	})

	It("should fail if no project selected", func() {
		t := target.NewTarget(gardenName, "", "", "")
		manager, targetProvider := createTestManager(t, cfg, clientProvider)

		res, unsetErr := manager.UnsetTargetProject(ctx)
		Expect(unsetErr).To(HaveOccurred())
		Expect(res).To(BeEmpty())
		assertTargetProvider(targetProvider, t)
	})

	It("should unset deeper target levels when unsetting project", func() {
		t := target.NewTarget(gardenName, prod1Project.Name, "", prod1AmbiguousShoot.Name).WithControlPlane(true)
		manager, targetProvider := createTestManager(t, cfg, clientProvider)

		// Unset Project
		Expect(manager.UnsetTargetProject(ctx)).Should(Equal(prod1Project.Name))

		// should also unset shoot
		assertTargetProvider(targetProvider, target.NewTarget(gardenName, "", "", ""))
	})

	It("should be able to unset selected shoot", func() {
		t := target.NewTarget(gardenName, prod1Project.Name, "", prod1AmbiguousShoot.Name)
		manager, targetProvider := createTestManager(t, cfg, clientProvider)

		Expect(manager.UnsetTargetShoot(ctx)).Should(Equal(prod1AmbiguousShoot.Name))
		assertTargetProvider(targetProvider, target.NewTarget(gardenName, prod1Project.Name, "", ""))
	})

	It("should fail if no shoot selected", func() {
		t := target.NewTarget(gardenName, prod1Project.Name, "", "")
		manager, targetProvider := createTestManager(t, cfg, clientProvider)

		res, unsetErr := manager.UnsetTargetShoot(ctx)
		Expect(unsetErr).To(HaveOccurred())
		Expect(res).To(BeEmpty())
		assertTargetProvider(targetProvider, t)
	})

	It("should be able to unset selected control plane", func() {
		t := target.NewTarget(gardenName, prod1Project.Name, "", prod1AmbiguousShoot.Name).WithControlPlane(true)
		manager, targetProvider := createTestManager(t, cfg, clientProvider)

		Expect(manager.UnsetTargetControlPlane(ctx)).To(Succeed())
		assertTargetProvider(targetProvider, t.WithControlPlane(false))
	})

	It("should fail if no control plane targeted", func() {
		t := target.NewTarget(gardenName, prod1Project.Name, "", "")
		manager, targetProvider := createTestManager(t, cfg, clientProvider)

		unsetErr := manager.UnsetTargetControlPlane(ctx)
		Expect(unsetErr).To(HaveOccurred())
		assertTargetProvider(targetProvider, t)
	})

	It("should be able to unset selected seed", func() {
		t := target.NewTarget(gardenName, "", seed.Name, "")
		manager, targetProvider := createTestManager(t, cfg, clientProvider)

		Expect(manager.UnsetTargetSeed(ctx)).Should(Equal(seed.Name))
		assertTargetProvider(targetProvider, target.NewTarget(gardenName, "", "", ""))
	})

	It("should fail if no seed selected", func() {
		t := target.NewTarget(gardenName, prod1Project.Name, "", "")
		manager, targetProvider := createTestManager(t, cfg, clientProvider)

		res, unsetErr := manager.UnsetTargetSeed(ctx)
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
				assertClientConfig(clientConfig, *prod1GoldenShoot.Spec.SeedName, prod1GoldenShoot.Status.TechnicalID)
			})
		})

		Context("when shoot is targeted", func() {
			BeforeEach(func() {
				t = target.NewTarget(gardenName, prod1Project.Name, "", prod1GoldenShoot.Name)
			})

			It("should return the client configuration", func() {
				clientConfig, err := manager.ClientConfig(ctx, t)
				Expect(err).NotTo(HaveOccurred())

				currentContextName := prod1GoldenShoot.Namespace + "--" + prod1GoldenShoot.Name + "-" + prod1GoldenShoot.Status.AdvertisedAddresses[0].Name
				assertClientConfig(clientConfig, currentContextName, "default")
			})
		})

		Context("when seed is targeted", func() {
			BeforeEach(func() {
				t = target.NewTarget(gardenName, prod1Project.Name, seed.Name, "")
			})

			It("should return the client configuration", func() {
				clientConfig, err := manager.ClientConfig(ctx, t)
				Expect(err).NotTo(HaveOccurred())
				assertClientConfig(clientConfig, seed.Name, "default")
			})
		})

		Context("when project is targeted", func() {
			BeforeEach(func() {
				t = target.NewTarget(gardenName, prod1Project.Name, "", "")
			})

			It("should return the client configuration", func() {
				clientConfig, err := manager.ClientConfig(ctx, t)
				Expect(err).NotTo(HaveOccurred())
				assertClientConfig(clientConfig, gardenName, *prod1Project.Spec.Namespace)
			})
		})

		Context("when garden is targeted", func() {
			BeforeEach(func() {
				t = target.NewTarget(gardenName, "", "", "")
			})

			It("should return the client configuration", func() {
				clientConfig, err := manager.ClientConfig(ctx, t)
				Expect(err).NotTo(HaveOccurred())
				assertClientConfig(clientConfig, gardenName, "default")
			})
		})
	})

	Context("FlagCompletors", func() {
		BeforeEach(func() {
			cfg = &config.Config{
				LinkKubeconfig: pointer.Bool(false),
				Gardens: []config.Garden{
					{
						Name:       "garden1",
						Kubeconfig: gardenKubeconfig,
					},
					{
						Name:       "garden2",
						Kubeconfig: gardenKubeconfig,
					},
					{
						Name:       gardenName,
						Kubeconfig: gardenKubeconfig,
					},
				},
			}
		})

		Describe("List garden names", func() {
			It("should return the list of gardens from the configuration file", func() {
				t := target.NewTarget("", "", "", "")
				manager, _ := createTestManager(t, cfg, clientProvider)
				gardenNames, err := manager.GardenNames()

				Expect(err).To(Succeed(), "GardenNames should not error")
				Expect(gardenNames).To(HaveLen(3))
				Expect(gardenNames[0]).To(Equal(cfg.Gardens[0].Name))
				Expect(gardenNames[1]).To(Equal(cfg.Gardens[1].Name))
				Expect(gardenNames[2]).To(Equal(cfg.Gardens[2].Name))
			})
		})

		Describe("List project names", func() {
			It("should return the list of projects for the current garden", func() {
				t := target.NewTarget(gardenName, "", "", "")
				manager, _ := createTestManager(t, cfg, clientProvider)
				projectNames, err := manager.ProjectNames(ctx)

				Expect(err).To(Succeed())
				Expect(projectNames).To(HaveLen(3))
				Expect(projectNames[0]).To(Equal(prod1Project.Name))
			})

			It("should fail if no garden is targeted", func() {
				t := target.NewTarget("", "", "", "")
				manager, _ := createTestManager(t, cfg, clientProvider)
				projectNames, err := manager.ProjectNames(ctx)

				Expect(err).ToNot(Succeed())
				Expect(projectNames).To(BeNil())
			})
		})

		Describe("List seed names", func() {
			It("should return the list of seeds for the current garden", func() {
				t := target.NewTarget(gardenName, "", "", "")
				manager, _ := createTestManager(t, cfg, clientProvider)
				seedNames, err := manager.SeedNames(ctx)

				Expect(err).To(Succeed())
				Expect(seedNames).To(HaveLen(1))
				Expect(seedNames[0]).To(Equal(seed.Name))
			})

			It("should fail if no garden is targeted", func() {
				t := target.NewTarget("", "", "", "")
				manager, _ := createTestManager(t, cfg, clientProvider)
				seedNames, err := manager.SeedNames(ctx)

				Expect(err).ToNot(Succeed())
				Expect(seedNames).To(BeNil())
			})
		})

		Describe("List shoot names", func() {
			It("should return the list of shoots for the current garden", func() {
				t := target.NewTarget(gardenName, "", "", "")
				manager, _ := createTestManager(t, cfg, clientProvider)
				shootNames, err := manager.ShootNames(ctx)

				Expect(err).To(Succeed())
				Expect(shootNames).To(HaveLen(3))
				Expect(shootNames[0]).To(Equal(prod1AmbiguousShoot.Name))
				Expect(shootNames[1]).To(Equal(prod1GoldenShoot.Name))
				Expect(shootNames[2]).To(Equal(prod1PendingShoot.Name))
			})

			It("should fail if no garden is targeted", func() {
				t := target.NewTarget("", "", "", "")
				manager, _ := createTestManager(t, cfg, clientProvider)
				shootNames, err := manager.ShootNames(ctx)

				Expect(err).ToNot(Succeed())
				Expect(shootNames).To(BeNil())
			})
		})
	})
})
