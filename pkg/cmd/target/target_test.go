/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package target_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	internalfake "github.com/gardener/gardenctl-v2/internal/fake"
	"github.com/gardener/gardenctl-v2/internal/util"
	cmdtarget "github.com/gardener/gardenctl-v2/pkg/cmd/target"
	"github.com/gardener/gardenctl-v2/pkg/config"
	"github.com/gardener/gardenctl-v2/pkg/target"
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
		project          *gardencorev1beta1.Project
		seed             *gardencorev1beta1.Seed
		shoot            *gardencorev1beta1.Shoot
		cfg              *config.Config
		fakeGardenClient client.Client
		clientProvider   *internalfake.ClientProvider
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

		fakeGardenClient = fake.NewClientBuilder().WithObjects(project, seed, shoot).Build()
		clientProvider = internalfake.NewFakeClientProvider()
		clientProvider.WithClient(gardenKubeconfig, fakeGardenClient)
	})

	It("should reject bad options", func() {
		streams, _, _, _ := util.NewTestIOStreams()
		o := cmdtarget.NewTargetOptions(streams)
		cmd := cmdtarget.NewCmdTarget(&util.FactoryImpl{}, o)

		Expect(cmd.RunE(cmd, nil)).NotTo(Succeed())
	})

	It("should be able to target a garden", func() {
		streams, _, out, _ := util.NewTestIOStreams()

		targetProvider := internalfake.NewFakeTargetProvider(target.NewTarget("", "", "", ""))
		factory := internalfake.NewFakeFactory(cfg, nil, nil, nil, targetProvider)
		cmd := cmdtarget.NewCmdTarget(factory, cmdtarget.NewTargetOptions(streams))

		Expect(cmd.RunE(cmd, []string{"garden", gardenName})).To(Succeed())
		Expect(out.String()).To(ContainSubstring("Successfully targeted garden %q\n", gardenName))

		currentTarget, err := targetProvider.Read()
		Expect(err).NotTo(HaveOccurred())
		Expect(currentTarget.GardenName()).To(Equal(gardenName))
	})

	It("should be able to target a project", func() {
		streams, _, out, _ := util.NewTestIOStreams()

		// user has already targeted a garden
		currentTarget := target.NewTarget(gardenName, "", "", "")

		// setup command
		targetProvider := internalfake.NewFakeTargetProvider(currentTarget)

		factory := internalfake.NewFakeFactory(cfg, nil, clientProvider, nil, targetProvider)
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
		streams, _, out, _ := util.NewTestIOStreams()

		// user has already targeted a garden
		currentTarget := target.NewTarget(gardenName, "", "", "")

		// setup command
		targetProvider := internalfake.NewFakeTargetProvider(currentTarget)

		factory := internalfake.NewFakeFactory(cfg, nil, clientProvider, nil, targetProvider)
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
		streams, _, out, _ := util.NewTestIOStreams()

		// user has already targeted a garden and project
		currentTarget := target.NewTarget(gardenName, projectName, "", "")

		// setup command
		targetProvider := internalfake.NewFakeTargetProvider(currentTarget)

		factory := internalfake.NewFakeFactory(cfg, nil, clientProvider, nil, targetProvider)
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
		streams, _, out, _ := util.NewTestIOStreams()

		// user has already targeted a garden, project and shoot
		currentTarget := target.NewTarget(gardenName, projectName, "", shootName)

		// setup command
		targetProvider := internalfake.NewFakeTargetProvider(currentTarget)

		factory := internalfake.NewFakeFactory(cfg, nil, clientProvider, nil, targetProvider)
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
		Expect(currentTarget.ControlPlaneFlag()).To(BeTrue())
	})
})

var _ = Describe("TargetOptions", func() {
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
