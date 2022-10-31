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
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/utils/pointer"

	internalfake "github.com/gardener/gardenctl-v2/internal/fake"
	gc "github.com/gardener/gardenctl-v2/internal/gardenclient"
	gcmocks "github.com/gardener/gardenctl-v2/internal/gardenclient/mocks"
	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/ssh"
	"github.com/gardener/gardenctl-v2/pkg/config"
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
		clientProvider         *targetmocks.MockClientProvider
		cfg                    *config.Config
		streams                util.IOStreams
		stdout                 *util.SafeBytesBuffer
		factory                *internalfake.Factory
		gardenClient           *gcmocks.MockClient
		manager                *targetmocks.MockManager
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
		ctxType                   = reflect.TypeOf((*context.Context)(nil)).Elem()
		isCtx                     = gomock.AssignableToTypeOf(ctxType)
		getMockGetCurrentUserFunc = func(username string, err error) func(context.Context, gc.Client, *api.AuthInfo) (string, error) {
			return func(_ context.Context, _ gc.Client, _ *api.AuthInfo) (string, error) {
				return username, err
			}
		}
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

		cfg = &config.Config{
			LinkKubeconfig: pointer.Bool(false),
			Gardens: []config.Garden{{
				Name:       gardenName,
				Kubeconfig: gardenKubeconfigFile,
			}},
		}

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

		streams, _, stdout, _ = util.NewTestIOStreams()
		currentTarget = target.NewTarget(gardenName, testProject.Name, testSeed.Name, testShoot.Name)

		ctrl = gomock.NewController(GinkgoT())
		gardenClient = gcmocks.NewMockClient(ctrl)
		clientProvider = targetmocks.NewMockClientProvider(ctrl)
		targetProvider := internalfake.NewFakeTargetProvider(currentTarget)

		manager = targetmocks.NewMockManager(ctrl)
		manager.EXPECT().ClientConfig(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, _ target.Target) (clientcmd.ClientConfig, error) {
			// DoAndReturn allows us to modify the apiConfig within the testcase
			clientcmdConfig := clientcmd.NewDefaultClientConfig(*apiConfig, nil)
			return clientcmdConfig, nil
		}).AnyTimes()
		manager.EXPECT().CurrentTarget().Return(currentTarget, nil).AnyTimes()
		manager.EXPECT().GardenClient(gomock.Eq(gardenName)).Return(gardenClient, nil).AnyTimes()

		factory = internalfake.NewFakeFactory(cfg, nil, clientProvider, targetProvider)
		factory.ManagerImpl = manager

		ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
		factory.ContextImpl = ctx
	})

	AfterEach(func() {
		cancel()
		ctrl.Finish()
		ssh.ResetGetCurrentUser()
		ssh.ResetTimeNow()
	})

	Describe("Validate", func() {
		var fakeBastion gardenoperationsv1alpha1.Bastion

		BeforeEach(func() {
			fakeBastion = createBastion("user", "bastion-name")
		})

		It("Should fail when no CIDRs are provided", func() {
			o := ssh.NewSSHPatchOptions(streams)
			o.BastionName = fakeBastion.Name
			o.Bastion = &fakeBastion
			Expect(o.Validate()).NotTo(Succeed())
		})

		It("Should fail when Bastion is nil", func() {
			o := ssh.NewSSHPatchOptions(streams)
			o.CIDRs = append(o.CIDRs, "1.1.1.1/16")
			o.BastionName = fakeBastion.Name
			Expect(o.Validate()).NotTo(Succeed())
		})

		It("Should fail when BastionName is nil", func() {
			o := ssh.NewSSHPatchOptions(streams)
			o.CIDRs = append(o.CIDRs, "1.1.1.1/16")
			o.Bastion = &fakeBastion
			Expect(o.Validate()).NotTo(Succeed())
		})

		It("Should fail when BastionName does not equal Bastion.Name", func() {
			o := ssh.NewSSHPatchOptions(streams)
			o.CIDRs = append(o.CIDRs, "1.1.1.1/16")
			o.BastionName = "foo"
			o.Bastion = &fakeBastion
			Expect(o.Validate()).NotTo(Succeed())
		})
	})

	Describe("Complete", func() {
		var fakeBastionList *gardenoperationsv1alpha1.BastionList

		BeforeEach(func() {
			fakeBastionList = &gardenoperationsv1alpha1.BastionList{
				Items: []gardenoperationsv1alpha1.Bastion{
					createBastion("user1", "user1-bastion1"),
					createBastion("user2", "user2-bastion1"),
					createBastion("user2", "user2-bastion2"),
				},
			}
		})

		Describe("Auto-completion of the bastion name when it is not provided by user", func() {
			It("should fail if no bastions created by current user exist", func() {
				o := ssh.NewSSHPatchOptions(streams)
				cmd := ssh.NewCmdSSHPatch(factory, streams)

				gardenClient.EXPECT().ListBastions(isCtx, gomock.Any()).Return(fakeBastionList, nil).Times(1)
				ssh.SetGetCurrentUser(getMockGetCurrentUserFunc("user-wo-bastions", nil))

				err := o.Complete(factory, cmd, []string{})
				out := stdout.String()

				Expect(err).To(BeNil(), "Should not return an error")
				Expect(o.BastionName).To(Equal(""), "bastion name should not be set in SSHPatchOptions")
				Expect(out).To(ContainSubstring("No bastions were found"))
			})

			It("should succeed if exactly one bastion created by current user exists", func() {
				o := ssh.NewSSHPatchOptions(streams)
				cmd := ssh.NewCmdSSHPatch(factory, streams)

				gardenClient.EXPECT().ListBastions(isCtx, gomock.Any()).Return(fakeBastionList, nil).Times(1)
				ssh.SetGetCurrentUser(getMockGetCurrentUserFunc("user1", nil))

				err := o.Complete(factory, cmd, []string{})
				out := stdout.String()

				Expect(out).To(ContainSubstring("Auto-selected bastion"))
				Expect(err).To(BeNil(), "Should not return an error")
				Expect(o.BastionName).To(Equal("user1-bastion1"), "Should set bastion name in SSHPatchOptions to the one bastion the user has created")
				Expect(o.Bastion).ToNot(BeNil())
				Expect(o.Factory).ToNot(BeNil())
			})

			It("should fail if more then one bastion created by current user exists", func() {
				o := ssh.NewSSHPatchOptions(streams)
				cmd := ssh.NewCmdSSHPatch(factory, streams)

				gardenClient.EXPECT().ListBastions(isCtx, gomock.Any()).Return(fakeBastionList, nil).Times(1)
				ssh.SetGetCurrentUser(getMockGetCurrentUserFunc("user2", nil))

				err := o.Complete(factory, cmd, []string{})
				out := stdout.String()

				Expect(err).To(BeNil(), "Should not return an error")
				Expect(o.BastionName).To(Equal(""), "bastion name should not be set in SSHPatchOptions")
				Expect(out).To(ContainSubstring("Multiple bastions were found"))
			})
		})

		Describe("Bastion for provided bastion name should be loaded", func() {
			It("should succeed if the bastion with the name provided exists", func() {
				bastionName := "user1-bastion1"
				fakeBastion := createBastion("user1", bastionName)
				o := ssh.NewSSHPatchOptions(streams)
				cmd := ssh.NewCmdSSHPatch(factory, streams)

				gardenClient.EXPECT().GetBastion(isCtx, gomock.Any(), gomock.Eq(bastionName)).Return(&fakeBastion, nil).Times(1)
				gardenClient.EXPECT().FindShoot(isCtx, gomock.Any()).Return(testShoot, nil).Times(1)
				ssh.SetGetCurrentUser(getMockGetCurrentUserFunc("user1", nil))

				err := o.Complete(factory, cmd, []string{bastionName})

				Expect(err).To(BeNil(), "Should not return an error")
				Expect(o.BastionName).To(Equal(bastionName), "Should set bastion name in SSHPatchOptions to the value of args[0]")
				Expect(o.Bastion).ToNot(BeNil())
				Expect(o.Factory).ToNot(BeNil())
			})
		})

		It("The flag completion (suggestion) func should return names of ", func() {
			o := ssh.NewSSHPatchOptions(streams)
			cmd := ssh.NewCmdSSHPatch(factory, streams)

			ssh.SetGetCurrentUser(getMockGetCurrentUserFunc("user2", nil))
			ssh.SetTimeNow(func() time.Time {
				return now.Add(time.Second * 3728)
			})
			gardenClient.EXPECT().ListBastions(isCtx, gomock.Any()).Return(fakeBastionList, nil).Times(1)

			sut := ssh.GetGetBastionNameCompletions(o)
			suggestions, err := sut(factory, cmd, "")

			Expect(err).To(BeNil())
			Expect(suggestions).ToNot(BeNil())
			Expect(len(suggestions)).To(Equal(2))
			Expect(suggestions[0]).To(ContainSubstring("user2-bastion1\t created 1h2m8s ago"))
			Expect(suggestions[1]).To(ContainSubstring("user2-bastion2\t created 1h2m8s ago"))
		})
	})

	Describe("Run", func() {
		var fakeBastion *gardenoperationsv1alpha1.Bastion

		BeforeEach(func() {
			tmp := createBastion("fake-created-by", "fake-bastion-name")
			fakeBastion = &tmp
		})

		It("It should update the bastion ingress policy", func() {
			o := ssh.NewSSHPatchOptions(streams)
			o.CIDRs = []string{"8.8.8.8/16"}
			o.BastionName = fakeBastion.Name
			o.Factory = factory
			o.Bastion = fakeBastion

			ctxType := reflect.TypeOf((*context.Context)(nil)).Elem()
			isCtx := gomock.AssignableToTypeOf(ctxType)
			gardenClient.EXPECT().PatchBastion(isCtx, gomock.Any(), gomock.Any()).Return(nil).Times(1)

			err := o.Run(nil)
			Expect(err).To(BeNil())

			Expect(len(fakeBastion.Spec.Ingress)).To(Equal(1), "Should only have one Ingress policy (had 2)")
			Expect(fakeBastion.Spec.Ingress[0].IPBlock.CIDR).To(Equal(o.CIDRs[0]))
		})
	})

	Describe("getCurrentUser", func() {
		var getCurrentUserFn func(ctx context.Context, gardenclient gc.Client, authInfo *api.AuthInfo) (string, error)
		var getAuthInfoFn func(ctx context.Context, manager target.Manager) (*api.AuthInfo, error)

		BeforeEach(func() {
			ssh.ResetGetCurrentUser()
			getCurrentUserFn = ssh.GetGetCurrentUser()
			getAuthInfoFn = ssh.GetGetAuthInfo()
		})

		It("getAuthInfo should return the currently active auth info", func() {
			apiConfig.CurrentContext = "client-cert"
			resultingAuthInfo, err := getAuthInfoFn(ctx, manager)

			Expect(err).To(BeNil())
			Expect(resultingAuthInfo).ToNot(BeNil())
			Expect(len(resultingAuthInfo.ClientCertificateData)).ToNot(Equal(0))

			apiConfig.CurrentContext = "no-auth"
			resultingAuthInfo, err = getAuthInfoFn(ctx, manager)

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

			username, err := getCurrentUserFn(ctx, gardenClient, &api.AuthInfo{
				Token: token,
			})

			Expect(err).To(BeNil())
			Expect(username).To(Equal(user))
		})

		It("Should return the user when a client certificate is used", func() {
			username, err := getCurrentUserFn(ctx, gardenClient, &api.AuthInfo{
				ClientCertificateData: sampleClientCertficate,
			})
			Expect(err).To(BeNil())
			Expect(username).To(Equal("client-cn"))
		})
	})
})
