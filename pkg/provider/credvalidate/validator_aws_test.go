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

var _ = Describe("AWS Validator", func() {
	var validator *credvalidate.AWSValidator

	BeforeEach(func() {
		validator = credvalidate.NewAWSValidator(context.Background())
	})

	Describe("Secret Validation", func() {
		var secret *corev1.Secret

		BeforeEach(func() {
			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "aws-secret",
					Namespace: "test-namespace",
				},
				Data: map[string][]byte{
					"accessKeyID":     []byte("AKIAIOSFODNN7EXAMPLE"),
					"secretAccessKey": []byte("wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"),
				},
			}
		})

		Context("Valid credentials", func() {
			DescribeTable("should succeed with valid credentials",
				func(accessKeyID string) {
					secret.Data["accessKeyID"] = []byte(accessKeyID)
					creds, err := validator.ValidateSecret(secret)
					Expect(err).NotTo(HaveOccurred())
					Expect(creds).To(HaveKeyWithValue("accessKeyID", accessKeyID))
					Expect(creds).To(HaveKeyWithValue("secretAccessKey", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"))
					Expect(len(creds)).To(Equal(2))
				},
				Entry("permanent credentials", "AKIAIOSFODNN7EXAMPLE"),
				Entry("temporary credentials", "ASIAIOSFODNN7EXAMPLE"),
			)

			It("should ignore extra top-level secret keys (Permissive mode)", func() {
				// Add an unrelated extra key at the top level alongside valid AWS keys
				secret.Data["foo"] = []byte("bar")
				creds, err := validator.ValidateSecret(secret)
				Expect(err).NotTo(HaveOccurred())
				Expect(creds).To(HaveKey("accessKeyID"))
				Expect(creds).To(HaveKey("secretAccessKey"))
				Expect(creds).NotTo(HaveKey("foo"))
			})

			DescribeTable("should succeed with different valid access key IDs",
				func(accessKey string) {
					secret.Data["accessKeyID"] = []byte(accessKey)
					creds, err := validator.ValidateSecret(secret)
					Expect(err).NotTo(HaveOccurred(), "should accept valid access key: %s", accessKey)
					Expect(creds).To(HaveKeyWithValue("accessKeyID", accessKey))
				},
				Entry("AKIA prefix", "AKIA1234567890ABCDEF"),
				Entry("ASI prefix", "ASIABCDEFGHIJKLMNOPQ"),
				Entry("ASIA temporary key", "ASIA0123456789ABCDEF"),
			)

			DescribeTable("should succeed with different valid secret access keys",
				func(secretKey string) {
					secret.Data["secretAccessKey"] = []byte(secretKey)
					creds, err := validator.ValidateSecret(secret)
					Expect(err).NotTo(HaveOccurred(), "should accept valid secret key: %s", secretKey)
					Expect(creds).To(HaveKeyWithValue("secretAccessKey", secretKey))
				},
				Entry("canonical example", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"),
				Entry("lowercase letters allowed", "1234567890abcdefghijklmnopqrstuvwxyzABCD"),
				Entry("uppercase letters allowed", "ABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890abcd"),
				Entry("symbols allowed", "aBcDeFgHiJkLmNoPqRsTuVwXyZ1234567890+/=A"),
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
				Entry("missing secretAccessKey",
					func(s *corev1.Secret) { delete(s.Data, "secretAccessKey") },
					"validation error in field \"secretAccessKey\": required field is missing",
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
				Entry("nil secretAccessKey",
					func(s *corev1.Secret) { s.Data["secretAccessKey"] = nil },
					"validation error in field \"secretAccessKey\": required field cannot be empty",
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
				Entry("too short", "AKIA123", "validation error in field \"accessKeyID\": field value must be exactly 20 characters, got 7"),
				Entry("too long", "AKIAIOSFODNN7EXAMPLEEXTRA", "validation error in field \"accessKeyID\": field value must be exactly 20 characters, got 25"),
				Entry("empty", "", "validation error in field \"accessKeyID\": required field cannot be empty"),
				Entry("19 characters", "AKIAIOSFODNN7EXAMPL", "validation error in field \"accessKeyID\": field value must be exactly 20 characters, got 19"),
				Entry("21 characters", "AKIAIOSFODNN7EXAMPLEX", "validation error in field \"accessKeyID\": field value must be exactly 20 characters, got 21"),
			)

			DescribeTable("should fail with invalid accessKeyID format",
				func(accessKeyID string) {
					secret.Data["accessKeyID"] = []byte(accessKeyID)
					_, err := validator.ValidateSecret(secret)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(ContainSubstring(`pattern mismatch in field "accessKeyID": does not match any allowed patterns`)))
					Expect(err).To(MatchError(ContainSubstring(fmt.Sprintf("actual: %q", accessKeyID))))
				},
				Entry("wrong prefix", "BKIAIOSFODNN7EXAMPLE"),
				Entry("lowercase prefix", "akiaIOSFODNN7EXAMPLE"),
				Entry("mixed case prefix", "AkiaIOSFODNN7EXAMPLE"),
				Entry("invalid characters", "AKIA@OSFODNN7EXAMPLE"),
				Entry("special characters", "AKIA!@#$%^&*()EXAMPL"),
				Entry("spaces", "AKIA OSFODNN7EXAMPLE"),
				Entry("lowercase letters", "AKIAiosfodnn7example"),
				Entry("no prefix", "1234567890ABCDEFGHIJ"),
				Entry("wrong length prefix", "AK1234567890ABCDEFGH"),
				Entry("AWSKEY prefix", "AWSKIOSFODNN7EXAMPLE"),
			)
		})

		Context("Invalid secretAccessKey", func() {
			DescribeTable("should fail with invalid secretAccessKey length",
				func(secretAccessKey, expectedError string) {
					secret.Data["secretAccessKey"] = []byte(secretAccessKey)
					_, err := validator.ValidateSecret(secret)
					Expect(err).To(MatchError(ContainSubstring(expectedError)))
				},
				Entry("too short", "shortkey", "validation error in field \"secretAccessKey\": field value must be exactly 40 characters, got 8"),
				Entry("too long", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEYEXTRA", "validation error in field \"secretAccessKey\": field value must be exactly 40 characters, got 45"),
				Entry("empty", "", "validation error in field \"secretAccessKey\": required field cannot be empty"),
				Entry("39 characters", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKE", "validation error in field \"secretAccessKey\": field value must be exactly 40 characters, got 39"),
				Entry("41 characters", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEYX", "validation error in field \"secretAccessKey\": field value must be exactly 40 characters, got 41"),
			)

			DescribeTable("should fail with invalid secretAccessKey format",
				func(secretAccessKey string) {
					secret.Data["secretAccessKey"] = []byte(secretAccessKey)
					_, err := validator.ValidateSecret(secret)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(ContainSubstring(`pattern mismatch in field "secretAccessKey": does not match any allowed patterns`)))
				},
				Entry("invalid characters", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLE@EY"),
				Entry("spaces", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLE EY"),
				Entry("special characters", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLE#EY"),
				Entry("unicode", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEüE"),
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
					"accessKeyID", "AKIA\x00OSFODNN7EXAMPLE"),
				Entry("secretAccessKey with null byte",
					"secretAccessKey", "wJalrXUtnFEMI/K7MDENG/bPxRfi\x00YEXAMPLEKEY"),
			)
		})
	})
})
