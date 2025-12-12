/*
SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package credvalidate_test

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
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardenctl-v2/pkg/provider/common/allowpattern"
	commoncredvalidate "github.com/gardener/gardenctl-v2/pkg/provider/common/credvalidate"
	credvalidate "github.com/gardener/gardenctl-v2/pkg/provider/credvalidate"
)

// generateDummyPrivateKey creates a dummy RSA private key for testing purposes.
func generateDummyPrivateKey() string {
	// Generate a larger RSA key for testing (2048 bits to meet length requirements)
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(fmt.Sprintf("Failed to generate dummy private key: %v", err))
	}

	// Convert to PKCS#8 format
	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		panic(fmt.Sprintf("Failed to marshal private key: %v", err))
	}

	// Create PEM block
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyBytes,
	})

	// Return the raw PEM string (not JSON-escaped)
	return string(privateKeyPEM)
}

func parseServiceaccountJSON(fields map[string]interface{}) map[string]interface{} {
	serviceaccountJSON := map[string]interface{}{}
	raw := fields["serviceaccount.json"].(string)
	Expect(raw).NotTo(BeNil(), "expected serviceaccount.json key in returned credentials")

	Expect(json.Unmarshal([]byte(raw), &serviceaccountJSON)).To(Succeed())

	return serviceaccountJSON
}

var _ = Describe("GCP Validator", func() {
	var validator *credvalidate.GCPValidator

	BeforeEach(func() {
		allowedPatterns := credvalidate.DefaultGCPAllowedPatterns()
		validator = credvalidate.NewGCPValidator(context.Background(), allowedPatterns)
	})

	Describe("Default Allowed Patterns", func() {
		It("should return valid default patterns", func() {
			patterns := credvalidate.DefaultGCPAllowedPatterns()
			Expect(patterns).NotTo(BeEmpty())

			// Verify all patterns are valid
			for _, pattern := range patterns {
				Expect(pattern.ValidateWithContext(credvalidate.GetGCPValidationContext())).To(Succeed())
			}
		})

		It("should include expected fields in default patterns", func() {
			patterns := credvalidate.DefaultGCPAllowedPatterns()
			fields := make(map[string]bool)
			for _, pattern := range patterns {
				fields[pattern.Field] = true
			}

			Expect(fields).To(HaveLen(8))

			Expect(fields).To(HaveKey("private_key_id"))
			Expect(fields).To(HaveKey("client_id"))
			Expect(fields).To(HaveKey("client_email"))

			// Check for domain fields
			Expect(fields).To(HaveKey("universe_domain"))

			// Check for expected service account fields
			Expect(fields).To(HaveKey("token_uri"))
			Expect(fields).To(HaveKey("auth_uri"))
			Expect(fields).To(HaveKey("auth_provider_x509_cert_url"))
			Expect(fields).To(HaveKey("client_x509_cert_url"))

			// Verify project_id is NOT in patterns (hardcoded validation only)
			Expect(fields).NotTo(HaveKey("project_id"))
		})
	})

	Describe("Secret Validation", func() {
		var (
			validMinimalJSON = `{
				"type": "service_account",
				"project_id": "test-project-12345"
			}`

			secretName = "gcp"
			secret     *corev1.Secret
		)

		// generateCompleteValidJSON creates a complete valid JSON with a dynamically generated private key
		generateCompleteValidJSON := func() string {
			dummyPrivateKey := generateDummyPrivateKey()

			return fmt.Sprintf(`{
				"type": "service_account",
				"project_id": "test-project-12345",
				"private_key_id": "1234567890abcdef1234567890abcdef12345678",
				"private_key": %q,
				"client_email": "test-service-account@test-project-12345.iam.gserviceaccount.com",
				"client_id": "123456789012345678901",
				"auth_uri": "https://accounts.google.com/o/oauth2/auth",
				"token_uri": "https://oauth2.googleapis.com/token",
				"auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
				"client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/test-service-account%%40test-project-12345.iam.gserviceaccount.com",
				"universe_domain": "googleapis.com"
			}`, dummyPrivateKey)
		}

		BeforeEach(func() {
			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: secretName,
				},
				Data: map[string][]byte{
					"serviceaccount.json": []byte(validMinimalJSON),
				},
			}
		})

		Context("Valid JSON parsing", func() {
			It("should succeed for minimal valid JSON", func() {
				fields, err := validator.ValidateSecret(secret)
				Expect(err).NotTo(HaveOccurred())
				Expect(fields).To(HaveKey("serviceaccount.json"))
				Expect(len(fields)).To(Equal(1))
				serviceaccountJSON := parseServiceaccountJSON(fields)
				Expect(serviceaccountJSON).To(HaveKeyWithValue("type", "service_account"))
				Expect(serviceaccountJSON).To(HaveKeyWithValue("project_id", "test-project-12345"))
				Expect(len(serviceaccountJSON)).To(Equal(2))
			})

			It("should ignore extra top-level secret keys (Permissive mode)", func() {
				// Add an unrelated extra key at the top level alongside a valid serviceaccount.json
				secret.Data["foo"] = []byte("bar")
				fields, err := validator.ValidateSecret(secret)
				Expect(err).NotTo(HaveOccurred())
				// Only registered fields should be returned; extra keys are ignored
				Expect(fields).To(HaveKey("serviceaccount.json"))
				Expect(fields).NotTo(HaveKey("foo"))
			})

			It("should succeed for complete valid JSON", func() {
				secret.Data["serviceaccount.json"] = []byte(generateCompleteValidJSON())
				fields, err := validator.ValidateSecret(secret)
				Expect(err).NotTo(HaveOccurred())
				serviceaccountJSON := parseServiceaccountJSON(fields)
				Expect(serviceaccountJSON).To(HaveKeyWithValue("type", "service_account"))
				Expect(serviceaccountJSON).To(HaveKeyWithValue("project_id", "test-project-12345"))
				Expect(serviceaccountJSON).To(HaveKeyWithValue("private_key_id", "1234567890abcdef1234567890abcdef12345678"))
				Expect(serviceaccountJSON).To(HaveKey("private_key"))
				Expect(serviceaccountJSON).To(HaveKeyWithValue("client_email", "test-service-account@test-project-12345.iam.gserviceaccount.com"))
				Expect(serviceaccountJSON).To(HaveKeyWithValue("client_id", "123456789012345678901"))
				Expect(serviceaccountJSON).To(HaveKeyWithValue("auth_uri", "https://accounts.google.com/o/oauth2/auth"))
				Expect(serviceaccountJSON).To(HaveKeyWithValue("token_uri", "https://oauth2.googleapis.com/token"))
				Expect(serviceaccountJSON).To(HaveKeyWithValue("auth_provider_x509_cert_url", "https://www.googleapis.com/oauth2/v1/certs"))
				Expect(serviceaccountJSON).To(HaveKeyWithValue("client_x509_cert_url", "https://www.googleapis.com/robot/v1/metadata/x509/test-service-account%40test-project-12345.iam.gserviceaccount.com"))
				Expect(serviceaccountJSON).To(HaveKeyWithValue("universe_domain", "googleapis.com"))
				Expect(serviceaccountJSON).To(HaveLen(11))
			})
		})

		Context("Invalid JSON and missing data", func() {
			It("should fail when secret.Data is nil", func() {
				secret.Data = nil
				_, err := validator.ValidateSecret(secret)
				Expect(err).To(MatchError(ContainSubstring("required field is missing")))
			})

			It("should fail with missing secret data", func() {
				secret.Data["serviceaccount.json"] = nil
				_, err := validator.ValidateSecret(secret)
				Expect(err).To(MatchError("validation error in field \"serviceaccount.json\": required field cannot be empty"))
			})

			It("should fail with missing serviceaccount.json key", func() {
				secret.Data = map[string][]byte{}
				_, err := validator.ValidateSecret(secret)
				Expect(err).To(MatchError("validation error in field \"serviceaccount.json\": required field is missing"))
			})

			It("should fail with invalid json", func() {
				secret.Data["serviceaccount.json"] = []byte("{")
				_, err := validator.ValidateSecret(secret)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("failed to unmarshal service account JSON")))
				Expect(err).To(MatchError(ContainSubstring("unexpected end of JSON input")))
			})

			It("should fail with completely malformed JSON", func() {
				secret.Data["serviceaccount.json"] = []byte("not json at all")
				_, err := validator.ValidateSecret(secret)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("failed to unmarshal service account JSON")))
			})
		})

		Context("Field type validation", func() {
			It("should fail with non-string values", func() {
				secret.Data["serviceaccount.json"] = []byte(`{
					"type": "service_account",
					"project_id": 123,
					"client_email": "test-service-account@test-project-12345.iam.gserviceaccount.com"
				}`)
				_, err := validator.ValidateSecret(secret)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("validation error in field \"project_id\": field value must be a string"))
			})

			It("should fail with boolean values", func() {
				secret.Data["serviceaccount.json"] = []byte(`{
					"type": true,
					"project_id": "test-project-12345"
				}`)
				_, err := validator.ValidateSecret(secret)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("validation error in field \"type\": field value must be a string"))
			})

			It("should fail with array values", func() {
				secret.Data["serviceaccount.json"] = []byte(`{
					"type": "service_account",
					"project_id": ["test-project-12345"]
				}`)
				_, err := validator.ValidateSecret(secret)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("validation error in field \"project_id\": field value must be a string"))
			})
		})

		Context("Required field validation", func() {
			It("should fail with missing type field", func() {
				secret.Data["serviceaccount.json"] = []byte(`{
					"project_id": "test-project-12345"
				}`)
				_, err := validator.ValidateSecret(secret)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("validation error in field \"type\": required field is missing"))
			})

			It("should fail with missing project_id field", func() {
				secret.Data["serviceaccount.json"] = []byte(`{
					"type": "service_account"
				}`)
				_, err := validator.ValidateSecret(secret)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("validation error in field \"project_id\": required field is missing"))
			})

			It("should fail with empty type field", func() {
				secret.Data["serviceaccount.json"] = []byte(`{
					"type": "",
					"project_id": "test-project-12345"
				}`)
				_, err := validator.ValidateSecret(secret)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("validation error in field \"type\": required field cannot be empty"))
			})

			It("should fail with empty project_id field", func() {
				secret.Data["serviceaccount.json"] = []byte(`{
					"type": "service_account",
					"project_id": ""
				}`)
				_, err := validator.ValidateSecret(secret)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("validation error in field \"project_id\": required field cannot be empty"))
			})
		})

		Context("Type field validation", func() {
			It("should fail with incorrect type value", func() {
				secret.Data["serviceaccount.json"] = []byte(`{
					"type": "wrong_type",
					"project_id": "test-project-12345"
				}`)
				_, err := validator.ValidateSecret(secret)
				Expect(err).To(MatchError("validation error in field \"type\": type must be 'service_account' (value: \"wrong_type\")"))
			})

			It("should fail with external_account type", func() {
				secret.Data["serviceaccount.json"] = []byte(`{
					"type": "external_account",
					"project_id": "test-project-12345"
				}`)
				_, err := validator.ValidateSecret(secret)
				Expect(err).To(MatchError("validation error in field \"type\": type must be 'service_account' (value: \"external_account\")"))
			})
		})

		Context("Project ID validation", func() {
			It("should succeed with valid project IDs", func() {
				validProjectIDs := []string{
					"test-project-12345",
					"abcdef",                        // minimum 6 chars (1 + 4 + 1)
					"my-project-name-1234567890123", // exactly 30 chars (max allowed)
					"project-with-dashes-123",
				}

				for _, projectID := range validProjectIDs {
					secret.Data["serviceaccount.json"] = []byte(fmt.Sprintf(`{
						"type": "service_account",
						"project_id": "%s"
					}`, projectID))
					_, err := validator.ValidateSecret(secret)
					Expect(err).NotTo(HaveOccurred(), "should accept valid project ID: %s", projectID)
				}
			})

			DescribeTable("should fail with invalid project IDs",
				func(projectID, reason string) {
					secret.Data["serviceaccount.json"] = []byte(fmt.Sprintf(`{
						"type": "service_account",
						"project_id": "%s"
					}`, projectID))
					_, err := validator.ValidateSecret(secret)
					Expect(err).To(HaveOccurred(), "should reject invalid project ID: %s (%s)", projectID, reason)
					Expect(err).To(MatchError(ContainSubstring("validation error in field \"project_id\": field does not match the expected format")))
					Expect(err).To(MatchError(ContainSubstring(fmt.Sprintf("value: %q", projectID))))
				},
				Entry("starts with number", "123-starts-with-number", "project IDs cannot start with numbers"),
				Entry("too short", "abc", "project IDs must be at least 6 characters"),
				Entry("too long", "project-name-that-is-way-too-long-12345", "project IDs cannot exceed 30 characters"),
				Entry("contains underscores", "project_with_underscores", "project IDs cannot contain underscores"),
				Entry("contains capitals", "Project-With-Capitals", "project IDs must be lowercase"),
				Entry("ends with dash", "project-ending-with-dash-", "project IDs cannot end with dash"),
				Entry("starts with dash", "-project-starting-with-dash", "project IDs cannot start with dash"),
				Entry("contains double dots", "project..double.dots", "project IDs cannot contain consecutive dots"),
			)
		})

		Context("Field length validation", func() {
			DescribeTable("should fail when fields are too long",
				func(field string, value string, expectedError string) {
					jsonData := fmt.Sprintf(`{
						"type": "service_account",
						"project_id": "test-project-12345",
						"%s": "%s"
					}`, field, value)
					secret.Data["serviceaccount.json"] = []byte(jsonData)
					_, err := validator.ValidateSecret(secret)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(ContainSubstring(expectedError)))
				},
				Entry("private_key_id too long (41 chars)",
					"private_key_id", "1234567890abcdef1234567890abcdef123456789",
					`pattern mismatch in field "private_key_id": does not match any allowed patterns`,
				),
				Entry("client_id too long (26 chars)",
					"client_id", "12345678901234567890123456",
					`pattern mismatch in field "client_id": does not match any allowed patterns`,
				),
			)
		})

		Context("Disallowed fields", func() {
			It("should fail with disallowed fields", func() {
				secret.Data["serviceaccount.json"] = []byte(`{
					"type": "service_account",
					"project_id": "test-project-12345",
					"client_email": "test-service-account@test-project-12345.iam.gserviceaccount.com",
					"unknown_field": "value"
				}`)
				_, err := validator.ValidateSecret(secret)
				Expect(err).To(MatchError("validation error in field \"unknown_field\": field is not allowed"))
			})

			It("should fail with multiple disallowed fields", func() {
				secret.Data["serviceaccount.json"] = []byte(`{
					"type": "service_account",
					"project_id": "test-project-12345",
					"malicious_field": "value",
					"another_bad_field": "value2"
				}`)
				_, err := validator.ValidateSecret(secret)
				Expect(err).To(HaveOccurred())
				// Only assert generic message once for any of the fields
				Expect(strings.Count(err.Error(), "field is not allowed")).To(BeNumerically(">=", 1))
			})
		})

		Context("URI validation", func() {
			It("should fail with invalid URI scheme (ftp)", func() {
				secret.Data["serviceaccount.json"] = []byte(`{
					"type": "service_account",
					"project_id": "test-project-12345",
					"client_email": "test-service-account@test-project-12345.iam.gserviceaccount.com",
					"auth_uri": "ftp://accounts.google.com/o/oauth2/auth"
				}`)
				_, err := validator.ValidateSecret(secret)
				Expect(err).To(MatchError("validation error in field \"auth_uri\": failed to validate URI: scheme must be one of {https, http}, got \"ftp\""))
			})

			It("should fail with invalid URI scheme (http)", func() {
				secret.Data["serviceaccount.json"] = []byte(`{
					"type": "service_account",
					"project_id": "test-project-12345",
					"client_email": "test-service-account@test-project-12345.iam.gserviceaccount.com",
					"auth_uri": "http://accounts.google.com/o/oauth2/auth"
				}`)
				_, err := validator.ValidateSecret(secret)
				Expect(err).To(MatchError(ContainSubstring("pattern mismatch in field \"auth_uri\": does not match any allowed patterns")))
				Expect(err).To(MatchError(ContainSubstring("actual: \"http://accounts.google.com/o/oauth2/auth\"")))
			})

			It("should fail with empty hostname in URI", func() {
				secret.Data["serviceaccount.json"] = []byte(`{
					"type": "service_account",
					"project_id": "test-project-12345",
					"client_email": "test-service-account@test-project-12345.iam.gserviceaccount.com",
					"auth_uri": "https:///o/oauth2/auth"
				}`)
				_, err := validator.ValidateSecret(secret)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("validation error in field \"auth_uri\": failed to validate URI: hostname is required")))
			})

			It("should fail with ftp scheme", func() {
				secret.Data["serviceaccount.json"] = []byte(`{
					"type": "service_account",
					"project_id": "test-project-12345",
					"auth_uri": "ftp://accounts.google.com/o/oauth2/auth"
				}`)
				_, err := validator.ValidateSecret(secret)
				Expect(err).To(MatchError("validation error in field \"auth_uri\": failed to validate URI: scheme must be one of {https, http}, got \"ftp\""))
			})

			It("should fail with query parameters in URI", func() {
				secret.Data["serviceaccount.json"] = []byte(`{
					"type": "service_account",
					"project_id": "test-project-12345",
					"client_email": "test-service-account@test-project-12345.iam.gserviceaccount.com",
					"auth_uri": "https://accounts.google.com/o/oauth2/auth?param=value"
				}`)
				_, err := validator.ValidateSecret(secret)
				Expect(err).To(MatchError("validation error in field \"auth_uri\": failed to validate URI: must not contain query parameters"))
			})

			It("should fail with fragments in URI", func() {
				secret.Data["serviceaccount.json"] = []byte(`{
					"type": "service_account",
					"project_id": "test-project-12345",
					"client_email": "test-service-account@test-project-12345.iam.gserviceaccount.com",
					"token_uri": "https://oauth2.googleapis.com/token#fragment"
				}`)
				_, err := validator.ValidateSecret(secret)
				Expect(err).To(MatchError("validation error in field \"token_uri\": failed to validate URI: must not contain fragments"))
			})

			It("should fail with both query parameters and fragments in URI", func() {
				secret.Data["serviceaccount.json"] = []byte(`{
					"type": "service_account",
					"project_id": "test-project-12345",
					"client_email": "test-service-account@test-project-12345.iam.gserviceaccount.com",
					"auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs?param=value#fragment"
				}`)
				_, err := validator.ValidateSecret(secret)
				Expect(err).To(MatchError("validation error in field \"auth_provider_x509_cert_url\": failed to validate URI: must not contain query parameters"))
			})

			It("should fail with userinfo in URI", func() {
				secret.Data["serviceaccount.json"] = []byte(`{
					"type": "service_account",
					"project_id": "test-project-12345",
					"client_email": "test-service-account@test-project-12345.iam.gserviceaccount.com",
					"auth_uri": "https://user@accounts.google.com/o/oauth2/auth"
				}`)
				_, err := validator.ValidateSecret(secret)
				Expect(err).To(MatchError("validation error in field \"auth_uri\": failed to validate URI: must not contain userinfo"))
			})

			It("should fail with userinfo containing password", func() {
				secret.Data["serviceaccount.json"] = []byte(`{
					"type": "service_account",
					"project_id": "test-project-12345",
					"token_uri": "https://user:pass@oauth2.googleapis.com/token"
				}`)
				_, err := validator.ValidateSecret(secret)
				Expect(err).To(MatchError("validation error in field \"token_uri\": failed to validate URI: must not contain userinfo"))
			})

			It("should fail with malformed URI", func() {
				secret.Data["serviceaccount.json"] = []byte(`{
					"type": "service_account",
					"project_id": "test-project-12345",
					"auth_uri": "://invalid-uri"
				}`)
				_, err := validator.ValidateSecret(secret)
				Expect(err).To(MatchError("validation error in field \"auth_uri\": failed to validate URI: invalid URI"))
			})
		})

		Context("Pattern matching", func() {
			It("should fail with disallowed URIs", func() {
				secret.Data["serviceaccount.json"] = []byte(`{
					"type": "service_account",
					"project_id": "test-project-12345",
					"client_email": "test-service-account@test-project-12345.iam.gserviceaccount.com",
					"auth_uri": "https://malicious-domain.com/o/oauth2/auth"
				}`)
				_, err := validator.ValidateSecret(secret)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring(`pattern mismatch in field "auth_uri": does not match any allowed patterns`)))
				Expect(err).To(MatchError(ContainSubstring(fmt.Sprintf("actual: %q", "https://malicious-domain.com/o/oauth2/auth"))))
			})

			It("should fail with disallowed universe domain", func() {
				secret.Data["serviceaccount.json"] = []byte(`{
					"type": "service_account",
					"project_id": "test-project-12345",
					"client_email": "test-service-account@test-project-12345.iam.gserviceaccount.com",
					"universe_domain": "malicious-domain.com"
				}`)
				_, err := validator.ValidateSecret(secret)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring(`pattern mismatch in field "universe_domain": does not match any allowed patterns`)))
				Expect(err).To(MatchError(ContainSubstring("actual: \"malicious-domain.com\"")))
			})

			It("should succeed with additional allowed patterns", func() {
				secret.Data["serviceaccount.json"] = []byte(`{
					"type": "service_account",
					"project_id": "test-project-12345",
					"client_email": "test-service-account@test-project-12345.iam.gserviceaccount.com",
					"auth_uri": "https://custom-domain.com/auth",
					"universe_domain": "custom-domain.com"
				}`)
				allowedPatterns := credvalidate.DefaultGCPAllowedPatterns()
				// Add additional patterns for custom domains
				allowedPatterns = append(allowedPatterns, []allowpattern.Pattern{
					{
						Field: "auth_uri",
						URI:   "https://custom-domain.com/auth",
					},
					{
						Field: "universe_domain",
						Host:  ptr.To("custom-domain.com"),
						Path:  ptr.To(""),
					},
				}...)
				customValidator := credvalidate.NewGCPValidator(context.Background(), allowedPatterns)
				validatedFields, err := customValidator.ValidateSecret(secret)
				Expect(err).NotTo(HaveOccurred())
				Expect(validatedFields).To(HaveLen(1))
				serviceaccountJSON := parseServiceaccountJSON(validatedFields)
				Expect(serviceaccountJSON).To(HaveKeyWithValue("auth_uri", "https://custom-domain.com/auth"))
				Expect(serviceaccountJSON).To(HaveKeyWithValue("universe_domain", "custom-domain.com"))
				Expect(serviceaccountJSON).To(HaveLen(5))
			})
		})

		Context("All URI fields validation", func() {
			It("should test all URI fields", func() {
				secret.Data["serviceaccount.json"] = []byte(`{
					"type": "service_account",
					"project_id": "test-project-12345",
					"client_email": "test-service-account@test-project-12345.iam.gserviceaccount.com",
					"auth_uri": "https://accounts.google.com/o/oauth2/auth",
					"token_uri": "https://oauth2.googleapis.com/token",
					"auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
					"client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/test-service-account%40test-project-12345.iam.gserviceaccount.com"
				}`)
				validatedFields, err := validator.ValidateSecret(secret)
				Expect(err).NotTo(HaveOccurred())
				Expect(validatedFields).To(HaveLen(1))
				serviceaccountJSON := parseServiceaccountJSON(validatedFields)
				Expect(serviceaccountJSON).To(HaveKeyWithValue("auth_uri", "https://accounts.google.com/o/oauth2/auth"))
				Expect(serviceaccountJSON).To(HaveKeyWithValue("token_uri", "https://oauth2.googleapis.com/token"))
				Expect(serviceaccountJSON).To(HaveKeyWithValue("auth_provider_x509_cert_url", "https://www.googleapis.com/oauth2/v1/certs"))
				Expect(serviceaccountJSON).To(HaveKeyWithValue("client_x509_cert_url", "https://www.googleapis.com/robot/v1/metadata/x509/test-service-account%40test-project-12345.iam.gserviceaccount.com"))
				Expect(serviceaccountJSON).To(HaveLen(7))
			})

			It("should succeed with alternative token_uri", func() {
				secret.Data["serviceaccount.json"] = []byte(`{
					"type": "service_account",
					"project_id": "test-project-12345",
					"client_email": "test-service-account@test-project-12345.iam.gserviceaccount.com",
					"token_uri": "https://accounts.google.com/o/oauth2/token"
				}`)
				_, err := validator.ValidateSecret(secret)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should fail with invalid token_uri", func() {
				secret.Data["serviceaccount.json"] = []byte(`{
					"type": "service_account",
					"project_id": "test-project-12345",
					"client_email": "test-service-account@test-project-12345.iam.gserviceaccount.com",
					"token_uri": "https://invalid.com/token"
				}`)
				_, err := validator.ValidateSecret(secret)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring(`pattern mismatch in field "token_uri": does not match any allowed patterns`)))
				Expect(err).To(MatchError(ContainSubstring(fmt.Sprintf("actual: %q", "https://invalid.com/token"))))
			})
		})

		Context("Compute Engine default service account email", func() {
			It("should accept compute default SA client_email", func() {
				secret.Data["serviceaccount.json"] = []byte(`{
					"type": "service_account",
					"project_id": "test-project-12345",
					"client_email": "123456789012-compute@developer.gserviceaccount.com"
				}`)
				_, err := validator.ValidateSecret(secret)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("Client X509 certificate URL validation", func() {
			It("should succeed with correct client_x509_cert_url", func() {
				secret.Data["serviceaccount.json"] = []byte(`{
					"type": "service_account",
					"project_id": "test-project-12345",
					"client_email": "test-service-account@test-project-12345.iam.gserviceaccount.com",
					"client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/test-service-account%40test-project-12345.iam.gserviceaccount.com"
				}`)
				_, err := validator.ValidateSecret(secret)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should fail with incorrect client_x509_cert_url", func() {
				secret.Data["serviceaccount.json"] = []byte(`{
					"type": "service_account",
					"project_id": "test-project-12345",
					"client_email": "test-service-account@test-project-12345.iam.gserviceaccount.com",
					"client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/wrong%40email.com"
				}`)
				_, err := validator.ValidateSecret(secret)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring(`pattern mismatch in field "client_x509_cert_url": does not match any allowed patterns`)))
				Expect(err).To(MatchError(ContainSubstring(fmt.Sprintf("actual: %q", "https://www.googleapis.com/robot/v1/metadata/x509/wrong%40email.com"))))
			})

			It("should fail with different host in client_x509_cert_url", func() {
				secret.Data["serviceaccount.json"] = []byte(`{
					"type": "service_account",
					"project_id": "test-project-12345",
					"client_email": "test-service-account@test-project-12345.iam.gserviceaccount.com",
					"client_x509_cert_url": "https://malicious.com/robot/v1/metadata/x509/test-service-account%40test-project-12345.iam.gserviceaccount.com"
				}`)
				_, err := validator.ValidateSecret(secret)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring(`pattern mismatch in field "client_x509_cert_url": does not match any allowed patterns`)))
				Expect(err).To(MatchError(ContainSubstring(fmt.Sprintf("actual: %q", "https://malicious.com/robot/v1/metadata/x509/test-service-account%40test-project-12345.iam.gserviceaccount.com"))))
			})

			It("should fail if client_email is missing for client_x509_cert_url", func() {
				secret.Data["serviceaccount.json"] = []byte(`{
					"type": "service_account",
					"project_id": "test-project-12345",
					"client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/test-service-account%40test-project-12345.iam.gserviceaccount.com"
				}`)
				_, err := validator.ValidateSecret(secret)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring(`pattern mismatch in field "client_x509_cert_url": does not match any allowed patterns`)))
				Expect(err).To(MatchError(ContainSubstring(fmt.Sprintf("actual: %q", "https://www.googleapis.com/robot/v1/metadata/x509/test-service-account%40test-project-12345.iam.gserviceaccount.com"))))
			})

			It("should fail with query parameters in client_x509_cert_url", func() {
				secret.Data["serviceaccount.json"] = []byte(`{
					"type": "service_account",
					"project_id": "test-project-12345",
					"client_email": "test-service-account@test-project-12345.iam.gserviceaccount.com",
					"client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/test-service-account%40test-project-12345.iam.gserviceaccount.com?malicious=param"
				}`)
				_, err := validator.ValidateSecret(secret)
				Expect(err).To(MatchError("validation error in field \"client_x509_cert_url\": failed to validate URI: must not contain query parameters"))
			})

			It("should fail with fragments in client_x509_cert_url", func() {
				secret.Data["serviceaccount.json"] = []byte(`{
					"type": "service_account",
					"project_id": "test-project-12345",
					"client_email": "test-service-account@test-project-12345.iam.gserviceaccount.com",
					"client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/test-service-account%40test-project-12345.iam.gserviceaccount.com#malicious"
				}`)
				_, err := validator.ValidateSecret(secret)
				Expect(err).To(MatchError("validation error in field \"client_x509_cert_url\": failed to validate URI: must not contain fragments"))
			})
		})
	})

	Describe("NewPatternMismatchError", func() {
		It("redacts values for sensitive fields by default", func() {
			err := commoncredvalidate.NewPatternMismatchErrorWithValues(
				"private_key", "does not match any allowed patterns", "SECRET", "EXPECTED", false,
			)
			Expect(err.Error()).To(Equal(`pattern mismatch in field "private_key": does not match any allowed patterns`))
			Expect(err.Error()).ToNot(ContainSubstring("SECRET"))
			Expect(err.Error()).ToNot(ContainSubstring("EXPECTED"))
			Expect(err.Error()).ToNot(ContainSubstring("actual:"))
		})

		It("includes values when field is NonSensitive without debug", func() {
			err := commoncredvalidate.NewPatternMismatchErrorWithValues(
				"auth_uri", "does not match allowed scheme", "http", "https", true,
			)
			Expect(err).To(MatchError(ContainSubstring(`actual: "http"`)))
			Expect(err).To(MatchError(ContainSubstring(`expected: "https"`)))
		})

		It("includes values when unsafe debug is enabled for sensitive fields", func() {
			GinkgoT().Setenv("GCTL_UNSAFE_DEBUG", "true")
			err := commoncredvalidate.NewPatternMismatchErrorWithValues(
				"private_key", "does not match any allowed patterns", "SECRET", "EXPECTED", false,
			)
			Expect(err).To(MatchError(ContainSubstring(`actual: "SECRET"`)))
			Expect(err).To(MatchError(ContainSubstring(`expected: "EXPECTED"`)))
		})
	})

	Describe("validatePrivateKey", func() {
		var validator *commoncredvalidate.BaseValidator

		BeforeEach(func() {
			validator = commoncredvalidate.NewBaseValidator(context.Background(), nil) // No patterns needed for private key validation
		})

		// Helper function to generate a valid RSA private key in PKCS#8 PEM format
		generateValidPrivateKey := func(bits int) string {
			privateKey, err := rsa.GenerateKey(rand.Reader, bits)
			Expect(err).NotTo(HaveOccurred())

			privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
			Expect(err).NotTo(HaveOccurred())

			privateKeyPEM := pem.EncodeToMemory(&pem.Block{
				Type:  "PRIVATE KEY",
				Bytes: privateKeyBytes,
			})

			return string(privateKeyPEM)
		}

		// Helper function to generate an invalid RSA private key (corrupted)
		generateInvalidRSAKey := func() string {
			privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
			Expect(err).NotTo(HaveOccurred())

			privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
			Expect(err).NotTo(HaveOccurred())

			// Corrupt the key data
			privateKeyBytes[len(privateKeyBytes)-10] = 0xFF
			privateKeyBytes[len(privateKeyBytes)-5] = 0xFF

			privateKeyPEM := pem.EncodeToMemory(&pem.Block{
				Type:  "PRIVATE KEY",
				Bytes: privateKeyBytes,
			})

			return string(privateKeyPEM)
		}

		Context("Valid private keys", func() {
			It("should accept valid 2048-bit RSA private key", func() {
				validKey := generateValidPrivateKey(2048)
				err := credvalidate.ValidatePrivateKey(validator, "private_key", validKey, nil, false)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should accept valid 4096-bit RSA private key", func() {
				validKey := generateValidPrivateKey(4096)
				err := credvalidate.ValidatePrivateKey(validator, "private_key", validKey, nil, false)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should accept valid 1024-bit RSA private key", func() {
				validKey := generateValidPrivateKey(1024)
				err := credvalidate.ValidatePrivateKey(validator, "private_key", validKey, nil, false)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("Invalid input types", func() {
			It("should fail with non-string input", func() {
				err := credvalidate.ValidatePrivateKey(validator, "private_key", 123, nil, false)
				Expect(err).To(MatchError("validation error in field \"private_key\": field value must be a string"))
			})

			It("should fail with nil input", func() {
				err := credvalidate.ValidatePrivateKey(validator, "private_key", nil, nil, false)
				Expect(err).To(MatchError("validation error in field \"private_key\": field value must be a string"))
			})

			It("should fail with boolean input", func() {
				err := credvalidate.ValidatePrivateKey(validator, "private_key", true, nil, false)
				Expect(err).To(MatchError("validation error in field \"private_key\": field value must be a string"))
			})

			It("should fail with array input", func() {
				err := credvalidate.ValidatePrivateKey(validator, "private_key", []string{"key"}, nil, false)
				Expect(err).To(MatchError("validation error in field \"private_key\": field value must be a string"))
			})
		})

		Context("Invalid PEM format", func() {
			It("should fail with empty string", func() {
				err := credvalidate.ValidatePrivateKey(validator, "private_key", "", nil, false)
				Expect(err).To(MatchError("validation error in field \"private_key\": field value must start with a PEM BEGIN line"))
			})

			It("should fail with string not starting with PEM BEGIN", func() {
				err := credvalidate.ValidatePrivateKey(validator, "private_key", "not a pem key", nil, false)
				Expect(err).To(MatchError("validation error in field \"private_key\": field value must start with a PEM BEGIN line"))
			})

			It("should fail with malformed PEM (missing END)", func() {
				invalidPEM := "-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQC"
				err := credvalidate.ValidatePrivateKey(validator, "private_key", invalidPEM, nil, false)
				Expect(err).To(MatchError("validation error in field \"private_key\": field value must be a valid PEM-encoded private key"))
			})

			It("should fail with completely malformed PEM", func() {
				invalidPEM := "-----BEGIN PRIVATE KEY-----\ninvalid base64 content!@#$%\n-----END PRIVATE KEY-----"
				err := credvalidate.ValidatePrivateKey(validator, "private_key", invalidPEM, nil, false)
				Expect(err).To(MatchError("validation error in field \"private_key\": field value must be a valid PEM-encoded private key"))
			})

			It("should fail with data before PEM block", func() {
				validKey := generateValidPrivateKey(2048)
				invalidKey := "some junk data\n" + validKey
				err := credvalidate.ValidatePrivateKey(validator, "private_key", invalidKey, nil, false)
				Expect(err).To(MatchError("validation error in field \"private_key\": field value must start with a PEM BEGIN line"))
			})

			It("should fail with data after PEM block", func() {
				validKey := generateValidPrivateKey(2048)
				invalidKey := validKey + "\nsome trailing data"
				err := credvalidate.ValidatePrivateKey(validator, "private_key", invalidKey, nil, false)
				Expect(err).To(MatchError("validation error in field \"private_key\": field value must contain exactly one PEM block (unexpected data after END line)"))
			})

			It("should fail with multiple PEM blocks", func() {
				validKey1 := generateValidPrivateKey(2048)
				validKey2 := generateValidPrivateKey(2048)
				multipleKeys := validKey1 + validKey2
				err := credvalidate.ValidatePrivateKey(validator, "private_key", multipleKeys, nil, false)
				Expect(err).To(MatchError("validation error in field \"private_key\": field value must contain exactly one PEM block (unexpected data after END line)"))
			})

			It("should accept whitespace after PEM block", func() {
				validKey := generateValidPrivateKey(2048)
				keyWithWhitespace := validKey + "   \n  \t  "
				err := credvalidate.ValidatePrivateKey(validator, "private_key", keyWithWhitespace, nil, false)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("Wrong PEM type", func() {
			It("should fail with RSA PRIVATE KEY type", func() {
				// Generate a valid RSA key but encode it as "RSA PRIVATE KEY" instead of "PRIVATE KEY"
				privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
				Expect(err).NotTo(HaveOccurred())

				privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
				privateKeyPEM := pem.EncodeToMemory(&pem.Block{
					Type:  "RSA PRIVATE KEY", // Wrong type
					Bytes: privateKeyBytes,
				})

				err = credvalidate.ValidatePrivateKey(validator, "private_key", string(privateKeyPEM), nil, false)
				Expect(err).To(MatchError("validation error in field \"private_key\": field value must be a PKCS#8 PEM block (BEGIN PRIVATE KEY)"))
			})

			It("should fail with CERTIFICATE type", func() {
				privateKeyPEM := `-----BEGIN CERTIFICATE-----
MIIDXTCCAkWgAwIBAgIJAKoK/heBjcOuMA0GCSqGSIb3DQEBBQUAMEUxCzAJBgNV
BAYTAkFVMRMwEQYDVQQIDApTb21lLVN0YXRlMSEwHwYDVQQKDBhJbnRlcm5ldCBX
aWRnaXRzIFB0eSBMdGQwHhcNMTIwOTEyMjE1MjAyWhcNMTUwOTEyMjE1MjAyWjBF
MQswCQYDVQQGEwJBVTETMBEGA1UECAwKU29tZS1TdGF0ZTEhMB8GA1UECgwYSW50
ZXJuZXQgV2lkZ2l0cyBQdHkgTHRkMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIB
CgKCAQEAwUdHPiQnXwxmkEs2Sa3rJXiVWAiuqiclsZ6SUOb5nrhpXahNLZ/w/8+G
mcHmu6BeB6xTEHyUBTpXJVDwxQ5XYDdodGHepTYwEgfIBhFKdNRcNxioe7aqJ7uR
WxQOi7tZnHlRnmCiXgjuaOlaQip4RVSjFrRDNiHSiYwuL+pSRYHphJSMb4+v2Hfq
OHo96JEUJi7RnuJmwOWfp7yt/dAdNUZ5y9QHslHdniXxDFcEt8cM5bAiUK88sgjH
tVaP3HRFHn3OO+/bdHHjwCUcE60yvBWINXNh4ExVqjc0hl+as+mpdqhplfFpb4oy
IKflrg8EkVvvmfZqKMxnK9Z4l0DvxwIDAQABo1AwTjAdBgNVHQ4EFgQUU3m/Wqor
Ss9UgOHYm8Cd8rIDZsswHwYDVR0jBBgwFoAUU3m/WqorSs9UgOHYm8Cd8rIDZssw
DAYDVR0TBAUwAwEB/zANBgkqhkiG9w0BAQUFAAOCAQEAy8DbkqP8IUmr1LGXXx4o
lTaUahqRN9cmhJ9GvVfKr2kMZplserQDgo1WiAAhX5stfTw3mt/107ArxWeaeinx
6v4Zb7X2s2YS1TlTQrIBx181gsk3UWqXhTkjjqyxHfUTgBc038gVNxzI5oqsOmFg
H9G7Z4EHLdPE8osaFuFjODGHuDqDTuKwjyeq5EYfogRqO7xfZuAasTw5Erg2XfXQ
joGWFJgNgomJKMWqGnPqaBcZcQAKR6Gs1SxCb1MF4BKGX8iBGKepZnyFBa5nHpfm
6uEu4NuZiC5QrJ2XVWU2jQxVs8+dBtPp6OfOi9/XeDhTAniTh7LrXx/e7qUqIwuY
wg==
-----END CERTIFICATE-----`

				err := credvalidate.ValidatePrivateKey(validator, "private_key", privateKeyPEM, nil, false)
				Expect(err).To(MatchError("validation error in field \"private_key\": field value must be a PKCS#8 PEM block (BEGIN PRIVATE KEY)"))
			})

			It("should fail with PUBLIC KEY type", func() {
				privateKeyPEM := `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAwUdHPiQnXwxmkEs2Sa3r
JXiVWAiuqiclsZ6SUOb5nrhpXahNLZ/w/8+GmcHmu6BeB6xTEHyUBTpXJVDwxQ5X
YDdodGHepTYwEgfIBhFKdNRcNxioe7aqJ7uRWxQOi7tZnHlRnmCiXgjuaOlaQip4
RVSjFrRDNiHSiYwuL+pSRYHphJSMb4+v2HfqOHo96JEUJi7RnuJmwOWfp7yt/dAd
NUZ5y9QHslHdniXxDFcEt8cM5bAiUK88sgjHtVaP3HRFHn3OO+/bdHHjwCUcE60y
vBWINXNh4ExVqjc0hl+as+mpdqhplfFpb4oyIKflrg8EkVvvmfZqKMxnK9Z4l0Dv
xwIDAQAB
-----END PUBLIC KEY-----`

				err := credvalidate.ValidatePrivateKey(validator, "private_key", privateKeyPEM, nil, false)
				Expect(err).To(MatchError("validation error in field \"private_key\": field value must be a PKCS#8 PEM block (BEGIN PRIVATE KEY)"))
			})
		})

		Context("PEM with headers", func() {
			It("should fail with PEM headers", func() {
				privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
				Expect(err).NotTo(HaveOccurred())

				privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
				Expect(err).NotTo(HaveOccurred())

				// Create PEM with headers
				privateKeyPEM := pem.EncodeToMemory(&pem.Block{
					Type:  "PRIVATE KEY",
					Bytes: privateKeyBytes,
					Headers: map[string]string{
						"Proc-Type": "4,ENCRYPTED",
						"DEK-Info":  "AES-256-CBC,1234567890ABCDEF",
					},
				})

				err = credvalidate.ValidatePrivateKey(validator, "private_key", string(privateKeyPEM), nil, false)
				Expect(err).To(MatchError("validation error in field \"private_key\": field value must not include PEM headers"))
			})

			It("should fail with single PEM header", func() {
				privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
				Expect(err).NotTo(HaveOccurred())

				privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
				Expect(err).NotTo(HaveOccurred())

				// Create PEM with single header
				privateKeyPEM := pem.EncodeToMemory(&pem.Block{
					Type:  "PRIVATE KEY",
					Bytes: privateKeyBytes,
					Headers: map[string]string{
						"Comment": "Test key",
					},
				})

				err = credvalidate.ValidatePrivateKey(validator, "private_key", string(privateKeyPEM), nil, false)
				Expect(err).To(MatchError("validation error in field \"private_key\": field value must not include PEM headers"))
			})
		})

		Context("Invalid PKCS#8 data", func() {
			It("should fail with invalid PKCS#8 data", func() {
				invalidPKCS8Data := []byte("invalid pkcs8 data")
				privateKeyPEM := pem.EncodeToMemory(&pem.Block{
					Type:  "PRIVATE KEY",
					Bytes: invalidPKCS8Data,
				})

				err := credvalidate.ValidatePrivateKey(validator, "private_key", string(privateKeyPEM), nil, false)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("cannot be parsed")))
			})

			It("should fail with corrupted PKCS#8 data", func() {
				privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
				Expect(err).NotTo(HaveOccurred())

				privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
				Expect(err).NotTo(HaveOccurred())

				// Corrupt the first few bytes
				privateKeyBytes[0] = 0xFF
				privateKeyBytes[1] = 0xFF
				privateKeyBytes[2] = 0xFF

				privateKeyPEM := pem.EncodeToMemory(&pem.Block{
					Type:  "PRIVATE KEY",
					Bytes: privateKeyBytes,
				})

				err = credvalidate.ValidatePrivateKey(validator, "private_key", string(privateKeyPEM), nil, false)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("cannot be parsed")))
			})
		})

		Context("Non-RSA keys", func() {
			It("should fail with ECDSA private key", func() {
				// Generate ECDSA key
				ecdsaKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
				Expect(err).NotTo(HaveOccurred())

				// Marshal as PKCS#8
				privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(ecdsaKey)
				Expect(err).NotTo(HaveOccurred())

				privateKeyPEM := pem.EncodeToMemory(&pem.Block{
					Type:  "PRIVATE KEY",
					Bytes: privateKeyBytes,
				})

				err = credvalidate.ValidatePrivateKey(validator, "private_key", string(privateKeyPEM), nil, false)
				Expect(err).To(MatchError("validation error in field \"private_key\": field value must be an RSA private key"))
			})

			It("should fail with Ed25519 private key", func() {
				// Generate Ed25519 key
				_, ed25519Key, err := ed25519.GenerateKey(rand.Reader)
				Expect(err).NotTo(HaveOccurred())

				// Marshal as PKCS#8
				privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(ed25519Key)
				Expect(err).NotTo(HaveOccurred())

				privateKeyPEM := pem.EncodeToMemory(&pem.Block{
					Type:  "PRIVATE KEY",
					Bytes: privateKeyBytes,
				})

				err = credvalidate.ValidatePrivateKey(validator, "private_key", string(privateKeyPEM), nil, false)
				Expect(err).To(MatchError("validation error in field \"private_key\": field value must be an RSA private key"))
			})
		})

		Context("Invalid RSA keys", func() {
			It("should fail with invalid RSA key", func() {
				invalidKey := generateInvalidRSAKey()
				err := credvalidate.ValidatePrivateKey(validator, "private_key", invalidKey, nil, false)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("cannot be parsed")))
			})

			It("should fail with only whitespace", func() {
				err := credvalidate.ValidatePrivateKey(validator, "private_key", "   \n\t  ", nil, false)
				Expect(err).To(MatchError("validation error in field \"private_key\": field value must start with a PEM BEGIN line"))
			})

			It("should fail with partial PEM BEGIN line", func() {
				err := credvalidate.ValidatePrivateKey(validator, "private_key", "-----BEGIN", nil, false)
				Expect(err).To(MatchError("validation error in field \"private_key\": field value must start with a PEM BEGIN line"))
			})

			It("should fail with custom field name in error message", func() {
				err := credvalidate.ValidatePrivateKey(validator, "custom_field", "invalid", nil, false)
				Expect(err).To(MatchError("validation error in field \"custom_field\": field value must start with a PEM BEGIN line"))
			})
		})
	})
})
