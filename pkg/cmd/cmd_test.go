/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package cmd_test

import (
	"context"
	"errors"
	"fmt"
	"strings"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardenctl-v2/internal/fake"
	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd"
	"github.com/gardener/gardenctl-v2/pkg/target"
	targetmocks "github.com/gardener/gardenctl-v2/pkg/target/mocks"
)

var _ = Describe("Gardenctl command", func() {
	const (
		projectName  = "prod1"
		shootName    = "test-shoot1"
		nodeHostname = "example.host.invalid"
	)

	var (
		gardenName1 string
		gardenName2 string
		streams     util.IOStreams
		out         *util.SafeBytesBuffer
		errOut      *util.SafeBytesBuffer
		targetFlags target.TargetFlags
	)

	BeforeEach(func() {
		gardenName1 = cfg.Gardens[0].Name
		gardenName2 = cfg.Gardens[1].Name

		streams, _, out, errOut = util.NewTestIOStreams()

		targetFlags = target.NewTargetFlags("", "", "", "", false)

		targetProvider := target.NewTargetProvider(targetFile, nil)
		Expect(targetProvider.Write(target.NewTarget(gardenName1, projectName, "", shootName))).To(Succeed())
	})

	Describe("Depending on the factory interface", func() {
		var (
			ctrl          *gomock.Controller
			testProject1  *gardencorev1beta1.Project
			testProject2  *gardencorev1beta1.Project
			testSeed1     *gardencorev1beta1.Seed
			testSeed2     *gardencorev1beta1.Seed
			testShoot1    *gardencorev1beta1.Shoot
			testShoot2    *gardencorev1beta1.Shoot
			testShoot3    *gardencorev1beta1.Shoot
			testNode      *corev1.Node
			gardenClient1 client.Client
			gardenClient2 client.Client
			shootClient   client.Client
			factory       util.Factory
		)

		BeforeEach(func() {
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

			testSeed1 = &gardencorev1beta1.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-seed1",
				},
			}

			testSeed2 = &gardencorev1beta1.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-seed2",
				},
			}

			testShoot1 = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      shootName,
					Namespace: *testProject1.Spec.Namespace,
				},
				Spec: gardencorev1beta1.ShootSpec{
					SeedName: pointer.String(testSeed1.Name),
				},
			}

			testShoot2 = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-shoot2",
					Namespace: *testProject1.Spec.Namespace,
				},
				Spec: gardencorev1beta1.ShootSpec{
					SeedName: pointer.String(testSeed1.Name),
				},
			}

			testShoot3 = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-shoot3",
					Namespace: *testProject2.Spec.Namespace,
				},
				Spec: gardencorev1beta1.ShootSpec{
					SeedName: pointer.String(testSeed1.Name),
				},
			}

			gardenClient1 = fake.NewClientWithObjects(
				testProject1,
				testProject2,
				testSeed1,
				testSeed2,
				testShoot1,
				testShoot2,
				testShoot3,
			)
			Expect(gardenClient1).NotTo(BeNil())

			gardenClient2 = fake.NewClientWithObjects(
				testProject2,
				testSeed2,
				testShoot2,
			)
			Expect(gardenClient2).NotTo(BeNil())

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
			shootClient = fake.NewClientWithObjects(testNode)
			Expect(shootClient).NotTo(BeNil())

			ctrl = gomock.NewController(GinkgoT())
			clientProvider := targetmocks.NewMockClientProvider(ctrl)

			// ensure the clientprovider provides the proper clients to the manager
			clientConfig1, err := cfg.ClientConfig(gardenName1)
			Expect(err).ToNot(HaveOccurred())
			clientProvider.EXPECT().FromClientConfig(gomock.Eq(clientConfig1)).Return(gardenClient1, nil).AnyTimes()
			clientConfig2, err := cfg.ClientConfig(gardenName2)
			Expect(err).ToNot(HaveOccurred())
			clientProvider.EXPECT().FromClientConfig(gomock.Eq(clientConfig2)).Return(gardenClient2, nil).AnyTimes()

			currentTarget := target.NewTarget(gardenName1, testProject1.Name, "", testShoot1.Name)
			targetProvider := fake.NewFakeTargetProvider(currentTarget)

			factory = fake.NewFakeFactory(cfg, nil, clientProvider, targetProvider)
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		Describe("Complete garden flag values", func() {
			It("should return all garden names, alphabetically sorted", func() {
				manager, err := factory.Manager()
				Expect(err).NotTo(HaveOccurred())

				values, err := cmd.GardenFlagCompletionFunc(factory.Context(), manager)
				Expect(err).NotTo(HaveOccurred())
				Expect(values).To(Equal([]string{gardenName2, gardenName1}))
			})
		})

		Describe("Complete project flag values", func() {
			It("should return all project names for first garden, alphabetically sorted", func() {
				manager, err := factory.Manager()
				Expect(err).NotTo(HaveOccurred())

				values, err := cmd.ProjectFlagCompletionFunc(factory.Context(), manager)
				Expect(err).NotTo(HaveOccurred())
				Expect(values).To(Equal([]string{testProject1.Name, testProject2.Name}))
			})

			It("should fail when no target is defined", func() {
				manager, err := target.NewManager(cfg, fake.NewFakeTargetProvider(nil), nil, sessionDir)
				Expect(err).NotTo(HaveOccurred())

				_, err = cmd.ProjectFlagCompletionFunc(factory.Context(), manager)
				Expect(err).To(MatchError(HavePrefix("no target set")))
			})
		})

		Describe("Complete seed flag values", func() {
			It("should return all seed names for first garden, alphabetically sorted", func() {
				manager, err := factory.Manager()
				Expect(err).NotTo(HaveOccurred())

				values, err := cmd.SeedFlagCompletionFunc(factory.Context(), manager)
				Expect(err).NotTo(HaveOccurred())
				Expect(values).To(Equal([]string{testSeed1.Name, testSeed2.Name}))
			})

			It("should fail when no target is defined", func() {
				manager, err := target.NewManager(cfg, fake.NewFakeTargetProvider(nil), nil, sessionDir)
				Expect(err).NotTo(HaveOccurred())

				_, err = cmd.SeedFlagCompletionFunc(factory.Context(), manager)
				Expect(err).To(MatchError(HavePrefix("no target set")))
			})
		})

		Describe("Complete shoot flag values", func() {
			It("should return all shoot names for first project, alphabetically sorted", func() {
				manager, err := factory.Manager()
				Expect(err).NotTo(HaveOccurred())

				values, err := cmd.ShootFlagCompletionFunc(factory.Context(), manager)
				Expect(err).NotTo(HaveOccurred())
				Expect(values).To(Equal([]string{testShoot1.Name, testShoot2.Name}))
			})

			It("should fail when no target is defined", func() {
				manager, err := target.NewManager(cfg, fake.NewFakeTargetProvider(nil), nil, sessionDir)
				Expect(err).NotTo(HaveOccurred())

				_, err = cmd.ShootFlagCompletionFunc(factory.Context(), manager)
				Expect(err).To(MatchError(HavePrefix("no target set")))
			})
		})
	})

	Describe("Depending on the factory implementation", func() {
		var factory *util.FactoryImpl

		BeforeEach(func() {
			factory = &util.FactoryImpl{
				TargetFlags: targetFlags,
				ConfigFile:  configFile,
			}
		})

		Context("when wrapping completion functions", func() {
			It("should respect the prefix", func() {
				wrapped := cmd.CompletionWrapper(factory, streams, func(ctx context.Context, manager target.Manager) ([]string, error) {
					return []string{"foo", "bar"}, nil
				})

				returned, directory := wrapped(nil, nil, "f")
				Expect(directory).To(Equal(cobra.ShellCompDirectiveNoFileComp))
				Expect(returned).To(Equal([]string{"foo"}))
			})

			It("should fail when executing the completer", func() {
				wrapped := cmd.CompletionWrapper(factory, streams, func(ctx context.Context, manager target.Manager) ([]string, error) {
					return nil, errors.New("completion failed")
				})

				returned, directory := wrapped(nil, nil, "f")
				Expect(directory).To(Equal(cobra.ShellCompDirectiveNoFileComp))
				Expect(returned).To(BeNil())
				head := strings.Split(errOut.String(), "\n")[0]
				Expect(head).To(Equal("completion failed"))
			})

			It("should fail when loading the config", func() {
				factory.ConfigFile = string([]byte{0})
				wrapped := cmd.CompletionWrapper(factory, streams, func(ctx context.Context, manager target.Manager) ([]string, error) {
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
				shootName := "newshoot"
				args := []string{
					fmt.Sprintf("--shoot=%s", shootName),
					"completion",
					"zsh",
				}

				cmd := cmd.NewGardenctlCommand(factory, streams)
				cmd.SetArgs(args)
				Expect(cmd.Execute()).To(Succeed())

				head := strings.Split(out.String(), "\n")[0]
				Expect(head).To(Equal("#compdef _gardenctl gardenctl"))
				Expect(factory.ConfigFile).To(Equal(configFile))
				Expect(factory.TargetFile).To(Equal(targetFile))
				Expect(factory.GardenHomeDirectory).To(Equal(gardenHomeDir))

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
				Expect(current.GardenName()).To(Equal(gardenName1))
				Expect(current.ProjectName()).To(Equal(projectName))
				Expect(current.SeedName()).To(BeEmpty())
				Expect(current.ShootName()).To(Equal(shootName))
			})
		})
	})
})
