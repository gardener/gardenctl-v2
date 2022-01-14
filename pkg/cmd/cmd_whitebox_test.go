/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	internalfake "github.com/gardener/gardenctl-v2/internal/fake"
	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/config"
	"github.com/gardener/gardenctl-v2/pkg/target"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
)

var _ = Describe("Gardenctl command", func() {
	utilruntime.Must(gardencorev1beta1.AddToScheme(scheme.Scheme))
	utilruntime.Must(operationsv1alpha1.AddToScheme(scheme.Scheme))

	const (
		gardenName           = "mygarden"
		projectName          = "myproject"
		gardenKubeconfigFile = "/not/a/real/garden/kubeconfig"
		foobarKubeconfigFile = "/not/a/real/foobar/kubeconfig"
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
		testShoot3           *gardencorev1beta1.Shoot
		testShoot1Kubeconfig *corev1.Secret
		testNode             *corev1.Node
		gardenClient         client.Client
		foobarClient         client.Client
		shootClient          client.Client
		factory              util.Factory
		targetFlags          target.TargetFlags
		gardenDir            string
		configFile           string
		targetFile           string
	)

	BeforeEach(func() {
		cfg = &config.Config{
			Gardens: []config.Garden{{
				Name:       gardenName,
				Kubeconfig: gardenKubeconfigFile,
			}, {
				Name:       "foobar",
				Kubeconfig: foobarKubeconfigFile,
			}},
		}

		dir, err := os.MkdirTemp(os.TempDir(), "gctlv2-*")
		Expect(err).ToNot(HaveOccurred())

		gardenDir = dir
		configFile = filepath.Join(gardenDir, configName+".yaml")
		targetFile = filepath.Join(gardenDir, targetFilename)
		Expect(cfg.SaveToFile(configFile)).To(Succeed())
		fsTargetProvider := target.NewTargetProvider(targetFile, nil)
		Expect(fsTargetProvider.Write(target.NewTarget(gardenName, projectName, "", "myshoot"))).To(Succeed())
		Expect(os.Setenv(envGardenHomeDir, gardenDir)).To(Succeed())

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

		testShoot3 = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "other-shoot",
				Namespace: *testProject2.Spec.Namespace,
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
			testShoot3,
			testShoot1Kubeconfig,
			testShoot1Keypair,
		).Build()

		foobarClient = fakeclient.NewClientBuilder().WithObjects(
			testProject2,
			testSeed2,
			testShoot2,
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
		clientProvider.WithClient(
			gardenKubeconfigFile,
			gardenClient,
		).WithClient(
			foobarKubeconfigFile,
			foobarClient,
		)

		// prepare command
		factory = internalfake.NewFakeFactory(cfg, nil, clientProvider, nil, targetProvider)

		// prepare CLI flags
		targetFlags = target.NewTargetFlags("", "", "", "", false)

		Expect(shootClient).NotTo(BeNil())
		Expect(gardenClient).NotTo(BeNil())
	})

	AfterEach(func() {
		Expect(os.Unsetenv(envGardenHomeDir)).To(Succeed())
		Expect(os.RemoveAll(gardenDir)).To(Succeed())
	})

	Describe("Complete garden flag values", func() {
		It("should return all garden names, alphabetically sorted", func() {
			manager, err := factory.Manager()
			Expect(err).NotTo(HaveOccurred())

			values, err := gardenFlagCompletionFunc(factory.Context(), manager, targetFlags)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal([]string{"foobar", gardenName}))
		})
	})

	Describe("Complete project flag values", func() {
		It("should return all project names for first garden, alphabetically sorted", func() {
			manager, err := factory.Manager()
			Expect(err).NotTo(HaveOccurred())

			values, err := projectFlagCompletionFunc(factory.Context(), manager, targetFlags)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal([]string{testProject1.Name, testProject2.Name}))
		})

		It("should return all project names for second garden, alphabetically sorted", func() {
			manager, err := factory.Manager()
			Expect(err).NotTo(HaveOccurred())

			targetFlags = target.NewTargetFlags("foobar", "", "", "", false)
			values, err := projectFlagCompletionFunc(factory.Context(), manager, targetFlags)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal([]string{testProject2.Name}))
		})

		It("should fail when no target is defined", func() {
			manager, err := target.NewManager(cfg, internalfake.NewFakeTargetProvider(nil), nil, nil)
			Expect(err).NotTo(HaveOccurred())

			_, err = projectFlagCompletionFunc(factory.Context(), manager, targetFlags)
			Expect(err).To(MatchError(HavePrefix("failed to read current target:")))
		})
	})

	Describe("Complete seed flag values", func() {
		It("should return all seed names for first garden, alphabetically sorted", func() {
			manager, err := factory.Manager()
			Expect(err).NotTo(HaveOccurred())

			values, err := seedFlagCompletionFunc(factory.Context(), manager, targetFlags)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal([]string{testSeed2.Name, testSeed1.Name}))
		})

		It("should return all seed names for second garden, alphabetically sorted", func() {
			manager, err := factory.Manager()
			Expect(err).NotTo(HaveOccurred())

			targetFlags = target.NewTargetFlags("foobar", "", "", "", false)
			values, err := seedFlagCompletionFunc(factory.Context(), manager, targetFlags)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal([]string{testSeed2.Name}))
		})

		It("should fail when no target is defined", func() {
			manager, err := target.NewManager(cfg, internalfake.NewFakeTargetProvider(nil), nil, nil)
			Expect(err).NotTo(HaveOccurred())

			_, err = seedFlagCompletionFunc(factory.Context(), manager, targetFlags)
			Expect(err).To(MatchError(HavePrefix("failed to read current target:")))
		})
	})

	Describe("Complete shoot flag values", func() {
		It("should return all shoot names for first project, alphabetically sorted", func() {
			manager, err := factory.Manager()
			Expect(err).NotTo(HaveOccurred())

			values, err := shootFlagCompletionFunc(factory.Context(), manager, targetFlags)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal([]string{testShoot2.Name, testShoot1.Name}))
		})

		It("should return all shoot names for second project, alphabetically sorted", func() {
			manager, err := factory.Manager()
			Expect(err).NotTo(HaveOccurred())

			targetFlags = target.NewTargetFlags("", "prod2", "", "", false)
			values, err := shootFlagCompletionFunc(factory.Context(), manager, targetFlags)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal([]string{testShoot3.Name}))
		})

		It("should return all shoot names for first seed, alphabetically sorted", func() {
			manager, err := factory.Manager()
			Expect(err).NotTo(HaveOccurred())

			targetFlags = target.NewTargetFlags("", "", testSeed1.Name, "", false)
			values, err := shootFlagCompletionFunc(factory.Context(), manager, targetFlags)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal([]string{testShoot2.Name, testShoot3.Name, testShoot1.Name}))
		})

		It("should fail when no target is defined", func() {
			manager, err := target.NewManager(cfg, internalfake.NewFakeTargetProvider(nil), nil, nil)
			Expect(err).NotTo(HaveOccurred())

			_, err = shootFlagCompletionFunc(factory.Context(), manager, targetFlags)
			Expect(err).To(MatchError(HavePrefix("failed to read current target:")))
		})
	})

	Describe("Wrapping completion functions", func() {
		It("should respect the prefix", func() {
			factory := &util.FactoryImpl{
				ConfigFile: configFile,
			}
			streams, _, _, _ := util.NewTestIOStreams()
			wrapped := completionWrapper(factory, streams, func(ctx context.Context, manager target.Manager, tf target.TargetFlags) ([]string, error) {
				return []string{"foo", "bar"}, nil
			})

			returned, directory := wrapped(nil, nil, "f")
			Expect(directory).To(Equal(cobra.ShellCompDirectiveNoFileComp))
			Expect(returned).To(Equal([]string{"foo"}))
		})

		It("should fail when executing the completer", func() {
			factory := &util.FactoryImpl{
				ConfigFile: configFile,
			}
			streams, _, _, errOut := util.NewTestIOStreams()
			wrapped := completionWrapper(factory, streams, func(ctx context.Context, manager target.Manager, tf target.TargetFlags) ([]string, error) {
				return nil, errors.New("completion failed")
			})

			returned, directory := wrapped(nil, nil, "f")
			Expect(directory).To(Equal(cobra.ShellCompDirectiveNoFileComp))
			Expect(returned).To(BeNil())
			head := strings.Split(errOut.String(), "\n")[0]
			Expect(head).To(Equal("completion failed"))
		})

		It("should fail when loading the config", func() {
			factory := &util.FactoryImpl{}
			streams, _, _, errOut := util.NewTestIOStreams()
			wrapped := completionWrapper(factory, streams, func(ctx context.Context, manager target.Manager, tf target.TargetFlags) ([]string, error) {
				return []string{"foo", "bar"}, nil
			})

			returned, directory := wrapped(nil, nil, "f")
			Expect(directory).To(Equal(cobra.ShellCompDirectiveNoFileComp))
			Expect(returned).To(BeNil())
			head := strings.Split(errOut.String(), "\n")[0]
			Expect(head).To(HavePrefix("failed to load config:"))
		})
	})

	Context("when running the completion command", func() {
		It("should initialize command and factory correctly", func() {
			factory := &util.FactoryImpl{
				TargetFlags: targetFlags,
			}
			streams, _, out, _ := util.NewTestIOStreams()
			shootName := "newshoot"
			args := []string{
				fmt.Sprintf("--shoot=%s", shootName),
				"completion",
				"zsh",
			}

			cmd := NewGardenctlCommand(factory, streams)
			cmd.SetArgs(args)
			Expect(cmd.Execute()).To(Succeed())

			head := strings.Split(out.String(), "\n")[0]
			Expect(head).To(Equal("#compdef _gardenctl gardenctl"))
			Expect(factory.ConfigFile).To(Equal(configFile))
			Expect(factory.TargetFile).To(Equal(targetFile))
			Expect(factory.GardenHomeDirectory).To(Equal(gardenDir))

			manager, err := factory.Manager()
			Expect(err).NotTo(HaveOccurred())

			// check target flags values
			tf := manager.TargetFlags()
			Expect(tf).To(BeIdenticalTo(factory.TargetFlags))
			Expect(tf.GardenName()).To(BeEmpty())
			Expect(tf.ProjectName()).To(BeEmpty())
			Expect(tf.SeedName()).To(BeEmpty())
			Expect(tf.ShootName()).To(Equal(shootName))

			// check current target values
			current, err := manager.CurrentTarget()
			Expect(err).NotTo(HaveOccurred())
			Expect(current.GardenName()).To(Equal(gardenName))
			Expect(current.ProjectName()).To(Equal(projectName))
			Expect(current.SeedName()).To(BeEmpty())
			Expect(current.ShootName()).To(Equal(shootName))
		})
	})
})
