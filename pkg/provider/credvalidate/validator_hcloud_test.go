/*
SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package credvalidate_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	credvalidate "github.com/gardener/gardenctl-v2/pkg/provider/credvalidate"
)

var _ = Describe("HCloud Validator", func() {
	var validator *credvalidate.HCloudValidator

	BeforeEach(func() {
		validator = credvalidate.NewHCloudValidator(context.Background())
	})

	Describe("Secret Validation", func() {
		var secret *corev1.Secret

		BeforeEach(func() {
			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hcloud-secret",
					Namespace: "test-namespace",
				},
				Data: map[string][]byte{
					"hcloudToken": []byte("1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ12"),
				},
			}
		})

		Context("Valid credentials", func() {
			It("should succeed with valid credentials", func() {
				creds, err := validator.ValidateSecret(secret)
				Expect(err).NotTo(HaveOccurred())
				Expect(creds).To(HaveKeyWithValue("hcloudToken", "1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ12"))
				Expect(len(creds)).To(Equal(1))
			})

			It("should ignore extra top-level secret keys (Permissive mode)", func() {
				// Add an unrelated extra key at the top level alongside valid HCloud token
				secret.Data["foo"] = []byte("bar")
				creds, err := validator.ValidateSecret(secret)
				Expect(err).NotTo(HaveOccurred())
				Expect(creds).To(HaveKey("hcloudToken"))
				Expect(creds).NotTo(HaveKey("foo"))
			})

			DescribeTable("should succeed with different valid tokens",
				func(token string) {
					secret.Data["hcloudToken"] = []byte(token)
					creds, err := validator.ValidateSecret(secret)
					Expect(err).NotTo(HaveOccurred(), "should accept valid token: %s", token)
					Expect(creds).To(HaveKeyWithValue("hcloudToken", token))
					Expect(len(creds)).To(Equal(1))
				},
				Entry("random-like token A", "c61iu7rOJasN9PTdJTXHzrt1YFHZN5iIQVuWVb2v9mKxK8Xoj8RzJX3ksEXAMPLE"),
				Entry("alpha-numeric mix 1", "1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ12"),
				Entry("alpha-numeric mix 2", "ABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890abcdefghijklmnopqrstuvwxyz12"),
				Entry("mixed case", "aBcDeFgHiJkLmNoPqRsTuVwXyZ1234567890abcdefghijklmnopqrstuvwxyz12"),
				Entry("descending then ascending", "9876543210zyxwvutsrqponmlkjihgfedcbaZYXWVUTSRQPONMLKJIHGFEDCBA12"),
				Entry("all zeros", "0000000000000000000000000000000000000000000000000000000000000000"),
				Entry("all Fs", "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF"),
			)
		})

		Context("Missing fields", func() {
			It("should fail when hcloudToken is missing", func() {
				delete(secret.Data, "hcloudToken")
				_, err := validator.ValidateSecret(secret)
				Expect(err).To(MatchError("validation error in field \"hcloudToken\": required field is missing"))
			})
		})

		Context("Nil data fields", func() {
			It("should fail when hcloudToken contains nil data", func() {
				secret.Data["hcloudToken"] = nil
				_, err := validator.ValidateSecret(secret)
				Expect(err).To(MatchError("validation error in field \"hcloudToken\": required field cannot be empty"))
			})

			It("should fail when secret.Data is nil", func() {
				secret.Data = nil
				_, err := validator.ValidateSecret(secret)
				Expect(err).To(MatchError(ContainSubstring("required field is missing")))
			})
		})

		Context("Invalid hcloudToken", func() {
			DescribeTable("should fail with invalid hcloudToken length",
				func(hcloudToken, expectedError string) {
					secret.Data["hcloudToken"] = []byte(hcloudToken)
					_, err := validator.ValidateSecret(secret)
					Expect(err).To(MatchError(ContainSubstring(expectedError)))
				},
				Entry("too short", "shorttoken", "validation error in field \"hcloudToken\": field value must be exactly 64 characters, got 10"),
				Entry("too long", "1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ12EXTRA", "validation error in field \"hcloudToken\": field value must be exactly 64 characters, got 69"),
				Entry("empty", "", "validation error in field \"hcloudToken\": required field cannot be empty"),
				Entry("63 characters", "1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1", "validation error in field \"hcloudToken\": field value must be exactly 64 characters, got 63"),
				Entry("65 characters", "1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ123", "validation error in field \"hcloudToken\": field value must be exactly 64 characters, got 65"),
			)

			DescribeTable("should fail with invalid hcloudToken format",
				func(hcloudToken string) {
					secret.Data["hcloudToken"] = []byte(hcloudToken)
					_, err := validator.ValidateSecret(secret)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(ContainSubstring(`pattern mismatch in field "hcloudToken": does not match any allowed patterns`)))
				},
				Entry("invalid characters", "1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ@2"),
				Entry("spaces", "1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ 2"),
				Entry("special characters", "1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ#2"),
				Entry("unicode", "1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZü"), // ü counts as 2 characters in UTF-8
				Entry("hyphens", "1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ-2"),
				Entry("underscores", "1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ_2"),
			)

			DescribeTable("should fail when fields contain non-printable characters",
				func(fieldName string, fieldValue string) {
					secret.Data[fieldName] = []byte(fieldValue)
					_, err := validator.ValidateSecret(secret)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(ContainSubstring("field value contains non-printable character")))
				},
				Entry("hcloudToken with null byte",
					"hcloudToken", "1234567890abcdefghijklmnopqrstuvwxyzABCDEF\x00HIJKLMNOPQRSTUV12"),
			)
		})
	})
})
