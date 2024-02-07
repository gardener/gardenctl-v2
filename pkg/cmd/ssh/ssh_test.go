/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package ssh_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	corev1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/secrets"
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardenclientmocks "github.com/gardener/gardenctl-v2/internal/client/garden/mocks"
	clientmocks "github.com/gardener/gardenctl-v2/internal/client/mocks"
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

	Eventually(func() error {
		bastion := &operationsv1alpha1.Bastion{ObjectMeta: metav1.ObjectMeta{
			Name:      bastionName,
			Namespace: namespace,
		}}
		if err := gardenClient.Get(ctx, client.ObjectKeyFromObject(bastion), bastion); err != nil {
			return err
		}

		patch := client.MergeFrom(bastion.DeepCopy())
		patcher(&bastion.Status)

		return gardenClient.Status().Patch(ctx, bastion, patch)
	}).Should(Succeed())
}

func waitForBastionThenSetBastionReady(ctx context.Context, gardenClient client.Client, bastionName string, namespace string, bastionHostname string, bastionIP string) {
	waitForBastionThenPatchStatus(ctx, gardenClient, bastionName, namespace, func(status *operationsv1alpha1.BastionStatus) {
		status.Ingress = &corev1.LoadBalancerIngress{
			Hostname: bastionHostname,
			IP:       bastionIP,
		}
		status.Conditions = []gardencorev1beta1.Condition{{
			Type:   "BastionReady",
			Status: gardencorev1beta1.ConditionTrue,
			Reason: "Testing",
		}}
	})
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
		ctrl                 *gomock.Controller
		clientProvider       *clientmocks.MockProvider
		cfg                  *config.Config
		streams              util.IOStreams
		out                  *util.SafeBytesBuffer
		factory              *internalfake.Factory
		ctx                  context.Context
		cancel               context.CancelFunc
		ctxTimeout           context.Context
		cancelTimeout        context.CancelFunc
		currentTarget        target.Target
		testProject          *gardencorev1beta1.Project
		testSeed             *gardencorev1beta1.Seed
		testShoot            *gardencorev1beta1.Shoot
		testNode             *corev1.Node
		testMachine          *machinev1alpha1.Machine
		pendingMachine       *machinev1alpha1.Machine
		seedKubeconfigSecret *corev1.Secret
		gardenClient         client.Client
		shootClient          client.Client
		seedClient           client.Client
		nodePrivateKeyFile   string
		logs                 *util.SafeBytesBuffer
		signalChan           chan os.Signal
	)

	BeforeEach(func() {
		logs = &util.SafeBytesBuffer{}
		klog.SetOutput(logs)
		klog.LogToStderr(false) // must set to false, otherwise klog will log to os.stderr instead of to our buffer

		// all fake bastions are always immediately available
		ssh.SetBastionAvailabilityChecker(func(hostname string, port string, privateKey []byte) error {
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

		ssh.SetCreateSignalChannel(func() chan os.Signal {
			signalChan = make(chan os.Signal, 1)

			return signalChan
		})

		// do not waste time in tests
		ssh.SetPollBastionStatusInterval(1 * time.Second)

		cfg = &config.Config{
			LinkKubeconfig: pointer.Bool(false),
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
				Kubernetes: gardencorev1beta1.Kubernetes{
					Version: "1.20.0", // >= 1.20.0 for non-legacy shoot kubeconfigs
				},
				Provider: gardencorev1beta1.Provider{
					WorkersSettings: &gardencorev1beta1.WorkersSettings{
						SSHAccess: &gardencorev1beta1.SSHAccess{
							Enabled: true,
						},
					},
				},
			},
			Status: gardencorev1beta1.ShootStatus{
				AdvertisedAddresses: []gardencorev1beta1.ShootAdvertisedAddress{
					{
						Name: "shoot-address1",
						URL:  "https://api.bar.baz",
					},
				},
				TechnicalID: "shoot--prod1--test-shoot",
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
		testSeedKubeconfig, err := internalfake.NewConfigData("test-seed")
		Expect(err).ToNot(HaveOccurred())

		seedKubeconfigSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-seed.login",
				Namespace: "garden",
			},
			Data: map[string][]byte{
				"kubeconfig": testSeedKubeconfig,
			},
		}

		csc := &secrets.CertificateSecretConfig{
			Name:       "ca-test",
			CommonName: "ca-test",
			CertType:   secrets.CACert,
		}
		ca, err := csc.GenerateCertificate()
		Expect(err).NotTo(HaveOccurred())

		caConfigMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testShoot.Name + ".ca-cluster",
				Namespace: testShoot.Namespace,
			},
			Data: map[string]string{
				"ca.crt": string(ca.CertificatePEM),
			},
		}

		gardenClient = internalfake.Wrap(
			fakeclient.NewClientBuilder().
				WithObjects(
					testProject,
					testSeed,
					testShoot,
					testShootKeypair,
					seedKubeconfigSecret,
					caConfigMap,
				).
				WithStatusSubresource(&operationsv1alpha1.Bastion{}).
				Build())

		// create a fake shoot cluster with two machines, where one node has already joined the cluster
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

		testMachine = &machinev1alpha1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "machine1",
				Namespace: "shoot--prod1--test-shoot",
				Labels:    map[string]string{machinev1alpha1.NodeLabelKey: "node1"},
			},
		}

		pendingMachine = &machinev1alpha1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "monitoring1",
				Namespace: "shoot--prod1--test-shoot",
				Labels:    map[string]string{machinev1alpha1.NodeLabelKey: "monitoring1"},
			},
		}

		shootClient = internalfake.NewClientWithObjects(testNode)
		seedClient = internalfake.NewClientWithObjects(testMachine, pendingMachine)

		streams, _, out, _ = util.NewTestIOStreams()

		ctrl = gomock.NewController(GinkgoT())

		clientProvider = clientmocks.NewMockProvider(ctrl)
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

	AfterEach(func() {
		cancelTimeout()
		cancel()
		ctrl.Finish()
	})

	Describe("RunE", func() {
		BeforeEach(func() {
			seedClientConfig, err := clientcmd.NewClientConfigFromBytes(seedKubeconfigSecret.Data["kubeconfig"])
			Expect(err).NotTo(HaveOccurred())
			clientProvider.EXPECT().FromClientConfig(gomock.Eq(seedClientConfig)).Return(seedClient, nil).AnyTimes()

			clientProvider.EXPECT().FromClientConfig(gomock.Any()).Return(shootClient, nil).AnyTimes().
				Do(func(clientConfig clientcmd.ClientConfig) {
					config, err := clientConfig.RawConfig()
					Expect(err).NotTo(HaveOccurred())
					Expect(config.CurrentContext).To(Equal(testShoot.Namespace + "--" + testShoot.Name + "-" + testShoot.Status.AdvertisedAddresses[0].Name))
				})
		})

		It("should reject bad options", func() {
			o := ssh.NewSSHOptions(streams)
			cmd := ssh.NewCmdSSH(util.NewFactoryImpl(), o)

			Expect(cmd.RunE(cmd, nil)).NotTo(Succeed())
		})

		It("should print the SSH command and then wait for user interrupt", func() {
			options := ssh.NewSSHOptions(streams)
			cmd := ssh.NewCmdSSH(factory, options)

			go func() {
				defer GinkgoRecover()
				defer func() {
					signalChan <- os.Interrupt
				}()

				// simulate an external controller processing the bastion and proving a successful status
				waitForBastionThenSetBastionReady(ctx, gardenClient, bastionName, *testProject.Spec.Namespace, bastionHostname, bastionIP)

				Eventually(func() bool {
					return strings.Contains(logs.String(), bastionIP)
				}).Should(BeTrue())
			}()

			// let the magic happen
			Expect(cmd.RunE(cmd, nil)).To(Succeed())

			// assert the output
			Expect(logs.String()).To(ContainSubstring(bastionName))
			Expect(logs.String()).To(ContainSubstring(bastionHostname))
			Expect(out.String()).To(ContainSubstring(bastionIP))

			// assert that the bastion has been cleaned up
			key := types.NamespacedName{Name: bastionName, Namespace: *testProject.Spec.Namespace}
			bastion := &operationsv1alpha1.Bastion{}

			Expect(gardenClient.Get(ctx, key, bastion)).NotTo(Succeed())

			// assert that no temporary SSH keypair remained on disk
			_, err := os.Stat(options.SSHPublicKeyFile.String())
			Expect(err).To(HaveOccurred())

			_, err = os.Stat(options.SSHPrivateKeyFile.String())
			Expect(err).To(HaveOccurred())
		})

		It("should connect to a given node", func() {
			options := ssh.NewSSHOptions(streams)
			cmd := ssh.NewCmdSSH(factory, options)

			// simulate an external controller processing the bastion and proving a successful status
			go waitForBastionThenSetBastionReady(ctx, gardenClient, bastionName, *testProject.Spec.Namespace, bastionHostname, bastionIP)

			// do not actually execute any commands
			executedCommands := 0
			ssh.SetExecCommand(func(ctx context.Context, command string, args []string, ioStreams util.IOStreams) error {
				defer func() {
					signalChan <- os.Interrupt
				}()
				executedCommands++

				Expect(command).To(Equal("ssh"))
				Expect(args).To(Equal([]string{
					"-oStrictHostKeyChecking=no",
					"-oIdentitiesOnly=yes",
					fmt.Sprintf("-i%s", nodePrivateKeyFile),
					fmt.Sprintf(
						"-oProxyCommand=ssh -W%%h:%%p -oStrictHostKeyChecking=no -oIdentitiesOnly=yes '-i%s' '%s@%s' '-p22'",
						options.SSHPrivateKeyFile,
						ssh.SSHBastionUsername,
						bastionIP,
					),
					fmt.Sprintf("%s@%s", options.User, nodeHostname),
				}))

				return nil
			})

			// let the magic happen
			Expect(cmd.RunE(cmd, []string{testNode.Name})).To(Succeed())

			// assert output
			Expect(executedCommands).To(Equal(1))
			Expect(logs.String()).To(ContainSubstring(bastionName))
			Expect(logs.String()).To(ContainSubstring(bastionHostname))
			Expect(out.String()).To(ContainSubstring(bastionIP))

			// assert that the bastion has been cleaned up
			key := types.NamespacedName{Name: bastionName, Namespace: *testProject.Spec.Namespace}
			bastion := &operationsv1alpha1.Bastion{}

			Expect(gardenClient.Get(ctx, key, bastion)).NotTo(Succeed())

			// assert that no temporary SSH keypair remained on disk
			_, err := os.Stat(options.SSHPublicKeyFile.String())
			Expect(err).To(HaveOccurred())

			_, err = os.Stat(options.SSHPrivateKeyFile.String())
			Expect(err).To(HaveOccurred())
		})

		It("should connect to a given node that has not yet joined the cluster", func() {
			options := ssh.NewSSHOptions(streams)
			cmd := ssh.NewCmdSSH(factory, options)

			nodeName := "unjoined-node"

			// simulate an external controller processing the bastion and proving a successful status
			go waitForBastionThenSetBastionReady(ctx, gardenClient, bastionName, *testProject.Spec.Namespace, bastionHostname, bastionIP)

			// do not actually execute any commands
			executedCommands := 0
			ssh.SetExecCommand(func(ctx context.Context, command string, args []string, ioStreams util.IOStreams) error {
				defer func() {
					signalChan <- os.Interrupt
				}()
				executedCommands++

				Expect(command).To(Equal("ssh"))
				Expect(args).To(Equal([]string{
					"-oStrictHostKeyChecking=no",
					"-oIdentitiesOnly=yes",
					fmt.Sprintf("-i%s", nodePrivateKeyFile),
					fmt.Sprintf(
						"-oProxyCommand=ssh -W%%h:%%p -oStrictHostKeyChecking=no -oIdentitiesOnly=yes '-i%s' '%s@%s' '-p22'",
						options.SSHPrivateKeyFile,
						ssh.SSHBastionUsername,
						bastionIP,
					),
					fmt.Sprintf("%s@%s", options.User, nodeName),
				}))

				return nil
			})

			// let the magic happen
			Expect(cmd.RunE(cmd, []string{nodeName})).To(Succeed())

			// assert output
			Expect(executedCommands).To(Equal(1))
			Expect(logs.String()).To(ContainSubstring(bastionName))
			Expect(logs.String()).To(ContainSubstring(bastionHostname))
			Expect(out.String()).To(ContainSubstring(bastionIP))
			Expect(logs.String()).To(ContainSubstring("node did not yet join the cluster"))

			// assert that the bastion has been cleaned up
			key := types.NamespacedName{Name: bastionName, Namespace: *testProject.Spec.Namespace}
			bastion := &operationsv1alpha1.Bastion{}

			Expect(gardenClient.Get(ctx, key, bastion)).NotTo(Succeed())

			// assert that no temporary SSH keypair remained on disk
			_, err := os.Stat(options.SSHPublicKeyFile.String())
			Expect(err).To(HaveOccurred())

			_, err = os.Stat(options.SSHPrivateKeyFile.String())
			Expect(err).To(HaveOccurred())
		})

		It("should keep the bastion alive", func() {
			options := ssh.NewSSHOptions(streams)
			options.KeepBastion = true // we need to assert its annotations later

			cmd := ssh.NewCmdSSH(factory, options)

			ssh.SetKeepAliveInterval(50 * time.Millisecond)

			key := types.NamespacedName{Name: bastionName, Namespace: *testProject.Spec.Namespace}

			go func() {
				GinkgoRecover()

				defer func() {
					signalChan <- os.Interrupt
				}()

				// simulate an external controller processing the bastion and proving a successful status
				waitForBastionThenSetBastionReady(ctx, gardenClient, bastionName, *testProject.Spec.Namespace, bastionHostname, bastionIP)

				Eventually(func() bool {
					bastion := &operationsv1alpha1.Bastion{}
					if err := gardenClient.Get(ctx, key, bastion); apierrors.IsNotFound(err) {
						return false
					}

					return bastion.Annotations != nil && bastion.Annotations[corev1beta1constants.GardenerOperation] == corev1beta1constants.GardenerOperationKeepalive
				}).Should(BeTrue())

				// delete the GardenerOperation annotation
				bastion := &operationsv1alpha1.Bastion{}
				Expect(gardenClient.Get(ctx, key, bastion)).To(Succeed())
				delete(bastion.Annotations, corev1beta1constants.GardenerOperation)
				patch := client.MergeFrom(bastion.DeepCopy())
				Expect(gardenClient.Patch(ctx, bastion, patch)).To(Succeed())

				// expect that the keepalive annotation will be added again
				Eventually(func() bool {
					bastion := &operationsv1alpha1.Bastion{}
					if err := gardenClient.Get(ctx, key, bastion); apierrors.IsNotFound(err) {
						return false
					}

					return bastion.Annotations != nil && bastion.Annotations[corev1beta1constants.GardenerOperation] == corev1beta1constants.GardenerOperationKeepalive
				}).Should(BeTrue())

				// wait until connect information is printed to be sure that the command ran through and is just waiting for the user to interrupt
				Eventually(func() bool {
					return strings.Contains(logs.String(), bastionIP)
				}).Should(BeTrue())
			}()

			// let the magic happen
			Expect(cmd.RunE(cmd, nil)).To(Succeed())

			// Double check that the annotation was really set
			bastion := &operationsv1alpha1.Bastion{}
			Expect(gardenClient.Get(ctx, key, bastion)).To(Succeed())
			Expect(bastion.Annotations).To(HaveKeyWithValue(corev1beta1constants.GardenerOperation, corev1beta1constants.GardenerOperationKeepalive))
		})

		It("should stop keepalive when bastion is deleted ", func() {
			options := ssh.NewSSHOptions(streams)
			options.KeepBastion = true // we need to assert its annotations later

			cmd := ssh.NewCmdSSH(factory, options)

			// simulate an external controller processing the bastion and proving a successful status
			go waitForBastionThenSetBastionReady(ctx, gardenClient, bastionName, *testProject.Spec.Namespace, bastionHostname, bastionIP)

			// end the test after a couple of seconds (enough seconds for the keep-alive
			// goroutine to do its thing)
			ssh.SetKeepAliveInterval(100 * time.Millisecond)
			signalChan := make(chan os.Signal, 1)
			ssh.SetCreateSignalChannel(func() chan os.Signal {
				return signalChan
			})

			// Once the waitForSignal function is called we delete the bastion
			ssh.SetWaitForSignal(func(ctx context.Context, o *ssh.SSHOptions, signalChan <-chan struct{}) {
				By("deleting bastion")
				bastion := &operationsv1alpha1.Bastion{}
				key := types.NamespacedName{Name: bastionName, Namespace: *testProject.Spec.Namespace}
				Expect(gardenClient.Get(ctx, key, bastion)).To(Succeed())

				Expect(gardenClient.Delete(ctx, bastion)).To(Succeed())

				<-signalChan
			})

			// let the magic happen
			Expect(cmd.RunE(cmd, nil)).To(Succeed())

			Expect(logs.String()).To(ContainSubstring("Can't keep bastion alive. Bastion is already gone."))
		})

		It("should skip the availability check", func() {
			options := ssh.NewSSHOptions(streams)
			options.SkipAvailabilityCheck = true

			cmd := ssh.NewCmdSSH(factory, options)

			ssh.SetBastionAvailabilityChecker(func(hostname string, port string, privateKey []byte) error {
				err := errors.New("this function should not be executed as of SkipAvailabilityCheck = true")
				Fail(err.Error())
				return err
			})

			// simulate an external controller processing the bastion and proving a successful status
			go waitForBastionThenSetBastionReady(ctx, gardenClient, bastionName, *testProject.Spec.Namespace, bastionHostname, bastionIP)

			Expect(cmd.RunE(cmd, nil)).To(Succeed())

			Expect(logs.String()).To(ContainSubstring("Bastion is ready, skipping availability check"))
		})

		It("should not keep alive the bastion", func() {
			options := ssh.NewSSHOptions(streams)
			options.NoKeepalive = true
			options.KeepBastion = true
			options.Interactive = false

			cmd := ssh.NewCmdSSH(factory, options)

			ssh.SetWaitForSignal(func(ctx context.Context, o *ssh.SSHOptions, signalChan <-chan struct{}) {
				Fail("this function should not be executed as of NoKeepalive = true")
			})
			ssh.SetExecCommand(func(ctx context.Context, command string, args []string, ioStreams util.IOStreams) error {
				err := errors.New("this function should not be executed as of NoKeepalive = true")
				Fail(err.Error())
				return err
			})

			// simulate an external controller processing the bastion and proving a successful status
			go waitForBastionThenSetBastionReady(ctx, gardenClient, bastionName, *testProject.Spec.Namespace, bastionHostname, bastionIP)

			Expect(cmd.RunE(cmd, nil)).To(Succeed())

			Expect(logs.String()).To(ContainSubstring("Bastion host became available."))
		})

		It("should output as json", func() {
			options := ssh.NewSSHOptions(streams)
			options.NoKeepalive = true
			options.KeepBastion = true
			options.Interactive = false

			options.Output = "json"

			cmd := ssh.NewCmdSSH(factory, options)

			ssh.SetWaitForSignal(func(ctx context.Context, o *ssh.SSHOptions, signalChan <-chan struct{}) {
				Fail("this function should not be executed as of NoKeepalive = true")
			})
			ssh.SetExecCommand(func(ctx context.Context, command string, args []string, ioStreams util.IOStreams) error {
				err := errors.New("this function should not be executed as of NoKeepalive = true")
				Fail(err.Error())
				return err
			})

			// simulate an external controller processing the bastion and proving a successful status
			go waitForBastionThenSetBastionReady(ctx, gardenClient, bastionName, *testProject.Spec.Namespace, bastionHostname, bastionIP)

			Expect(cmd.RunE(cmd, nil)).To(Succeed())

			var info ssh.ConnectInformation
			Expect(json.Unmarshal([]byte(out.String()), &info)).To(Succeed())
			Expect(info.Bastion.Name).To(Equal(bastionName))
			Expect(info.Bastion.PreferredAddress).To(Equal("0.0.0.0"))
			Expect(info.Bastion.SSHPrivateKeyFile).To(Equal(options.SSHPrivateKeyFile))
			Expect(info.Bastion.SSHPublicKeyFile).To(Equal(options.SSHPublicKeyFile))
			Expect(info.Nodes).To(ConsistOf([]ssh.Node{
				{
					Name:   pendingMachine.Labels[machinev1alpha1.NodeLabelKey],
					Status: "Unknown",
				},
				{
					Name:   testNode.Name,
					Status: "Not Ready",
					Address: ssh.Address{
						Hostname: nodeHostname,
					},
				},
			}))
			Expect(info.NodePrivateKeyFiles).NotTo(BeEmpty())
		})

		It("should return an error when SSHAccess is disabled", func() {
			options := ssh.NewSSHOptions(streams)
			cmd := ssh.NewCmdSSH(factory, options)

			testShootBase := testShoot.DeepCopy()
			testShoot.Spec.Provider.WorkersSettings.SSHAccess.Enabled = false
			Expect(gardenClient.Patch(ctx, testShoot, client.MergeFrom(testShootBase))).To(Succeed())

			Expect(cmd.RunE(cmd, nil)).To(MatchError("node SSH access disabled, SSH not allowed"))
		})
	})

	Describe("ValidArgsFunction", func() {
		var (
			manager *targetmocks.MockManager
			client  *gardenclientmocks.MockClient
		)

		BeforeEach(func() {
			manager = targetmocks.NewMockManager(ctrl)
			client = gardenclientmocks.NewMockClient(ctrl)

			factory.ManagerImpl = manager
			manager.EXPECT().CurrentTarget().Return(currentTarget, nil)
			manager.EXPECT().GardenClient(currentTarget.GardenName()).Return(client, nil)

			client.EXPECT().FindShoot(ctx, currentTarget.AsListOption()).Return(testShoot, nil)
		})

		It("should return all names based on machine objects", func() {
			manager.EXPECT().SeedClient(ctx, gomock.Any()).Return(seedClient, nil)

			options := ssh.NewSSHOptions(streams)
			cmd := ssh.NewCmdSSH(factory, options)

			suggestions, directive := cmd.ValidArgsFunction(cmd, nil, "")
			Expect(directive).To(Equal(cobra.ShellCompDirectiveNoFileComp))
			Expect(suggestions).To(HaveLen(2))
			Expect(suggestions).To(Equal([]string{"node1", "monitoring1"}))
		})

		It("should return all names based on node objects", func() {
			errForbidden := &apierrors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonForbidden}}
			manager.EXPECT().SeedClient(ctx, gomock.Any()).Return(nil, errForbidden)
			manager.EXPECT().ShootClient(ctx, currentTarget).Return(shootClient, nil)

			options := ssh.NewSSHOptions(streams)
			cmd := ssh.NewCmdSSH(factory, options)

			suggestions, directive := cmd.ValidArgsFunction(cmd, nil, "")
			Expect(directive).To(Equal(cobra.ShellCompDirectiveNoFileComp))
			Expect(suggestions).To(HaveLen(1))
			Expect(suggestions).To(Equal([]string{"node1"}))
		})

		It("should find nodes based on their prefix from machine objects", func() {
			manager.EXPECT().SeedClient(ctx, gomock.Any()).Return(seedClient, nil)

			options := ssh.NewSSHOptions(streams)
			cmd := ssh.NewCmdSSH(factory, options)

			suggestions, directive := cmd.ValidArgsFunction(cmd, nil, "mon")
			Expect(directive).To(Equal(cobra.ShellCompDirectiveNoFileComp))
			Expect(suggestions).To(HaveLen(1))
			Expect(suggestions).To(Equal([]string{"monitoring1"}))
		})
	})
})

var _ = Describe("SSH Options", func() {
	var (
		streams util.IOStreams
		o       *ssh.SSHOptions
	)

	BeforeEach(func() {
		streams, _, _, _ = util.NewTestIOStreams()

		o = ssh.NewSSHOptions(streams)
	})

	Describe("Complete", func() {
		var factory *internalfake.Factory
		BeforeEach(func() {
			factory = internalfake.NewFakeFactory(nil, nil, nil, nil)
		})

		AfterEach(func() {
			Expect(os.Remove(o.SSHPublicKeyFile.String())).To(Succeed())
			Expect(os.Remove(o.SSHPrivateKeyFile.String())).To(Succeed())
		})

		It("should complete node name", func() {
			Expect(o.Complete(factory, nil, []string{"my-node"})).To(Succeed())

			Expect(o.NodeName).To(Equal("my-node"))
		})

		It("should complete public and private key", func() {
			Expect(o.Complete(factory, nil, nil)).To(Succeed())

			Expect(o.SSHPublicKeyFile).NotTo(BeEmpty())
			Expect(o.SSHPrivateKeyFile).NotTo(BeEmpty())
			Expect(o.GeneratedSSHKeys).To(BeTrue())
		})

		It("should complete bastion name", func() {
			Expect(o.Complete(factory, nil, nil)).To(Succeed())

			Expect(o.BastionName).To(Not(BeEmpty()))
		})

		It("should not touch bastion name if set", func() {
			o.BastionName = "cli-xxxxxx"

			Expect(o.Complete(factory, nil, nil)).To(Succeed())

			Expect(o.BastionName).To(Equal("cli-xxxxxx"))
		})

		It("should switch to non-interactive mode if no node name given", func() {
			o.Interactive = true

			Expect(o.Complete(factory, nil, []string{""})).To(Succeed())

			Expect(o.Interactive).To(BeFalse())
		})
	})

	Describe("Validate", func() {
		var publicSSHKeyFile ssh.PublicKeyFile

		BeforeEach(func() {
			tmpFile, err := os.CreateTemp("", "")
			Expect(err).NotTo(HaveOccurred())
			defer tmpFile.Close()

			// write dummy SSH public key
			_, err = io.WriteString(tmpFile, "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDouNkxsNuApuKVIfgL6Yz3Ep+DqX84Yde9DArwLBSWgLnl/pH9AbbcDcAmdB2CPVXAATo4qxK7xprvyyZp52SQRCcAZpAy4D6gAWwAG3OfzrRbxRiB5pQDaaWATSzNbLtoy0ecVwFeTJe2w71q+wxbI7tfxbvo9XbXIN4I0cQy2KLICzkYkQmygGnHztv1Mvi338+sgcG7Gwq2tdSyggDaAggwDIuT39S4/L7QpR27tWH79J4Ls8tTHud2eRbkOcF98vXlQAIzb6w8iHBXylOjMM/oODwoA7V4mtRL9o13AoocvZSsD1UvfOjGxDHuLrCfFXN+/rEw0hEiYo0cnj7F")
			Expect(err).NotTo(HaveOccurred())

			publicSSHKeyFile = ssh.PublicKeyFile(tmpFile.Name())

			o.CIDRs = []string{"8.8.8.8/32"}
			o.SSHPublicKeyFile = publicSSHKeyFile
		})

		AfterEach(func() {
			Expect(os.Remove(publicSSHKeyFile.String())).To(Succeed())
		})

		It("should validate", func() {
			Expect(o.Validate()).To(Succeed())
		})

		It("should require a non-zero wait time", func() {
			o.WaitTimeout = 0

			Expect(o.Validate()).NotTo(Succeed())
		})

		Context("no-keepalive", func() {
			BeforeEach(func() {
				o.NoKeepalive = true

				o.KeepBastion = true
				o.Interactive = false
			})

			It("should validate", func() {
				Expect(o.Validate()).Should(Succeed())
			})

			It("should require non-interactive mode", func() {
				o.Interactive = true

				Expect(o.Validate()).NotTo(Succeed())
			})

			It("should require keep bastion", func() {
				o.KeepBastion = false

				Expect(o.Validate()).NotTo(Succeed())
			})
		})

		Describe("output flag not empty", func() {
			BeforeEach(func() {
				o.Output = "yaml" // or json - does not matter

				o.Interactive = false
			})

			It("should validate", func() {
				Expect(o.Validate()).Should(Succeed())
			})

			It("should require non-interactive mode", func() {
				o.Interactive = true

				Expect(o.Validate()).NotTo(Succeed())
			})
		})
		It("should require a public SSH key file", func() {
			o := ssh.NewSSHOptions(streams)
			o.CIDRs = []string{"8.8.8.8/32"}

			Expect(o.Validate()).NotTo(Succeed())
		})

		It("should require a valid public SSH key file", func() {
			Expect(os.WriteFile(publicSSHKeyFile.String(), []byte("not a key"), 0o644)).To(Succeed())

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
})
