/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package ssh

import (
	"context"
	"fmt"
	"os"
	"time"

	internalfake "github.com/gardener/gardenctl-v2/internal/fake"
	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/config"
	"github.com/gardener/gardenctl-v2/pkg/target"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func init() {
	utilruntime.Must(gardencorev1beta1.AddToScheme(scheme.Scheme))
	utilruntime.Must(operationsv1alpha1.AddToScheme(scheme.Scheme))
}

type bastionStatusPatch func(status *operationsv1alpha1.BastionStatus)

func waitForBastionThenPatchStatus(ctx context.Context, gardenClient client.Client, bastionName string, namespace string, patcher bastionStatusPatch) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			key := types.NamespacedName{Name: bastionName, Namespace: namespace}
			bastion := &operationsv1alpha1.Bastion{}

			if err := gardenClient.Get(ctx, key, bastion); err != nil {
				break
			}

			patch := client.MergeFrom(bastion.DeepCopy())
			patcher(&bastion.Status)

			Expect(gardenClient.Status().Patch(ctx, bastion, patch)).To(Succeed())
			return
		}
	}
}

var _ = Describe("Command", func() {
	const (
		gardenName           = "mygarden"
		gardenKubeconfigFile = "/not/a/real/kubeconfig"
		bastionName          = "test-bastion"
		bastionHostname      = "example.invalid"
		bastionIP            = "0.0.0.0"
	)

	var (
		cfg                 *config.Config
		testProject         *gardencorev1beta1.Project
		testSeed            *gardencorev1beta1.Seed
		testShoot           *gardencorev1beta1.Shoot
		testShootKubeconfig *corev1.Secret
		gardenClient        client.Client
	)

	BeforeEach(func() {
		// all fake bastions are always immediately available
		portAvailabilityChecker = func(ctx context.Context, hostname string) error {
			return nil
		}

		// give all fake bastions a fixed name
		bastionNameProvider = func() (string, error) {
			return bastionName, nil
		}

		// simulate the user immediately exiting via Ctrl-C
		createSignalChannel = func() <-chan os.Signal {
			signalChan := make(chan os.Signal, 1)
			close(signalChan)

			return signalChan
		}

		// do not waste time in tests
		pollBastionStatusInterval = 1 * time.Second

		cfg = &config.Config{
			Gardens: []config.Garden{{
				Name:       gardenName,
				Kubeconfig: gardenKubeconfigFile,
			}},
		}

		testProject = &gardencorev1beta1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name: "prod1",
			},
			Spec: gardencorev1beta1.ProjectSpec{
				Namespace: pointer.String("garden-prod1"),
			},
		}

		testSeedKubeconfig := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-seed-kubeconfig",
				Namespace: "garden",
			},
			Data: map[string][]byte{
				"data": []byte("not-used"),
			},
		}

		testSeed = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-seed",
			},
			Spec: gardencorev1beta1.SeedSpec{
				SecretRef: &corev1.SecretReference{
					Name:      testSeedKubeconfig.Name,
					Namespace: testSeedKubeconfig.Namespace,
				},
			},
		}

		testShoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-shoot",
				Namespace: *testProject.Spec.Namespace,
			},
			Spec: gardencorev1beta1.ShootSpec{
				SeedName: pointer.String(testSeed.Name),
			},
		}

		testShootKubeconfig = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s.kubeconfig", testShoot.Name),
				Namespace: *testProject.Spec.Namespace,
			},
			Data: map[string][]byte{
				"data": []byte("not-used"),
			},
		}

		testShootKeypair := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s.ssh-keypair", testShoot.Name),
				Namespace: *testProject.Spec.Namespace,
			},
			Data: map[string][]byte{
				"data": []byte("not-used"),
			},
		}

		gardenClient = fakeclient.NewClientBuilder().WithObjects(
			testProject,
			testSeed,
			testSeedKubeconfig,
			testShoot,
			testShootKubeconfig,
			testShootKeypair,
		).Build()
	})

	It("should reject bad options", func() {
		streams, _, _, _ := genericclioptions.NewTestIOStreams()
		o := NewOptions(streams)
		cmd := NewCommand(&util.FactoryImpl{}, o)

		Expect(cmd.RunE(cmd, nil)).NotTo(Succeed())
	})

	It("should print the SSH command and then wait for user interrupt", func() {
		streams, _, out, _ := genericclioptions.NewTestIOStreams()

		ctxTimeout, cancelTimeout := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancelTimeout()

		ctx, cancel := context.WithCancel(ctxTimeout)
		defer cancel()

		// setup fakes
		currentTarget := target.NewTarget(gardenName, testProject.Name, "", testShoot.Name)
		targetProvider := internalfake.NewFakeTargetProvider(currentTarget)
		clientProvider := internalfake.NewFakeClientProvider()
		clientProvider.WithClient(gardenKubeconfigFile, gardenClient)

		// prepare command
		factory := internalfake.NewFakeFactory(cfg, nil, clientProvider, nil, targetProvider)
		factory.ContextImpl = ctx

		options := NewOptions(streams)
		cmd := NewCommand(factory, options)

		// simulate an external controller processing the bastion and proving a successful status
		go waitForBastionThenPatchStatus(ctx, gardenClient, bastionName, *testProject.Spec.Namespace, func(status *operationsv1alpha1.BastionStatus) {
			status.Ingress = &corev1.LoadBalancerIngress{
				Hostname: bastionHostname,
				IP:       bastionIP,
			}
			status.Conditions = []gardencorev1alpha1.Condition{{
				Type:   "BastionReady",
				Status: gardencorev1alpha1.ConditionTrue,
				Reason: "Testing",
			}}
		})

		// let the magic happen
		Expect(cmd.RunE(cmd, nil)).To(Succeed())

		// assert the output
		Expect(out.String()).To(ContainSubstring(bastionName))
		Expect(out.String()).To(ContainSubstring(bastionHostname))
		Expect(out.String()).To(ContainSubstring(bastionIP))

		// asser that the bastion has been cleaned up
		key := types.NamespacedName{Name: bastionName, Namespace: *testProject.Spec.Namespace}
		bastion := &operationsv1alpha1.Bastion{}

		Expect(gardenClient.Get(ctx, key, bastion)).NotTo(Succeed())

		// assert that no temporary SSH keypair remained on disk
		_, err := os.Stat(options.SSHPublicKeyFile)
		Expect(err).To(HaveOccurred())

		_, err = os.Stat(options.SSHPrivateKeyFile)
		Expect(err).To(HaveOccurred())
	})

	It("should connect to a given node", func() {
		streams, _, out, _ := genericclioptions.NewTestIOStreams()

		ctxTimeout, cancelTimeout := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancelTimeout()

		ctx, cancel := context.WithCancel(ctxTimeout)
		defer cancel()

		currentTarget := target.NewTarget(gardenName, testProject.Name, "", testShoot.Name)
		targetProvider := internalfake.NewFakeTargetProvider(currentTarget)
		clientProvider := internalfake.NewFakeClientProvider()

		// create a fake shoot cluster with a single node in it
		testNode := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node1",
			},
			Status: corev1.NodeStatus{
				Addresses: []corev1.NodeAddress{{
					Type:    corev1.NodeExternalDNS,
					Address: bastionHostname,
				}},
			},
		}

		shootClient := fakeclient.NewClientBuilder().WithObjects(testNode).Build()

		// simulate a cached shoot kubeconfig
		fakeKubeconfig := "<not-a-shoot-kubeconfig>"
		kubeconfigCache := internalfake.NewFakeKubeconfigCache()
		Expect(kubeconfigCache.Write(currentTarget, []byte(fakeKubeconfig))).To(Succeed())

		// ensure the clientprovider provides the proper clients to the manager
		clientProvider.WithClient(gardenKubeconfigFile, gardenClient)
		clientProvider.WithClient(fakeKubeconfig, shootClient)

		// prepare the command
		factory := internalfake.NewFakeFactory(cfg, nil, clientProvider, kubeconfigCache, targetProvider)
		factory.ContextImpl = ctx

		options := NewOptions(streams)
		cmd := NewCommand(factory, options)

		// simulate an external controller processing the bastion and proving a successful status
		go waitForBastionThenPatchStatus(ctx, gardenClient, bastionName, *testProject.Spec.Namespace, func(status *operationsv1alpha1.BastionStatus) {
			status.Ingress = &corev1.LoadBalancerIngress{
				Hostname: bastionHostname,
				IP:       bastionIP,
			}
			status.Conditions = []gardencorev1alpha1.Condition{{
				Type:   "BastionReady",
				Status: gardencorev1alpha1.ConditionTrue,
				Reason: "Testing",
			}}
		})

		// do not actually execute any commands
		executedCommands := 0
		execCommand = func(ctx context.Context, command string, args []string, o *Options) error {
			executedCommands++
			return nil
		}

		// let the magic happen
		Expect(cmd.RunE(cmd, []string{testNode.Name})).To(Succeed())

		// assert output
		Expect(executedCommands).To(Equal(1))
		Expect(out.String()).To(ContainSubstring(bastionName))
		Expect(out.String()).To(ContainSubstring(bastionHostname))
		Expect(out.String()).To(ContainSubstring(bastionIP))

		// asser that the bastion has been cleaned up
		key := types.NamespacedName{Name: bastionName, Namespace: *testProject.Spec.Namespace}
		bastion := &operationsv1alpha1.Bastion{}

		Expect(gardenClient.Get(ctx, key, bastion)).NotTo(Succeed())

		// assert that no temporary SSH keypair remained on disk
		_, err := os.Stat(options.SSHPublicKeyFile)
		Expect(err).To(HaveOccurred())

		_, err = os.Stat(options.SSHPrivateKeyFile)
		Expect(err).To(HaveOccurred())
	})

	Describe("ValidArgsFunction", func() {
		It("should find nodes based on their prefix", func() {
			streams, _, _, _ := genericclioptions.NewTestIOStreams()

			ctxTimeout, cancelTimeout := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancelTimeout()

			ctx, cancel := context.WithCancel(ctxTimeout)
			defer cancel()

			currentTarget := target.NewTarget(gardenName, testProject.Name, "", testShoot.Name)
			targetProvider := internalfake.NewFakeTargetProvider(currentTarget)
			clientProvider := internalfake.NewFakeClientProvider()

			// create a fake shoot cluster with a single node in it
			monitoringNode := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "monitoring",
				},
				Status: corev1.NodeStatus{
					Addresses: []corev1.NodeAddress{{
						Type:    corev1.NodeExternalDNS,
						Address: bastionHostname,
					}},
				},
			}

			workerHostname := "hostname.worker.invalid"
			workerNode := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "worker",
				},
				Status: corev1.NodeStatus{
					Addresses: []corev1.NodeAddress{{
						Type:    corev1.NodeExternalDNS,
						Address: workerHostname,
					}},
				},
			}

			shootClient := fakeclient.NewClientBuilder().WithObjects(monitoringNode, workerNode).Build()

			// simulate a cached shoot kubeconfig
			fakeKubeconfig := "<not-a-shoot-kubeconfig>"
			kubeconfigCache := internalfake.NewFakeKubeconfigCache()
			Expect(kubeconfigCache.Write(currentTarget, []byte(fakeKubeconfig))).To(Succeed())

			// ensure the clientprovider provides the proper clients to the manager
			clientProvider.WithClient(gardenKubeconfigFile, gardenClient)
			clientProvider.WithClient(fakeKubeconfig, shootClient)

			// prepare the command
			factory := internalfake.NewFakeFactory(cfg, nil, clientProvider, kubeconfigCache, targetProvider)
			factory.ContextImpl = ctx

			options := NewOptions(streams)
			cmd := NewCommand(factory, options)

			// let the magic happen; should find "monitoring" node based on this prefix
			suggestions, directive := cmd.ValidArgsFunction(cmd, nil, "mon")
			Expect(directive).To(Equal(cobra.ShellCompDirectiveNoFileComp))
			Expect(suggestions).To(HaveLen(1))
		})
	})
})
