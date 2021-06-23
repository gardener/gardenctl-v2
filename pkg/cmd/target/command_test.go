/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package target_test

import (
	internalfake "github.com/gardener/gardenctl-v2/internal/fake"
	"github.com/gardener/gardenctl-v2/internal/util"
	. "github.com/gardener/gardenctl-v2/pkg/cmd/target"
	"github.com/gardener/gardenctl-v2/pkg/config"
	"github.com/gardener/gardenctl-v2/pkg/target"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
		o := NewOptions(streams)
		cmd := NewCommand(&util.FactoryImpl{}, o)

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
		cmd := NewCommand(factory, NewOptions(streams))

		Expect(cmd.RunE(cmd, []string{"garden", gardenName})).To(Succeed())
		Expect(out.String()).To(ContainSubstring("Successfully targeted"))

		currentTarget, err := targetProvider.Read()
		Expect(err).NotTo(HaveOccurred())
		Expect(currentTarget.GardenName()).To(Equal(gardenName))
	})

	It("should be able to target a project", func() {
		streams, _, _, _ := util.NewTestIOStreams()

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
		cmd := NewCommand(factory, NewOptions(streams))

		// run command
		Expect(cmd.RunE(cmd, []string{"project", projectName})).To(Succeed())

		currentTarget, err := targetProvider.Read()
		Expect(err).NotTo(HaveOccurred())
		Expect(currentTarget.GardenName()).To(Equal(gardenName))
		Expect(currentTarget.ProjectName()).To(Equal(projectName))
	})
})
