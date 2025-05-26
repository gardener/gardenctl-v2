/*
SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package providerenv_test

import (
	"encoding/json"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardenctl-v2/pkg/cmd/providerenv"
)

var _ = Describe("parsing gcp credentials", func() {
	var (
		validMinimalJSON = `{
            "type": "service_account"
        }`

		completeValidJSON = `{
            "type": "service_account",
            "project_id": "test-project",
            "private_key_id": "key-id",
            "private_key": "private-key",
            "client_email": "test@example.org",
            "client_id": "123456789",
            "auth_uri": "https://accounts.google.com/o/oauth2/auth",
            "token_uri": "https://oauth2.googleapis.com/token",
            "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
            "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/test%40example.org",
            "universe_domain": "googleapis.com"
        }`

		secretName  = "gcp"
		secret      *corev1.Secret
		credentials map[string]interface{}
	)

	BeforeEach(func() {
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: secretName,
			},
			Data: map[string][]byte{
				"serviceaccount.json": []byte(validMinimalJSON),
			},
		}
		credentials = make(map[string]interface{})
	})

	It("should succeed for minimal valid JSON", func() {
		allowedPatterns := providerenv.DefaultAllowedPatterns()
		data, err := providerenv.ValidateAndParseGCPServiceAccount(secret, &credentials, allowedPatterns)
		Expect(err).NotTo(HaveOccurred())
		Expect(credentials).To(HaveKeyWithValue("type", "service_account"))

		// Verify the returned JSON is correct
		var parsedData map[string]interface{}
		err = json.Unmarshal(data, &parsedData)
		Expect(err).NotTo(HaveOccurred())
		Expect(parsedData).To(HaveKeyWithValue("type", "service_account"))
	})

	It("should succeed for complete valid JSON", func() {
		secret.Data["serviceaccount.json"] = []byte(completeValidJSON)
		allowedPatterns := providerenv.DefaultAllowedPatterns()
		data, err := providerenv.ValidateAndParseGCPServiceAccount(secret, &credentials, allowedPatterns)
		Expect(err).NotTo(HaveOccurred())

		// Check all expected fields are present
		Expect(credentials).To(HaveKeyWithValue("type", "service_account"))
		Expect(credentials).To(HaveKeyWithValue("project_id", "test-project"))
		Expect(credentials).To(HaveKeyWithValue("private_key_id", "key-id"))
		Expect(credentials).To(HaveKeyWithValue("private_key", "private-key"))
		Expect(credentials).To(HaveKeyWithValue("client_email", "test@example.org"))
		Expect(credentials).To(HaveKeyWithValue("client_id", "123456789"))
		Expect(credentials).To(HaveKeyWithValue("auth_uri", "https://accounts.google.com/o/oauth2/auth"))
		Expect(credentials).To(HaveKeyWithValue("token_uri", "https://oauth2.googleapis.com/token"))
		Expect(credentials).To(HaveKeyWithValue("auth_provider_x509_cert_url", "https://www.googleapis.com/oauth2/v1/certs"))
		Expect(credentials).To(HaveKeyWithValue("client_x509_cert_url", "https://www.googleapis.com/robot/v1/metadata/x509/test%40example.org"))
		Expect(credentials).To(HaveKeyWithValue("universe_domain", "googleapis.com"))

		// Verify JSON marshalling worked
		Expect(data).NotTo(BeEmpty())
	})

	It("should fail with missing secret data", func() {
		secret.Data["serviceaccount.json"] = nil
		allowedPatterns := providerenv.DefaultAllowedPatterns()
		_, err := providerenv.ValidateAndParseGCPServiceAccount(secret, &credentials, allowedPatterns)
		Expect(err).To(MatchError(fmt.Sprintf("no \"serviceaccount.json\" data in Secret %q", secretName)))
	})

	It("should fail with invalid json", func() {
		secret.Data["serviceaccount.json"] = []byte("{")
		allowedPatterns := providerenv.DefaultAllowedPatterns()
		_, err := providerenv.ValidateAndParseGCPServiceAccount(secret, &credentials, allowedPatterns)
		Expect(err).To(MatchError("failed to unmarshal service account JSON: unexpected end of JSON input"))
	})

	It("should fail with non-string values", func() {
		secret.Data["serviceaccount.json"] = []byte(`{
            "type": "service_account",
            "project_id": 123,
            "client_email": "test@example.org"
        }`)
		allowedPatterns := providerenv.DefaultAllowedPatterns()
		_, err := providerenv.ValidateAndParseGCPServiceAccount(secret, &credentials, allowedPatterns)
		Expect(err).To(MatchError("field project_id is not a string"))
	})

	It("should fail with missing type field", func() {
		secret.Data["serviceaccount.json"] = []byte(`{
            "project_id": "test",
            "client_email": "test@example.org"
        }`)
		allowedPatterns := providerenv.DefaultAllowedPatterns()
		_, err := providerenv.ValidateAndParseGCPServiceAccount(secret, &credentials, allowedPatterns)
		Expect(err).To(MatchError("type field is missing"))
	})

	It("should fail with incorrect type value", func() {
		secret.Data["serviceaccount.json"] = []byte(`{
            "type": "wrong_type",
            "project_id": "test",
            "client_email": "test@example.org"
        }`)
		allowedPatterns := providerenv.DefaultAllowedPatterns()
		_, err := providerenv.ValidateAndParseGCPServiceAccount(secret, &credentials, allowedPatterns)
		Expect(err).To(MatchError("type must be 'service_account'"))
	})

	It("should fail with disallowed fields", func() {
		secret.Data["serviceaccount.json"] = []byte(`{
            "type": "service_account",
            "project_id": "test",
            "client_email": "test@example.org",
            "unknown_field": "value"
        }`)
		allowedPatterns := providerenv.DefaultAllowedPatterns()
		_, err := providerenv.ValidateAndParseGCPServiceAccount(secret, &credentials, allowedPatterns)
		Expect(err).To(MatchError("disallowed field found: unknown_field"))
	})

	It("should fail with invalid URI scheme", func() {
		secret.Data["serviceaccount.json"] = []byte(`{
            "type": "service_account",
            "project_id": "test",
            "client_email": "test@example.org",
            "auth_uri": "http://accounts.google.com/o/oauth2/auth"
        }`)
		allowedPatterns := providerenv.DefaultAllowedPatterns()
		_, err := providerenv.ValidateAndParseGCPServiceAccount(secret, &credentials, allowedPatterns)
		Expect(err).To(MatchError("URI in auth_uri must use https scheme"))
	})

	It("should fail with disallowed URI", func() {
		secret.Data["serviceaccount.json"] = []byte(`{
            "type": "service_account",
            "project_id": "test",
            "client_email": "test@example.org",
            "auth_uri": "https://malicious-domain.com/o/oauth2/auth"
        }`)
		allowedPatterns := providerenv.DefaultAllowedPatterns()
		_, err := providerenv.ValidateAndParseGCPServiceAccount(secret, &credentials, allowedPatterns)
		Expect(err).To(MatchError("URI for auth_uri (https://malicious-domain.com/o/oauth2/auth) does not match any allowed patterns"))
	})

	It("should fail with disallowed universe domain", func() {
		secret.Data["serviceaccount.json"] = []byte(`{
            "type": "service_account",
            "project_id": "test",
            "client_email": "test@example.org",
            "universe_domain": "malicious-domain.com"
        }`)
		allowedPatterns := providerenv.DefaultAllowedPatterns()
		_, err := providerenv.ValidateAndParseGCPServiceAccount(secret, &credentials, allowedPatterns)
		Expect(err).To(MatchError("untrusted universe_domain: malicious-domain.com"))
	})

	It("should succeed with additional allowed patterns", func() {
		secret.Data["serviceaccount.json"] = []byte(`{
            "type": "service_account",
            "project_id": "test",
            "client_email": "test@example.org",
            "auth_uri": "https://custom-domain.com/auth",
            "universe_domain": "custom-domain.com"
        }`)
		allowedPatterns := providerenv.DefaultAllowedPatterns()
		allowedPatterns["auth_uri"] = append(allowedPatterns["auth_uri"], "https://custom-domain.com/auth")
		allowedPatterns["universe_domain"] = append(allowedPatterns["universe_domain"], "custom-domain.com")
		_, err := providerenv.ValidateAndParseGCPServiceAccount(secret, &credentials, allowedPatterns)
		Expect(err).NotTo(HaveOccurred())
		Expect(credentials).To(HaveKeyWithValue("auth_uri", "https://custom-domain.com/auth"))
		Expect(credentials).To(HaveKeyWithValue("universe_domain", "custom-domain.com"))
	})

	It("should test all URI fields", func() {
		secret.Data["serviceaccount.json"] = []byte(`{
            "type": "service_account",
            "project_id": "test",
            "client_email": "test@example.org",
            "auth_uri": "https://accounts.google.com/o/oauth2/auth",
            "token_uri": "https://oauth2.googleapis.com/token",
            "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
            "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/test%40example.org"
        }`)
		allowedPatterns := providerenv.DefaultAllowedPatterns()
		_, err := providerenv.ValidateAndParseGCPServiceAccount(secret, &credentials, allowedPatterns)
		Expect(err).NotTo(HaveOccurred())

		// Verify all URI fields are properly parsed
		Expect(credentials).To(HaveKeyWithValue("auth_uri", "https://accounts.google.com/o/oauth2/auth"))
		Expect(credentials).To(HaveKeyWithValue("token_uri", "https://oauth2.googleapis.com/token"))
		Expect(credentials).To(HaveKeyWithValue("auth_provider_x509_cert_url", "https://www.googleapis.com/oauth2/v1/certs"))
		Expect(credentials).To(HaveKeyWithValue("client_x509_cert_url", "https://www.googleapis.com/robot/v1/metadata/x509/test%40example.org"))
	})

	It("should succeed with correct client_x509_cert_url", func() {
		secret.Data["serviceaccount.json"] = []byte(`{
            "type": "service_account",
            "project_id": "test",
            "client_email": "test@example.org",
            "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/test%40example.org"
        }`)
		allowedPatterns := providerenv.DefaultAllowedPatterns()
		_, err := providerenv.ValidateAndParseGCPServiceAccount(secret, &credentials, allowedPatterns)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should fail with incorrect client_x509_cert_url", func() {
		secret.Data["serviceaccount.json"] = []byte(`{
            "type": "service_account",
            "project_id": "test",
            "client_email": "test@example.org",
            "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/wrong%40email.com"
        }`)
		allowedPatterns := providerenv.DefaultAllowedPatterns()
		_, err := providerenv.ValidateAndParseGCPServiceAccount(secret, &credentials, allowedPatterns)
		Expect(err).To(MatchError("URI for client_x509_cert_url (https://www.googleapis.com/robot/v1/metadata/x509/wrong%40email.com) does not match any allowed patterns"))
	})

	It("should fail with different host in client_x509_cert_url", func() {
		secret.Data["serviceaccount.json"] = []byte(`{
            "type": "service_account",
            "project_id": "test",
            "client_email": "test@example.org",
            "client_x509_cert_url": "https://malicious.com/robot/v1/metadata/x509/test%40example.org"
        }`)
		allowedPatterns := providerenv.DefaultAllowedPatterns()
		_, err := providerenv.ValidateAndParseGCPServiceAccount(secret, &credentials, allowedPatterns)
		Expect(err).To(MatchError("URI for client_x509_cert_url (https://malicious.com/robot/v1/metadata/x509/test%40example.org) does not match any allowed patterns"))
	})

	It("should fail if client_email is missing for client_x509_cert_url", func() {
		secret.Data["serviceaccount.json"] = []byte(`{
            "type": "service_account",
            "project_id": "test",
            "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/test%40example.org"
        }`)
		allowedPatterns := providerenv.DefaultAllowedPatterns()
		_, err := providerenv.ValidateAndParseGCPServiceAccount(secret, &credentials, allowedPatterns)
		Expect(err).To(MatchError("client_email is missing or not a string for client_x509_cert_url with pattern requiring {encoded_client_email}"))
	})

	// Additional test for multiple allowed patterns
	It("should succeed with alternative token_uri", func() {
		secret.Data["serviceaccount.json"] = []byte(`{
            "type": "service_account",
            "project_id": "test",
            "client_email": "test@example.org",
            "token_uri": "https://accounts.google.com/o/oauth2/token"
        }`)
		allowedPatterns := providerenv.DefaultAllowedPatterns()
		_, err := providerenv.ValidateAndParseGCPServiceAccount(secret, &credentials, allowedPatterns)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should fail with invalid token_uri", func() {
		secret.Data["serviceaccount.json"] = []byte(`{
            "type": "service_account",
            "project_id": "test",
            "client_email": "test@example.org",
            "token_uri": "https://invalid.com/token"
        }`)
		allowedPatterns := providerenv.DefaultAllowedPatterns()
		_, err := providerenv.ValidateAndParseGCPServiceAccount(secret, &credentials, allowedPatterns)
		Expect(err).To(MatchError("URI for token_uri (https://invalid.com/token) does not match any allowed patterns"))
	})

	It("should fail with placeholder in hostname", func() {
		secret.Data["serviceaccount.json"] = []byte(`{
            "type": "service_account",
            "project_id": "test",
            "client_email": "malicious.com",
            "client_x509_cert_url": "https://malicious.com/foo"
        }`)
		allowedPatterns := providerenv.DefaultAllowedPatterns()
		// Override the default pattern with a potentially malicious one
		allowedPatterns["client_x509_cert_url"] = []string{"https://{encoded_client_email}/foo"} // not
		_, err := providerenv.ValidateAndParseGCPServiceAccount(secret, &credentials, allowedPatterns)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("invalid character \"{\" in host name"))
	})
})
