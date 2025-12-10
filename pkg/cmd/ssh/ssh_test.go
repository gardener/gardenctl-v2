/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package ssh_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
	cryptossh "golang.org/x/crypto/ssh"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
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
		bastionIP            = "192.0.2.1"
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
		gardenHomeDir        string
		gardenTempDir        string
	)

	BeforeEach(func() {
		logs = &util.SafeBytesBuffer{}
		klog.SetOutput(logs)
		klog.LogToStderr(false) // must set to false, otherwise klog will log to os.stderr instead of to our buffer

		// all fake bastions are always immediately available
		ssh.SetBastionAvailabilityChecker(func(hostname string, port string, privateKey []byte, hostKeyCallback cryptossh.HostKeyCallback) error {
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

		ssh.SetPollBastionStatusInterval(100 * time.Millisecond)
		ssh.SetKeepAliveInterval(200 * time.Millisecond)
		ssh.SetRSAKeyBitsForTest(1024)

		cfg = &config.Config{
			LinkKubeconfig: ptr.To(false),
			Gardens: []config.Garden{{
				Name:       gardenName,
				Kubeconfig: gardenKubeconfigFile,
			}},
		}

		testProject = &gardencorev1beta1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name: "prod1",
				UID:  "00000000-0000-0000-0000-000000000000",
			},
			Spec: gardencorev1beta1.ProjectSpec{
				Namespace: ptr.To("garden-prod1"),
			},
		}

		testSeed = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-seed",
				UID:  "00000000-0000-0000-0000-000000000000",
			},
		}

		testShoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-shoot",
				Namespace: *testProject.Spec.Namespace,
				UID:       "00000000-0000-0000-0000-000000000000",
			},
			Spec: gardencorev1beta1.ShootSpec{
				SeedName: ptr.To(testSeed.Name),
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
						Name: "external",
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
				UID:       "00000000-0000-0000-0000-000000000000",
			},
		}

		privateKey, err := rsa.GenerateKey(rand.Reader, 1024)
		Expect(err).NotTo(HaveOccurred())

		privateKeyPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
		})

		testShootKeypair.Data = map[string][]byte{
			secrets.DataKeyRSAPrivateKey: privateKeyPEM,
		}

		testSeedKubeconfig, err := internalfake.NewConfigData("test-seed")
		Expect(err).ToNot(HaveOccurred())

		seedKubeconfigSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-seed.login",
				Namespace: "garden",
				UID:       "00000000-0000-0000-0000-000000000000",
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
				UID:       "00000000-0000-0000-0000-000000000000",
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
				UID:  "00000000-0000-0000-0000-000000000000",
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
				UID:       "00000000-0000-0000-0000-000000000000",
				Labels:    map[string]string{machinev1alpha1.NodeLabelKey: "node1"},
			},
		}

		pendingMachine = &machinev1alpha1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "monitoring1",
				Namespace: "shoot--prod1--test-shoot",
				UID:       "00000000-0000-0000-0000-000000000000",
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

		gardenHomeDir, err = os.MkdirTemp("", "garden-home-*")
		Expect(err).ToNot(HaveOccurred())
		factory.GardenHomeDirectory = gardenHomeDir

		// Create a temporary directory for GardenTempDirectory
		gardenTempDir, err = os.MkdirTemp("", "garden-temp-*")
		Expect(err).ToNot(HaveOccurred())
		factory.GardenTempDirectory = gardenTempDir
	})

	AfterEach(func() {
		cancelTimeout()
		cancel()
		ctrl.Finish()

		// Remove the temporary directories
		Expect(os.RemoveAll(gardenHomeDir)).To(Succeed())
		Expect(os.RemoveAll(gardenTempDir)).To(Succeed())
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

			bastionKey := client.ObjectKey{Name: bastionName, Namespace: *testProject.Spec.Namespace}

			// do not actually execute any commands
			executedCommands := 0
			ssh.SetExecCommand(func(ctx context.Context, command string, args []string, ioStreams util.IOStreams) error {
				defer func() {
					signalChan <- os.Interrupt
				}()
				executedCommands++

				// Retrieve the bastion object to get its UID
				bastion := &operationsv1alpha1.Bastion{}
				Expect(gardenClient.Get(ctx, bastionKey, bastion)).To(Succeed())

				bastionUID := string(bastion.UID)
				shootUID := string(testShoot.UID)

				defaultBastionKnownHostsFile := filepath.Join(gardenTempDir, "cache", bastionUID, ".ssh", "known_hosts")
				defaultNodeKnownHostsFile := filepath.Join(gardenHomeDir, "cache", shootUID, ".ssh", "known_hosts")

				Expect(command).To(Equal("ssh"))
				Expect(args).To(Equal([]string{
					"-oIdentitiesOnly=yes",
					"-oStrictHostKeyChecking=ask",
					fmt.Sprintf("-oUserKnownHostsFile='%s'", defaultNodeKnownHostsFile),
					fmt.Sprintf("-i%s", nodePrivateKeyFile),
					fmt.Sprintf(
						"-oProxyCommand=ssh '-W[%%h]:%%p' -oStrictHostKeyChecking=ask -oIdentitiesOnly=yes '-i%s' '-oUserKnownHostsFile='\"'\"'%s'\"'\"'' '%s@%s' '-p22'",
						options.SSHPrivateKeyFile,
						defaultBastionKnownHostsFile,
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
			bastion := &operationsv1alpha1.Bastion{}
			Expect(gardenClient.Get(ctx, bastionKey, bastion)).NotTo(Succeed())

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

			// Retrieve the bastion object to get its UID
			bastionKey := client.ObjectKey{Name: bastionName, Namespace: *testProject.Spec.Namespace}

			// do not actually execute any commands
			executedCommands := 0
			ssh.SetExecCommand(func(ctx context.Context, command string, args []string, ioStreams util.IOStreams) error {
				defer func() {
					signalChan <- os.Interrupt
				}()
				executedCommands++

				bastion := &operationsv1alpha1.Bastion{}
				Expect(gardenClient.Get(ctx, bastionKey, bastion)).To(Succeed())

				bastionUID := string(bastion.UID)
				shootUID := string(testShoot.UID)

				defaultBastionKnownHostsFile := filepath.Join(gardenTempDir, "cache", bastionUID, ".ssh", "known_hosts")
				defaultNodeKnownHostsFile := filepath.Join(gardenHomeDir, "cache", shootUID, ".ssh", "known_hosts")

				Expect(command).To(Equal("ssh"))
				Expect(args).To(Equal([]string{
					"-oIdentitiesOnly=yes",
					"-oStrictHostKeyChecking=ask",
					fmt.Sprintf("-oUserKnownHostsFile='%s'", defaultNodeKnownHostsFile),
					fmt.Sprintf("-i%s", nodePrivateKeyFile),
					fmt.Sprintf(
						"-oProxyCommand=ssh '-W[%%h]:%%p' -oStrictHostKeyChecking=ask -oIdentitiesOnly=yes '-i%s' '-oUserKnownHostsFile='\"'\"'%s'\"'\"'' '%s@%s' '-p22'",
						options.SSHPrivateKeyFile,
						defaultBastionKnownHostsFile,
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
			bastion := &operationsv1alpha1.Bastion{}
			Expect(gardenClient.Get(ctx, bastionKey, bastion)).NotTo(Succeed())

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

			ssh.SetBastionAvailabilityChecker(func(hostname string, port string, privateKey []byte, hostKeyCallback cryptossh.HostKeyCallback) error {
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
			Expect(info.Bastion.PreferredAddress).To(Equal(bastionIP))
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

		It("should use custom known hosts files when provided", func() {
			options := ssh.NewSSHOptions(streams)
			cmd := ssh.NewCmdSSH(factory, options)

			// Set custom known hosts files
			options.BastionUserKnownHostsFiles = []string{"/custom/bastion/known_hosts"}
			options.NodeUserKnownHostsFiles = []string{"/custom/node/known_hosts"}

			// Simulate the bastion being ready
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
					"-oIdentitiesOnly=yes",
					"-oStrictHostKeyChecking=ask",
					"-oUserKnownHostsFile='/custom/node/known_hosts'",
					fmt.Sprintf("-i%s", nodePrivateKeyFile),
					fmt.Sprintf(
						"-oProxyCommand=ssh '-W[%%h]:%%p' -oStrictHostKeyChecking=ask -oIdentitiesOnly=yes '-i%s' '-oUserKnownHostsFile='\"'\"'/custom/bastion/known_hosts'\"'\"'' '%s@%s' '-p22'",
						options.SSHPrivateKeyFile,
						ssh.SSHBastionUsername,
						bastionIP,
					),
					fmt.Sprintf("%s@%s", options.User, nodeHostname),
				}))

				return nil
			})

			// Execute the command
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

		DescribeTable("user validation - valid cases",
			func(user string) {
				o.User = user
				Expect(o.Validate()).To(Succeed())
			},
			Entry("user with exactly maximum length (32 chars)", "abcdefghijklmnopqrstuvwxyz123456"),
			Entry("user starting with lowercase letter", "gardener"),
			Entry("user starting with underscore", "_gardener"),
			Entry("user with lowercase letters, digits, underscores, and hyphens", "user_name-123"),
			Entry("user with all valid characters", "a0_-z9"),
			Entry("user with single character", "a"),
			Entry("user with underscore only", "_"),
		)

		DescribeTable("user validation - invalid cases",
			func(user string, expectedError string) {
				o.User = user
				Expect(o.Validate()).To(MatchError(expectedError))
			},
			Entry("empty user", "", "user must not be empty"),
			Entry("user exceeding maximum length (33 chars)", "abcdefghijklmnopqrstuvwxyz1234567", fmt.Sprintf("user must not exceed %d characters", ssh.MaxUsernameLength)),
			Entry("user starting with uppercase letter", "Gardener", "user must start with a lowercase letter or underscore, followed by lowercase letters, digits, underscores, or hyphens"),
			Entry("user starting with digit", "1user", "user must start with a lowercase letter or underscore, followed by lowercase letters, digits, underscores, or hyphens"),
			Entry("user with @ character", "user@domain", "user must start with a lowercase letter or underscore, followed by lowercase letters, digits, underscores, or hyphens"),
			Entry("user with spaces", "user name", "user must start with a lowercase letter or underscore, followed by lowercase letters, digits, underscores, or hyphens"),
			Entry("user with dot", "user.name", "user must start with a lowercase letter or underscore, followed by lowercase letters, digits, underscores, or hyphens"),
			Entry("user starting with hyphen", "-user", "user must start with a lowercase letter or underscore, followed by lowercase letters, digits, underscores, or hyphens"),
			Entry("user with uppercase letters in middle", "userNAME", "user must start with a lowercase letter or underscore, followed by lowercase letters, digits, underscores, or hyphens"),
		)

		DescribeTable("bastion port validation - valid cases",
			func(port string) {
				o.BastionPort = port
				Expect(o.Validate()).To(Succeed())
			},
			Entry("empty bastion port", ""),
			Entry("valid bastion port 22", "22"),
			Entry("minimum valid port 1", "1"),
			Entry("maximum valid port 65535", "65535"),
			Entry("common SSH port 2222", "2222"),
		)

		DescribeTable("bastion port validation - invalid cases",
			func(port string, expectedError string) {
				o.BastionPort = port
				Expect(o.Validate()).To(MatchError(expectedError))
			},
			Entry("port 0", "0", "bastion port must be a valid port number between 1 and 65535"),
			Entry("port exceeding maximum 65536", "65536", "bastion port must be a valid port number between 1 and 65535"),
			Entry("negative port", "-1", "bastion port must be a valid port number between 1 and 65535"),
			Entry("non-numeric port", "abc", "bastion port must be a valid port number between 1 and 65535"),
			Entry("port with spaces", "22 ", "bastion port must be a valid port number between 1 and 65535"),
			Entry("empty string with spaces", " ", "bastion port must be a valid port number between 1 and 65535"),
		)

		DescribeTable("bastion name validation - valid cases",
			func(name string) {
				o.BastionName = name
				Expect(o.Validate()).To(Succeed())
			},
			Entry("empty bastion name", ""),
			Entry("valid bastion name", "test-bastion"),
			Entry("bastion name with lowercase letters and hyphens", "my-test-bastion-123"),
			Entry("bastion name with single character", "a"),
			Entry("bastion name with digits only", "123"),
			Entry("bastion name at maximum length (63 chars)", "a12345678901234567890123456789012345678901234567890123456789012"),
		)

		DescribeTable("bastion name validation - invalid cases",
			func(name string) {
				o.BastionName = name
				Expect(o.Validate()).To(MatchError(ContainSubstring("bastion name is invalid")))
			},
			Entry("bastion name starting with hyphen", "-bastion"),
			Entry("bastion name ending with hyphen", "bastion-"),
			Entry("bastion name with uppercase letters", "TestBastion"),
			Entry("bastion name with dots", "test.bastion"),
			Entry("bastion name exceeding DNS label length (64 chars)", "a123456789012345678901234567890123456789012345678901234567890123"),
			Entry("bastion name with underscore", "test_bastion"),
			Entry("bastion name with special characters", "test@bastion"),
			Entry("bastion name with spaces", "test bastion"),
		)

		DescribeTable("node name validation - valid cases",
			func(name string) {
				o.NodeName = name
				Expect(o.Validate()).To(Succeed())
			},
			Entry("empty node name", ""),
			Entry("valid node name", "my-node"),
			Entry("node name with lowercase letters and hyphens", "my-test-node-123"),
			Entry("node name with single character", "a"),
			Entry("node name with digits only", "123"),
			Entry("node name with dots (DNS subdomain)", "node.my-cluster.local"),
			Entry("IPv4 address", "192.168.1.1"),
			Entry("IPv4 loopback", "127.0.0.1"),
			Entry("IPv6 address", "2001:db8::1"),
			Entry("IPv6 loopback", "::1"),
			Entry("IPv6 full form", "2001:0db8:0000:0000:0000:0000:0000:0001"),
			Entry("AWS-style private DNS hostname", "ip-10-0-1-42.ec2.internal"),
			Entry("GCP-style private DNS hostname", "10-0-1-42.us-central1-b.c.my-project.internal"),
			Entry("Azure-style VM hostname", "aks-nodepool1-12345678-vmss000001"),
		)

		DescribeTable("node name validation - invalid cases",
			func(name string) {
				o.NodeName = name
				err := o.Validate()
				Expect(err).To(MatchError(ContainSubstring("invalid node name: does not conform to DNS naming rules")))
			},
			Entry("node name starting with hyphen", "-node"),
			Entry("node name ending with hyphen", "node-"),
			Entry("node name with uppercase letters", "MyNode"),
			Entry("node name with underscore", "my_node"),
			Entry("node name with special characters", "node@test"),
			Entry("node name with spaces", "my node"),
		)

		DescribeTable("bastion host validation - valid cases",
			func(host string) {
				o.BastionHost = host
				Expect(o.Validate()).To(Succeed())
			},
			Entry("empty bastion host", ""),
			Entry("valid IP", "192.0.2.1"),
			Entry("valid hostname", "bastion.example.com"),
			Entry("loopback IP", "127.0.0.1"),
			Entry("localhost", "localhost"),
			Entry("link-local IP", "169.254.0.1"),
		)

		DescribeTable("bastion host validation - invalid cases",
			func(host string, expectedError string) {
				o.BastionHost = host
				Expect(o.Validate()).To(MatchError(ContainSubstring(expectedError)))
			},
			Entry("unspecified IP", "0.0.0.0", "unspecified addresses are not allowed"),
			Entry("multicast IP", "224.0.0.1", "multicast addresses are not allowed"),
			Entry("hostname containing semicolon", "bastion;example.com", "does not conform to DNS naming rules"),
			Entry("hostname not conforming to DNS rules", "INVALID_HOST", "does not conform to DNS naming rules"),
		)
	})

	Describe("ValidateHost", func() {
		DescribeTable("valid cases",
			func(host string) {
				Expect(ssh.ValidateHost(host)).To(Succeed())
			},
			Entry("valid IPv4", "192.0.2.1"),
			Entry("valid IPv6", "2001:db8::1"),
			Entry("valid hostname", "foo.example.com"),
			Entry("loopback IPv4", "127.0.0.1"),
			Entry("loopback IPv6", "::1"),
			Entry("localhost", "localhost"),
			Entry("link-local IP", "169.254.0.1"),
		)

		DescribeTable("invalid cases",
			func(host string, expectedError string) {
				Expect(ssh.ValidateHost(host)).To(MatchError(ContainSubstring(expectedError)))
			},
			Entry("unspecified IPv4", "0.0.0.0", "unspecified addresses are not allowed"),
			Entry("unspecified IPv6", "::", "unspecified addresses are not allowed"),
			Entry("multicast IPv4", "224.0.0.1", "multicast addresses are not allowed"),
			Entry("multicast IPv6", "ff02::1", "multicast addresses are not allowed"),
			Entry("hostname containing semicolon", "bastion;example.com", "does not conform to DNS naming rules"),
			Entry("hostname with uppercase", "INVALID_HOST", "does not conform to DNS naming rules"),
			Entry("hostname with underscore", "invalid_host", "does not conform to DNS naming rules"),
			Entry("hostname with spaces", "invalid host", "does not conform to DNS naming rules"),
		)
	})

	Context("SSH key validation", Ordered, func() {
		var (
			privateKey       *rsa.PrivateKey
			originalKeyBytes []byte
		)

		// Generate key once for all SSH validation tests to improve performance
		BeforeAll(func() {
			var err error
			privateKey, err = rsa.GenerateKey(rand.Reader, 2048)
			Expect(err).NotTo(HaveOccurred())

			originalKeyBytes = x509.MarshalPKCS1PrivateKey(privateKey)
		})

		Describe("ValidateSSHPrivateKey", Ordered, func() {
			var (
				pemBlock         *pem.Block
				pemBytes         []byte
				originalPemBytes []byte
			)

			BeforeAll(func() {
				pemBlockTemp := &pem.Block{
					Type:  "RSA PRIVATE KEY",
					Bytes: originalKeyBytes,
				}
				originalPemBytes = pem.EncodeToMemory(pemBlockTemp)
			})

			BeforeEach(func() {
				// Create fresh pemBlock for each test (some tests mutate it)
				keyBytesCopy := make([]byte, len(originalKeyBytes))
				copy(keyBytesCopy, originalKeyBytes)
				pemBlock = &pem.Block{
					Type:  "RSA PRIVATE KEY",
					Bytes: keyBytesCopy,
				}
				// Copy for tests that append to it
				pemBytes = make([]byte, len(originalPemBytes))
				copy(pemBytes, originalPemBytes)
			})

			It("should succeed for a valid private key", func() {
				Expect(ssh.ValidateSSHPrivateKey(pemBytes)).To(Succeed())
			})

			It("should succeed for a valid private key with trailing whitespace", func() {
				keyWithWhitespace := append(pemBytes, []byte(" \n\t")...)
				Expect(ssh.ValidateSSHPrivateKey(keyWithWhitespace)).To(Succeed())
			})

			It("should fail if there is unexpected data after the END line", func() {
				keyWithGarbage := append(pemBytes, []byte("garbage")...)
				Expect(ssh.ValidateSSHPrivateKey(keyWithGarbage)).To(MatchError("private key must contain exactly one PEM block (unexpected data after END line)"))
			})

			It("should fail if the private key does not start with a PEM BEGIN line", func() {
				keyWithGarbage := append([]byte("foo\n-----BEGIN "), pemBytes...)
				Expect(ssh.ValidateSSHPrivateKey(keyWithGarbage)).To(MatchError("private key must start with a PEM BEGIN line"))
			})

			It("should fail if the PEM block includes headers", func() {
				pemBlockWithHeaders := &pem.Block{
					Type:    "RSA PRIVATE KEY",
					Headers: map[string]string{"Header": "Value"},
					Bytes:   pemBlock.Bytes,
				}
				keyWithHeaders := pem.EncodeToMemory(pemBlockWithHeaders)
				Expect(ssh.ValidateSSHPrivateKey(keyWithHeaders)).To(MatchError("private key must not include PEM headers"))
			})

			It("should fail if the private key is not a valid PEM-encoded key", func() {
				Expect(ssh.ValidateSSHPrivateKey([]byte("invalid-pem"))).To(MatchError("private key must start with a PEM BEGIN line"))
			})

			It("should fail if the private key cannot be parsed as SSH key", func() {
				// corrupt the key bytes
				pemBlock.Bytes[0] ^= 0xFF
				invalidKey := pem.EncodeToMemory(pemBlock)
				Expect(ssh.ValidateSSHPrivateKey(invalidKey)).To(MatchError(ContainSubstring("private key cannot be parsed as SSH key")))
			})
		})

		Describe("ValidateSSHPublicKey", Ordered, func() {
			var (
				publicKeyFile         []byte
				originalPublicKeyFile []byte
			)

			BeforeAll(func() {
				sshPublicKey, err := cryptossh.NewPublicKey(&privateKey.PublicKey)
				Expect(err).NotTo(HaveOccurred())

				originalPublicKeyFile = cryptossh.MarshalAuthorizedKey(sshPublicKey)
			})

			BeforeEach(func() {
				// Copy for tests that append to it
				publicKeyFile = make([]byte, len(originalPublicKeyFile))
				copy(publicKeyFile, originalPublicKeyFile)
			})

			It("should succeed for a valid RSA public key", func() {
				Expect(ssh.ValidateSSHPublicKey(publicKeyFile)).To(Succeed())
			})

			It("should succeed for a valid public key with trailing whitespace", func() {
				keyWithWhitespace := append(publicKeyFile, []byte(" \n\t")...)
				Expect(ssh.ValidateSSHPublicKey(keyWithWhitespace)).To(Succeed())
			})

			It("should fail if the public key file is empty", func() {
				Expect(ssh.ValidateSSHPublicKey([]byte(""))).To(MatchError("public key cannot be parsed: ssh: no key found"))
			})

			It("should fail if the public key file contains only whitespace", func() {
				Expect(ssh.ValidateSSHPublicKey([]byte("  \n\t  "))).To(MatchError("public key cannot be parsed: ssh: no key found"))
			})

			It("should fail if there is unexpected data after the first key", func() {
				keyWithGarbage := append(publicKeyFile, []byte("garbage data here")...)
				Expect(ssh.ValidateSSHPublicKey(keyWithGarbage)).To(MatchError("public key must contain exactly one key (unexpected data after first key)"))
			})

			It("should fail if the public key cannot be parsed", func() {
				invalidKey := []byte("ssh-rsa invalid-base64-data")
				Expect(ssh.ValidateSSHPublicKey(invalidKey)).To(MatchError(ContainSubstring("public key cannot be parsed")))
			})

			It("should succeed for a valid ED25519 public key", func() {
				// Generate an ED25519 key
				_, edPrivateKey, err := ed25519.GenerateKey(rand.Reader)
				Expect(err).NotTo(HaveOccurred())

				sshPublicKey, err := cryptossh.NewPublicKey(edPrivateKey.Public())
				Expect(err).NotTo(HaveOccurred())

				edPublicKeyFile := cryptossh.MarshalAuthorizedKey(sshPublicKey)
				Expect(ssh.ValidateSSHPublicKey(edPublicKeyFile)).To(Succeed())
			})

			It("should succeed for a valid ECDSA public key", func() {
				// Generate an ECDSA key
				ecPrivateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
				Expect(err).NotTo(HaveOccurred())

				sshPublicKey, err := cryptossh.NewPublicKey(&ecPrivateKey.PublicKey)
				Expect(err).NotTo(HaveOccurred())

				ecPublicKeyFile := cryptossh.MarshalAuthorizedKey(sshPublicKey)
				Expect(ssh.ValidateSSHPublicKey(ecPublicKeyFile)).To(Succeed())
			})
		})
	})
})
