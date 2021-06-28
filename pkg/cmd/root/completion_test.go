/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package root

import (
	"context"
	"fmt"
	"os"

	internalfake "github.com/gardener/gardenctl-v2/internal/fake"
	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/config"
	"github.com/gardener/gardenctl-v2/pkg/target"
	"github.com/spf13/cobra"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("Completion", func() {
	utilruntime.Must(gardencorev1beta1.AddToScheme(scheme.Scheme))
	utilruntime.Must(operationsv1alpha1.AddToScheme(scheme.Scheme))

	const (
		gardenName           = "mygarden"
		gardenKubeconfigFile = "/not/a/real/kubeconfig"
		nodeHostname         = "example.host.invalid"
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
		testNode             *corev1.Node
		gardenClient         client.Client
		shootClient          client.Client
		factory              util.Factory
		configFile           string
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

		f, err := os.CreateTemp(os.TempDir(), "gctlv2*")
		Expect(err).ToNot(HaveOccurred())
		f.Close()

		configFile = f.Name()
		Expect(config.SaveToFile(configFile, cfg)).To(Succeed())

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

		gardenClient = fakeclient.NewClientBuilder().WithObjects(
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

		// create a fake shoot cluster with a single node in it
		testNode = &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node1",
			},
			Status: corev1.NodeStatus{
				Addresses: []corev1.NodeAddress{{
					Type:    corev1.NodeExternalDNS,
					Address: nodeHostname,
				}},
			},
		}

		shootClient = fakeclient.NewClientBuilder().WithObjects(testNode).Build()

		// setup fakes
		currentTarget := target.NewTarget(gardenName, testProject1.Name, "", testShoot1.Name)
		targetProvider := internalfake.NewFakeTargetProvider(currentTarget)
		clientProvider := internalfake.NewFakeClientProvider()

		// ensure the clientprovider provides the proper clients to the manager
		clientProvider.WithClient(gardenKubeconfigFile, gardenClient)

		// prepare command
		factory = internalfake.NewFakeFactory(cfg, nil, clientProvider, nil, targetProvider)

		Expect(shootClient).NotTo(BeNil())
		Expect(gardenClient).NotTo(BeNil())
	})

	AfterEach(func() {
		Expect(os.Remove(configFile)).To(Succeed())
	})

	Describe("gardenFlagCompletionFunc", func() {
		It("should return all garden names, alphabetically sorted", func() {
			manager, err := factory.Manager()
			Expect(err).NotTo(HaveOccurred())

			values, err := gardenFlagCompletionFunc(factory.Context(), manager)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal([]string{"abc", gardenName}))
		})
	})

	Describe("projectFlagCompletionFunc", func() {
		It("should return all project names, alphabetically sorted", func() {
			manager, err := factory.Manager()
			Expect(err).NotTo(HaveOccurred())

			values, err := projectFlagCompletionFunc(factory.Context(), manager)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal([]string{testProject1.Name, testProject2.Name}))
		})
	})

	Describe("seedFlagCompletionFunc", func() {
		It("should return all seed names, alphabetically sorted", func() {
			manager, err := factory.Manager()
			Expect(err).NotTo(HaveOccurred())

			values, err := seedFlagCompletionFunc(factory.Context(), manager)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal([]string{testSeed2.Name, testSeed1.Name}))
		})
	})

	Describe("shootFlagCompletionFunc", func() {
		It("should return all shoot names, alphabetically sorted", func() {
			manager, err := factory.Manager()
			Expect(err).NotTo(HaveOccurred())

			values, err := shootFlagCompletionFunc(factory.Context(), manager)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal([]string{testShoot2.Name, testShoot1.Name}))
		})
	})

	Describe("completionWrapper", func() {
		It("should respect the prefix", func() {
			factory := &util.FactoryImpl{
				ConfigFile: configFile,
			}

			wrapped := completionWrapper(factory, func(ctx context.Context, manager target.Manager) ([]string, error) {
				return []string{"foo", "bar"}, nil
			})

			returned, directory := wrapped(nil, nil, "f")
			Expect(directory).To(Equal(cobra.ShellCompDirectiveNoFileComp))
			Expect(returned).To(Equal([]string{"foo"}))
		})
	})
})
