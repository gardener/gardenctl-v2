/*
SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package resolve_test

import (
	"context"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	clientgarden "github.com/gardener/gardenctl-v2/internal/client/garden"
	"github.com/gardener/gardenctl-v2/internal/fake"
	"github.com/gardener/gardenctl-v2/internal/util"
	utilmocks "github.com/gardener/gardenctl-v2/internal/util/mocks"
	"github.com/gardener/gardenctl-v2/pkg/cmd/resolve"
	"github.com/gardener/gardenctl-v2/pkg/config"
	"github.com/gardener/gardenctl-v2/pkg/target"
	targetmocks "github.com/gardener/gardenctl-v2/pkg/target/mocks"
)

var _ = Describe("Resolve Command", func() {
	var (
		ctrl    *gomock.Controller
		factory *utilmocks.MockFactory
		manager *targetmocks.MockManager
		cmd     *cobra.Command
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		factory = utilmocks.NewMockFactory(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("given a ProviderEnv instance", func() {
		var (
			ctx     context.Context
			cfg     *config.Config
			streams util.IOStreams
			t       target.Target
			out     *util.SafeBytesBuffer

			namespace string
			project   *gardencorev1beta1.Project
			seed      *gardencorev1beta1.Seed
			shoot     *gardencorev1beta1.Shoot
		)

		BeforeEach(func() {
			ctx = context.Background()
			factory.EXPECT().Context().Return(ctx)

			manager = targetmocks.NewMockManager(ctrl)
			factory.EXPECT().Manager().Return(manager, nil).AnyTimes()

			t = target.NewTarget("test", "project", "seed", "shoot")
			manager.EXPECT().CurrentTarget().Return(t, nil)

			targetFlags := target.NewTargetFlags("", "", "", "", false)
			factory.EXPECT().TargetFlags().Return(targetFlags).AnyTimes()

			cfg = &config.Config{
				Gardens: []config.Garden{
					{
						Name:  t.GardenName(),
						Alias: "myalias",
					},
				},
			}
			manager.EXPECT().Configuration().Return(cfg)

			streams, _, out, _ = util.NewTestIOStreams()

			namespace = "garden-" + t.ProjectName()

			project = &gardencorev1beta1.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: t.ProjectName(),
				},
				Spec: gardencorev1beta1.ProjectSpec{
					Namespace: pointer.String(namespace),
				},
			}

			seed = &gardencorev1beta1.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Name: t.SeedName(),
				},
			}

			shoot = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      t.ShootName(),
					Namespace: namespace,
				},
				Spec: gardencorev1beta1.ShootSpec{
					SeedName: pointer.String(seed.Name),
				},
			}

			client := clientgarden.NewClient(
				nil,
				fake.NewClientWithObjects(project, seed, shoot),
				t.GardenName(),
			)
			manager.EXPECT().GardenClient(t.GardenName()).Return(client, nil)

			cmd = resolve.NewCmdResolve(factory, streams)
		})

		Context("Resolve Garden", func() {
			It("should succeed", func() {
				cmd.SetArgs([]string{"garden"})
				Expect(cmd.Execute()).To(Succeed())
				Expect(out.String()).To(Equal(`garden:
  alias: myalias
  name: test
`))
			})
		})

		Context("Resolve Shoot", func() {
			It("should succeed", func() {
				cmd.SetArgs([]string{"shoot"})
				Expect(cmd.Execute()).To(Succeed())
				Expect(out.String()).To(Equal(`garden:
  alias: myalias
  name: test
project:
  name: project
  namespace: garden-project
seed:
  name: seed
shoot:
  name: shoot
  namespace: garden-project
`))
			})
		})
	})
})
