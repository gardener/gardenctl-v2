/*
SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package credvalidate_test

import (
	"context"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardenctl-v2/pkg/provider/common/allowpattern"
	credvalidate "github.com/gardener/gardenctl-v2/pkg/provider/credvalidate"
)

var _ = Describe("OpenStack Validator", func() {
	var validator *credvalidate.OpenStackValidator

	BeforeEach(func() {
		// Configure allowed patterns for authURL validation
		allowedPatterns := []allowpattern.Pattern{
			{Field: "authURL", URI: "https://keystone.example.com:5000/v3"},
			{Field: "authURL", URI: "https://keystone.example.com/identity/v3"},
			{Field: "authURL", URI: "https://keystone.example.com:35357/v3"},
			{Field: "authURL", URI: "https://keystone.example.com"},
			{Field: "authURL", URI: "https://192.168.1.100:5000/v3"},
			{Field: "authURL", URI: "https://[2001:db8::1]:5000/v3"},
			{Field: "authURL", URI: "http://allowed-http-scheme.example.com"},
		}

		validator = credvalidate.NewOpenStackValidator(context.Background(), allowedPatterns)
	})
	Describe("Secret Validation", func() {
		var secret *corev1.Secret

		BeforeEach(func() {
			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "openstack-secret",
					Namespace: "test-namespace",
				},
				Data: map[string][]byte{
					"domainName": []byte("default"),
					"tenantName": []byte("my-project"),
					"username":   []byte("myuser"),
					"password":   []byte("mypassword"),
				},
			}
		})

		Context("Valid credentials", func() {
			It("should succeed with basic auth credentials", func() {
				creds, err := validator.ValidateSecret(secret)
				Expect(err).NotTo(HaveOccurred())
				Expect(creds).To(HaveKeyWithValue("domainName", "default"))
				Expect(creds).To(HaveKeyWithValue("tenantName", "my-project"))
				Expect(creds).To(HaveKeyWithValue("username", "myuser"))
				Expect(creds).To(HaveKeyWithValue("password", "mypassword"))
				Expect(len(creds)).To(Equal(4))
			})

			It("should ignore extra top-level secret keys (Permissive mode)", func() {
				// Add an unrelated extra key at the top level alongside valid OpenStack keys
				secret.Data["foo"] = []byte("bar")
				creds, err := validator.ValidateSecret(secret)
				Expect(err).NotTo(HaveOccurred())
				Expect(creds).To(HaveKey("domainName"))
				Expect(creds).To(HaveKey("tenantName"))
				Expect(creds).To(HaveKey("username"))
				Expect(creds).To(HaveKey("password"))
				Expect(creds).NotTo(HaveKey("foo"))
			})

			It("should succeed with application credentials using ID", func() {
				secret.Data = map[string][]byte{
					"domainName":                  []byte("default"),
					"tenantName":                  []byte("my-project"),
					"applicationCredentialID":     []byte("app-cred-id"),
					"applicationCredentialSecret": []byte("app-cred-secret"),
				}

				creds, err := validator.ValidateSecret(secret)
				Expect(err).NotTo(HaveOccurred())
				Expect(creds).To(HaveKeyWithValue("domainName", "default"))
				Expect(creds).To(HaveKeyWithValue("applicationCredentialID", "app-cred-id"))
				Expect(creds).To(HaveKeyWithValue("applicationCredentialSecret", "app-cred-secret"))
				Expect(creds).NotTo(HaveKey("username"))
				Expect(creds).NotTo(HaveKey("password"))
				Expect(creds).NotTo(HaveKey("tenantName"))
			})

			It("should succeed with both application credential ID and name", func() {
				secret.Data = map[string][]byte{
					"domainName":                  []byte("default"),
					"tenantName":                  []byte("my-project"),
					"applicationCredentialID":     []byte("app-cred-id"),
					"applicationCredentialName":   []byte("app-cred-name"),
					"applicationCredentialSecret": []byte("app-cred-secret"),
				}

				creds, err := validator.ValidateSecret(secret)
				Expect(err).NotTo(HaveOccurred())
				Expect(creds).To(HaveKeyWithValue("domainName", "default"))
				Expect(creds).To(HaveKeyWithValue("applicationCredentialID", "app-cred-id"))
				Expect(creds).To(HaveKeyWithValue("applicationCredentialName", "app-cred-name"))
				Expect(creds).To(HaveKeyWithValue("applicationCredentialSecret", "app-cred-secret"))
				Expect(creds).NotTo(HaveKey("username"))
				Expect(creds).NotTo(HaveKey("password"))
				Expect(creds).NotTo(HaveKey("tenantName"))
			})

			It("should succeed with application credentials using name", func() {
				secret.Data = map[string][]byte{
					"domainName":                  []byte("default"),
					"tenantName":                  []byte("my-project"),
					"username":                    []byte("myuser"),
					"applicationCredentialName":   []byte("app-cred-name"),
					"applicationCredentialSecret": []byte("app-cred-secret"),
				}

				creds, err := validator.ValidateSecret(secret)
				Expect(err).NotTo(HaveOccurred())
				Expect(creds).To(HaveKeyWithValue("domainName", "default"))
				Expect(creds).To(HaveKeyWithValue("applicationCredentialName", "app-cred-name"))
				Expect(creds).To(HaveKeyWithValue("applicationCredentialSecret", "app-cred-secret"))
				Expect(creds).NotTo(HaveKey("password"))
				Expect(creds).NotTo(HaveKey("username"))
				Expect(creds).NotTo(HaveKey("tenantName"))
			})

			DescribeTable("should succeed with various valid tenant names",
				func(tenantName string) {
					secret.Data["tenantName"] = []byte(tenantName)
					creds, err := validator.ValidateSecret(secret)
					Expect(err).NotTo(HaveOccurred(), "should accept valid tenant name: %s", tenantName)
					Expect(creds).To(HaveKeyWithValue("tenantName", tenantName))
				},
				Entry("minimum length", "a"),
				Entry("with dash", "my-project"),
				Entry("with underscore", "my_project"),
				Entry("mixed case with numbers", "MyProject123"),
				Entry("maximum length", strings.Repeat("a", 64)),
				Entry("with dots", "project.with.dots"),
				Entry("with spaces", "project with spaces"),
			)
		})

		Context("Missing fields", func() {
			DescribeTable("should fail when required fields are missing",
				func(modifySecret func(*corev1.Secret), expectedError string) {
					modifySecret(secret)
					_, err := validator.ValidateSecret(secret)
					Expect(err).To(MatchError(expectedError))
				},
				Entry("missing domainName",
					func(s *corev1.Secret) { delete(s.Data, "domainName") },
					"validation error in field \"domainName\": required field is missing",
				),
				Entry("missing tenantName",
					func(s *corev1.Secret) { delete(s.Data, "tenantName") },
					"validation error in field \"tenantName\": required field is missing",
				),
				Entry("missing username",
					func(s *corev1.Secret) { delete(s.Data, "username") },
					"validation error in field \"username\": required field is missing",
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
				Entry("empty domainName",
					func(s *corev1.Secret) { s.Data["domainName"] = []byte("") },
					"validation error in field \"domainName\": required field cannot be empty",
				),
				Entry("empty tenantName",
					func(s *corev1.Secret) { s.Data["tenantName"] = []byte("") },
					"validation error in field \"tenantName\": required field cannot be empty",
				),
				Entry("empty username",
					func(s *corev1.Secret) { s.Data["username"] = []byte("") },
					"validation error in field \"username\": required field cannot be empty",
				),
			)

			It("should fail when secret.Data is nil", func() {
				secret.Data = nil
				_, err := validator.ValidateSecret(secret)
				Expect(err).To(MatchError("must either specify 'password' or 'applicationCredentialSecret'"))
			})
		})

		Context("Authentication method validation", func() {
			It("should fail when both password and application credential secret are provided", func() {
				secret.Data["applicationCredentialSecret"] = []byte("app-secret")
				_, err := validator.ValidateSecret(secret)
				Expect(err).To(MatchError("cannot specify both 'password' and 'applicationCredentialSecret'"))
			})

			It("should fail when password is provided without username", func() {
				delete(secret.Data, "username")
				_, err := validator.ValidateSecret(secret)
				Expect(err).To(MatchError("validation error in field \"username\": required field is missing"))
			})

			It("should fail when neither password nor application credential secret is provided", func() {
				delete(secret.Data, "password")
				_, err := validator.ValidateSecret(secret)
				Expect(err).To(MatchError("must either specify 'password' or 'applicationCredentialSecret'"))
			})

			It("should fail when application credentials are incomplete (missing ID and name)", func() {
				secret.Data = map[string][]byte{
					"domainName":                  []byte("default"),
					"applicationCredentialSecret": []byte("app-secret"),
				}
				_, err := validator.ValidateSecret(secret)
				Expect(err).To(MatchError("either 'applicationCredentialID' or 'applicationCredentialName' must be provided"))
			})
		})

		Context("Field-specific validation", func() {
			DescribeTable("should fail when fields are too long",
				func(fieldName string, length int, expectedError string) {
					secret.Data[fieldName] = []byte(strings.Repeat("a", length))
					_, err := validator.ValidateSecret(secret)
					Expect(err).To(MatchError(ContainSubstring(expectedError)))
				},
				Entry("tenantName",
					"tenantName", 65,
					"validation error in field \"tenantName\": field value must be at most 64 characters, got 65",
				),
				Entry("domainName",
					"domainName", 65,
					"validation error in field \"domainName\": field value must be at most 64 characters, got 65",
				),
				Entry("username",
					"username", 256,
					"validation error in field \"username\": field value must be at most 255 characters, got 256",
				),
				Entry("password",
					"password", 4097,
					"validation error in field \"password\": field value must be at most 4096 characters, got 4097",
				),
			)

			It("should allow password with spaces", func() {
				secret.Data["password"] = []byte(" my password with spaces ")
				creds, err := validator.ValidateSecret(secret)
				Expect(err).NotTo(HaveOccurred())
				Expect(creds).To(HaveKeyWithValue("password", " my password with spaces "))
			})
		})

		Context("Non-printable character validation", func() {
			DescribeTable("should fail when fields contain non-printable characters",
				func(fieldName string, fieldValue string, expectedErrorSubstring string) {
					secret.Data[fieldName] = []byte(fieldValue)
					_, err := validator.ValidateSecret(secret)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(ContainSubstring(expectedErrorSubstring)))
				},
				Entry("domainName with null byte",
					"domainName", "default\x00domain", "field value contains non-printable character"),
				Entry("tenantName with null byte",
					"tenantName", "project\x00name", "field value contains non-printable character"),
				Entry("username with null byte",
					"username", "user\x00name", "field value contains non-printable character"),
				Entry("password with null byte",
					"password", "pass\x00word", "field value contains non-printable character"),
			)
		})

		Context("Application credential field validation", func() {
			BeforeEach(func() {
				secret.Data = map[string][]byte{
					"domainName":                  []byte("default"),
					"applicationCredentialID":     []byte("app-cred-id"),
					"applicationCredentialSecret": []byte("app-cred-secret"),
				}
			})

			DescribeTable("should fail when application credential fields are too long",
				func(fieldName string, length int, expectedError string) {
					secret.Data[fieldName] = []byte(strings.Repeat("a", length))
					_, err := validator.ValidateSecret(secret)
					Expect(err).To(MatchError(ContainSubstring(expectedError)))
				},
				Entry("applicationCredentialID",
					"applicationCredentialID", 256,
					"validation error in field \"applicationCredentialID\": field value must be at most 255 characters, got 256",
				),
				Entry("applicationCredentialName",
					"applicationCredentialName", 256,
					"validation error in field \"applicationCredentialName\": field value must be at most 255 characters, got 256",
				),
				Entry("applicationCredentialSecret",
					"applicationCredentialSecret", 4097,
					"validation error in field \"applicationCredentialSecret\": field value must be at most 4096 characters, got 4097",
				),
			)

			DescribeTable("should fail when application credential fields contain non-printable characters",
				func(fieldName string, fieldValue string) {
					secret.Data[fieldName] = []byte(fieldValue)
					_, err := validator.ValidateSecret(secret)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(ContainSubstring("field value contains non-printable character")))
				},
				Entry("applicationCredentialID with null byte",
					"applicationCredentialID", "id\x00value"),
				Entry("applicationCredentialName with null byte",
					"applicationCredentialName", "name\x00value"),
				Entry("applicationCredentialSecret with null byte",
					"applicationCredentialSecret", "secret\x00value"),
			)
		})
	})

	Describe("AuthURL Validation", func() {
		Context("Valid URLs", func() {
			DescribeTable("should succeed with valid URLs",
				func(authURL string) {
					err := validator.ValidateAuthURL(authURL)
					Expect(err).NotTo(HaveOccurred())
				},
				Entry("HTTPS URL", "https://keystone.example.com:5000/v3"),
				Entry("URL with path", "https://keystone.example.com/identity/v3"),
				Entry("URL with port", "https://keystone.example.com:35357/v3"),
				Entry("URL without path", "https://keystone.example.com"),
				Entry("URL with IP address", "https://192.168.1.100:5000/v3"),
				Entry("URL with IPv6", "https://[2001:db8::1]:5000/v3"),
				Entry("URL with allowed HTTP scheme", "http://allowed-http-scheme.example.com"),
			)
		})

		Context("Invalid URLs", func() {
			DescribeTable("should fail with invalid URLs",
				func(authURL, expectedErrorSubstring string) {
					err := validator.ValidateAuthURL(authURL)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(ContainSubstring(expectedErrorSubstring)))
				},
				Entry("empty URL", "", "required field cannot be empty"),
				Entry("malformed URL", "ht!tp://invalid", "validation error in field \"authURL\": failed to validate URI: invalid URI"),
				Entry("URL with spaces", "https://keystone example.com/v3", "\"authURL\": failed to validate URI: invalid URI"),
				Entry("URL with invalid characters", "https://keystone.example.com:abc/v3", "\"authURL\": failed to validate URI: invalid URI"),
				Entry("incomplete URL", "://keystone.example.com/v3", "\"authURL\": failed to validate URI: invalid URI"),
				Entry("no pattern match", "http://keystone.example.com:5000/v3", "\"authURL\": does not match any allowed patterns (actual: \"http://keystone.example.com:5000/v3\""),
				Entry("forbidden scheme", "ftp://keystone.example.com", "\"authURL\": failed to validate URI: scheme must be one of {https, http}, got \"ftp\""),
				Entry("not allowed scheme (strict)", "http://keystone.example.com", "pattern mismatch in field \"authURL\": does not match any allowed patterns (actual: \"http://keystone.example.com\""),
			)
		})
	})

	Describe("Default Allowed Patterns", func() {
		It("should return empty by default and validate pattern parsing context", func() {
			patterns := credvalidate.DefaultOpenStackAllowedPatterns()
			Expect(patterns).To(BeEmpty())
		})
	})
})
