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
	"k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/utils/pointer"

	gcmocks "github.com/gardener/gardenctl-v2/internal/gardenclient/mocks"
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
		apiConfig              *api.Config
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

		apiConfig = api.NewConfig()
		apiConfig.Clusters["cluster"] = &api.Cluster{
			Server:                "https://kubernetes:6443/",
			InsecureSkipTLSVerify: true,
		}
		apiConfig.Contexts["client-cert"] = &api.Context{
			AuthInfo:  "client-cert",
			Namespace: "default",
			Cluster:   "cluster",
		}
		apiConfig.AuthInfos["client-cert"] = &api.AuthInfo{
			ClientCertificateData: sampleClientCertficate,
		}
		apiConfig.Contexts["no-auth"] = &api.Context{
			AuthInfo:  "no-auth",
			Namespace: "default",
			Cluster:   "cluster",
		}
		apiConfig.AuthInfos["no-auth"] = &api.AuthInfo{}
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
		var fakeBastionList *gardenoperationsv1alpha1.BastionList
		var sshutil *sshutilmocks.MocksshPatchUtils
		BeforeEach(func() {
			fakeBastionList = &gardenoperationsv1alpha1.BastionList{
				Items: []gardenoperationsv1alpha1.Bastion{
					createBastion("user1", "user1-bastion1"),
					createBastion("user2", "user2-bastion1"),
					createBastion("user2", "user2-bastion2"),
				},
			}

			sshutil = sshutilmocks.NewMocksshPatchUtils(ctrl)
		})

		Describe("Auto-completion of the bastion name when it is not provided by user", func() {
			It("should fail if no bastions created by current user exist", func() {
				o := ssh.NewTestSSHPatchOptions()
				cmd := ssh.NewCmdSSHPatch(factory, o.Streams)

				o.Utils = sshutil

				sshutil.EXPECT().GetCurrentUser(isCtx, gomock.Any(), gomock.Any()).Return("user-wo-bastions", nil).Times(1)
				gardenClient.EXPECT().ListBastions(isCtx, gomock.Any()).Return(fakeBastionList, nil).Times(1)

				err := o.Complete(factory, cmd, []string{})
				out := o.Out.String()

				Expect(err).To(BeNil(), "Should not return an error")
				Expect(o.BastionName).To(Equal(""), "bastion name should not be set in SSHPatchOptions")
				Expect(out).To(ContainSubstring("No bastions were found"))
			})

			It("should succeed if exactly one bastion created by current user exists", func() {
				o := ssh.NewTestSSHPatchOptions()
				cmd := ssh.NewCmdSSHPatch(factory, o.Streams)

				o.Utils = sshutil

				sshutil.EXPECT().GetCurrentUser(isCtx, gomock.Any(), gomock.Any()).Return("user1", nil).Times(1)
				gardenClient.EXPECT().ListBastions(isCtx, gomock.Any()).Return(fakeBastionList, nil).Times(1)
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

				sshutil.EXPECT().GetCurrentUser(isCtx, gomock.Any(), gomock.Any()).Return("user2", nil).Times(1)
				gardenClient.EXPECT().ListBastions(isCtx, gomock.Any()).Return(fakeBastionList, nil).Times(1)

				err := o.Complete(factory, cmd, []string{})
				out := o.Out.String()

				Expect(err).To(BeNil(), "Should not return an error")
				Expect(o.BastionName).To(Equal(""), "bastion name should not be set in SSHPatchOptions")
				Expect(out).To(ContainSubstring("Multiple bastions were found"))
			})
		})

		Describe("Bastion for provided bastion name should be loaded", func() {
			It("should succeed if the bastion with the name provided exists", func() {
				bastionName := "user1-bastion1"
				fakeBastion := createBastion("user1", bastionName)
				o := ssh.NewTestSSHPatchOptions()
				cmd := ssh.NewCmdSSHPatch(factory, o.Streams)

				gardenClient.EXPECT().GetBastion(isCtx, gomock.Any(), gomock.Eq(bastionName)).Return(&fakeBastion, nil).Times(1)
				gardenClient.EXPECT().FindShoot(isCtx, gomock.Any()).Return(testShoot, nil).Times(1)

				err := o.Complete(factory, cmd, []string{bastionName})

				Expect(err).To(BeNil(), "Should not return an error")
				Expect(o.BastionName).To(Equal(bastionName), "Should set bastion name in SSHPatchOptions to the value of args[0]")
				Expect(o.Bastion).ToNot(BeNil())
			})
		})

		It("The flag completion (suggestion) func should return names of ", func() {
			o := ssh.NewTestSSHPatchOptions()
			cmd := ssh.NewCmdSSHPatch(factory, o.Streams)

			o.Utils = sshutil

			sshutil.EXPECT().GetCurrentUser(isCtx, gomock.Any(), gomock.Any()).Return("user2", nil).Times(2)
			gardenClient.EXPECT().ListBastions(isCtx, gomock.Any()).Return(fakeBastionList, nil).Times(2)
			clock.EXPECT().Now().Return(now.Add(3728 * time.Second)).Times(2) // 3728s = 1h2m8s

			// Complete sets currentTarget etc. to SSHPatchOptions struct required by getBastionNameCompletions
			Expect(o.Complete(factory, cmd, []string{})).To(BeNil(), "Complete should not error")

			suggestions, err := o.GetBastionNameCompletions(factory, cmd, "")

			Expect(err).To(BeNil())
			Expect(suggestions).ToNot(BeNil())
			Expect(len(suggestions)).To(Equal(2))
			Expect(suggestions[0]).To(ContainSubstring("user2-bastion1\t created 1h2m8s ago"))
			Expect(suggestions[1]).To(ContainSubstring("user2-bastion2\t created 1h2m8s ago"))
		})
	})

	Describe("Run", func() {
		var fakeBastion *gardenoperationsv1alpha1.Bastion
		var cmd *cobra.Command

		BeforeEach(func() {
			tmp := createBastion("fake-created-by", "fake-bastion-name")
			fakeBastion = &tmp

			o := ssh.NewTestSSHPatchOptions()
			cmd = ssh.NewCmdSSHPatch(factory, o.Streams)

			gardenClient.EXPECT().FindShoot(isCtx, gomock.Any()).Return(testShoot, nil).Times(1)
			gardenClient.EXPECT().GetBastion(isCtx, gomock.Any(), gomock.Eq(fakeBastion.Name)).Return(fakeBastion, nil).Times(1)
		})

		It("It should update the bastion ingress policy", func() {
			o := ssh.NewTestSSHPatchOptions()
			o.CIDRs = []string{"8.8.8.8/16"}
			o.BastionName = fakeBastion.Name
			o.Bastion = fakeBastion

			gardenClient.EXPECT().PatchBastion(isCtx, gomock.Any(), gomock.Any()).Return(nil).Times(1)

			// Complete sets currentTarget etc. to SSHPatchOptions struct required by getBastionNameCompletions
			Expect(o.Complete(factory, cmd, []string{})).To(BeNil(), "Complete should not error")

			err := o.Run(factory)
			Expect(err).To(BeNil())

			Expect(len(fakeBastion.Spec.Ingress)).To(Equal(1), "Should only have one Ingress policy (had 2)")
			Expect(fakeBastion.Spec.Ingress[0].IPBlock.CIDR).To(Equal(o.CIDRs[0]))
		})
	})

	Describe("GetBastionNameCompletions", func() {
		var fakeBastionList *gardenoperationsv1alpha1.BastionList
		var sshutil *sshutilmocks.MocksshPatchUtils
		BeforeEach(func() {
			fakeBastionList = &gardenoperationsv1alpha1.BastionList{
				Items: []gardenoperationsv1alpha1.Bastion{
					createBastion("user1", "prefix1-bastion1-user1"),
					createBastion("user2", "prefix1-bastion1-user2"),
					createBastion("user2", "prefix1-bastion2-user2"),
					createBastion("user2", "prefix2-bastion1-user2"),
				},
			}

			sshutil = sshutilmocks.NewMocksshPatchUtils(ctrl)
		})

		It("should find bastions of current user with given prefix", func() {
			o := ssh.NewTestSSHPatchOptions()
			cmd := ssh.NewCmdSSHPatch(factory, o.Streams)

			o.Utils = sshutil

			sshutil.EXPECT().GetCurrentUser(isCtx, gomock.Any(), gomock.Any()).Return("user2", nil).Times(1)
			gardenClient.EXPECT().ListBastions(isCtx, gomock.Any()).Return(fakeBastionList, nil).Times(1)
			clock.EXPECT().Now().Return(now).AnyTimes()

			completions, err := o.GetBastionNameCompletions(factory, cmd, "prefix1")

			Expect(err).To(BeNil(), "Should not return an error")
			Expect(len(completions)).To(Equal(2), "should find two bastions with given prefix")
			Expect(completions[0]).To(ContainSubstring("prefix1-bastion1-user2"))
			Expect(completions[1]).To(ContainSubstring("prefix1-bastion2-user2"))
		})
	})

	Describe("getCurrentUser", func() {
		var options *ssh.TestSSHPatchOptions
		var cmd *cobra.Command

		BeforeEach(func() {
			fakeBastion := createBastion("user1", "fake-bastion")

			options = ssh.NewTestSSHPatchOptions()
			options.BastionName = fakeBastion.Name

			cmd = ssh.NewCmdSSHPatch(factory, options.Streams)
			gardenClient.EXPECT().GetBastion(isCtx, gomock.Any(), gomock.Eq(fakeBastion.Name)).Return(&fakeBastion, nil).AnyTimes()
			gardenClient.EXPECT().FindShoot(isCtx, gomock.Any()).Return(testShoot, nil).AnyTimes()

			// Complete sets currentTarget etc. to SSHPatchOptions struct required by getBastionNameCompletions
			Expect(options.Complete(factory, cmd, []string{})).To(BeNil(), "Complete should not error")
		})

		It("getAuthInfo should return the currently active auth info", func() {
			apiConfig.CurrentContext = "client-cert"
			resultingAuthInfo, err := options.GetAuthInfo(ctx)

			Expect(err).To(BeNil())
			Expect(resultingAuthInfo).ToNot(BeNil())
			Expect(len(resultingAuthInfo.ClientCertificateData)).ToNot(Equal(0))

			apiConfig.CurrentContext = "no-auth"
			// clientConfig is not stored as pointer in the SSHPatchOptions struct. So we cannot just modify
			// the client config/apiConfig but need to call Complete again to store the new value.
			Expect(options.Complete(factory, cmd, []string{})).To(BeNil(), "Complete should not error")

			resultingAuthInfo, err = options.GetAuthInfo(ctx)

			Expect(err).To(BeNil())
			Expect(resultingAuthInfo).ToNot(BeNil())
			Expect(len(resultingAuthInfo.ClientCertificateData)).To(Equal(0))
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

			username, err := options.Utils.GetCurrentUser(ctx, gardenClient, &api.AuthInfo{
				Token: token,
			})

			Expect(err).To(BeNil())
			Expect(username).To(Equal(user))
		})

		It("Should return the user when a client certificate is used", func() {
			username, err := options.Utils.GetCurrentUser(ctx, gardenClient, &api.AuthInfo{
				ClientCertificateData: sampleClientCertficate,
			})
			Expect(err).To(BeNil())
			Expect(username).To(Equal("client-cn"))
		})
	})
})
