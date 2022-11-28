/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package ssh_test

import (
	"context"
	"reflect"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardenoperationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	gardensecrets "github.com/gardener/gardener/pkg/utils/secrets"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gc "github.com/gardener/gardenctl-v2/internal/gardenclient"
	gcmocks "github.com/gardener/gardenctl-v2/internal/gardenclient/mocks"
	"github.com/gardener/gardenctl-v2/internal/util"
	utilmocks "github.com/gardener/gardenctl-v2/internal/util/mocks"
	"github.com/gardener/gardenctl-v2/pkg/cmd/ssh"
	sshutilmocks "github.com/gardener/gardenctl-v2/pkg/cmd/ssh/mocks"
	"github.com/gardener/gardenctl-v2/pkg/target"
	targetmocks "github.com/gardener/gardenctl-v2/pkg/target/mocks"
)

var _ = Describe("SSH Patch Command", func() {
	const (
		gardenName           = "mygarden"
		gardenKubeconfigFile = "/not/a/real/kubeconfig"
		seedName             = "test-seed"
		shootName            = "test-shoot"
	)

	// populated in top level BeforeEach
	var (
		ctrl                   *gomock.Controller
		factory                *utilmocks.MockFactory
		gardenClient           *gcmocks.MockClient
		manager                *targetmocks.MockManager
		clock                  *utilmocks.MockClock
		now                    time.Time
		ctx                    context.Context
		cancel                 context.CancelFunc
		currentTarget          target.Target
		sampleClientCertficate []byte
		testProject            *gardencorev1beta1.Project
		testSeed               *gardencorev1beta1.Seed
		testShoot              *gardencorev1beta1.Shoot
		apiConfig              *clientcmdapi.Config
		bastionDefaultPolicies []gardenoperationsv1alpha1.BastionIngressPolicy
	)

	// helpers
	var (
		ctxType       = reflect.TypeOf((*context.Context)(nil)).Elem()
		isCtx         = gomock.AssignableToTypeOf(ctxType)
		createBastion = func(createdBy, bastionName string) gardenoperationsv1alpha1.Bastion {
			return gardenoperationsv1alpha1.Bastion{
				ObjectMeta: metav1.ObjectMeta{
					Name:      bastionName,
					Namespace: testShoot.Namespace,
					UID:       "some UID",
					Annotations: map[string]string{
						"gardener.cloud/created-by": createdBy,
					},
					CreationTimestamp: metav1.Time{
						Time: now,
					},
				},
				Spec: gardenoperationsv1alpha1.BastionSpec{
					ShootRef: corev1.LocalObjectReference{
						Name: testShoot.Name,
					},
					SSHPublicKey: "some-dummy-public-key",
					Ingress:      bastionDefaultPolicies,
					ProviderType: pointer.String("aws"),
				},
			}
		}
		createBastionPtr = func(createdBy, bastionName string) *gardenoperationsv1alpha1.Bastion {
			b := createBastion(createdBy, bastionName)
			return &b
		}
	)

	// TODO: after migration to ginkgo v2: move to BeforeAll
	func() {
		// only run it once and not in BeforeEach as it is an expensive operation
		caCertCSC := &gardensecrets.CertificateSecretConfig{
			Name:       "issuer-name",
			CommonName: "issuer-cn",
			CertType:   gardensecrets.CACert,
		}
		caCert, _ := caCertCSC.GenerateCertificate()

		csc := &gardensecrets.CertificateSecretConfig{
			Name:         "client-name",
			CommonName:   "client-cn",
			Organization: []string{user.SystemPrivilegedGroup},
			CertType:     gardensecrets.ClientCert,
			SigningCA:    caCert,
		}
		generatedClientCert, _ := csc.GenerateCertificate()
		sampleClientCertficate = generatedClientCert.CertificatePEM
	}()

	BeforeEach(func() {
		now, _ = time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")

		apiConfig = clientcmdapi.NewConfig()
		apiConfig.Clusters["cluster"] = &clientcmdapi.Cluster{
			Server:                "https://kubernetes:6443/",
			InsecureSkipTLSVerify: true,
		}
		apiConfig.Contexts["client-cert"] = &clientcmdapi.Context{
			AuthInfo:  "client-cert",
			Namespace: "default",
			Cluster:   "cluster",
		}
		apiConfig.AuthInfos["client-cert"] = &clientcmdapi.AuthInfo{
			ClientCertificateData: sampleClientCertficate,
		}
		apiConfig.Contexts["no-auth"] = &clientcmdapi.Context{
			AuthInfo:  "no-auth",
			Namespace: "default",
			Cluster:   "cluster",
		}
		apiConfig.AuthInfos["no-auth"] = &clientcmdapi.AuthInfo{}
		apiConfig.CurrentContext = "client-cert"

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
				Name: seedName,
			},
		}

		testShoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      shootName,
				Namespace: *testProject.Spec.Namespace,
			},
			Spec: gardencorev1beta1.ShootSpec{
				SeedName: pointer.String(testSeed.Name),
				Kubernetes: gardencorev1beta1.Kubernetes{
					Version: "1.20.0", // >= 1.20.0 for non-legacy shoot kubeconfigs
				},
			},
			Status: gardencorev1beta1.ShootStatus{
				AdvertisedAddresses: []gardencorev1beta1.ShootAdvertisedAddress{
					{
						Name: "shoot-address1",
						URL:  "https://api.bar.baz",
					},
				},
			},
		}

		bastionDefaultPolicies = []gardenoperationsv1alpha1.BastionIngressPolicy{{
			IPBlock: networkingv1.IPBlock{
				CIDR: "1.1.1.1/16",
			},
		}, {
			IPBlock: networkingv1.IPBlock{
				CIDR: "dead:beef::/64",
			},
		}}

		currentTarget = target.NewTarget(gardenName, testProject.Name, testSeed.Name, testShoot.Name)

		ctrl = gomock.NewController(GinkgoT())
		gardenClient = gcmocks.NewMockClient(ctrl)

		manager = targetmocks.NewMockManager(ctrl)
		manager.EXPECT().ClientConfig(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, _ target.Target) (clientcmd.ClientConfig, error) {
			// DoAndReturn allows us to modify the apiConfig within the testcase
			clientcmdConfig := clientcmd.NewDefaultClientConfig(*apiConfig, nil)
			return clientcmdConfig, nil
		}).AnyTimes()
		manager.EXPECT().CurrentTarget().Return(currentTarget, nil).AnyTimes()
		manager.EXPECT().GardenClient(gomock.Eq(gardenName)).Return(gardenClient, nil).AnyTimes()

		ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
		clock = utilmocks.NewMockClock(ctrl)

		factory = utilmocks.NewMockFactory(ctrl)
		factory.EXPECT().Manager().Return(manager, nil).AnyTimes()
		factory.EXPECT().Context().Return(ctx).AnyTimes()
		factory.EXPECT().Clock().Return(clock).AnyTimes()
		fakeIPs := []string{"192.0.2.42", "2001:db8::8a2e:370:7334"}
		factory.EXPECT().PublicIPs(isCtx).Return(fakeIPs, nil).AnyTimes()
	})

	AfterEach(func() {
		cancel()
		ctrl.Finish()
	})

	Describe("sshPatchOptions", func() {
		Describe("Validate", func() {
			var fakeBastion gardenoperationsv1alpha1.Bastion

			BeforeEach(func() {
				fakeBastion = createBastion("user", "bastion-name")
			})

			It("Should fail when no CIDRs are provided", func() {
				o := ssh.NewTestSSHPatchOptions()
				o.BastionName = fakeBastion.Name
				o.Bastion = &fakeBastion
				Expect(o.Validate()).NotTo(Succeed())
			})

			It("Should fail when Bastion is nil", func() {
				o := ssh.NewTestSSHPatchOptions()
				o.CIDRs = append(o.CIDRs, "1.1.1.1/16")
				o.BastionName = fakeBastion.Name
				Expect(o.Validate()).NotTo(Succeed())
			})

			It("Should fail when BastionName is nil", func() {
				o := ssh.NewTestSSHPatchOptions()
				o.CIDRs = append(o.CIDRs, "1.1.1.1/16")
				o.Bastion = &fakeBastion
				Expect(o.Validate()).NotTo(Succeed())
			})

			It("Should fail when BastionName does not equal Bastion.Name", func() {
				o := ssh.NewTestSSHPatchOptions()
				o.CIDRs = append(o.CIDRs, "1.1.1.1/16")
				o.BastionName = "foo"
				o.Bastion = &fakeBastion
				Expect(o.Validate()).NotTo(Succeed())
			})
		})

		Describe("Complete", func() {
			var sshutil *sshutilmocks.MocksshPatchUtils

			BeforeEach(func() {
				sshutil = sshutilmocks.NewMocksshPatchUtils(ctrl)
				sshutil.EXPECT().GetAuthInfo(gomock.Any()).Return(apiConfig.AuthInfos["client-cert"], nil).AnyTimes()
				sshutil.EXPECT().TargetAsListOption(gomock.Eq(currentTarget)).Return(gc.ProjectFilter{}).AnyTimes()
			})

			Describe("Auto-completion of the bastion name when it is not provided by user", func() {
				It("should fail if no bastions created by current user exist", func() {
					o := ssh.NewTestSSHPatchOptions()
					cmd := ssh.NewCmdSSHPatch(factory, o.Streams)

					o.Utils = sshutil
					bastionsOfUser := []gardenoperationsv1alpha1.Bastion{}

					sshutil.EXPECT().GetCurrentUser(isCtx, gomock.Any(), gomock.Any()).Return("user-wo-bastions", nil).Times(1)
					sshutil.EXPECT().GetBastionsOfUser(isCtx, gomock.Eq("user-wo-bastions"), gomock.Any(), gomock.Any()).Return(bastionsOfUser, nil).Times(1)

					err := o.Complete(factory, cmd, []string{})

					Expect(err).ToNot(BeNil(), "Should return an error")
					Expect(o.BastionName).To(Equal(""), "Bastion name should not be set in SSHPatchOptions")
					Expect(err.Error()).To(ContainSubstring("no bastions found"))
				})

				It("should succeed if exactly one bastion created by current user exists", func() {
					o := ssh.NewTestSSHPatchOptions()
					cmd := ssh.NewCmdSSHPatch(factory, o.Streams)

					o.Utils = sshutil
					bastionsOfUser := []gardenoperationsv1alpha1.Bastion{
						createBastion("user1", "user1-bastion1"),
					}

					sshutil.EXPECT().GetCurrentUser(isCtx, gomock.Any(), gomock.Any()).Return("user1", nil).Times(1)
					sshutil.EXPECT().GetBastionsOfUser(isCtx, gomock.Eq("user1"), gomock.Any(), gomock.Any()).Return(bastionsOfUser, nil).Times(1)
					clock.EXPECT().Now().Return(now).AnyTimes()

					err := o.Complete(factory, cmd, []string{})
					out := o.Out.String()

					Expect(out).To(ContainSubstring("Auto-selected bastion"))
					Expect(err).To(BeNil(), "Should not return an error")
					Expect(o.BastionName).To(Equal("user1-bastion1"), "Should set bastion name in SSHPatchOptions to the one bastion the user has created")
					Expect(o.Bastion).ToNot(BeNil())
				})

				It("should fail if more then one bastion created by current user exists", func() {
					o := ssh.NewTestSSHPatchOptions()
					cmd := ssh.NewCmdSSHPatch(factory, o.Streams)

					o.Utils = sshutil
					bastionsOfUser := []gardenoperationsv1alpha1.Bastion{
						createBastion("user2", "user2-bastion1"),
						createBastion("user2", "user2-bastion1"),
					}

					sshutil.EXPECT().GetCurrentUser(isCtx, gomock.Any(), gomock.Any()).Return("user2", nil).Times(1)
					sshutil.EXPECT().GetBastionsOfUser(isCtx, gomock.Eq("user2"), gomock.Any(), gomock.Any()).Return(bastionsOfUser, nil).Times(1)

					err := o.Complete(factory, cmd, []string{})

					Expect(err).ToNot(BeNil(), "Should return an error")
					Expect(o.BastionName).To(Equal(""), "Bastion name should not be set in SSHPatchOptions")
					Expect(err.Error()).To(ContainSubstring("multiple bastions were found"))
				})
			})

			Describe("Bastion for provided bastion name should be loaded", func() {
				It("should succeed if the bastion with the name provided exists", func() {
					bastionName := "user1-bastion1"
					o := ssh.NewTestSSHPatchOptions()
					cmd := ssh.NewCmdSSHPatch(factory, o.Streams)

					o.Utils = sshutil
					bastionsOfUser := []gardenoperationsv1alpha1.Bastion{
						createBastion("user1", "user1-bastion1"),
					}

					sshutil.EXPECT().GetCurrentUser(isCtx, gomock.Any(), gomock.Any()).Return("user1", nil).Times(1)
					sshutil.EXPECT().GetBastionsOfUser(isCtx, gomock.Eq("user1"), gomock.Any(), gomock.Any()).Return(bastionsOfUser, nil).Times(1)

					err := o.Complete(factory, cmd, []string{bastionName})

					Expect(err).To(BeNil(), "Should not return an error")
					Expect(o.BastionName).To(Equal(bastionName), "Should set bastion name in SSHPatchOptions to the value of args[0]")
					Expect(o.Bastion).ToNot(BeNil())
				})
			})
		})

		Describe("Run", func() {
			var fakeBastion *gardenoperationsv1alpha1.Bastion
			var options *ssh.TestSSHPatchOptions
			var cmd *cobra.Command

			BeforeEach(func() {
				sshutil := sshutilmocks.NewMocksshPatchUtils(ctrl)
				fakeBastion = createBastionPtr("user", "fake-bastion-name")
				bastionsOfUser := []gardenoperationsv1alpha1.Bastion{
					*fakeBastion,
				}

				sshutil.EXPECT().GetBastionsOfUser(isCtx, gomock.Eq("user"), gomock.Any(), gomock.Any()).Return(bastionsOfUser, nil).Times(1)
				sshutil.EXPECT().GetAuthInfo(gomock.Any()).Return(apiConfig.AuthInfos["client-cert"], nil).Times(1)
				sshutil.EXPECT().GetCurrentUser(isCtx, gomock.Any(), gomock.Any()).Return("user", nil).Times(1)
				sshutil.EXPECT().TargetAsListOption(gomock.Eq(currentTarget)).Return(gc.ProjectFilter{}).AnyTimes()

				options = ssh.NewTestSSHPatchOptions()
				options.Utils = sshutil

				o := ssh.NewTestSSHPatchOptions()
				cmd = ssh.NewCmdSSHPatch(factory, o.Streams)
			})

			It("It should update the bastion ingress policy", func() {
				options.CIDRs = []string{"8.8.8.8/16"}
				options.BastionName = fakeBastion.Name
				options.Bastion = fakeBastion

				gardenClient.EXPECT().PatchBastion(isCtx, gomock.Any(), gomock.Any()).Return(nil).Times(1)

				Expect(options.Complete(factory, cmd, []string{})).To(BeNil(), "Complete should not error")

				err := options.Run(factory)
				Expect(err).To(BeNil())

				Expect(len(options.Bastion.Spec.Ingress)).To(Equal(1), "Should only have one Ingress policy (had 2)")
				Expect(options.Bastion.Spec.Ingress[0].IPBlock.CIDR).To(Equal(options.CIDRs[0]))
			})
		})
	})

	Describe("sshPatchCompletions", func() {
		Describe("GetBastionNameCompletions", func() {
			var bastionsOfUser []gardenoperationsv1alpha1.Bastion
			var sshutil *sshutilmocks.MocksshPatchUtils
			BeforeEach(func() {
				bastionsOfUser = []gardenoperationsv1alpha1.Bastion{
					createBastion("user", "prefix1-bastion1"),
					createBastion("user", "prefix1-bastion2"),
					createBastion("user", "prefix2-bastion1"),
				}

				sshutil = sshutilmocks.NewMocksshPatchUtils(ctrl)
				sshutil.EXPECT().GetAuthInfo(gomock.Any()).Return(apiConfig.AuthInfos["client-cert"], nil).Times(1)
				sshutil.EXPECT().TargetAsListOption(gomock.Eq(currentTarget)).Return(gc.ProjectFilter{}).AnyTimes()
			})

			It("should find bastions of current user with given prefix", func() {
				streams, _, _, _ := util.NewTestIOStreams()
				cmd := ssh.NewCmdSSHPatch(factory, streams)
				c := ssh.NewTestSSHPatchCompletions()
				c.Utils = sshutil

				sshutil.EXPECT().GetCurrentUser(isCtx, gomock.Any(), gomock.Any()).Return("user", nil).Times(1)
				sshutil.EXPECT().GetBastionsOfUser(isCtx, gomock.Eq("user"), gomock.Any(), gomock.Any()).Return(bastionsOfUser, nil).Times(1)
				clock.EXPECT().Now().Return(now).AnyTimes()

				completions, err := c.GetBastionNameCompletions(factory, cmd, "prefix1")

				Expect(err).To(BeNil(), "Should not return an error")
				Expect(len(completions)).To(Equal(2), "should find two bastions with given prefix")
				Expect(completions[0]).To(ContainSubstring("prefix1-bastion1\t created 0s ago"))
				Expect(completions[1]).To(ContainSubstring("prefix1-bastion2\t created 0s ago"))
			})
		})
	})

	Describe("sshPatchUtils", func() {
		Describe("GetCurrentUser", func() {
			var utils *ssh.TestSSHPatchUtils

			BeforeEach(func() {
				fakeBastionList := &gardenoperationsv1alpha1.BastionList{
					Items: []gardenoperationsv1alpha1.Bastion{
						createBastion("client-cn", "fake-bastion"),
					},
				}
				gardenClient.EXPECT().ListBastions(isCtx, gomock.Any()).Return(fakeBastionList, nil).AnyTimes()

				utils = ssh.NewTestSSHPatchUtils()
			})

			It("Should return the user when a Token is used", func() {
				token := "an-arbitrary-token"
				user := "an-arbitrary-user"

				reviewResult := &authenticationv1.TokenReview{
					Status: authenticationv1.TokenReviewStatus{
						User: authenticationv1.UserInfo{
							Username: user,
						},
					},
				}
				gardenClient.EXPECT().CreateTokenReview(gomock.Eq(ctx), gomock.Eq(token)).Return(reviewResult, nil).Times(1)

				username, err := utils.GetCurrentUser(ctx, gardenClient, &clientcmdapi.AuthInfo{
					Token: token,
				})

				Expect(err).To(BeNil())
				Expect(username).To(Equal(user))
			})

			It("Should return the user when a client certificate is used", func() {
				username, err := utils.GetCurrentUser(ctx, gardenClient, &clientcmdapi.AuthInfo{
					ClientCertificateData: sampleClientCertficate,
				})
				Expect(err).To(BeNil())
				Expect(username).To(Equal("client-cn"))
			})
		})

		Describe("GetAuthInfo", func() {
			var utils *ssh.TestSSHPatchUtils

			BeforeEach(func() {
				utils = ssh.NewTestSSHPatchUtils()
			})

			It("should return the currently active auth info", func() {
				apiConfig.CurrentContext = "client-cert"
				clientConfig := clientcmd.NewDefaultClientConfig(*apiConfig, nil)
				resultingAuthInfo, err := utils.GetAuthInfo(clientConfig)

				Expect(err).To(BeNil())
				Expect(resultingAuthInfo).ToNot(BeNil())
				Expect(len(resultingAuthInfo.ClientCertificateData)).ToNot(Equal(0))

				apiConfig.CurrentContext = "no-auth"
				clientConfig = clientcmd.NewDefaultClientConfig(*apiConfig, nil)
				resultingAuthInfo, err = utils.GetAuthInfo(clientConfig)

				Expect(err).To(BeNil())
				Expect(resultingAuthInfo).ToNot(BeNil())
				Expect(len(resultingAuthInfo.ClientCertificateData)).To(Equal(0))
			})
		})

		Describe("GetBastionsOfUser", func() {
			var listOption gc.ProjectFilter
			var fakeBastionList *gardenoperationsv1alpha1.BastionList
			var sshutil *ssh.TestSSHPatchUtils

			BeforeEach(func() {
				fakeBastionList = &gardenoperationsv1alpha1.BastionList{
					Items: []gardenoperationsv1alpha1.Bastion{
						createBastion("user1", "prefix1-bastion1-user1"),
						createBastion("user2", "prefix1-bastion1-user2"),
						createBastion("user2", "prefix1-bastion2-user2"),
						createBastion("user2", "prefix2-bastion1-user2"),
					},
				}
				listOption = gc.ProjectFilter{}

				sshutil = ssh.NewTestSSHPatchUtils()
			})

			It("should find bastions of current user", func() {
				gardenClient.EXPECT().ListBastions(isCtx, gomock.Any()).Return(fakeBastionList, nil).Times(1)
				completions, err := sshutil.GetBastionsOfUser(ctx, "user1", gardenClient, listOption)

				Expect(err).To(BeNil(), "Should not return an error")
				Expect(len(completions)).To(Equal(1))

				gardenClient.EXPECT().ListBastions(isCtx, gomock.Any()).Return(fakeBastionList, nil).Times(1)
				completions, err = sshutil.GetBastionsOfUser(ctx, "user2", gardenClient, listOption)

				Expect(err).To(BeNil(), "Should not return an error")
				Expect(len(completions)).To(Equal(3))
			})
		})

		Describe("TargetAsListOption", func() {
			var sshutil *ssh.TestSSHPatchUtils

			BeforeEach(func() {
				sshutil = ssh.NewTestSSHPatchUtils()
			})

			It("should find bastions of current user", func() {
				listOption := sshutil.TargetAsListOption(currentTarget)
				listOptions := &client.ListOptions{}

				listOption.ApplyToList(listOptions)

				selectorStr := listOptions.FieldSelector.String()
				Expect(selectorStr).To(ContainSubstring("spec.shootRef.name=test-shoot"))
				Expect(selectorStr).To(ContainSubstring("project=prod1"))
			})
		})
	})
})
