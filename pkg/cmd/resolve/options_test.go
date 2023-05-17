/*
SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package resolve_test

import (
	"context"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	clientgarden "github.com/gardener/gardenctl-v2/internal/client/garden"
	gardenclientmocks "github.com/gardener/gardenctl-v2/internal/client/garden/mocks"
	utilmocks "github.com/gardener/gardenctl-v2/internal/util/mocks"
	"github.com/gardener/gardenctl-v2/pkg/cmd/resolve"
	"github.com/gardener/gardenctl-v2/pkg/config"
	"github.com/gardener/gardenctl-v2/pkg/target"
	targetmocks "github.com/gardener/gardenctl-v2/pkg/target/mocks"
)

var _ = Describe("Resolve Command - Options", func() {
	const gardenName = "mygarden"
	var (
		ctrl         *gomock.Controller
		factory      *utilmocks.MockFactory
		garden       config.Garden
		gardenClient *gardenclientmocks.MockClient

		o *resolve.TestOptions
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		factory = utilmocks.NewMockFactory(ctrl)
		gardenClient = gardenclientmocks.NewMockClient(ctrl)

		garden = config.Garden{Name: gardenName}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("Complete", func() {
		var (
			manager *targetmocks.MockManager
			cfg     *config.Config
		)

		BeforeEach(func() {
			manager = targetmocks.NewMockManager(ctrl)

			factory.EXPECT().Manager().Return(manager, nil)

			cfg = &config.Config{
				LinkKubeconfig: pointer.Bool(false),
				Gardens:        []config.Garden{garden},
			}

			o = resolve.NewOptions(resolve.KindGarden)
		})

		It("should return an error if no garden is targeted", func() {
			t := target.NewTarget("", "", "", "")
			manager.EXPECT().CurrentTarget().Return(t, nil)

			err := o.Complete(factory, &cobra.Command{}, []string{})
			Expect(err).To(MatchError(target.ErrNoGardenTargeted))
		})

		It("should return an error if garden configuration is not found", func() {
			t := target.NewTarget("non-existing", "", "", "")
			manager.EXPECT().CurrentTarget().Return(t, nil)
			manager.EXPECT().Configuration().Return(cfg)

			err := o.Complete(factory, &cobra.Command{}, []string{})
			Expect(err).To(HaveOccurred())
		})

		It("should complete successfully", func() {
			t := target.NewTarget(gardenName, "", "", "")
			manager.EXPECT().CurrentTarget().Return(t, nil)
			manager.EXPECT().Configuration().Return(cfg)
			manager.EXPECT().GardenClient(gardenName).Return(gardenClient, nil)

			err := o.Complete(factory, &cobra.Command{}, []string{})
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("Validate", func() {
		BeforeEach(func() {
			o = resolve.NewOptions(resolve.KindGarden)
			o.Options.Output = "yaml"
		})

		It("should succeed", func() {
			Expect(o.Validate()).To(Succeed())
		})

		It("should fail if output is not set", func() {
			o.Options.Output = ""

			Expect(o.Validate()).To(MatchError("output must be 'yaml' or 'json'"))
		})
	})

	Describe("Run", func() {
		const (
			projectName = "myproject"
			seedName    = "myseed"
			soilName    = "mysoil"
			shootName   = "myshoot"
		)

		var (
			ctx           context.Context
			namespace     string
			project       *gardencorev1beta1.Project
			projectGarden *gardencorev1beta1.Project
			seed          *gardencorev1beta1.Seed
			soil          *gardencorev1beta1.Seed
			shoot         *gardencorev1beta1.Shoot
			shoot2        *gardencorev1beta1.Shoot

			kind resolve.Kind
		)

		BeforeEach(func() {
			ctx = context.Background()
			factory.EXPECT().Context().Return(ctx)

			namespace = "garden-" + projectName

			project = &gardencorev1beta1.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: projectName,
				},
				Spec: gardencorev1beta1.ProjectSpec{
					Namespace: pointer.String(namespace),
				},
			}

			projectGarden = &gardencorev1beta1.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: "garden",
				},
				Spec: gardencorev1beta1.ProjectSpec{
					Namespace: pointer.String("garden"),
				},
			}

			seed = &gardencorev1beta1.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Name: seedName,
				},
			}

			soil = &gardencorev1beta1.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Name: soilName,
				},
			}

			// shoot2 acts as shooted-seed
			shoot2 = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      seedName,
					Namespace: "garden",
				},
				Spec: gardencorev1beta1.ShootSpec{
					SeedName: pointer.String(soil.Name),
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
		})

		JustBeforeEach(func() {
			o = resolve.NewOptions(kind)
			o.Garden = &garden
			o.GardenClient = gardenClient
			o.Options.Output = "yaml"
		})

		Context("Resolve Garden", func() {
			BeforeEach(func() {
				kind = resolve.KindGarden
			})

			It("should succeed", func() {
				t := target.NewTarget(gardenName, "", "", "")
				o.CurrentTarget = t

				Expect(o.Run(factory)).To(Succeed())
				Expect(o.String()).To(Equal(`garden:
  name: mygarden
`))
			})
		})

		Context("Resolve Seed", func() {
			BeforeEach(func() {
				kind = resolve.KindSeed
			})

			Context("when garden and seed targeted", func() {
				It("should succeed", func() {
					t := target.NewTarget(gardenName, "", seedName, "")
					o.CurrentTarget = t

					gardenClient.EXPECT().GetSeed(ctx, seedName).Return(seed, nil)

					err := o.Run(factory)
					Expect(err).NotTo(HaveOccurred())
					Expect(o.String()).To(Equal(`garden:
  name: mygarden
seed:
  name: myseed
`))
				})
			})

			Context("when garden and shoot targeted", func() {
				It("should succeed", func() {
					t := target.NewTarget(gardenName, "", "", shootName)
					o.CurrentTarget = t

					gardenClient.EXPECT().FindShoot(ctx, t.AsListOption()).Return(shoot, nil)

					err := o.Run(factory)
					Expect(err).NotTo(HaveOccurred())
					Expect(o.String()).To(Equal(`garden:
  name: mygarden
seed:
  name: myseed
`))
				})
			})
		})

		Context("Resolve Project", func() {
			BeforeEach(func() {
				kind = resolve.KindProject
			})

			Context("when project is targeted", func() {
				It("should succeed", func() {
					t := target.NewTarget(gardenName, projectName, "", "")
					o.CurrentTarget = t

					gardenClient.EXPECT().GetProject(ctx, projectName).Return(project, nil)

					Expect(o.Run(factory)).To(Succeed())
					Expect(o.String()).To(Equal(`garden:
  name: mygarden
project:
  name: myproject
  namespace: garden-myproject
`))
				})
			})

			Context("when managed seed is targeted", func() {
				It("should succeed", func() {
					t := target.NewTarget(gardenName, "", seedName, "")
					o.CurrentTarget = t

					gardenClient.EXPECT().GetShootOfManagedSeed(ctx, seedName).Return(&seedmanagementv1alpha1.Shoot{Name: shoot2.Name}, nil)
					gardenClient.EXPECT().FindShoot(ctx, clientgarden.ProjectFilter{
						"metadata.name": seedName,
						"project":       "garden",
					}).Return(shoot2, nil)
					gardenClient.EXPECT().GetProjectByNamespace(ctx, "garden").Return(projectGarden, nil)

					Expect(o.Run(factory)).To(Succeed())
					Expect(o.String()).To(Equal(`garden:
  name: mygarden
project:
  name: garden
  namespace: garden
`))
				})
			})

			It("should fail if no project is targeted", func() {
				t := target.NewTarget(gardenName, "", "", "")
				o.CurrentTarget = t

				Expect(o.Run(factory)).To(MatchError(target.ErrNoProjectTargeted))
			})
		})

		Context("Resolve Shoot", func() {
			BeforeEach(func() {
				kind = resolve.KindShoot
			})

			Context("when garden and seed is targeted", func() {
				It("should succeed", func() {
					t := target.NewTarget(gardenName, "", seedName, "")
					o.CurrentTarget = t

					gardenClient.EXPECT().GetShootOfManagedSeed(ctx, seedName).Return(&seedmanagementv1alpha1.Shoot{Name: shoot2.Name}, nil)
					gardenClient.EXPECT().FindShoot(ctx, clientgarden.ProjectFilter{
						"metadata.name": seedName,
						"project":       "garden",
					}).Return(shoot2, nil)
					gardenClient.EXPECT().GetProjectByNamespace(ctx, "garden").Return(projectGarden, nil)

					Expect(o.Run(factory)).To(Succeed())
					Expect(o.String()).To(Equal(`garden:
  name: mygarden
project:
  name: garden
  namespace: garden
seed:
  name: mysoil
shoot:
  name: myseed
  namespace: garden
`))
				})
			})

			Context("when garden and shoot is targeted", func() {
				It("should succeed", func() {
					t := target.NewTarget(gardenName, "", "", shootName)
					o.CurrentTarget = t

					gardenClient.EXPECT().FindShoot(ctx, t.AsListOption()).Return(shoot, nil)
					gardenClient.EXPECT().GetProjectByNamespace(ctx, namespace).Return(project, nil)

					Expect(o.Run(factory)).To(Succeed())
					Expect(o.String()).To(Equal(`garden:
  name: mygarden
project:
  name: myproject
  namespace: garden-myproject
seed:
  name: myseed
shoot:
  name: myshoot
  namespace: garden-myproject
`))
				})

				It("should fail if no seed is assigned to shoot", func() {
					t := target.NewTarget(gardenName, "", "", shootName)
					o.CurrentTarget = t

					shoot.Spec.SeedName = nil

					gardenClient.EXPECT().FindShoot(ctx, t.AsListOption()).Return(shoot, nil)

					Expect(o.Run(factory)).To(MatchError("no seed assigned to shoot garden-myproject/myshoot"))
				})
			})

			Context("when garden, shoot and control-plane is targeted", func() {
				It("should succeed", func() {
					t := target.NewTarget(gardenName, "", "", shootName).WithControlPlane(true)
					o.CurrentTarget = t

					shoot2Target := target.NewTarget(gardenName, "garden", "", seedName).WithControlPlane(true)
					gardenClient.EXPECT().FindShoot(ctx, t.AsListOption()).Return(shoot, nil)
					gardenClient.EXPECT().FindShoot(ctx, shoot2Target.AsListOption()).Return(shoot2, nil)
					gardenClient.EXPECT().GetProjectByNamespace(ctx, "garden").Return(projectGarden, nil)

					Expect(o.Run(factory)).To(Succeed())
					Expect(o.String()).To(Equal(`garden:
  name: mygarden
project:
  name: garden
  namespace: garden
seed:
  name: mysoil
shoot:
  name: myseed
  namespace: garden
`))
				})
			})

			It("should fail if no shoot or seed is targeted", func() {
				t := target.NewTarget(gardenName, projectName, "", "")
				o.CurrentTarget = t

				Expect(o.Run(factory)).To(MatchError(target.ErrNoShootTargeted))
			})

			It("should fail if seed is not a managed seed", func() {
				t := target.NewTarget(gardenName, "", "seed", "")
				o.CurrentTarget = t

				gardenClient.EXPECT().GetShootOfManagedSeed(ctx, "seed").Return(nil, apierrors.NewNotFound(seedmanagementv1alpha1.Resource("managedseed"), "my-seed"))

				Expect(o.Run(factory)).To(MatchError(MatchRegexp("^seed is not a managed seed")))
			})
		})
	})
})
