/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package target_test

import (
	internalfake "github.com/gardener/gardenctl-v2/internal/fake"
	"github.com/gardener/gardenctl-v2/internal/util"
	cmdtarget "github.com/gardener/gardenctl-v2/pkg/cmd/target"
	"github.com/gardener/gardenctl-v2/pkg/config"
	"github.com/gardener/gardenctl-v2/pkg/target"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

func init() {
	utilruntime.Must(gardencorev1beta1.AddToScheme(scheme.Scheme))
}

var _ = Describe("Command", func() {
	const (
		gardenName       = "mygarden"
		gardenKubeconfig = "/not/a/real/file"
		projectName      = "myproject"
		seedName         = "myseed"
		shootName        = "myshoot"
		namespace        = "garden"
	)

	var (
		project *gardencorev1beta1.Project
		seed    *gardencorev1beta1.Seed
		shoot   *gardencorev1beta1.Shoot
		cfg     *config.Config
	)

	BeforeEach(func() {
		cfg = &config.Config{
			Gardens: []config.Garden{{
				Name:       gardenName,
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
			Spec: gardencorev1beta1.SeedSpec{
				SecretRef: &corev1.SecretReference{
					Namespace: namespace,
					Name:      seedName,
				},
			},
		}

		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      shootName,
				Namespace: namespace,
			},
		}
	})

	It("should reject bad options", func() {
		streams, _, _, _ := util.NewTestIOStreams()
		o := cmdtarget.NewUnsetOptions(streams)
		cmd := cmdtarget.NewCmdUnset(&util.FactoryImpl{}, o)

		Expect(cmd.RunE(cmd, nil)).NotTo(Succeed())
	})

	It("should be able to unset a targeted garden", func() {
		streams, _, out, _ := util.NewTestIOStreams()

		targetProvider := internalfake.NewFakeTargetProvider(target.NewTarget(gardenName, "", "", ""))
		factory := internalfake.NewFakeFactory(cfg, nil, nil, nil, targetProvider)
		cmd := cmdtarget.NewCmdUnset(factory, cmdtarget.NewUnsetOptions(streams))

		Expect(cmd.RunE(cmd, []string{"garden"})).To(Succeed())
		Expect(out.String()).To(ContainSubstring("Successfully unset targeted garden %q\n", gardenName))

		currentTarget, err := targetProvider.Read()
		Expect(err).NotTo(HaveOccurred())
		Expect(currentTarget.GardenName()).To(BeEmpty())
	})

	It("should be able to unset a targeted project", func() {
		streams, _, out, _ := util.NewTestIOStreams()

		// user has already targeted a garden and project
		currentTarget := target.NewTarget(gardenName, projectName, "", "")

		fakeGardenClient := fake.NewClientBuilder().WithObjects(project).Build()

		// setup command
		targetProvider := internalfake.NewFakeTargetProvider(currentTarget)
		clientProvider := internalfake.NewFakeClientProvider()
		clientProvider.WithClient(gardenKubeconfig, fakeGardenClient)

		factory := internalfake.NewFakeFactory(cfg, nil, clientProvider, nil, targetProvider)
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
		streams, _, out, _ := util.NewTestIOStreams()

		// user has already targeted a garden and seed
		currentTarget := target.NewTarget(gardenName, "", seedName, "")

		fakeGardenClient := fake.NewClientBuilder().WithObjects(seed).Build()

		// setup command
		targetProvider := internalfake.NewFakeTargetProvider(currentTarget)
		clientProvider := internalfake.NewFakeClientProvider()
		clientProvider.WithClient(gardenKubeconfig, fakeGardenClient)

		factory := internalfake.NewFakeFactory(cfg, nil, clientProvider, nil, targetProvider)
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
		streams, _, out, _ := util.NewTestIOStreams()

		// user has already targeted a garden, project and shoot
		currentTarget := target.NewTarget(gardenName, projectName, "", shootName)

		fakeGardenClient := fake.NewClientBuilder().WithObjects(project, shoot).Build()

		// setup command
		targetProvider := internalfake.NewFakeTargetProvider(currentTarget)
		clientProvider := internalfake.NewFakeClientProvider()
		clientProvider.WithClient(gardenKubeconfig, fakeGardenClient)

		factory := internalfake.NewFakeFactory(cfg, nil, clientProvider, nil, targetProvider)
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
		streams, _, out, _ := util.NewTestIOStreams()

		// user has already targeted a garden, project, shoot and control plane
		currentTarget := target.NewTarget(gardenName, projectName, "", shootName).WithControlPlane(true)

		fakeGardenClient := fake.NewClientBuilder().WithObjects(project, shoot).Build()

		// setup command
		targetProvider := internalfake.NewFakeTargetProvider(currentTarget)
		clientProvider := internalfake.NewFakeClientProvider()
		clientProvider.WithClient(gardenKubeconfig, fakeGardenClient)

		factory := internalfake.NewFakeFactory(cfg, nil, clientProvider, nil, targetProvider)
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

var _ = Describe("UnsetOptions", func() {
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
