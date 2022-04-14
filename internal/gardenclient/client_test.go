/*
SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package gardenclient_test

import (
	"context"
	"encoding/json"
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/utils/secrets"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/gardener/gardenctl-v2/internal/fake"
	"github.com/gardener/gardenctl-v2/internal/gardenclient"
)

var _ = Describe("Client", func() {
	const (
		gardenName = "my-garden"
	)

	var (
		ctx          context.Context
		gardenClient gardenclient.Client
	)

	Describe("GetSeedClientConfig", func() {
		BeforeEach(func() {
			ctx = context.Background()
			oidcSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "seed-1.oidc",
					Namespace: "garden",
				},
				Data: map[string][]byte{
					"kubeconfig": createTestKubeconfig("seed-1"),
				},
			}
			loginSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "seed-2.login",
					Namespace: "garden",
				},
				Data: map[string][]byte{
					"kubeconfig": createTestKubeconfig("seed-2"),
				},
			}
			gardenClient = gardenclient.NewGardenClient(
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

	Describe("GetShootClientConfig", func() {
		const (
			shootName = "test-shoot1"
			namespace = "garden-prod1"
			domain    = "foo.bar.baz"

			k8sVersion       = "1.20.0"
			k8sVersionLegacy = "1.19.0" // legacy kubeconfig should be rendered
		)
		var (
			testShoot1 *gardencorev1beta1.Shoot
			caSecret   *corev1.Secret
			ca         *secrets.Certificate
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
				gardenClient = gardenclient.NewGardenClient(
					fake.NewClientWithObjects(testShoot1, caSecret),
					gardenName,
				)
			})

			It("it should return the client config", func() {
				gardenClient = gardenclient.NewGardenClient(
					fake.NewClientWithObjects(testShoot1, caSecret),
					gardenName,
				)

				clientConfig, err := gardenClient.GetShootClientConfig(ctx, namespace, shootName)
				Expect(err).NotTo(HaveOccurred())

				rawConfig, err := clientConfig.RawConfig()
				Expect(err).NotTo(HaveOccurred())
				Expect(rawConfig.Clusters).To(HaveLen(2))
				currentCluster := rawConfig.Contexts[rawConfig.CurrentContext].Cluster
				Expect(rawConfig.Clusters[currentCluster].Server).To(Equal("https://api." + domain))
				Expect(rawConfig.Clusters[currentCluster].CertificateAuthorityData).To(Equal(ca.CertificatePEM))
				Expect(rawConfig.Clusters[currentCluster].Extensions).ToNot(BeEmpty())

				execConfig := rawConfig.Clusters[currentCluster].Extensions["client.authentication.k8s.io/exec"].(*runtime.Unknown)
				Expect(execConfig.Raw).ToNot(BeNil())

				var extension gardenclient.ExecPluginConfig
				Expect(json.Unmarshal(execConfig.Raw, &extension)).To(Succeed())
				Expect(extension).To(Equal(gardenclient.ExecPluginConfig{
					ShootRef: gardenclient.ShootRef{
						Namespace: namespace,
						Name:      shootName,
					},
					GardenClusterIdentity: gardenName,
				}))

				Expect(rawConfig.Contexts).To(HaveLen(2))

				Expect(rawConfig.AuthInfos).To(HaveLen(1))
				currentAuthInfo := rawConfig.Contexts[rawConfig.CurrentContext].AuthInfo
				Expect(rawConfig.AuthInfos[currentAuthInfo].Exec.Command).To(Equal("kubectl"))
				Expect(rawConfig.AuthInfos[currentAuthInfo].Exec.Args).To(Equal([]string{
					"gardenlogin",
					"get-client-certificate",
				}))
			})

			Context("legacy kubeconfig", func() {
				BeforeEach(func() {
					By("having shoot kubernetes version < v1.20.0")
					testShoot1.Spec.Kubernetes.Version = k8sVersionLegacy
				})

				It("should create legacy kubeconfig configMap", func() {
					clientConfig, err := gardenClient.GetShootClientConfig(ctx, namespace, shootName)
					Expect(err).NotTo(HaveOccurred())

					rawConfig, err := clientConfig.RawConfig()
					Expect(err).ToNot(HaveOccurred())

					Expect(rawConfig.Clusters).To(HaveLen(2))
					currentCluster := rawConfig.Contexts[rawConfig.CurrentContext].Cluster
					Expect(rawConfig.Clusters[currentCluster].Server).To(Equal("https://api." + domain))
					Expect(rawConfig.Clusters[currentCluster].CertificateAuthorityData).To(Equal(ca.CertificatePEM))
					Expect(rawConfig.Clusters[currentCluster].Extensions).To(BeEmpty())

					Expect(rawConfig.Contexts).To(HaveLen(2))

					Expect(rawConfig.AuthInfos).To(HaveLen(1))
					currentAuthInfo := rawConfig.Contexts[rawConfig.CurrentContext].AuthInfo
					Expect(rawConfig.AuthInfos[currentAuthInfo].Exec.Command).To(Equal("kubectl"))
					Expect(rawConfig.AuthInfos[currentAuthInfo].Exec.Args).To(Equal([]string{
						"gardenlogin",
						"get-client-certificate",
						fmt.Sprintf("--name=%s", shootName),
						fmt.Sprintf("--namespace=%s", namespace),
						fmt.Sprintf("--garden-cluster-identity=%s", gardenName),
					}))
				})
			})
		})

		Context("when the ca-cluster secret does not exist", func() {
			BeforeEach(func() {
				gardenClient = gardenclient.NewGardenClient(
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
})

// TODO copied from target_suite_test. Move into a test helper package for better reuse
func createTestKubeconfig(name string) []byte {
	config := clientcmdapi.NewConfig()
	config.Clusters["cluster"] = &clientcmdapi.Cluster{
		Server:                "https://kubernetes:6443/",
		InsecureSkipTLSVerify: true,
	}
	config.AuthInfos["user"] = &clientcmdapi.AuthInfo{
		Token: "token",
	}
	config.Contexts[name] = &clientcmdapi.Context{
		Namespace: "default",
		AuthInfo:  "user",
		Cluster:   "cluster",
	}
	config.CurrentContext = name
	data, err := clientcmd.Write(*config)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	return data
}
