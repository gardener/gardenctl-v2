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

	credvalidate "github.com/gardener/gardenctl-v2/pkg/provider/credvalidate"
)

var _ = Describe("AliCloud Validator", func() {
	var validator *credvalidate.AliCloudValidator

	BeforeEach(func() {
		validator = credvalidate.NewAliCloudValidator(context.Background())
	})

	Describe("Secret Validation", func() {
		var secret *corev1.Secret

		BeforeEach(func() {
			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "alicloud-secret",
					Namespace: "test-namespace",
				},
				Data: map[string][]byte{
					"accessKeyID":     []byte("LTAI5tQ8Bqj9KrS5vexample"),
					"accessKeySecret": []byte("1234567890abcdefghijklmnopqrst"),
				},
			}
		})

		Context("Valid credentials", func() {
			It("should succeed with valid credentials", func() {
				creds, err := validator.ValidateSecret(secret)
				Expect(err).NotTo(HaveOccurred())
				Expect(creds).To(HaveKeyWithValue("accessKeyID", "LTAI5tQ8Bqj9KrS5vexample"))
				Expect(creds).To(HaveKeyWithValue("accessKeySecret", "1234567890abcdefghijklmnopqrst"))
				Expect(len(creds)).To(Equal(2))
			})

			It("should ignore extra top-level secret keys (Permissive mode)", func() {
				// Add an unrelated extra key at the top level alongside valid AliCloud keys
				secret.Data["foo"] = []byte("bar")
				creds, err := validator.ValidateSecret(secret)
				Expect(err).NotTo(HaveOccurred())
				Expect(creds).To(HaveKey("accessKeyID"))
				Expect(creds).To(HaveKey("accessKeySecret"))
				Expect(creds).NotTo(HaveKey("foo"))
			})

			DescribeTable("should succeed with different valid access key IDs",
				func(accessKey string) {
					secret.Data["accessKeyID"] = []byte(accessKey)
					creds, err := validator.ValidateSecret(secret)
					Expect(err).NotTo(HaveOccurred(), "should accept valid access key: %s", accessKey)
					Expect(creds).To(HaveKeyWithValue("accessKeyID", accessKey))
				},
				Entry("uppercase letters and numbers", "LTAI1234567890ABCDEFGHIJ"),
				Entry("lowercase letters", "LTAIabcdefghijklmnopqrst"),
				Entry("mixed sample", "LTAI5tQ8Bqj9KrS5vexample"),
				Entry("uppercase sample", "LTAI9zY8XqW7eR6tPEXAMPLE"),
			)

			DescribeTable("should succeed with different valid access key secrets",
				func(secretKey string) {
					secret.Data["accessKeySecret"] = []byte(secretKey)
					creds, err := validator.ValidateSecret(secret)
					Expect(err).NotTo(HaveOccurred(), "should accept valid secret key: %s", secretKey)
					Expect(creds).To(HaveKeyWithValue("accessKeySecret", secretKey))
				},
				Entry("lowercase and numbers", "1234567890abcdefghijklmnopqrst"),
				Entry("uppercase and numbers", "ABCDEFGHIJKLMNOPQRSTUVWXYZ1234"),
				Entry("mixed case and numbers", "aBcDeFgHiJkLmNoPqRsTuVwXyZ1234"),
				Entry("numbers and lowercase reversed", "9876543210zyxwvutsrqponmlkjihg"),
			)
		})

		Context("Missing fields", func() {
			DescribeTable("should fail when fields are missing",
				func(modifySecret func(*corev1.Secret), expectedError string) {
					modifySecret(secret)
					_, err := validator.ValidateSecret(secret)
					Expect(err).To(MatchError(expectedError))
				},
				Entry("missing accessKeyID",
					func(s *corev1.Secret) { delete(s.Data, "accessKeyID") },
					"validation error in field \"accessKeyID\": required field is missing",
				),
				Entry("missing accessKeySecret",
					func(s *corev1.Secret) { delete(s.Data, "accessKeySecret") },
					"validation error in field \"accessKeySecret\": required field is missing",
				),
			)
		})

		Context("Nil data fields", func() {
			DescribeTable("should fail when fields contain nil data",
				func(modifySecret func(*corev1.Secret), expectedError string) {
					modifySecret(secret)
					_, err := validator.ValidateSecret(secret)
					Expect(err).To(MatchError(expectedError))
				},
				Entry("nil accessKeyID",
					func(s *corev1.Secret) { s.Data["accessKeyID"] = nil },
					"validation error in field \"accessKeyID\": required field cannot be empty",
				),
				Entry("nil accessKeySecret",
					func(s *corev1.Secret) { s.Data["accessKeySecret"] = nil },
					"validation error in field \"accessKeySecret\": required field cannot be empty",
				),
			)

			It("should fail when secret.Data is nil", func() {
				secret.Data = nil
				_, err := validator.ValidateSecret(secret)
				Expect(err).To(MatchError(ContainSubstring("required field is missing")))
			})
		})

		Context("Invalid accessKeyID", func() {
			DescribeTable("should fail with invalid accessKeyID length",
				func(accessKeyID, expectedError string) {
					secret.Data["accessKeyID"] = []byte(accessKeyID)
					_, err := validator.ValidateSecret(secret)
					Expect(err).To(MatchError(ContainSubstring(expectedError)))
				},
				Entry("too short", "LTAI123", "validation error in field \"accessKeyID\": field value must be exactly 24 characters, got 7"),
				Entry("too long", "LTAI5tQ8Bqj9KrS5vexampleEXTRA", "validation error in field \"accessKeyID\": field value must be exactly 24 characters, got 29"),
				Entry("empty", "", "validation error in field \"accessKeyID\": required field cannot be empty"),
				Entry("29 characters", "LTAI5tQ8Bqj9KrS5vexampl", "validation error in field \"accessKeyID\": field value must be exactly 24 characters, got 23"),
				Entry("31 characters", "LTAI5tQ8Bqj9KrS5vexampleX", "validation error in field \"accessKeyID\": field value must be exactly 24 characters, got 25"),
			)

			DescribeTable("should fail with invalid accessKeyID format",
				func(accessKeyID string) {
					secret.Data["accessKeyID"] = []byte(accessKeyID)
					_, err := validator.ValidateSecret(secret)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(ContainSubstring(`pattern mismatch in field "accessKeyID": does not match any allowed patterns`)))
					Expect(err).To(MatchError(ContainSubstring(fmt.Sprintf("actual: %q", accessKeyID))))
				},
				Entry("wrong prefix", "BKAI5tQ8Bqj9K2mN3example"),
				Entry("lowercase prefix", "ltai5tQ8Bqj9K2mNvexample"),
				Entry("mixed case prefix", "Ltai5tQ8Bqj9K2mN3example"),
				Entry("invalid characters", "LTAI@tQ8Bqj9K2mN3example"),
				Entry("special characters", "LTAI!@#$%^&*()5vX7yZ1aBc"),
				Entry("spaces", "LTAI 5tQ8Bqj9K2mNexample"),
				Entry("no prefix", "1234567890ABCDEFGHIJKLMN"),
				Entry("wrong length prefix", "LTA1234567890ABCDEFGHIJK"),
				Entry("AWSKEY prefix", "AKIA5tQ8Bqj9K2mN3example"),
			)
		})

		Context("Invalid accessKeySecret", func() {
			DescribeTable("should fail with invalid accessKeySecret length",
				func(accessKeySecret, expectedError string) {
					secret.Data["accessKeySecret"] = []byte(accessKeySecret)
					_, err := validator.ValidateSecret(secret)
					Expect(err).To(MatchError(expectedError))
				},
				Entry("too short", "shortkey", "validation error in field \"accessKeySecret\": field value must be exactly 30 characters, got 8"),
				Entry("too long", "1234567890abcdefghijklmnopqrstuvwxyz", "validation error in field \"accessKeySecret\": field value must be exactly 30 characters, got 36"),
				Entry("empty", "", "validation error in field \"accessKeySecret\": required field cannot be empty"),
				Entry("29 characters", "1234567890abcdefghijklmnopqrs", "validation error in field \"accessKeySecret\": field value must be exactly 30 characters, got 29"),
				Entry("31 characters", "1234567890abcdefghijklmnopqrstu", "validation error in field \"accessKeySecret\": field value must be exactly 30 characters, got 31"),
			)

			DescribeTable("should fail with invalid accessKeySecret format",
				func(accessKeySecret string) {
					secret.Data["accessKeySecret"] = []byte(accessKeySecret)
					_, err := validator.ValidateSecret(secret)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(ContainSubstring(`pattern mismatch in field "accessKeySecret": does not match any allowed patterns`)))
				},
				Entry("invalid characters", "1234567890abcdefghijklmnopqr@t"),
				Entry("spaces", "1234567890abcdefghijklmnopqr t"),
				Entry("special characters", "1234567890abcdefghijklmnopqr#t"),
				Entry("unicode", "1234567890abcdefghijklmnopqrü"),
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
				Entry("accessKeyID with null byte",
					"accessKeyID", "LTAI5tQ8Bqj9\x00KrS5vexample"),
				Entry("accessKeySecret with null byte",
					"accessKeySecret", "1234567890abcd\x00efghijklmnopqrst"),
			)
		})
	})

	Describe("Workload Identity Validation", func() {
		It("should fail as workload identity is not supported for alicloud", func() {
			_, err := validator.ValidateWorkloadIdentityConfig(nil)
			Expect(err).To(MatchError("workload identity not supported for alicloud"))
		})
	})
})
