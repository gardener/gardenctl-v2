/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package target_test

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	internalfake "github.com/gardener/gardenctl-v2/internal/fake"
	"github.com/gardener/gardenctl-v2/internal/util"
	cmdtarget "github.com/gardener/gardenctl-v2/pkg/cmd/target"
	"github.com/gardener/gardenctl-v2/pkg/config"
	"github.com/gardener/gardenctl-v2/pkg/target"
	targetmocks "github.com/gardener/gardenctl-v2/pkg/target/mocks"
)

var _ = Describe("Target Command", func() {
	const (
		gardenName       = "mygarden"
		gardenKubeconfig = "/not/a/real/file"
		projectName      = "myproject"
		seedName         = "myseed"
		shootName        = "myshoot"
		namespace        = "garden"
	)

	var (
		streams        util.IOStreams
		out            *util.SafeBytesBuffer
		ctrl           *gomock.Controller
		cfg            *config.Config
		clientProvider *targetmocks.MockClientProvider
		gardenClient   client.Client
		targetProvider *internalfake.TargetProvider
		factory        *internalfake.Factory
		project        *gardencorev1beta1.Project
		seed           *gardencorev1beta1.Seed
		shoot          *gardencorev1beta1.Shoot
	)

	BeforeEach(func() {
		cfg = &config.Config{
			Gardens: []config.Garden{{
				Name:       gardenName,
				Kubeconfig: gardenKubeconfig,
			}, {
				Name:       "another-garden",
				Kubeconfig: gardenKubeconfig,
			}},
		}

		// garden cluster contains the targeted project
		project = &gardencorev1beta1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name: projectName,
			},
			Spec: gardencorev1beta1.ProjectSpec{
				Namespace: pointer.String(namespace),
			},
		}

		// garden cluster contains the targeted seed
		seed = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				Name: seedName,
			},
		}

		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      shootName,
				Namespace: namespace,
			},
			Spec: gardencorev1beta1.ShootSpec{
				SeedName: pointer.String(seed.Name),
			},
		}

		streams, _, out, _ = util.NewTestIOStreams()

		ctrl = gomock.NewController(GinkgoT())

		clientProvider = targetmocks.NewMockClientProvider(ctrl)
		targetProvider = internalfake.NewFakeTargetProvider(target.NewTarget("", "", "", ""))
		factory = internalfake.NewFakeFactory(cfg, nil, clientProvider, targetProvider)
	})

	JustBeforeEach(func() {
		clientConfig, err := cfg.ClientConfig(gardenName)
		Expect(err).ToNot(HaveOccurred())
		clientProvider.EXPECT().FromClientConfig(gomock.Eq(clientConfig)).Return(gardenClient, nil).AnyTimes()
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("RunE", func() {
		BeforeEach(func() {
			gardenClient = internalfake.NewClientWithObjects(project, seed, shoot)
		})

		It("should reject bad options", func() {
			cmd := cmdtarget.NewCmdTarget(factory, cmdtarget.NewTargetOptions(streams))

			Expect(cmd.RunE(cmd, nil)).NotTo(Succeed())
		})

		It("should be able to target a garden", func() {
			cmd := cmdtarget.NewCmdTarget(factory, cmdtarget.NewTargetOptions(streams))

			Expect(cmd.RunE(cmd, []string{"garden", gardenName})).To(Succeed())
			Expect(out.String()).To(ContainSubstring("Successfully targeted garden %q\n", gardenName))

			currentTarget, err := targetProvider.Read()
			Expect(err).NotTo(HaveOccurred())
			Expect(currentTarget.GardenName()).To(Equal(gardenName))
		})

		It("should be able to target a project", func() {
			// user has already targeted a garden
			targetProvider.Target = target.NewTarget(gardenName, "", "", "")
			cmd := cmdtarget.NewCmdTarget(factory, cmdtarget.NewTargetOptions(streams))

			// run command
			Expect(cmd.RunE(cmd, []string{"project", projectName})).To(Succeed())
			Expect(out.String()).To(ContainSubstring("Successfully targeted project %q\n", projectName))

			currentTarget, err := targetProvider.Read()
			Expect(err).NotTo(HaveOccurred())
			Expect(currentTarget.GardenName()).To(Equal(gardenName))
			Expect(currentTarget.ProjectName()).To(Equal(projectName))
		})

		It("should be able to target a seed", func() {
			// user has already targeted a garden
			targetProvider.Target = target.NewTarget(gardenName, "", "", "")
			cmd := cmdtarget.NewCmdTarget(factory, cmdtarget.NewTargetOptions(streams))

			// run command
			Expect(cmd.RunE(cmd, []string{"seed", seedName})).To(Succeed())
			Expect(out.String()).To(ContainSubstring("Successfully targeted seed %q\n", seedName))

			currentTarget, err := targetProvider.Read()
			Expect(err).NotTo(HaveOccurred())
			Expect(currentTarget.GardenName()).To(Equal(gardenName))
			Expect(currentTarget.SeedName()).To(Equal(seedName))
		})

		It("should be able to target a shoot", func() {
			// user has already targeted a garden and project
			targetProvider.Target = target.NewTarget(gardenName, projectName, "", "")
			cmd := cmdtarget.NewCmdTarget(factory, cmdtarget.NewTargetOptions(streams))

			// run command
			Expect(cmd.RunE(cmd, []string{"shoot", shootName})).To(Succeed())
			Expect(out.String()).To(ContainSubstring("Successfully targeted shoot %q\n", shootName))

			currentTarget, err := targetProvider.Read()
			Expect(err).NotTo(HaveOccurred())
			Expect(currentTarget.GardenName()).To(Equal(gardenName))
			Expect(currentTarget.ProjectName()).To(Equal(projectName))
			Expect(currentTarget.SeedName()).To(BeEmpty())
			Expect(currentTarget.ShootName()).To(Equal(shootName))
		})

		It("should be able to target a control plane", func() {
			// user has already targeted a garden, project and shoot
			targetProvider.Target = target.NewTarget(gardenName, projectName, "", shootName)
			cmd := cmdtarget.NewCmdTarget(factory, cmdtarget.NewTargetOptions(streams))

			// run command
			Expect(cmd.RunE(cmd, []string{"control-plane"})).To(Succeed())
			Expect(out.String()).To(ContainSubstring("Successfully targeted control plane of shoot %q\n", shootName))

			currentTarget, err := targetProvider.Read()
			Expect(err).NotTo(HaveOccurred())
			Expect(currentTarget.GardenName()).To(Equal(gardenName))
			Expect(currentTarget.ProjectName()).To(Equal(projectName))
			Expect(currentTarget.SeedName()).To(BeEmpty())
			Expect(currentTarget.ShootName()).To(Equal(shootName))
			Expect(currentTarget.ControlPlane()).To(BeTrue())
		})
	})

	Describe("Completion", func() {
		var (
			testProject1 *gardencorev1beta1.Project
			testProject2 *gardencorev1beta1.Project
			testSeed1    *gardencorev1beta1.Seed
			testSeed2    *gardencorev1beta1.Seed
			testShoot1   *gardencorev1beta1.Shoot
			testShoot2   *gardencorev1beta1.Shoot
		)

		BeforeEach(func() {
			testProject1 = &gardencorev1beta1.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: "prod1",
				},
				Spec: gardencorev1beta1.ProjectSpec{
					Namespace: pointer.String("garden-prod1"),
				},
			}

			testProject2 = &gardencorev1beta1.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: "prod2",
				},
				Spec: gardencorev1beta1.ProjectSpec{
					Namespace: pointer.String("garden-prod2"),
				},
			}

			testSeed1 = &gardencorev1beta1.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-seed1",
				},
			}

			testSeed2 = &gardencorev1beta1.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Name: "aws-seed",
				},
			}

			testShoot1 = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-shoot",
					Namespace: *testProject1.Spec.Namespace,
				},
				Spec: gardencorev1beta1.ShootSpec{
					SeedName: pointer.String(testSeed1.Name),
				},
			}

			testShoot2 = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-shoot",
					Namespace: *testProject1.Spec.Namespace,
				},
				Spec: gardencorev1beta1.ShootSpec{
					SeedName: pointer.String(testSeed1.Name),
				},
			}

			gardenClient = internalfake.NewClientWithObjects(
				testProject1,
				testProject2,
				testSeed1,
				testSeed2,
				testShoot1,
				testShoot2,
			)
		})

		Describe("ValidTargetArgsFunction", func() {
			It("should return the allowed target types when no kind was given", func() {
				values, err := cmdtarget.ValidTargetArgsFunction(factory, nil, nil, "")
				Expect(err).NotTo(HaveOccurred())
				Expect(values).To(Equal([]string{
					string(cmdtarget.TargetKindGarden),
					string(cmdtarget.TargetKindProject),
					string(cmdtarget.TargetKindSeed),
					string(cmdtarget.TargetKindShoot),
					string(cmdtarget.TargetKindPattern),
					string(cmdtarget.TargetKindControlPlane),
				}))
			})

			It("should reject invalid kinds", func() {
				_, err := cmdtarget.ValidTargetArgsFunction(factory, nil, []string{"invalid"}, "")
				Expect(err).To(HaveOccurred())
			})

			It("should return all garden names", func() {
				values, err := cmdtarget.ValidTargetArgsFunction(factory, nil, []string{string(cmdtarget.TargetKindGarden)}, "")
				Expect(err).NotTo(HaveOccurred())
				Expect(values).To(Equal([]string{cfg.Gardens[1].Name, gardenName}))
			})

			It("should return all project names", func() {
				targetProvider.Target = target.NewTarget(gardenName, "", "", "")

				values, err := cmdtarget.ValidTargetArgsFunction(factory, nil, []string{string(cmdtarget.TargetKindProject)}, "")
				Expect(err).NotTo(HaveOccurred())
				Expect(values).To(Equal([]string{testProject1.Name, testProject2.Name}))
			})

			It("should return all seed names", func() {
				targetProvider.Target = target.NewTarget(gardenName, "", "", "")

				values, err := cmdtarget.ValidTargetArgsFunction(factory, nil, []string{string(cmdtarget.TargetKindSeed)}, "")
				Expect(err).NotTo(HaveOccurred())
				Expect(values).To(Equal([]string{testSeed2.Name, testSeed1.Name}))
			})

			It("should return all shoot names when using a project", func() {
				targetProvider.Target = target.NewTarget(gardenName, testProject1.Name, "", "")

				values, err := cmdtarget.ValidTargetArgsFunction(factory, nil, []string{string(cmdtarget.TargetKindShoot)}, "")
				Expect(err).NotTo(HaveOccurred())
				Expect(values).To(Equal([]string{testShoot2.Name, testShoot1.Name}))
			})

			It("should return all shoot names when using a seed", func() {
				targetProvider.Target = target.NewTarget(gardenName, "", testSeed1.Name, "")

				values, err := cmdtarget.ValidTargetArgsFunction(factory, nil, []string{string(cmdtarget.TargetKindShoot)}, "")
				Expect(err).NotTo(HaveOccurred())
				Expect(values).To(Equal([]string{testShoot2.Name, testShoot1.Name}))
			})
		})
	})
})

var _ = Describe("Target Options", func() {
	It("should validate", func() {
		streams, _, _, _ := util.NewTestIOStreams()
		o := cmdtarget.NewTargetOptions(streams)
		o.Kind = cmdtarget.TargetKindGarden
		o.TargetName = "foo"

		Expect(o.Validate()).To(Succeed())
	})

	It("should reject invalid kinds", func() {
		streams, _, _, _ := util.NewTestIOStreams()
		o := cmdtarget.NewTargetOptions(streams)
		o.Kind = cmdtarget.TargetKind("not a kind")
		o.TargetName = "foo"

		Expect(o.Validate()).NotTo(Succeed())
	})
})
