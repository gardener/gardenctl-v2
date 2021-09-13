/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package target_test

import (
	internalfake "github.com/gardener/gardenctl-v2/internal/fake"
	"github.com/gardener/gardenctl-v2/internal/util"
	targetCmd "github.com/gardener/gardenctl-v2/pkg/cmd/target"
	"github.com/gardener/gardenctl-v2/pkg/config"
	"github.com/gardener/gardenctl-v2/pkg/target"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func init() {
	utilruntime.Must(gardencorev1beta1.AddToScheme(scheme.Scheme))
}

var _ = Describe("Command", func() {
	It("should reject bad options", func() {
		streams, _, _, _ := util.NewTestIOStreams()
		o := targetCmd.NewOptions(streams)
		cmd := targetCmd.NewCommand(&util.FactoryImpl{}, o, &target.DynamicTargetProvider{})

		Expect(cmd.RunE(cmd, nil)).NotTo(Succeed())
	})

	It("should be able to target a garden", func() {
		streams, _, out, _ := util.NewTestIOStreams()

		gardenName := "mygarden"
		cfg := &config.Config{
			Gardens: []config.Garden{{
				Name:       gardenName,
				Kubeconfig: "",
			}},
		}
		targetProvider := internalfake.NewFakeTargetProvider(target.NewTarget("", "", "", ""))
		factory := internalfake.NewFakeFactory(cfg, nil, nil, nil, targetProvider)
		cmd := targetCmd.NewCommand(factory, targetCmd.NewOptions(streams), &target.DynamicTargetProvider{})

		Expect(cmd.RunE(cmd, []string{"garden", gardenName})).To(Succeed())
		Expect(out.String()).To(ContainSubstring("Successfully targeted garden %q\n", gardenName))

		currentTarget, err := targetProvider.Read()
		Expect(err).NotTo(HaveOccurred())
		Expect(currentTarget.GardenName()).To(Equal(gardenName))
	})

	It("should be able to target a project", func() {
		streams, _, out, _ := util.NewTestIOStreams()

		gardenName := "mygarden"
		gardenKubeconfig := ""
		cfg := &config.Config{
			Gardens: []config.Garden{{
				Name:       gardenName,
				Kubeconfig: gardenKubeconfig,
			}},
		}

		// user has already targeted a garden
		currentTarget := target.NewTarget(gardenName, "", "", "")

		// garden cluster contains the targeted project
		projectName := "myproject"
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
		cmd := targetCmd.NewCommand(factory, targetCmd.NewOptions(streams), &target.DynamicTargetProvider{})

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

		gardenName := "mygarden"
		gardenKubeconfig := ""
		cfg := &config.Config{
			Gardens: []config.Garden{{
				Name:       gardenName,
				Kubeconfig: gardenKubeconfig,
			}},
		}

		// user has already targeted a garden
		currentTarget := target.NewTarget(gardenName, "", "", "")

		// garden cluster contains the targeted seed
		seedName := "myseed"
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
		cmd := targetCmd.NewCommand(factory, targetCmd.NewOptions(streams), &target.DynamicTargetProvider{})

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

		gardenName := "mygarden"
		gardenKubeconfig := ""
		cfg := &config.Config{
			Gardens: []config.Garden{{
				Name:       gardenName,
				Kubeconfig: gardenKubeconfig,
			}},
		}

		// garden cluster contains the targeted project and shoot
		namespace := "garden-prod1"
		projectName := "myproject"
		project := &gardencorev1beta1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name: projectName,
			},
			Spec: gardencorev1beta1.ProjectSpec{
				Namespace: pointer.String("garden-prod1"),
			},
		}

		shootName := "myshoot"
		shoot := &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      shootName,
				Namespace: namespace,
			},
		}

		// user has already targeted a garden and project
		currentTarget := target.NewTarget(gardenName, projectName, "", "")

		fakeGardenClient := fake.NewClientBuilder().WithObjects(project, shoot).Build()

		// setup command
		targetProvider := internalfake.NewFakeTargetProvider(currentTarget)
		clientProvider := internalfake.NewFakeClientProvider()
		clientProvider.WithClient(gardenKubeconfig, fakeGardenClient)

		factory := internalfake.NewFakeFactory(cfg, nil, clientProvider, nil, targetProvider)
		cmd := targetCmd.NewCommand(factory, targetCmd.NewOptions(streams), &target.DynamicTargetProvider{})

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
})
