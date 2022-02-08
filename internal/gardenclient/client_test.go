/*
SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package gardenclient_test

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/gardener/gardenctl-v2/internal/fake"
	"github.com/gardener/gardenctl-v2/internal/gardenclient"
)

var _ = Describe("Client", func() {
	var (
		ctx          context.Context
		gardenClient gardenclient.Client
	)

	BeforeEach(func() {
		ctx = context.Background()
	})

	Describe("GetSeedClientConfig", func() {
		var (
			seed string
		)

		BeforeEach(func() {
			seed = "test-seed"
		})

		Context("when the secret exists", func() {
			var (
				secret string
			)

			JustBeforeEach(func() {
				seedKubeconfigSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      secret,
						Namespace: "garden",
					},
					Data: map[string][]byte{
						"kubeconfig": createTestKubeconfig(seed),
					},
				}

				gardenClient = gardenclient.NewGardenClient(
					fake.NewClientWithObjects(seedKubeconfigSecret),
				)
			})

			assertKubeconfig := func() {
				It("should return the kubeconfig", func() {
					sc, err := gardenClient.GetSeedClientConfig(ctx, seed)
					Expect(err).NotTo(HaveOccurred())

					rawConfig, err := sc.RawConfig()
					Expect(err).NotTo(HaveOccurred())
					Expect(rawConfig.CurrentContext).To(Equal(seed))
				})
			}

			Context(".login secret", func() {
				BeforeEach(func() {
					secret = "test-seed.login"
				})

				assertKubeconfig()
			})

			Context(".oidc secret", func() {
				BeforeEach(func() {
					secret = "test-seed.oidc"
				})

				assertKubeconfig()
			})
		})

		Context("when the secret does not exist", func() {
			JustBeforeEach(func() {
				gardenClient = gardenclient.NewGardenClient(
					fake.NewClientWithObjects(),
				)
			})

			It("it should fail with not found error", func() {
				_, err := gardenClient.GetSeedClientConfig(ctx, seed)
				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
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
