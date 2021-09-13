/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package target

import (
	"fmt"
	internalfake "github.com/gardener/gardenctl-v2/internal/fake"
	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/config"
	"github.com/gardener/gardenctl-v2/pkg/target"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func init() {
	utilruntime.Must(gardencorev1beta1.AddToScheme(scheme.Scheme))
}

var _ = Describe("Command", func() {
	It("should reject bad options", func() {
		streams, _, _, _ := util.NewTestIOStreams()
		o := NewTargetOptions(streams)
		cmd := NewTargetCommand(&util.FactoryImpl{}, o, &target.DynamicTargetProvider{})

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
		cmd := NewTargetCommand(factory, NewTargetOptions(streams), &target.DynamicTargetProvider{})

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
		cmd := NewTargetCommand(factory, NewTargetOptions(streams), &target.DynamicTargetProvider{})

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
		cmd := NewTargetCommand(factory, NewTargetOptions(streams), &target.DynamicTargetProvider{})

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
		cmd := NewTargetCommand(factory, NewTargetOptions(streams), &target.DynamicTargetProvider{})

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

var _ = Describe("Completion", func() {
	utilruntime.Must(gardencorev1beta1.AddToScheme(scheme.Scheme))
	utilruntime.Must(operationsv1alpha1.AddToScheme(scheme.Scheme))

	const (
		gardenName           = "mygarden"
		gardenKubeconfigFile = "/not/a/real/kubeconfig"
	)

	var (
		cfg                  *config.Config
		testProject1         *gardencorev1beta1.Project
		testProject2         *gardencorev1beta1.Project
		testSeed1            *gardencorev1beta1.Seed
		testSeed2            *gardencorev1beta1.Seed
		testShoot1           *gardencorev1beta1.Shoot
		testShoot2           *gardencorev1beta1.Shoot
		testShoot1Kubeconfig *corev1.Secret
		gardenClient         client.Client
		factory              util.Factory
		targetProvider       *internalfake.TargetProvider
	)

	BeforeEach(func() {
		cfg = &config.Config{
			Gardens: []config.Garden{{
				Name:       gardenName,
				Kubeconfig: gardenKubeconfigFile,
			}, {
				Name:       "abc",
				Kubeconfig: gardenKubeconfigFile,
			}},
		}

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

		testSeed1Kubeconfig := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-seed1-kubeconfig",
				Namespace: "garden",
			},
			Data: map[string][]byte{
				"data": []byte("not-used"),
			},
		}

		testSeed1 = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-seed1",
			},
			Spec: gardencorev1beta1.SeedSpec{
				SecretRef: &corev1.SecretReference{
					Name:      testSeed1Kubeconfig.Name,
					Namespace: testSeed1Kubeconfig.Namespace,
				},
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

		testShoot1Kubeconfig = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s.kubeconfig", testShoot1.Name),
				Namespace: *testProject1.Spec.Namespace,
			},
			Data: map[string][]byte{
				"data": []byte("not-used"),
			},
		}

		testShoot1Keypair := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s.ssh-keypair", testShoot1.Name),
				Namespace: *testProject1.Spec.Namespace,
			},
			Data: map[string][]byte{
				"data": []byte("not-used"),
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

		gardenClient = fake.NewClientBuilder().WithObjects(
			testProject1,
			testProject2,
			testSeed1,
			testSeed2,
			testSeed1Kubeconfig,
			testShoot1,
			testShoot2,
			testShoot1Kubeconfig,
			testShoot1Keypair,
		).Build()

		// setup fakes
		currentTarget := target.NewTarget(gardenName, testProject1.Name, "", testShoot1.Name)
		targetProvider = internalfake.NewFakeTargetProvider(currentTarget)
		clientProvider := internalfake.NewFakeClientProvider()

		// ensure the clientprovider provides the proper clients to the manager
		clientProvider.WithClient(gardenKubeconfigFile, gardenClient)

		// prepare command
		factory = internalfake.NewFakeFactory(cfg, nil, clientProvider, nil, targetProvider)

		Expect(gardenClient).NotTo(BeNil())
	})

	Describe("validTargetArgsFunction", func() {
		It("should return the allowed target types when no kind was given", func() {
			values, err := validTargetArgsFunction(factory, nil, nil, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal([]string{
				string(TargetKindGarden),
				string(TargetKindProject),
				string(TargetKindSeed),
				string(TargetKindShoot),
			}))
		})

		It("should reject invalid kinds", func() {
			_, err := validTargetArgsFunction(factory, nil, []string{"invalid"}, "")
			Expect(err).To(HaveOccurred())
		})

		It("should return all garden names", func() {
			values, err := validTargetArgsFunction(factory, nil, []string{string(TargetKindGarden)}, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal([]string{"abc", gardenName}))
		})

		It("should return all project names", func() {
			targetProvider.Target = target.NewTarget(gardenName, "", "", "")

			values, err := validTargetArgsFunction(factory, nil, []string{string(TargetKindProject)}, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal([]string{testProject1.Name, testProject2.Name}))
		})

		It("should return all seed names", func() {
			targetProvider.Target = target.NewTarget(gardenName, "", "", "")

			values, err := validTargetArgsFunction(factory, nil, []string{string(TargetKindSeed)}, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal([]string{testSeed2.Name, testSeed1.Name}))
		})

		It("should return all shoot names when using a project", func() {
			targetProvider.Target = target.NewTarget(gardenName, testProject1.Name, "", "")

			values, err := validTargetArgsFunction(factory, nil, []string{string(TargetKindShoot)}, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal([]string{testShoot2.Name, testShoot1.Name}))
		})

		It("should return all shoot names when using a seed", func() {
			targetProvider.Target = target.NewTarget(gardenName, "", testSeed1.Name, "")

			values, err := validTargetArgsFunction(factory, nil, []string{string(TargetKindShoot)}, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal([]string{testShoot2.Name, testShoot1.Name}))
		})
	})
})

var _ = Describe("TargetOptions", func() {
	It("should validate", func() {
		streams, _, _, _ := util.NewTestIOStreams()
		o := NewTargetOptions(streams)
		o.Kind = TargetKindGarden
		o.TargetName = "foo"

		Expect(o.Validate()).To(Succeed())
	})

	It("should reject invalid kinds", func() {
		streams, _, _, _ := util.NewTestIOStreams()
		o := NewTargetOptions(streams)
		o.Kind = TargetKind("not a kind")
		o.TargetName = "foo"

		Expect(o.Validate()).NotTo(Succeed())
	})
})
