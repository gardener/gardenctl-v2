/*
SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package credvalidate_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardenctl-v2/pkg/provider/credvalidate"
)

var _ = Describe("Azure Validator", func() {
	var validator *credvalidate.AzureValidator

	BeforeEach(func() {
		validator = credvalidate.NewAzureValidator(context.Background())
	})

	Describe("Secret Validation", func() {
		var secret *corev1.Secret

		BeforeEach(func() {
			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "azure-secret",
					Namespace: "test-namespace",
				},
				Data: map[string][]byte{
					"subscriptionID": []byte("12345678-1234-1234-1234-123456789012"),
					"tenantID":       []byte("87654321-4321-4321-4321-210987654321"),
					"clientID":       []byte("abcdef12-3456-7890-abcd-ef1234567890"),
					"clientSecret":   []byte("AbCdE~fGhI.-jKlMnOpQrStUvWxYz0_123456789"),
				},
			}
		})

		Context("Valid credentials", func() {
			It("should succeed with valid service principal credentials", func() {
				creds, err := validator.ValidateSecret(secret)
				Expect(err).NotTo(HaveOccurred())
				Expect(creds).To(HaveKeyWithValue("subscriptionID", "12345678-1234-1234-1234-123456789012"))
				Expect(creds).To(HaveKeyWithValue("tenantID", "87654321-4321-4321-4321-210987654321"))
				Expect(creds).To(HaveKeyWithValue("clientID", "abcdef12-3456-7890-abcd-ef1234567890"))
				Expect(creds).To(HaveKeyWithValue("clientSecret", "AbCdE~fGhI.-jKlMnOpQrStUvWxYz0_123456789"))
				Expect(len(creds)).To(Equal(4))
			})

			It("should ignore extra top-level secret keys (Permissive mode)", func() {
				// Add an unrelated extra key at the top level alongside valid Azure keys
				secret.Data["foo"] = []byte("bar")
				creds, err := validator.ValidateSecret(secret)
				Expect(err).NotTo(HaveOccurred())
				Expect(creds).To(HaveKey("subscriptionID"))
				Expect(creds).To(HaveKey("tenantID"))
				Expect(creds).To(HaveKey("clientID"))
				Expect(creds).To(HaveKey("clientSecret"))
				Expect(creds).NotTo(HaveKey("foo"))
			})

			DescribeTable("should succeed with different valid GUIDs",
				func(guid string) {
					secret.Data["subscriptionID"] = []byte(guid)
					secret.Data["tenantID"] = []byte(guid)
					secret.Data["clientID"] = []byte(guid)
					creds, err := validator.ValidateSecret(secret)
					Expect(err).NotTo(HaveOccurred())
					Expect(creds["subscriptionID"]).To(Equal(guid))
				},
				Entry("standard GUID", "12345678-1234-1234-1234-123456789012"),
				Entry("uppercase GUID", "ABCDEF12-3456-7890-ABCD-EF1234567890"),
				Entry("all zeros", "00000000-0000-0000-0000-000000000000"),
				Entry("all f's", "ffffffff-ffff-ffff-ffff-ffffffffffff"),
			)

			DescribeTable("should succeed with different valid client secrets",
				func(secretValue string) {
					secret.Data["clientSecret"] = []byte(secretValue)
					creds, err := validator.ValidateSecret(secret)
					Expect(err).NotTo(HaveOccurred())
					Expect(creds["clientSecret"]).To(Equal(secretValue))
				},
				Entry("Azure doc example", "Bb2Cc~3Dd4.-Ee5Ff6Gg7Hh8Ii9Jj0_Kk1Ll2Mm3"),
				Entry("mixed case", "AbCdEfGhIjKlMnOpQrStUvWxYz0123456789abcd"),
				Entry("special chars", "AbcdEFghIJklMNopQRstUVwxYZ01234:?._~/-+=@[]"),
				Entry("only punctuation ASCII !..~", "!@#$%^&*()!@#$%^&*()!@#$%^&*()!@#$%^&*()"),
				Entry("min length (34 chars)", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
				Entry("max length (44 chars)", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
			)
		})

		Context("Missing fields", func() {
			DescribeTable("should fail when fields are missing or empty",
				func(field string, isMissing bool) {
					if isMissing {
						delete(secret.Data, field)
					} else {
						secret.Data[field] = []byte("")
					}
					_, err := validator.ValidateSecret(secret)
					if isMissing {
						Expect(err).To(MatchError(fmt.Sprintf("validation error in field %q: required field is missing", field)))
					} else {
						// Empty fields should report as "cannot be empty"
						Expect(err).To(MatchError(fmt.Sprintf("validation error in field %q: required field cannot be empty", field)))
					}
				},
				Entry("missing subscriptionID", "subscriptionID", true),
				Entry("empty subscriptionID", "subscriptionID", false),
				Entry("missing tenantID", "tenantID", true),
				Entry("empty tenantID", "tenantID", false),
				Entry("missing clientID", "clientID", true),
				Entry("empty clientID", "clientID", false),
				Entry("missing clientSecret", "clientSecret", true),
				Entry("empty clientSecret", "clientSecret", false),
			)
		})

		Context("Nil data fields", func() {
			DescribeTable("should fail when fields contain nil data",
				func(field string) {
					secret.Data[field] = nil
					_, err := validator.ValidateSecret(secret)
					// Nil data is treated as empty, so should report as "cannot be empty"
					Expect(err).To(MatchError(fmt.Sprintf("validation error in field %q: required field cannot be empty", field)))
				},
				Entry("nil subscriptionID", "subscriptionID"),
				Entry("nil tenantID", "tenantID"),
				Entry("nil clientID", "clientID"),
				Entry("nil clientSecret", "clientSecret"),
			)

			It("should fail when secret.Data is nil", func() {
				secret.Data = nil
				_, err := validator.ValidateSecret(secret)
				Expect(err).To(MatchError(ContainSubstring("required field is missing")))
			})
		})

		Context("Invalid formats", func() {
			DescribeTable("should fail with invalid GUID formats",
				func(field, invalidGUID string) {
					secret.Data[field] = []byte(invalidGUID)
					_, err := validator.ValidateSecret(secret)
					Expect(err).To(MatchError(ContainSubstring(fmt.Sprintf(`pattern mismatch in field "%s": does not match any allowed patterns`, field))))
					Expect(err).To(MatchError(ContainSubstring(fmt.Sprintf("actual: %q", invalidGUID))))
				},
				Entry("subscriptionID too short", "subscriptionID", "12345678-1234-1234-1234-12345678901"),
				Entry("subscriptionID too long", "subscriptionID", "12345678-1234-1234-1234-1234567890123"),
				Entry("subscriptionID missing hyphens", "subscriptionID", "12345678123412341234123456789012"),
				Entry("subscriptionID invalid characters", "subscriptionID", "12345678-1234-1234-1234-12345678901g"),
				Entry("tenantID wrong hyphen positions", "tenantID", "876543214-321-4321-4321-210987654321"),
				Entry("tenantID spaces", "tenantID", "87654321-4321-4321-4321-210987654 21"),
				Entry("clientID not a GUID", "clientID", "invalid-client-id"),
				Entry("clientID partial", "clientID", "12345678-1234"),
			)

			It("should fail with client secret shorter than minimum length", func() {
				secret.Data["clientSecret"] = []byte("1234567890123456789012345678901")
				_, err := validator.ValidateSecret(secret)
				Expect(err).To(MatchError(ContainSubstring(`field value must be at least 32 characters`)))
			})

			It("should fail with client secret longer than maximum length", func() {
				secret.Data["clientSecret"] = []byte("123456789012345678901234567890123456789012345")
				_, err := validator.ValidateSecret(secret)
				Expect(err).To(MatchError(ContainSubstring(`field value must be at most 44 characters`)))
			})

			DescribeTable("should fail with invalid client secret formats (pattern)",
				func(invalidSecret string) {
					secret.Data["clientSecret"] = []byte(invalidSecret)
					_, err := validator.ValidateSecret(secret)
					Expect(err).To(MatchError(ContainSubstring(`pattern mismatch in field "clientSecret": does not match any allowed patterns`)))
				},
				Entry("contains whitespace (spaces/tabs/newlines)", "test test test test test test test test  "),
				Entry("contains unicode/emoji", "secretüwithunicode😀valueeeeeeeeeeee"),
				Entry("mixed valid and invalid", "validPart!invalid.and~more-1234567890_ "),
			)
		})

		Context("Non-printable character validation", func() {
			DescribeTable("should fail when fields contain non-printable characters",
				func(fieldName string, fieldValue string) {
					secret.Data[fieldName] = []byte(fieldValue)
					_, err := validator.ValidateSecret(secret)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(ContainSubstring("field value contains non-printable character")))
				},
				Entry("subscriptionID with null byte",
					"subscriptionID", "12345678-1234-1234-1234-\x0012345678901"),
				Entry("tenantID with null byte",
					"tenantID", "87654321-4321-4321-4321-\x00210987654321"),
				Entry("clientID with null byte",
					"clientID", "abcdef12-3456-7890-abcd-\x00ef1234567890"),
				Entry("clientSecret with null byte",
					"clientSecret", "AbCdE~fGhI.-jKlMnOpQrStUvWxY\x00z0_123456789"),
			)
		})
	})
})
