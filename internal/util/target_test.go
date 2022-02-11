/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package util_test

import (
	"context"
	"fmt"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"

	"github.com/gardener/gardenctl-v2/internal/fake"
	"github.com/gardener/gardenctl-v2/internal/gardenclient"
	. "github.com/gardener/gardenctl-v2/internal/util"
	utilMocks "github.com/gardener/gardenctl-v2/internal/util/mocks"
	"github.com/gardener/gardenctl-v2/pkg/config"
	"github.com/gardener/gardenctl-v2/pkg/target"
	targetMocks "github.com/gardener/gardenctl-v2/pkg/target/mocks"
)

var _ = Describe("Target Utilities", func() {
	var (
		testReadyProject   *gardencorev1beta1.Project
		testUnreadyProject *gardencorev1beta1.Project
		testSeed           *gardencorev1beta1.Seed
		testOtherSeed      *gardencorev1beta1.Seed
		testShoot          *gardencorev1beta1.Shoot
		testOtherShoot     *gardencorev1beta1.Shoot
		gardenClient       gardenclient.Client
		ctrl               *gomock.Controller
		mockFactory        *utilMocks.MockFactory
		mockManager        *targetMocks.MockManager
		cfg                *config.Config
		ctx                context.Context
		gardenName1        string
		gardenName2        string
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

		testSeed = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-seed",
			},
		}

		testOtherSeed = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				Name: "A-test-seed",
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

		testOtherShoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "a-test-shoot",
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

		gardenClient = gardenclient.NewGardenClient(fake.NewClientWithObjects(
			testReadyProject,
			testUnreadyProject,
			testSeed,
			testOtherSeed,
			testShootKubeconfig,
			testShoot,
			testOtherShoot,
		))

		cfg = &config.Config{
			Gardens: []config.Garden{{
				Name:       "foo",
				Kubeconfig: "/not/a/real/garden-foo/kubeconfig",
			}, {
				Name:       "bar",
				Kubeconfig: "/not/a/real/garden-bar/kubeconfig",
			}},
		}

		gardenName1 = cfg.Gardens[0].Name
		gardenName2 = cfg.Gardens[1].Name

		ctx = context.Background()

		ctrl = gomock.NewController(GinkgoT())
		mockFactory = utilMocks.NewMockFactory(ctrl)
		mockManager = targetMocks.NewMockManager(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("SeedForTarget", func() {
		It("should require a targeted seed", func() {
			t := target.NewTarget("a", "", "", "")
			ctx := context.Background()

			seed, err := SeedForTarget(ctx, gardenClient, t)
			Expect(seed).To(BeNil())
			Expect(err).To(HaveOccurred())
		})

		It("should return a valid seed", func() {
			t := target.NewTarget("a", "", testSeed.Name, "")
			ctx := context.Background()

			seed, err := SeedForTarget(ctx, gardenClient, t)
			Expect(err).NotTo(HaveOccurred())
			Expect(seed).NotTo(BeNil())
			Expect(seed.Name).To(Equal(testSeed.Name))
		})

		It("should require a targeted project", func() {
			t := target.NewTarget("a", "", "", "")
			ctx := context.Background()

			project, err := ProjectForTarget(ctx, gardenClient, t)
			Expect(project).To(BeNil())
			Expect(err).To(HaveOccurred())
		})

		It("should return a valid project", func() {
			t := target.NewTarget("a", testReadyProject.Name, "", "")
			ctx := context.Background()

			project, err := ProjectForTarget(ctx, gardenClient, t)
			Expect(err).NotTo(HaveOccurred())
			Expect(project).NotTo(BeNil())
			Expect(project.Name).To(Equal(testReadyProject.Name))
		})

		It("should return an unready project", func() {
			t := target.NewTarget("a", testUnreadyProject.Name, "", "")
			ctx := context.Background()

			project, err := ProjectForTarget(ctx, gardenClient, t)
			Expect(err).NotTo(HaveOccurred())
			Expect(project).NotTo(BeNil())
			Expect(project.Name).To(Equal(testUnreadyProject.Name))
		})

		It("should return a valid shoot when not using a project or seed", func() {
			t := target.NewTarget("a", "", "", testShoot.Name)
			ctx := context.Background()

			shoot, err := ShootForTarget(ctx, gardenClient, t)
			Expect(err).NotTo(HaveOccurred())
			Expect(shoot).NotTo(BeNil())
			Expect(shoot.Name).To(Equal(testShoot.Name))
		})

		It("should return a valid shoot when using a project", func() {
			t := target.NewTarget("a", testReadyProject.Name, "", testShoot.Name)
			ctx := context.Background()

			shoot, err := ShootForTarget(ctx, gardenClient, t)
			Expect(err).NotTo(HaveOccurred())
			Expect(shoot).NotTo(BeNil())
			Expect(shoot.Name).To(Equal(testShoot.Name))
		})

		It("should return an error when using an unready project", func() {
			t := target.NewTarget("a", testUnreadyProject.Name, "", testShoot.Name)
			ctx := context.Background()

			shoot, err := ShootForTarget(ctx, gardenClient, t)
			Expect(shoot).To(BeNil())
			Expect(err).To(HaveOccurred())
		})

		It("should return a valid shoot when using a seed", func() {
			t := target.NewTarget("a", "", testSeed.Name, testShoot.Name)
			ctx := context.Background()

			shoot, err := ShootForTarget(ctx, gardenClient, t)
			Expect(err).NotTo(HaveOccurred())
			Expect(shoot).NotTo(BeNil())
			Expect(shoot.Name).To(Equal(testShoot.Name))
		})
	})

	Describe("GardenNames", func() {
		BeforeEach(func() {
			mockManager.EXPECT().Configuration().Return(cfg)
			mockFactory.EXPECT().Manager().Return(mockManager, nil)
		})

		It("should return all garden names, alphabetically sorted", func() {
			manager, err := mockFactory.Manager()
			Expect(err).NotTo(HaveOccurred())

			values, err := GardenNames(manager)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal([]string{gardenName2, gardenName1}))
		})
	})

	Describe("ProjectNamesForTarget", func() {
		BeforeEach(func() {
			mockManager.EXPECT().GardenClient(gardenName1).Return(gardenClient, nil)
			mockFactory.EXPECT().Manager().Return(mockManager, nil)
			mockFactory.EXPECT().Context().Return(ctx)
			mockManager.EXPECT().CurrentTarget().Return(target.NewTarget(gardenName1, "", "", ""), nil)
		})

		It("should return all project names for first garden, alphabetically sorted", func() {
			manager, err := mockFactory.Manager()
			Expect(err).NotTo(HaveOccurred())

			values, err := ProjectNamesForTarget(mockFactory.Context(), manager)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal([]string{testReadyProject.Name, testUnreadyProject.Name}))
		})
	})

	Describe("SeedNamesForTarget", func() {
		BeforeEach(func() {
			mockManager.EXPECT().GardenClient(gardenName1).Return(gardenClient, nil)
			mockFactory.EXPECT().Manager().Return(mockManager, nil)
			mockFactory.EXPECT().Context().Return(ctx)
			mockManager.EXPECT().CurrentTarget().Return(target.NewTarget(gardenName1, "", "", ""), nil)
		})

		It("should return all seed names for first garden, alphabetically sorted", func() {
			manager, err := mockFactory.Manager()
			Expect(err).NotTo(HaveOccurred())

			values, err := SeedNamesForTarget(mockFactory.Context(), manager)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal([]string{testOtherSeed.Name, testSeed.Name}))
		})
	})

	Describe("ShootNamesForTarget", func() {
		BeforeEach(func() {
			mockManager.EXPECT().GardenClient(gardenName1).Return(gardenClient, nil)
			mockFactory.EXPECT().Manager().Return(mockManager, nil)
			mockFactory.EXPECT().Context().Return(ctx)
			mockManager.EXPECT().CurrentTarget().Return(target.NewTarget(gardenName1, "", "", ""), nil)
		})

		It("should return all shoot names for first project, alphabetically sorted", func() {
			manager, err := mockFactory.Manager()
			Expect(err).NotTo(HaveOccurred())

			values, err := ShootNamesForTarget(mockFactory.Context(), manager)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal([]string{testOtherShoot.Name, testShoot.Name}))
		})
	})
})
