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
	It("should reject bad options", func() {
		streams, _, _, _ := util.NewTestIOStreams()
		o := cmdtarget.NewUnsetOptions(streams)
		cmd := cmdtarget.NewCmdUnset(&util.FactoryImpl{}, o)

		Expect(cmd.RunE(cmd, nil)).NotTo(Succeed())
	})

	It("should be able to unset a targeted garden", func() {
		streams, _, out, _ := util.NewTestIOStreams()

		gardenName := "mygarden"
		cfg := &config.Config{
			Gardens: []config.Garden{{
				Name:       gardenName,
				Kubeconfig: "",
			}},
		}
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

		gardenName := "mygarden"
		projectName := "myproject"
		gardenKubeconfig := ""
		cfg := &config.Config{
			Gardens: []config.Garden{{
				Name:       gardenName,
				Kubeconfig: gardenKubeconfig,
			}},
		}

		// user has already targeted a garden and project
		currentTarget := target.NewTarget(gardenName, projectName, "", "")

		// garden cluster contains the targeted project
		project := &gardencorev1beta1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name: projectName,
			},
			Spec: gardencorev1beta1.ProjectSpec{
				Namespace: pointer.String("garden-prod1"),
			},
		}

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

		gardenName := "mygarden"
		seedName := "myseed"
		gardenKubeconfig := ""
		cfg := &config.Config{
			Gardens: []config.Garden{{
				Name:       gardenName,
				Kubeconfig: gardenKubeconfig,
			}},
		}

		// user has already targeted a garden and seed
		currentTarget := target.NewTarget(gardenName, "", seedName, "")

		// garden cluster contains the targeted seed
		seed := &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				Name: seedName,
			},
			Spec: gardencorev1beta1.SeedSpec{
				SecretRef: &corev1.SecretReference{
					Namespace: "garden",
					Name:      seedName,
				},
			},
		}

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

		gardenName := "mygarden"
		gardenKubeconfig := ""
		projectName := "myproject"
		shootName := "myshoot"
		cfg := &config.Config{
			Gardens: []config.Garden{{
				Name:       gardenName,
				Kubeconfig: gardenKubeconfig,
			}},
		}

		// garden cluster contains the targeted project and shoot
		namespace := "garden-prod1"
		project := &gardencorev1beta1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name: projectName,
			},
			Spec: gardencorev1beta1.ProjectSpec{
				Namespace: pointer.String("garden-prod1"),
			},
		}

		shoot := &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      shootName,
				Namespace: namespace,
			},
		}

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
