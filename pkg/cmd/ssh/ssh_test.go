/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package ssh_test

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"time"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	corev1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	internalfake "github.com/gardener/gardenctl-v2/internal/fake"
	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/ssh"
	"github.com/gardener/gardenctl-v2/pkg/config"
	"github.com/gardener/gardenctl-v2/pkg/target"
	targetmocks "github.com/gardener/gardenctl-v2/pkg/target/mocks"
)

type bastionStatusPatch func(status *operationsv1alpha1.BastionStatus)

func waitForBastionThenPatchStatus(ctx context.Context, gardenClient client.Client, bastionName string, namespace string, patcher bastionStatusPatch) {
	defer GinkgoRecover()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			key := types.NamespacedName{Name: bastionName, Namespace: namespace}

			Eventually(func() error {
				bastion := &operationsv1alpha1.Bastion{}
				if err := gardenClient.Get(ctx, key, bastion); err != nil {
					return err
				}

				patch := client.MergeFrom(bastion.DeepCopy())
				patcher(&bastion.Status)

				return gardenClient.Status().Patch(ctx, bastion, patch)
			}).Should(Succeed())

			return
		}
	}
}

var _ = Describe("SSH Command", func() {
	const (
		gardenName           = "mygarden"
		gardenKubeconfigFile = "/not/a/real/kubeconfig"
		bastionName          = "test-bastion"
		bastionHostname      = "example.invalid"
		bastionIP            = "0.0.0.0"
		nodeHostname         = "example.host.invalid"
	)

	var (
		ctrl                *gomock.Controller
		clientProvider      *targetmocks.MockClientProvider
		cfg                 *config.Config
		streams             util.IOStreams
		out                 *util.SafeBytesBuffer
		factory             *internalfake.Factory
		ctx                 context.Context
		cancel              context.CancelFunc
		ctxTimeout          context.Context
		cancelTimeout       context.CancelFunc
		currentTarget       target.Target
		testProject         *gardencorev1beta1.Project
		testSeed            *gardencorev1beta1.Seed
		testShoot           *gardencorev1beta1.Shoot
		testShootKubeconfig *corev1.ConfigMap
		testNode            *corev1.Node
		gardenClient        client.Client
		shootClient         client.Client
		nodePrivateKeyFile  string
	)

	BeforeEach(func() {
		// all fake bastions are always immediately available
		ssh.SetBastionAvailabilityChecker(func(hostname string, privateKey []byte) error {
			return nil
		})

		// put the node SSH key into a known location
		ssh.SetTempFileCreator(func() (*os.File, error) {
			f, err := os.CreateTemp(os.TempDir(), "gctlv2*")
			Expect(err).ToNot(HaveOccurred())

			nodePrivateKeyFile = f.Name()

			return f, nil
		})

		// give all fake bastions a fixed name
		ssh.SetBastionNameProvider(func() (string, error) {
			return bastionName, nil
		})

		// simulate the user immediately exiting via Ctrl-C
		ssh.SetCreateSignalChannel(func() chan os.Signal {
			signalChan := make(chan os.Signal, 1)
			signalChan <- os.Interrupt

			return signalChan
		})

		// do not waste time in tests
		ssh.SetPollBastionStatusInterval(1 * time.Second)

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

		testSeed = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-seed",
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

		testShootKubeconfig = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testShoot.Name + ".kubeconfig",
				Namespace: *testProject.Spec.Namespace,
			},
			Data: map[string]string{
				"kubeconfig": string(createTestKubeconfig(testShoot.Name)),
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

		gardenClient = internalfake.NewClientWithObjects(
			testProject,
			testSeed,
			testShoot,
			testShootKubeconfig,
			testShootKeypair,
		)

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

		streams, _, out, _ = util.NewTestIOStreams()

		ctrl = gomock.NewController(GinkgoT())

		clientProvider = targetmocks.NewMockClientProvider(ctrl)
		clientConfig, err := cfg.ClientConfig(gardenName)
		Expect(err).ToNot(HaveOccurred())
		clientProvider.EXPECT().FromClientConfig(gomock.Eq(clientConfig)).Return(gardenClient, nil).AnyTimes()

		currentTarget = target.NewTarget(gardenName, testProject.Name, "", testShoot.Name)
		targetProvider := internalfake.NewFakeTargetProvider(currentTarget)

		factory = internalfake.NewFakeFactory(cfg, nil, clientProvider, targetProvider)

		ctxTimeout, cancelTimeout = context.WithTimeout(context.Background(), 30*time.Second)
		ctx, cancel = context.WithCancel(ctxTimeout)
		factory.ContextImpl = ctx
	})

	JustBeforeEach(func() {
		clientProvider.EXPECT().FromClientConfig(gomock.Any()).Return(shootClient, nil).AnyTimes().
			Do(func(clientConfig clientcmd.ClientConfig) {
				config, err := clientConfig.RawConfig()
				Expect(err).NotTo(HaveOccurred())
				Expect(config.CurrentContext).To(Equal(testShoot.Name))
			})
	})

	AfterEach(func() {
		cancelTimeout()
		cancel()
		ctrl.Finish()
	})

	Describe("RunE", func() {
		BeforeEach(func() {
			shootClient = internalfake.NewClientWithObjects(testNode)
		})

		It("should reject bad options", func() {
			o := ssh.NewSSHOptions(streams)
			cmd := ssh.NewCmdSSH(&util.FactoryImpl{}, o)

			Expect(cmd.RunE(cmd, nil)).NotTo(Succeed())
		})

		It("should print the SSH command and then wait for user interrupt", func() {
			options := ssh.NewSSHOptions(streams)
			cmd := ssh.NewCmdSSH(factory, options)

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

			// assert that the bastion has been cleaned up
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
			options := ssh.NewSSHOptions(streams)
			cmd := ssh.NewCmdSSH(factory, options)

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
			ssh.SetExecCommand(func(ctx context.Context, command string, args []string, o *ssh.SSHOptions) error {
				executedCommands++

				Expect(command).To(Equal("ssh"))
				Expect(args).To(Equal([]string{
					"-o", "StrictHostKeyChecking=no",
					"-o", "IdentitiesOnly=yes",
					"-o", fmt.Sprintf(
						"ProxyCommand=ssh -W%%h:%%p -o StrictHostKeyChecking=no -o IdentitiesOnly=yes -i %s %s@%s",
						o.SSHPrivateKeyFile,
						ssh.SSHBastionUsername,
						bastionIP,
					),
					"-i", nodePrivateKeyFile,
					fmt.Sprintf("%s@%s", ssh.SSHNodeUsername, nodeHostname),
				}))

				return nil
			})

			// let the magic happen
			Expect(cmd.RunE(cmd, []string{testNode.Name})).To(Succeed())

			// assert output
			Expect(executedCommands).To(Equal(1))
			Expect(out.String()).To(ContainSubstring(bastionName))
			Expect(out.String()).To(ContainSubstring(bastionHostname))
			Expect(out.String()).To(ContainSubstring(bastionIP))

			// assert that the bastion has been cleaned up
			key := types.NamespacedName{Name: bastionName, Namespace: *testProject.Spec.Namespace}
			bastion := &operationsv1alpha1.Bastion{}

			Expect(gardenClient.Get(ctx, key, bastion)).NotTo(Succeed())

			// assert that no temporary SSH keypair remained on disk
			_, err := os.Stat(options.SSHPublicKeyFile)
			Expect(err).To(HaveOccurred())

			_, err = os.Stat(options.SSHPrivateKeyFile)
			Expect(err).To(HaveOccurred())
		})

		It("should keep the bastion alive", func() {
			options := ssh.NewSSHOptions(streams)
			options.KeepBastion = true // we need to assert its annotations later

			cmd := ssh.NewCmdSSH(factory, options)

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

			// end the test after a couple of seconds (enough seconds for the keep-alive
			// goroutine to do its thing)
			ssh.SetKeepAliveInterval(100 * time.Millisecond)
			signalChan := make(chan os.Signal, 1)
			ssh.SetCreateSignalChannel(func() chan os.Signal {
				return signalChan
			})

			key := types.NamespacedName{Name: bastionName, Namespace: *testProject.Spec.Namespace}

			go func() {
				defer GinkgoRecover()

				Eventually(func() bool {
					bastion := &operationsv1alpha1.Bastion{}
					if err := gardenClient.Get(ctx, key, bastion); apierrors.IsNotFound(err) {
						return false
					}

					return bastion.Annotations != nil && bastion.Annotations[corev1beta1constants.GardenerOperation] == corev1beta1constants.GardenerOperationKeepalive
				}, "2s", "10ms").Should(BeTrue())

				signalChan <- os.Interrupt
			}()

			// let the magic happen
			Expect(cmd.RunE(cmd, nil)).To(Succeed())

			// Double check that the annotation was really set
			bastion := &operationsv1alpha1.Bastion{}
			Expect(gardenClient.Get(ctx, key, bastion)).To(Succeed())
			Expect(bastion.Annotations).To(HaveKeyWithValue(corev1beta1constants.GardenerOperation, corev1beta1constants.GardenerOperationKeepalive))
		})
	})

	Describe("ValidArgsFunction", func() {
		BeforeEach(func() {
			monitoringNode := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "monitoring",
				},
			}

			workerNode := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "worker",
				},
			}

			shootClient = internalfake.NewClientWithObjects(monitoringNode, workerNode)
		})

		It("should find nodes based on their prefix", func() {
			options := ssh.NewSSHOptions(streams)
			cmd := ssh.NewCmdSSH(factory, options)

			// let the magic happen; should find "monitoring" node based on this prefix
			suggestions, directive := cmd.ValidArgsFunction(cmd, nil, "mon")
			Expect(directive).To(Equal(cobra.ShellCompDirectiveNoFileComp))
			Expect(suggestions).To(HaveLen(1))
			Expect(suggestions).To(Equal([]string{"monitoring"}))
		})
	})
})

var _ = Describe("SSH Options", func() {
	var (
		streams          util.IOStreams
		publicSSHKeyFile string
	)

	BeforeEach(func() {
		streams, _, _, _ = util.NewTestIOStreams()

		tmpFile, err := os.CreateTemp("", "")
		Expect(err).NotTo(HaveOccurred())
		defer tmpFile.Close()

		// write dummy SSH public key
		_, err = io.WriteString(tmpFile, "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDouNkxsNuApuKVIfgL6Yz3Ep+DqX84Yde9DArwLBSWgLnl/pH9AbbcDcAmdB2CPVXAATo4qxK7xprvyyZp52SQRCcAZpAy4D6gAWwAG3OfzrRbxRiB5pQDaaWATSzNbLtoy0ecVwFeTJe2w71q+wxbI7tfxbvo9XbXIN4I0cQy2KLICzkYkQmygGnHztv1Mvi338+sgcG7Gwq2tdSyggDaAggwDIuT39S4/L7QpR27tWH79J4Ls8tTHud2eRbkOcF98vXlQAIzb6w8iHBXylOjMM/oODwoA7V4mtRL9o13AoocvZSsD1UvfOjGxDHuLrCfFXN+/rEw0hEiYo0cnj7F")
		Expect(err).NotTo(HaveOccurred())

		publicSSHKeyFile = tmpFile.Name()
	})

	AfterEach(func() {
		Expect(os.Remove(publicSSHKeyFile)).To(Succeed())
	})

	It("should validate", func() {
		o := ssh.NewSSHOptions(streams)
		o.CIDRs = []string{"8.8.8.8/32"}
		o.SSHPublicKeyFile = publicSSHKeyFile

		Expect(o.Validate()).To(Succeed())
	})

	It("should require a non-zero wait time", func() {
		o := ssh.NewSSHOptions(streams)
		o.CIDRs = []string{"8.8.8.8/32"}
		o.SSHPublicKeyFile = publicSSHKeyFile
		o.WaitTimeout = 0

		Expect(o.Validate()).NotTo(Succeed())
	})

	It("should require a public SSH key file", func() {
		o := ssh.NewSSHOptions(streams)
		o.CIDRs = []string{"8.8.8.8/32"}

		Expect(o.Validate()).NotTo(Succeed())
	})

	It("should require a valid public SSH key file", func() {
		Expect(ioutil.WriteFile(publicSSHKeyFile, []byte("not a key"), 0644)).To(Succeed())

		o := ssh.NewSSHOptions(streams)
		o.CIDRs = []string{"8.8.8.8/32"}
		o.SSHPublicKeyFile = publicSSHKeyFile

		Expect(o.Validate()).NotTo(Succeed())
	})

	It("should require at least one CIDR", func() {
		o := ssh.NewSSHOptions(streams)
		o.SSHPublicKeyFile = publicSSHKeyFile

		Expect(o.Validate()).NotTo(Succeed())
	})

	It("should reject invalid CIDRs", func() {
		o := ssh.NewSSHOptions(streams)
		o.CIDRs = []string{"8.8.8.8"}
		o.SSHPublicKeyFile = publicSSHKeyFile

		Expect(o.Validate()).NotTo(Succeed())
	})
})
