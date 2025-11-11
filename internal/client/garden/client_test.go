/*
SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package garden_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardensecurityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/secrets"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clientauthenticationv1 "k8s.io/client-go/pkg/apis/clientauthentication/v1"
	clientauthenticationv1beta1 "k8s.io/client-go/pkg/apis/clientauthentication/v1beta1"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/utils/ptr"
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

	Describe("validateObjectMetadata", func() {
		Context("when UID is a valid UUID", func() {
			It("should not return an error", func() {
				obj := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						UID: types.UID("550e8400-e29b-41d4-a716-446655440000"),
					},
				}
				Expect(clientgarden.ValidateObjectMetadata(obj)).To(Succeed())
			})
		})

		Context("when UID is invalid", func() {
			It("should return an error for path traversal attempts", func() {
				obj := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						UID: types.UID("../../etc/passwd"),
					},
				}
				Expect(clientgarden.ValidateObjectMetadata(obj)).To(HaveOccurred())
			})

			It("should return an error for empty UID", func() {
				obj := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						UID: types.UID(""),
					},
				}
				Expect(clientgarden.ValidateObjectMetadata(obj)).To(HaveOccurred())
			})

			It("should return an error for malformed UUIDs", func() {
				obj := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						UID: types.UID("not-a-uuid"),
					},
				}
				Expect(clientgarden.ValidateObjectMetadata(obj)).To(HaveOccurred())
			})
		})
	})

	Describe("GetShoot", func() {
		const (
			shootName      = "test-shoot"
			shootNamespace = "garden-test"
		)

		BeforeEach(func() {
			ctx = context.Background()
		})

		Context("when UID is a valid UUID", func() {
			It("should return the shoot without error", func() {
				shoot := &gardencorev1beta1.Shoot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      shootName,
						Namespace: shootNamespace,
						UID:       types.UID("550e8400-e29b-41d4-a716-446655440000"),
					},
				}

				gardenClient = clientgarden.NewClient(
					nil,
					fake.NewClientWithObjects(shoot),
					gardenName,
				)

				result, err := gardenClient.GetShoot(ctx, shootNamespace, shootName)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.UID).To(Equal(shoot.UID))
			})
		})

		Context("when UID is invalid", func() {
			It("should fail with a validation error", func() {
				shoot := &gardencorev1beta1.Shoot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      shootName,
						Namespace: shootNamespace,
						UID:       types.UID("not-a-uuid"),
					},
				}

				gardenClient = clientgarden.NewClient(
					nil,
					fake.NewClientWithObjects(shoot),
					gardenName,
				)

				result, err := gardenClient.GetShoot(ctx, shootNamespace, shootName)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("invalid UUID")))
				Expect(result).To(BeNil())
			})
		})
	})

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
					UID:       "00000000-0000-0000-0000-000000000000",
				},
				Data: map[string][]byte{
					"kubeconfig": seed1Kubeconfig,
				},
			}
			loginSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "seed-2.login",
					Namespace: "garden",
					UID:       "00000000-0000-0000-0000-000000000000",
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
					UID:       "00000000-0000-0000-0000-000000000000",
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
			ca          *secrets.Certificate
		)

		BeforeEach(func() {
			ctx = context.Background()
			testShoot1 = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      shootName,
					Namespace: namespace,
					UID:       "00000000-0000-0000-0000-000000000000",
				},
				Spec: gardencorev1beta1.ShootSpec{
					Kubernetes: gardencorev1beta1.Kubernetes{
						Version: k8sVersion,
					},
				},
				Status: gardencorev1beta1.ShootStatus{
					AdvertisedAddresses: []gardencorev1beta1.ShootAdvertisedAddress{
						{
							Name: "external",
							URL:  "https://api." + domain,
						},
						{
							Name: "internal",
							URL:  "https://api2." + domain,
						},
						{
							Name: "service-account-issuer",
							URL:  "https://foo.bar/projects/prod1/shoots/test-shoot1/issuer",
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
					UID:       "00000000-0000-0000-0000-000000000000",
				},
				Data: map[string]string{
					"ca.crt": string(ca.CertificatePEM),
				},
			}
		})

		Context("good case", func() {
			Context("when ca-cluster configmap exists", func() {
				It("it should return the client config", func() {
					gardenClient = clientgarden.NewClient(
						nil,
						fake.NewClientWithObjects(testShoot1, caConfigMap),
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
					gardenClient = clientgarden.NewClient(
						nil,
						fake.NewClientWithObjects(testShoot1, caConfigMap),
						gardenName,
					)

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
		It("Should return the user", func() {
			user := "an-arbitrary-user"

			config := fake.NewTokenConfig("garden")

			cic := createInterceptingClient{
				Client: fake.NewClientWithObjects(),
				createInterceptor: func(ctx context.Context, object client.Object, option ...client.CreateOption) error {
					if tr, ok := object.(*authenticationv1.SelfSubjectReview); ok { // patch the TokenReview
						tr.Name = "foo" // must be set or else the fake client will error because no name was provided
						tr.Status.UserInfo = authenticationv1.UserInfo{
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
	})

	Describe("ValidateJWTFormat", func() {
		It("should reject tokens that are too large", func() {
			// Create a token larger than 16KB (16384 bytes)
			largeToken := strings.Repeat("a", 16385)
			err := clientgarden.ValidateJWTFormat(largeToken)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("token too large"))
		})

		DescribeTable("should reject invalid JWT format",
			func(token, description string) {
				err := clientgarden.ValidateJWTFormat(token)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid JWT format"))
			},
			Entry("missing signature part", "invalid.jwt", "missing signature part"),
			Entry("completely invalid", "not-a-jwt-at-all", "completely invalid"),
			Entry("only two parts", "a.b", "missing signature part"),
			Entry("invalid base64 encoding", "invalid.base64.data", "invalid base64 encoding"),
		)

		It("should accept valid JWT format", func() {
			// Create a minimal valid JWT using base64 encoding to avoid hardcoding
			header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
			payload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"test","iat":1234567890}`))
			signature := base64.RawURLEncoding.EncodeToString([]byte("fake-signature-for-testing"))

			validJWT := fmt.Sprintf("%s.%s.%s", header, payload, signature)

			err := clientgarden.ValidateJWTFormat(validJWT)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should accept valid JWT with proper structure", func() {
			// Create a valid JWT structure that matches what go-jose expects
			// This simulates what a real JWT would look like without needing actual RSA keys
			header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
			payload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"test-subject","iss":"test-issuer","exp":9999999999}`))
			// Create a longer, more realistic signature
			signature := base64.RawURLEncoding.EncodeToString([]byte("this-is-a-longer-fake-signature-that-looks-more-realistic-for-testing-purposes-only"))

			validJWT := fmt.Sprintf("%s.%s.%s", header, payload, signature)

			// The validation should pass (it only checks format, not signature)
			err := clientgarden.ValidateJWTFormat(validJWT)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("CreateWorkloadIdentityToken", func() {
		var (
			ctx          context.Context
			testServer   *httptest.Server
			gardenClient clientgarden.Client
			kubeConfig   *clientcmdapi.Config
		)

		BeforeEach(func() {
			ctx = context.Background()

			kubeConfig = clientcmdapi.NewConfig()
			kubeConfig.Clusters["test-cluster"] = &clientcmdapi.Cluster{
				InsecureSkipTLSVerify: true,
			}
			kubeConfig.AuthInfos["test-user"] = &clientcmdapi.AuthInfo{
				Token: "test-token",
			}
			kubeConfig.Contexts["test-context"] = &clientcmdapi.Context{
				Cluster:  "test-cluster",
				AuthInfo: "test-user",
			}
			kubeConfig.CurrentContext = "test-context"
		})

		AfterEach(func() {
			if testServer != nil {
				testServer.Close()
			}
		})

		It("should reject malformed JWT tokens returned by the server", func() {
			// Create a test server that returns a malformed token
			testServer = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.Contains(r.URL.Path, "/token") {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					// Return a response with a malformed JWT
					response := gardensecurityv1alpha1.TokenRequest{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "security.gardener.cloud/v1alpha1",
							Kind:       "TokenRequest",
						},
						Status: gardensecurityv1alpha1.TokenRequestStatus{
							Token:               "not.a.valid.jwt", // Malformed JWT
							ExpirationTimestamp: metav1.NewTime(time.Now().Add(1 * time.Hour)),
						},
					}
					_ = json.NewEncoder(w).Encode(response)
					return
				}
				w.WriteHeader(http.StatusNotFound)
			}))

			kubeConfig.Clusters["test-cluster"].Server = testServer.URL
			clientConfig := clientcmd.NewDefaultClientConfig(*kubeConfig, &clientcmd.ConfigOverrides{})
			gardenClient = clientgarden.NewClient(clientConfig, fake.NewClientWithObjects(), gardenName)

			// Attempt to create a workload identity token
			_, err := gardenClient.CreateWorkloadIdentityToken(ctx, "test-namespace", "test-identity", 1*time.Hour)

			// The client should reject the malformed token
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("received malformed token"))
			Expect(err.Error()).To(ContainSubstring("invalid JWT format"))
		})

		It("should accept valid JWT tokens returned by the server", func() {
			// Create a valid JWT for testing
			header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
			payload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"test-identity","iss":"test-issuer","exp":9999999999}`))
			signature := base64.RawURLEncoding.EncodeToString([]byte("fake-signature-for-testing"))
			validJWT := fmt.Sprintf("%s.%s.%s", header, payload, signature)

			// Create a test server that returns a valid token
			testServer = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.Contains(r.URL.Path, "/token") {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					response := gardensecurityv1alpha1.TokenRequest{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "security.gardener.cloud/v1alpha1",
							Kind:       "TokenRequest",
						},
						Spec: gardensecurityv1alpha1.TokenRequestSpec{
							ExpirationSeconds: ptr.To(int64(3600)),
						},
						Status: gardensecurityv1alpha1.TokenRequestStatus{
							Token:               validJWT,
							ExpirationTimestamp: metav1.NewTime(time.Now().Add(1 * time.Hour)),
						},
					}
					_ = json.NewEncoder(w).Encode(response)
					return
				}
				w.WriteHeader(http.StatusNotFound)
			}))

			kubeConfig.Clusters["test-cluster"].Server = testServer.URL
			clientConfig := clientcmd.NewDefaultClientConfig(*kubeConfig, &clientcmd.ConfigOverrides{})
			gardenClient = clientgarden.NewClient(clientConfig, fake.NewClientWithObjects(), gardenName)

			// Attempt to create a workload identity token
			tokenRequest, err := gardenClient.CreateWorkloadIdentityToken(ctx, "test-namespace", "test-identity", 1*time.Hour)

			// The client should accept the valid token
			Expect(err).NotTo(HaveOccurred())
			Expect(tokenRequest).NotTo(BeNil())
			Expect(tokenRequest.Status.Token).To(Equal(validJWT))
		})

		It("should reject tokens that are too large", func() {
			// Create an oversized token (> 16KB)
			largeToken := strings.Repeat("a", 16385)

			// Create a test server that returns an oversized token
			testServer = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.Contains(r.URL.Path, "/token") {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					response := gardensecurityv1alpha1.TokenRequest{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "security.gardener.cloud/v1alpha1",
							Kind:       "TokenRequest",
						},
						Status: gardensecurityv1alpha1.TokenRequestStatus{
							Token:               largeToken,
							ExpirationTimestamp: metav1.NewTime(time.Now().Add(1 * time.Hour)),
						},
					}
					_ = json.NewEncoder(w).Encode(response)
					return
				}
				w.WriteHeader(http.StatusNotFound)
			}))

			kubeConfig.Clusters["test-cluster"].Server = testServer.URL
			clientConfig := clientcmd.NewDefaultClientConfig(*kubeConfig, &clientcmd.ConfigOverrides{})
			gardenClient = clientgarden.NewClient(clientConfig, fake.NewClientWithObjects(), gardenName)

			// Attempt to create a workload identity token
			_, err := gardenClient.CreateWorkloadIdentityToken(ctx, "test-namespace", "test-identity", 1*time.Hour)

			// The client should reject the oversized token
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("received malformed token"))
			Expect(err.Error()).To(ContainSubstring("token too large"))
		})
	})
})
