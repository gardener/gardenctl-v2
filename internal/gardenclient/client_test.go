/*
SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package gardenclient_test

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"

	"github.com/gardener/gardenctl-v2/internal/fake"
	"github.com/gardener/gardenctl-v2/internal/gardenclient"
)

var _ = Describe("Client", func() {
	Describe("GetSeedClientConfig", func() {
		var (
			ctx          context.Context
			gardenClient gardenclient.Client
		)

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
			managedSeed1Secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "managedSeed-1.login",
					Namespace: "garden",
				},
				Data: map[string][]byte{
					"kubeconfig": createTestKubeconfig("managedSeed-1"),
				},
			}
			managedSeed1 := &seedmanagementv1alpha1.ManagedSeed{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "managedSeed-1",
					Namespace: "garden",
				},
			}
			ms1ShootConfigMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "managedSeed-1.kubeconfig",
					Namespace: "garden",
				},
				Data: map[string]string{
					"kubeconfig": string(createTestKubeconfig("managedSeed-1")),
				},
			}
			managedSeed2 := &seedmanagementv1alpha1.ManagedSeed{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "managedSeed-2",
					Namespace: "garden",
				},
			}
			ms2ShootConfigMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "managedSeed-2.kubeconfig",
					Namespace: "garden",
				},
				Data: map[string]string{
					"kubeconfig": string(createTestKubeconfig("managedSeed-2")),
				},
			}
			managedSeed3 := &seedmanagementv1alpha1.ManagedSeed{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "managedSeed-3",
					Namespace: "garden",
				},
			}
			loginSecret3 := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "managedSeed-3.login",
					Namespace: "garden",
				},
				Data: map[string][]byte{
					"kubeconfig": createTestKubeconfig("managedSeed-3"),
				},
			}
			gardenClient = gardenclient.NewGardenClient(
				fake.NewClientWithObjects(oidcSecret, loginSecret, managedSeed1Secret, managedSeed1, ms1ShootConfigMap, managedSeed2, ms2ShootConfigMap, managedSeed3, loginSecret3),
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
			Entry("and seed is a managed seed, return shoot client config", "managedSeed-1"),
			Entry("and seed is a managed seed, but shoot client config return not found, fallback to seed secret", "managedSeed-3"),
		)

		Context("when the secret does not exist", func() {
			It("it should fail with not found error", func() {
				_, err := gardenClient.GetSeedClientConfig(ctx, "seed-3")
				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
			})
			It("and seed is a managed seed, return shoot client config", func() {
				clientConfig, err := gardenClient.GetSeedClientConfig(ctx, "managedSeed-2")
				Expect(err).NotTo(HaveOccurred())

				rawConfig, err := clientConfig.RawConfig()
				Expect(err).NotTo(HaveOccurred())
				Expect(rawConfig.CurrentContext).To(Equal("managedSeed-2"))
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
