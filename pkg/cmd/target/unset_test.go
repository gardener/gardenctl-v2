/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package target_test

import (
	"context"

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
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

var _ = Describe("Target Unset Command", func() {
	const (
		gardenName       = "mygarden"
		gardenKubeconfig = "/not/a/real/file"
		projectName      = "myproject"
		seedName         = "myseed"
		shootName        = "myshoot"
		namespace        = "garden-prod1"
	)

	var (
		ctrl           *gomock.Controller
		cfg            *config.Config
		clientProvider *targetmocks.MockClientProvider
		gardenClient   client.Client
		streams        util.IOStreams
		out            *util.SafeBytesBuffer
		ctx            context.Context
		factory        *internalfake.Factory
		targetProvider *internalfake.TargetProvider
		currentTarget  target.Target
		project        *gardencorev1beta1.Project
		seed           *gardencorev1beta1.Seed
		shoot          *gardencorev1beta1.Shoot
	)

	BeforeEach(func() {
		cfg = &config.Config{
			Gardens: []config.Garden{{
				Name:       gardenName,
				Kubeconfig: gardenKubeconfig,
			}},
		}

		project = &gardencorev1beta1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name: projectName,
			},
			Spec: gardencorev1beta1.ProjectSpec{
				Namespace: pointer.String(namespace),
			},
		}

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
		}

		ctrl = gomock.NewController(GinkgoT())

		streams, _, out, _ = util.NewTestIOStreams()

		currentTarget = target.NewTarget(gardenName, "", "", "")

		clientProvider = targetmocks.NewMockClientProvider(ctrl)
		gardenClient = internalfake.NewClientWithObjects(project, seed, shoot)
		clientConfig, err := cfg.ClientConfig(gardenName)
		Expect(err).ToNot(HaveOccurred())
		clientProvider.EXPECT().FromClientConfig(gomock.Eq(clientConfig)).Return(gardenClient, nil).AnyTimes()

		factory = internalfake.NewFakeFactory(cfg, nil, clientProvider, targetProvider)
		ctx = context.Background()
	})

	JustBeforeEach(func() {
		targetProvider = internalfake.NewFakeTargetProvider(currentTarget)

		factory = internalfake.NewFakeFactory(cfg, nil, clientProvider, targetProvider)
		factory.ContextImpl = ctx
	})

	It("should reject bad options", func() {
		o := cmdtarget.NewUnsetOptions(streams)
		cmd := cmdtarget.NewCmdUnset(&util.FactoryImpl{}, o)

		Expect(cmd.RunE(cmd, nil)).NotTo(Succeed())
	})

	It("should be able to unset a targeted garden", func() {
		// user has already targeted a garden
		cmd := cmdtarget.NewCmdUnset(factory, cmdtarget.NewUnsetOptions(streams))

		Expect(cmd.RunE(cmd, []string{"garden"})).To(Succeed())
		Expect(out.String()).To(ContainSubstring("Successfully unset targeted garden %q\n", gardenName))

		currentTarget, err := targetProvider.Read()
		Expect(err).NotTo(HaveOccurred())
		Expect(currentTarget.GardenName()).To(BeEmpty())
	})

	It("should be able to unset a targeted project", func() {
		// user has already targeted a garden and project
		targetProvider.Target = currentTarget.WithProjectName(projectName)
		cmd := cmdtarget.NewCmdUnset(factory, cmdtarget.NewUnsetOptions(streams))

		// run command
		Expect(cmd.RunE(cmd, []string{"project"})).To(Succeed())
		Expect(out.String()).To(ContainSubstring("Successfully unset targeted project %q\n", projectName))

		currentTarget, err := targetProvider.Read()
		Expect(err).NotTo(HaveOccurred())
		Expect(currentTarget.GardenName()).To(Equal(gardenName))
		Expect(currentTarget.ProjectName()).To(BeEmpty())
	})

	It("should be able to unset targeted seed", func() {
		// user has already targeted a garden and seed
		targetProvider.Target = currentTarget.WithSeedName(seedName)
		cmd := cmdtarget.NewCmdUnset(factory, cmdtarget.NewUnsetOptions(streams))

		// run command
		Expect(cmd.RunE(cmd, []string{"seed"})).To(Succeed())
		Expect(out.String()).To(ContainSubstring("Successfully unset targeted seed %q\n", seedName))

		currentTarget, err := targetProvider.Read()
		Expect(err).NotTo(HaveOccurred())
		Expect(currentTarget.GardenName()).To(Equal(gardenName))
		Expect(currentTarget.SeedName()).To(BeEmpty())
	})

	It("should be able to unset targeted shoot", func() {
		// user has already targeted a garden, project and shoot
		targetProvider.Target = currentTarget.WithProjectName(projectName).WithShootName(shootName)
		cmd := cmdtarget.NewCmdUnset(factory, cmdtarget.NewUnsetOptions(streams))

		// run command
		Expect(cmd.RunE(cmd, []string{"shoot"})).To(Succeed())
		Expect(out.String()).To(ContainSubstring("Successfully unset targeted shoot %q\n", shootName))

		currentTarget, err := targetProvider.Read()
		Expect(err).NotTo(HaveOccurred())
		Expect(currentTarget.GardenName()).To(Equal(gardenName))
		Expect(currentTarget.ProjectName()).To(Equal(projectName))
		Expect(currentTarget.SeedName()).To(BeEmpty())
		Expect(currentTarget.ShootName()).To(BeEmpty())
	})

	It("should be able to unset targeted control plane", func() {
		// user has already targeted a garden, project, shoot and control-plane
		targetProvider.Target = currentTarget.WithProjectName(projectName).WithShootName(shootName).WithControlPlane(true)
		cmd := cmdtarget.NewCmdUnset(factory, cmdtarget.NewUnsetOptions(streams))

		// run command
		Expect(cmd.RunE(cmd, []string{"control-plane"})).To(Succeed())
		Expect(out.String()).To(ContainSubstring("Successfully unset targeted control plane for %q\n", shootName))

		currentTarget, err := targetProvider.Read()
		Expect(err).NotTo(HaveOccurred())
		Expect(currentTarget.GardenName()).To(Equal(gardenName))
		Expect(currentTarget.ProjectName()).To(Equal(projectName))
		Expect(currentTarget.SeedName()).To(BeEmpty())
		Expect(currentTarget.ShootName()).To(Equal(shootName))
		Expect(currentTarget.ControlPlane()).To(BeFalse())
	})
})

var _ = Describe("Target Unset Options", func() {
	It("should validate", func() {
		streams, _, _, _ := util.NewTestIOStreams()
		o := cmdtarget.NewUnsetOptions(streams)
		o.Kind = cmdtarget.TargetKindGarden

		Expect(o.Validate()).To(Succeed())
	})

	It("should reject invalid kinds", func() {
		streams, _, _, _ := util.NewTestIOStreams()
		o := cmdtarget.NewUnsetOptions(streams)
		o.Kind = "not a kind"

		err := o.Validate()
		Expect(err).To(MatchError(ContainSubstring("invalid target kind given, must be one of")))
	})
})
