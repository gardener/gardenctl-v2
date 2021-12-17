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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
)

func init() {
	utilruntime.Must(gardencorev1beta1.AddToScheme(scheme.Scheme))
}

var _ = Describe("Completion", func() {
	utilruntime.Must(gardencorev1beta1.AddToScheme(scheme.Scheme))
	utilruntime.Must(operationsv1alpha1.AddToScheme(scheme.Scheme))

	const (
		gardenIdentity       = "mygarden"
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
			Gardens: []config.Garden{
				{
					Identity:   gardenIdentity,
					Kubeconfig: gardenKubeconfigFile,
				}, {
					Identity:   "abc",
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
		currentTarget := target.NewTarget(gardenIdentity, testProject1.Name, "", testShoot1.Name)
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
				string(TargetKindPattern),
			}))
		})

		It("should reject invalid kinds", func() {
			_, err := validTargetArgsFunction(factory, nil, []string{"invalid"}, "")
			Expect(err).To(HaveOccurred())
		})

		It("should return all garden names", func() {
			values, err := validTargetArgsFunction(factory, nil, []string{string(TargetKindGarden)}, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal([]string{"abc", gardenIdentity}))
		})

		It("should return all project names", func() {
			targetProvider.Target = target.NewTarget(gardenIdentity, "", "", "")

			values, err := validTargetArgsFunction(factory, nil, []string{string(TargetKindProject)}, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal([]string{testProject1.Name, testProject2.Name}))
		})

		It("should return all seed names", func() {
			targetProvider.Target = target.NewTarget(gardenIdentity, "", "", "")

			values, err := validTargetArgsFunction(factory, nil, []string{string(TargetKindSeed)}, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal([]string{testSeed2.Name, testSeed1.Name}))
		})

		It("should return all shoot names when using a project", func() {
			targetProvider.Target = target.NewTarget(gardenIdentity, testProject1.Name, "", "")

			values, err := validTargetArgsFunction(factory, nil, []string{string(TargetKindShoot)}, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal([]string{testShoot2.Name, testShoot1.Name}))
		})

		It("should return all shoot names when using a seed", func() {
			targetProvider.Target = target.NewTarget(gardenIdentity, "", testSeed1.Name, "")

			values, err := validTargetArgsFunction(factory, nil, []string{string(TargetKindShoot)}, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal([]string{testShoot2.Name, testShoot1.Name}))
		})
	})
})
