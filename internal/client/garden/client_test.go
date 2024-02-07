/*
SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package garden_test

import (
	"context"
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/secrets"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientauthenticationv1 "k8s.io/client-go/pkg/apis/clientauthentication/v1"
	clientauthenticationv1beta1 "k8s.io/client-go/pkg/apis/clientauthentication/v1beta1"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clientgarden "github.com/gardener/gardenctl-v2/internal/client/garden"
	"github.com/gardener/gardenctl-v2/internal/fake"
)

type createInterceptingClient struct {
	client.Client
	createInterceptor func(context.Context, client.Object, ...client.CreateOption) error
}

func (cic createInterceptingClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if cic.createInterceptor != nil {
		err := cic.createInterceptor(ctx, obj, opts...)
		if err != nil {
			return err
		}
	}

	return cic.Client.Create(ctx, obj, opts...)
}

var _ = Describe("Client", func() {
	const (
		gardenName = "my-garden"
	)

	var (
		ctx          context.Context
		gardenClient clientgarden.Client
	)

	Describe("GetSeedClientConfig", func() {
		BeforeEach(func() {
			ctx = context.Background()

			seed1Kubeconfig, err := fake.NewConfigData("seed-1")
			Expect(err).NotTo(HaveOccurred())

			seed2Kubeconfig, err := fake.NewConfigData("seed-2")
			Expect(err).NotTo(HaveOccurred())

			oidcSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "seed-1.oidc",
					Namespace: "garden",
				},
				Data: map[string][]byte{
					"kubeconfig": seed1Kubeconfig,
				},
			}
			loginSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "seed-2.login",
					Namespace: "garden",
				},
				Data: map[string][]byte{
					"kubeconfig": seed2Kubeconfig,
				},
			}
			gardenClient = clientgarden.NewClient(
				nil,
				fake.NewClientWithObjects(oidcSecret, loginSecret),
				gardenName,
			)
		})

		DescribeTable("when the secret exists", func(seedName string) {
			clientConfig, err := gardenClient.GetSeedClientConfig(ctx, seedName)
			Expect(err).NotTo(HaveOccurred())

			rawConfig, err := clientConfig.RawConfig()
			Expect(err).NotTo(HaveOccurred())
			Expect(rawConfig.CurrentContext).To(Equal(seedName))
		},
			Entry("and the secretName has suffix .oidc", "seed-1"),
			Entry("and the secretName has suffix .login", "seed-2"),
		)

		Context("when the secret does not exist", func() {
			It("it should fail with not found error", func() {
				_, err := gardenClient.GetSeedClientConfig(ctx, "seed-3")
				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
			})
		})
	})

	Describe("GetShootOfManagedSeed", func() {
		BeforeEach(func() {
			ctx = context.Background()
			managedSeed := &seedmanagementv1alpha1.ManagedSeed{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "managedSeed",
					Namespace: "garden",
				},
				Spec: seedmanagementv1alpha1.ManagedSeedSpec{
					Shoot: &seedmanagementv1alpha1.Shoot{
						Name: "shootOfManagedSeed",
					},
				},
			}
			gardenClient = clientgarden.NewClient(
				nil,
				fake.NewClientWithObjects(managedSeed),
				gardenName,
			)
		})

		It("should return spec.shoot of managed seed ", func() {
			shoot, err := gardenClient.GetShootOfManagedSeed(ctx, "managedSeed")
			Expect(err).NotTo(HaveOccurred())
			Expect(shoot.Name).To(Equal("shootOfManagedSeed"))
		})
	})

	Describe("GetShootClientConfig", func() {
		const (
			shootName = "test-shoot1"
			namespace = "garden-prod1"
			domain    = "foo.bar.baz"

			k8sVersion       = "1.20.0"
			k8sVersionLegacy = "1.19.0" // legacy kubeconfig should be rendered
		)
		var (
			testShoot1  *gardencorev1beta1.Shoot
			caConfigMap *corev1.ConfigMap
			caSecret    *corev1.Secret
			ca          *secrets.Certificate
		)

		BeforeEach(func() {
			ctx = context.Background()
			testShoot1 = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      shootName,
					Namespace: namespace,
				},
				Spec: gardencorev1beta1.ShootSpec{
					Kubernetes: gardencorev1beta1.Kubernetes{
						Version: k8sVersion,
					},
				},
				Status: gardencorev1beta1.ShootStatus{
					AdvertisedAddresses: []gardencorev1beta1.ShootAdvertisedAddress{
						{
							Name: "shoot-address1",
							URL:  "https://api." + domain,
						},
						{
							Name: "shoot-address2",
							URL:  "https://api2." + domain,
						},
					},
				},
			}

			csc := &secrets.CertificateSecretConfig{
				Name:       "ca-test",
				CommonName: "ca-test",
				CertType:   secrets.CACert,
			}
			var err error
			ca, err = csc.GenerateCertificate()
			Expect(err).NotTo(HaveOccurred())

			caConfigMap = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testShoot1.Name + ".ca-cluster",
					Namespace: testShoot1.Namespace,
				},
				Data: map[string]string{
					"ca.crt": string(ca.CertificatePEM),
				},
			}

			caSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testShoot1.Name + ".ca-cluster",
					Namespace: testShoot1.Namespace,
				},
				Data: map[string][]byte{
					"ca.crt": ca.CertificatePEM,
				},
			}
		})

		Context("good case", func() {
			JustBeforeEach(func() {
				gardenClient = clientgarden.NewClient(
					nil,
					fake.NewClientWithObjects(testShoot1, caConfigMap),
					gardenName,
				)
			})

			Context("when ca-cluster configmap exists", func() {
				It("it should return the client config", func() {
					gardenClient = clientgarden.NewClient(
						nil,
						fake.NewClientWithObjects(testShoot1, caSecret),
						gardenName,
					)

					clientConfig, err := gardenClient.GetShootClientConfig(ctx, namespace, shootName)
					Expect(err).NotTo(HaveOccurred())

					rawConfig, err := clientConfig.RawConfig()
					Expect(err).NotTo(HaveOccurred())
					Expect(rawConfig.Clusters).To(HaveLen(2))
					context := rawConfig.Contexts[rawConfig.CurrentContext]
					cluster := rawConfig.Clusters[context.Cluster]
					Expect(cluster.Server).To(Equal("https://api." + domain))
					Expect(cluster.CertificateAuthorityData).To(Equal(ca.CertificatePEM))

					extension := &clientgarden.ExecPluginConfig{}
					extension.GardenClusterIdentity = gardenName
					extension.ShootRef.Namespace = namespace
					extension.ShootRef.Name = shootName

					Expect(cluster.Extensions["client.authentication.k8s.io/exec"]).To(Equal(extension.ToRuntimeObject()))

					Expect(rawConfig.Contexts).To(HaveLen(2))

					Expect(rawConfig.AuthInfos).To(HaveLen(1))
					authInfo := rawConfig.AuthInfos[context.AuthInfo]
					Expect(authInfo.Exec.APIVersion).To(Equal(clientauthenticationv1.SchemeGroupVersion.String()))
					Expect(authInfo.Exec.Command).To(Equal("kubectl-gardenlogin"))
					Expect(authInfo.Exec.Args).To(Equal([]string{
						"get-client-certificate",
					}))
					Expect(authInfo.Exec.InstallHint).ToNot(BeEmpty())
				})
			})

			Context("when ca-cluster secret exists", func() {
				It("it should return the client config", func() {
					gardenClient = clientgarden.NewClient(
						nil,
						fake.NewClientWithObjects(testShoot1, caSecret),
						gardenName,
					)

					clientConfig, err := gardenClient.GetShootClientConfig(ctx, namespace, shootName)
					Expect(err).NotTo(HaveOccurred())

					rawConfig, err := clientConfig.RawConfig()
					Expect(err).NotTo(HaveOccurred())
					Expect(rawConfig.Clusters).To(HaveLen(2))
					context := rawConfig.Contexts[rawConfig.CurrentContext]
					cluster := rawConfig.Clusters[context.Cluster]
					Expect(cluster.Server).To(Equal("https://api." + domain))
					Expect(cluster.CertificateAuthorityData).To(Equal(ca.CertificatePEM))

					extension := &clientgarden.ExecPluginConfig{}
					extension.GardenClusterIdentity = gardenName
					extension.ShootRef.Namespace = namespace
					extension.ShootRef.Name = shootName

					Expect(cluster.Extensions["client.authentication.k8s.io/exec"]).To(Equal(extension.ToRuntimeObject()))

					Expect(rawConfig.Contexts).To(HaveLen(2))

					Expect(rawConfig.AuthInfos).To(HaveLen(1))
					authInfo := rawConfig.AuthInfos[context.AuthInfo]
					Expect(authInfo.Exec.APIVersion).To(Equal(clientauthenticationv1.SchemeGroupVersion.String()))
					Expect(authInfo.Exec.Command).To(Equal("kubectl-gardenlogin"))
					Expect(authInfo.Exec.Args).To(Equal([]string{
						"get-client-certificate",
					}))
					Expect(authInfo.Exec.InstallHint).ToNot(BeEmpty())
				})
			})

			Context("legacy kubeconfig", func() {
				BeforeEach(func() {
					By("having shoot kubernetes version < v1.20.0")
					testShoot1.Spec.Kubernetes.Version = k8sVersionLegacy
				})

				It("should create legacy kubeconfig", func() {
					clientConfig, err := gardenClient.GetShootClientConfig(ctx, namespace, shootName)
					Expect(err).NotTo(HaveOccurred())

					rawConfig, err := clientConfig.RawConfig()
					Expect(err).ToNot(HaveOccurred())

					Expect(rawConfig.Clusters).To(HaveLen(2))
					context := rawConfig.Contexts[rawConfig.CurrentContext]
					cluster := rawConfig.Clusters[context.Cluster]
					Expect(cluster.Server).To(Equal("https://api." + domain))
					Expect(cluster.CertificateAuthorityData).To(Equal(ca.CertificatePEM))
					Expect(cluster.Extensions).To(BeEmpty())

					Expect(rawConfig.Contexts).To(HaveLen(2))

					Expect(rawConfig.AuthInfos).To(HaveLen(1))
					authInfo := rawConfig.AuthInfos[context.AuthInfo]
					Expect(authInfo.Exec.APIVersion).To(Equal(clientauthenticationv1beta1.SchemeGroupVersion.String()))
					Expect(authInfo.Exec.Command).To(Equal("kubectl-gardenlogin"))
					Expect(authInfo.Exec.Args).To(Equal([]string{
						"get-client-certificate",
						fmt.Sprintf("--name=%s", shootName),
						fmt.Sprintf("--namespace=%s", namespace),
						fmt.Sprintf("--garden-cluster-identity=%s", gardenName),
					}))
				})
			})
		})

		Context("when the ca-cluster does not exist", func() {
			BeforeEach(func() {
				gardenClient = clientgarden.NewClient(
					nil,
					fake.NewClientWithObjects(testShoot1),
					gardenName,
				)
			})

			It("it should fail with not found error", func() {
				_, err := gardenClient.GetShootClientConfig(ctx, namespace, shootName)
				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
				Expect(err.Error()).To(ContainSubstring(shootName + ".ca-cluster"))
			})
		})
	})

	Describe("CurrentUser", func() {
		It("Should return the user when a token is used", func() {
			user := "an-arbitrary-user"

			config := fake.NewTokenConfig("garden")

			cic := createInterceptingClient{
				Client: fake.NewClientWithObjects(),
				createInterceptor: func(ctx context.Context, object client.Object, option ...client.CreateOption) error {
					if tr, ok := object.(*authenticationv1.TokenReview); ok { // patch the TokenReview
						tr.ObjectMeta.Name = "foo" // must be set or else the fake client will error because no name was provided
						tr.Status.Authenticated = true
						tr.Status.User = authenticationv1.UserInfo{
							Username: user,
						}
					}

					return nil
				},
			}

			gardenClient = clientgarden.NewClient(
				clientcmd.NewDefaultClientConfig(*config, nil),
				cic,
				gardenName,
			)

			username, err := gardenClient.CurrentUser(ctx)

			Expect(err).To(BeNil())
			Expect(username).To(Equal(user))
		})

		It("Should return the user when a client certificate is used", func() {
			userCN := "client-cn"

			caCert, err := fake.NewCaCert()
			Expect(err).NotTo(HaveOccurred())

			generatedClientCert, err := fake.NewClientCert(caCert, userCN, nil)
			Expect(err).NotTo(HaveOccurred())

			config := fake.NewCertConfig("client-cert", generatedClientCert.CertificatePEM)

			gardenClient = clientgarden.NewClient(
				clientcmd.NewDefaultClientConfig(*config, nil),
				nil, // no client needed for this test
				gardenName,
			)

			username, err := gardenClient.CurrentUser(ctx)
			Expect(err).To(BeNil())
			Expect(username).To(Equal(userCN))
		})
	})
})
