/*
SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package credvalidate_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	credvalidate "github.com/gardener/gardenctl-v2/pkg/provider/credvalidate"
)

func generateValidServiceAccountJsons() []string {
	var serviceaccountJsons []string

	serviceaccountJsons = append(
		serviceaccountJsons,
		getJsonForServiceaccount(getDefaultSTACKITServiceaccount()))

	sa := getDefaultSTACKITServiceaccount()
	sa["keyType"] = "SYSTEM_MANAGED"
	serviceaccountJsons = append(
		serviceaccountJsons,
		getJsonForServiceaccount(sa))

	sa = getDefaultSTACKITServiceaccount()
	sa["keyOrigin"] = "GENERATED"
	serviceaccountJsons = append(
		serviceaccountJsons,
		getJsonForServiceaccount(sa))

	sa = getDefaultSTACKITServiceaccount()
	sa["keyAlgorithm"] = "RSA_4096"
	serviceaccountJsons = append(
		serviceaccountJsons,
		getJsonForServiceaccount(sa))

	sa = getDefaultSTACKITServiceaccount()
	sa["credentials"].(map[string]interface{})["iss"] = "foo-bar@foo.sa.stackit.cloud"
	serviceaccountJsons = append(
		serviceaccountJsons,
		getJsonForServiceaccount(sa))

	return serviceaccountJsons
}

func getDefaultSTACKITServiceaccount() map[string]interface{} {
	privateKey, publicKey := generateDummyKeys()

	return map[string]interface{}{
		"id":           uuid.New().String(),
		"publicKey":    publicKey,
		"createdAt":    time.Now().Format(time.RFC3339),
		"validUntil":   time.Now().Format(time.RFC3339),
		"keyType":      "USER_MANAGED",
		"keyOrigin":    "USER_PROVIDED",
		"keyAlgorithm": "RSA_2048",
		"active":       true,
		"credentials": map[string]interface{}{
			"kid":        uuid.New().String(),
			"iss":        "foo-bar@sa.stackit.cloud",
			"sub":        uuid.New().String(),
			"aud":        "https://foo-bar.stackit.cloud",
			"privateKey": privateKey,
		},
	}
}

func getJsonForServiceaccount(sa map[string]interface{}) string {
	saJson, _ := json.Marshal(sa)
	return string(saJson)
}

var _ = Describe("STACKIT Validator", func() {
	var validator *credvalidate.STACKITValidator

	BeforeEach(func() {
		validator = credvalidate.NewSTACKITValidator(context.Background(), credvalidate.DefaultSTACKITAllowedPatterns())
	})
	Describe("Secret Validation", func() {
		var secret, secretWithOpenstack *corev1.Secret

		BeforeEach(func() {
			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "stackit-secret",
					Namespace: "test-namespace",
				},
				Data: map[string][]byte{
					"project-id":          []byte("7971c1b2-7ae1-4094-9375-db08dee917d1"),
					"serviceaccount.json": []byte(getJsonForServiceaccount(getDefaultSTACKITServiceaccount())),
				},
			}
			secretWithOpenstack = secret.DeepCopy()
			secretWithOpenstack.Data["domainName"] = []byte("default")
			secretWithOpenstack.Data["tenantName"] = []byte("7971c1b2-7ae1-4094-9375-db08dee917d1")
			secretWithOpenstack.Data["username"] = []byte("myuser")
			secretWithOpenstack.Data["password"] = []byte("mypassword")
		})

		Context("Valid credentials", func() {
			It("should succeed with STACKIT credentials", func() {
				creds, err := validator.ValidateSecret(secret)
				Expect(err).NotTo(HaveOccurred())
				Expect(creds).To(HaveKeyWithValue("project-id", "7971c1b2-7ae1-4094-9375-db08dee917d1"))
				Expect(creds).To(HaveKeyWithValue("serviceaccount.json", string(secret.Data["serviceaccount.json"])))
				Expect(len(creds)).To(Equal(2))
			})

			It("should succeed with STACKIT andopenstack credentials", func() {
				creds, err := validator.ValidateSecret(secretWithOpenstack)
				Expect(err).NotTo(HaveOccurred())
				Expect(creds).To(HaveKeyWithValue("domainName", "default"))
				Expect(creds).To(HaveKeyWithValue("tenantName", "7971c1b2-7ae1-4094-9375-db08dee917d1"))
				Expect(creds).To(HaveKeyWithValue("username", "myuser"))
				Expect(creds).To(HaveKeyWithValue("password", "mypassword"))
				Expect(creds).To(HaveKeyWithValue("project-id", "7971c1b2-7ae1-4094-9375-db08dee917d1"))
				Expect(creds).To(HaveKeyWithValue("serviceaccount.json", string(secret.Data["serviceaccount.json"])))
				Expect(len(creds)).To(Equal(6))
			})

			It("should ignore extra top-level secret keys (Permissive mode)", func() {
				secret.Data["foo"] = []byte("bar")
				creds, err := validator.ValidateSecret(secretWithOpenstack)
				Expect(err).NotTo(HaveOccurred())
				Expect(creds).To(HaveKey("domainName"))
				Expect(creds).To(HaveKey("tenantName"))
				Expect(creds).To(HaveKey("username"))
				Expect(creds).To(HaveKey("password"))
				Expect(creds).To(HaveKey("project-id"))
				Expect(creds).To(HaveKey("serviceaccount.json"))
				Expect(creds).NotTo(HaveKey("foo"))
				Expect(len(creds)).To(Equal(6))
			})
		})

		Context("Missing fields", func() {
			DescribeTable("should fail when required fields are missing",
				func(modifySecret func(*corev1.Secret), expectedError string) {
					modifySecret(secret)
					_, err := validator.ValidateSecret(secret)
					Expect(err).To(MatchError(expectedError))
				},
				Entry("missing serviceaccount.json",
					func(s *corev1.Secret) { delete(s.Data, "serviceaccount.json") },
					"validation error in field \"serviceaccount.json\": required field is missing",
				),
				Entry("missing project-id",
					func(s *corev1.Secret) { delete(s.Data, "project-id") },
					"validation error in field \"project-id\": required field is missing",
				),
			)
		})

		Context("Empty fields", func() {
			DescribeTable("should fail when required fields are empty",
				func(modifySecret func(*corev1.Secret), expectedError string) {
					modifySecret(secret)
					_, err := validator.ValidateSecret(secret)
					Expect(err).To(MatchError(expectedError))
				},
				Entry("empty serviceaccount.json",
					func(s *corev1.Secret) { s.Data["serviceaccount.json"] = []byte("") },
					"validation error in field \"serviceaccount.json\": required field cannot be empty",
				),
				Entry("empty project-id",
					func(s *corev1.Secret) { s.Data["project-id"] = []byte("") },
					"validation error in field \"project-id\": required field cannot be empty",
				),
			)

			It("should fail when secret.Data is nil", func() {
				secret.Data = nil
				_, err := validator.ValidateSecret(secret)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("Invalid fields", func() {
			DescribeTable("should fail when required fields are invalid",
				func(modifySecret func(*corev1.Secret), expectedError string) {
					modifySecret(secret)
					_, err := validator.ValidateSecret(secret)
					Expect(err).To(MatchError(expectedError))
				},
				Entry("not json serviceaccount.json",
					func(s *corev1.Secret) { s.Data["serviceaccount.json"] = []byte("{{{") },
					"validation error in field \"serviceaccount.json\": no valid json invalid character '{' looking for beginning of object key string",
				),
				Entry("serviceaccount.json keyType",
					func(s *corev1.Secret) {
						sa := getDefaultSTACKITServiceaccount()
						sa["keyType"] = "INVALID"
						s.Data["serviceaccount.json"] = []byte(getJsonForServiceaccount(sa))
					},
					"pattern mismatch in field \"keyType\": does not match any allowed patterns (actual: \"INVALID\")",
				),
				Entry("serviceaccount.json keyOrigin",
					func(s *corev1.Secret) {
						sa := getDefaultSTACKITServiceaccount()
						sa["keyOrigin"] = "INVALID"
						s.Data["serviceaccount.json"] = []byte(getJsonForServiceaccount(sa))
					},
					"pattern mismatch in field \"keyOrigin\": does not match any allowed patterns (actual: \"INVALID\")",
				),
				Entry("serviceaccount.json keyAlgorithm",
					func(s *corev1.Secret) {
						sa := getDefaultSTACKITServiceaccount()
						sa["keyAlgorithm"] = "INVALID"
						s.Data["serviceaccount.json"] = []byte(getJsonForServiceaccount(sa))
					},
					"pattern mismatch in field \"keyAlgorithm\": does not match any allowed patterns (actual: \"INVALID\")",
				),
				Entry("serviceaccount.json iss",
					func(s *corev1.Secret) {
						sa := getDefaultSTACKITServiceaccount()
						sa["credentials"].(map[string]interface{})["iss"] = "foobar@example.com"
						s.Data["serviceaccount.json"] = []byte(getJsonForServiceaccount(sa))
					},
					"pattern mismatch in field \"iss\": does not match any allowed patterns (actual: \"foobar@example.com\")",
				),
				Entry("serviceaccount.json privateKey",
					func(s *corev1.Secret) {
						sa := getDefaultSTACKITServiceaccount()
						sa["credentials"].(map[string]interface{})["privateKey"] = "invalid"
						s.Data["serviceaccount.json"] = []byte(getJsonForServiceaccount(sa))
					},
					"validation error in field \"privateKey\": field value must start with a PEM BEGIN line",
				),
				Entry("non uuid ad project-id",
					func(s *corev1.Secret) { s.Data["project-id"] = []byte("foobar") },
					"pattern mismatch in field \"project-id\": does not match any allowed patterns (actual: \"foobar\")",
				),
			)

			It("should fail when secret.Data is nil", func() {
				secret.Data = nil
				_, err := validator.ValidateSecret(secret)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("Valid serviceaccout.json", func() {
			It("Should work with all valid serviceaccount.json", func() {
				for _, sa := range generateValidServiceAccountJsons() {
					secret.Data["serviceaccount.json"] = []byte(sa)
					_, err := validator.ValidateSecret(secret)
					Expect(err).To(Succeed())
				}
			})
		})
	})
})

// generateDummyPrivateKey creates a dummy RSA private key for testing purposes.
func generateDummyKeys() (string, string) {
	// Generate a larger RSA key for testing (2048 bits to meet length requirements)
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(fmt.Sprintf("Failed to generate dummy private key: %v", err))
	}

	// Convert to PKCS#8 format
	publicKeyBytes, _ := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)

	// Create PEM block
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	})

	// Convert to PKCS#8 format
	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)

	// Create PEM block
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyBytes,
	})
	// Return the raw PEM string (not JSON-escaped)
	return string(privateKeyPEM), string(publicKeyPEM)
}
