/*
SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package credvalidate_test

import (
	"context"
	"encoding/json"
	"fmt"

	gardensecurityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

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

	Describe("Workload Identity Validation", func() {
		var workloadIdentity *gardensecurityv1alpha1.WorkloadIdentity

		Context("Valid configurations", func() {
			It("should succeed with valid role ARN", func() {
				workloadIdentity = &gardensecurityv1alpha1.WorkloadIdentity{
					Spec: gardensecurityv1alpha1.WorkloadIdentitySpec{
						TargetSystem: gardensecurityv1alpha1.TargetSystem{
							Type: "aws",
							ProviderConfig: &runtime.RawExtension{
								Raw: []byte(`{"roleARN": "arn:aws:iam::123456789012:role/MyRole"}`),
							},
						},
					},
				}

				validatedConfig, err := validator.ValidateWorkloadIdentityConfig(workloadIdentity)
				Expect(err).NotTo(HaveOccurred())
				Expect(validatedConfig).To(HaveKeyWithValue("roleARN", "arn:aws:iam::123456789012:role/MyRole"))
			})

			DescribeTable("should succeed with complex role names and different partitions",
				func(roleARN string) {
					workloadIdentity = &gardensecurityv1alpha1.WorkloadIdentity{
						Spec: gardensecurityv1alpha1.WorkloadIdentitySpec{
							TargetSystem: gardensecurityv1alpha1.TargetSystem{
								Type: "aws",
								ProviderConfig: &runtime.RawExtension{
									Raw: []byte(fmt.Sprintf(`{"roleARN": "%s"}`, roleARN)),
								},
							},
						},
					}

					_, err := validator.ValidateWorkloadIdentityConfig(workloadIdentity)
					Expect(err).NotTo(HaveOccurred(), "should accept valid role ARN: %s", roleARN)
				},
				Entry("simple role name", "arn:aws:iam::123456789012:role/MyRole"),
				Entry("service-role path", "arn:aws:iam::123456789012:role/service-role/MyComplexRole-123"),
				Entry("underscores", "arn:aws:iam::123456789012:role/MyRole_With_Underscores"),
				Entry("dots", "arn:aws:iam::123456789012:role/MyRole.With.Dots"),
				Entry("plus signs", "arn:aws:iam::123456789012:role/MyRole+With+Plus"),
				Entry("equals", "arn:aws:iam::123456789012:role/MyRole=With=Equals"),
				Entry("commas", "arn:aws:iam::123456789012:role/MyRole,With,Commas"),
				Entry("at signs", "arn:aws:iam::123456789012:role/MyRole@With@At"),
				Entry("dashes", "arn:aws:iam::123456789012:role/MyRole-With-Dashes"),
				Entry("gov partition", "arn:aws-us-gov:iam::123456789012:role/GovRole"),
				Entry("cn partition", "arn:aws-cn:iam::123456789012:role/ChinaRole"),
				Entry("iso partition", "arn:aws-iso:iam::123456789012:role/IsoRole"),
				Entry("iso-b partition", "arn:aws-iso-b:iam::123456789012:role/IsoBRole"),
				Entry("role path", "arn:aws:iam::123456789012:role/path/to/MyRole"),
				Entry("deep role path", "arn:aws:iam::123456789012:role/deep/nested/path/MyRole"),
			)
		})

		Context("Invalid configurations", func() {
			BeforeEach(func() {
				baseConfig := map[string]string{
					"roleARN": "arn:aws:iam::123456789012:role/MyRole",
				}
				configBytes, _ := json.Marshal(baseConfig)
				workloadIdentity = &gardensecurityv1alpha1.WorkloadIdentity{
					Spec: gardensecurityv1alpha1.WorkloadIdentitySpec{
						TargetSystem: gardensecurityv1alpha1.TargetSystem{
							Type: "aws",
							ProviderConfig: &runtime.RawExtension{
								Raw: configBytes,
							},
						},
					},
				}
			})

			It("should fail with empty role ARN", func() {
				workloadIdentity.Spec.TargetSystem.ProviderConfig.Raw = []byte(`{"roleARN": ""}`)

				_, err := validator.ValidateWorkloadIdentityConfig(workloadIdentity)
				Expect(err).To(MatchError("validation error in field \"roleARN\": required field cannot be empty"))
			})

			It("should fail with missing roleARN field", func() {
				workloadIdentity = &gardensecurityv1alpha1.WorkloadIdentity{
					Spec: gardensecurityv1alpha1.WorkloadIdentitySpec{
						TargetSystem: gardensecurityv1alpha1.TargetSystem{
							Type: "aws",
							ProviderConfig: &runtime.RawExtension{
								Raw: []byte(`{}`),
							},
						},
					},
				}

				_, err := validator.ValidateWorkloadIdentityConfig(workloadIdentity)
				Expect(err).To(MatchError("validation error in field \"roleARN\": required field is missing"))
			})

			DescribeTable("should fail with invalid role ARN length",
				func(roleARN, expectedError string) {
					workloadIdentity.Spec.TargetSystem.ProviderConfig.Raw = []byte(fmt.Sprintf(`{"roleARN": "%s"}`, roleARN))

					_, err := validator.ValidateWorkloadIdentityConfig(workloadIdentity)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(ContainSubstring(expectedError)))
					Expect(err).To(MatchError(ContainSubstring(fmt.Sprintf("value: %q", roleARN))))
				},
				Entry("too short - 19 characters", "arn:aws:iam::123456", "validation error in field \"roleARN\": field value must be at least 20 characters, got 19"),
				Entry("too long - over 2048 characters",
					func() string {
						baseARN := "arn:aws:iam::123456789012:role/"
						paddingLength := 2049 - len(baseARN)
						padding := make([]byte, paddingLength)
						for i := range padding {
							padding[i] = 'a'
						}
						return baseARN + string(padding)
					}(),
					func() string {
						baseARN := "arn:aws:iam::123456789012:role/"
						paddingLength := 2049 - len(baseARN)
						padding := make([]byte, paddingLength)
						for i := range padding {
							padding[i] = 'a'
						}
						longARN := baseARN + string(padding)
						return fmt.Sprintf("validation error in field \"roleARN\": field value must be at most 2048 characters, got %d", len(longARN))
					}(),
				),
				Entry("malformed ARN - too short", "not-an-arn", "validation error in field \"roleARN\": field value must be at least 20 characters, got 10"),
			)

			DescribeTable("should fail with invalid role ARN format",
				func(roleARN string) {
					workloadIdentity.Spec.TargetSystem.ProviderConfig.Raw = []byte(fmt.Sprintf(`{"roleARN": "%s"}`, roleARN))

					_, err := validator.ValidateWorkloadIdentityConfig(workloadIdentity)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(ContainSubstring(`pattern mismatch in field "roleARN": does not match any allowed patterns`)))
					Expect(err).To(MatchError(ContainSubstring(fmt.Sprintf("actual: %q", roleARN))))
				},
				Entry("wrong service", "arn:aws:s3::123456789012:role/MyRole"),
				Entry("wrong resource type", "arn:aws:iam::123456789012:user/MyUser"),
				Entry("invalid account ID - too short", "arn:aws:iam::12345:role/MyRole"),
				Entry("invalid account ID - too long", "arn:aws:iam::1234567890123:role/MyRole"),
				Entry("invalid account ID - letters", "arn:aws:iam::12345678901a:role/MyRole"),
				Entry("missing account ID", "arn:aws:iam:::role/MyRole"),
				Entry("wrong partition", "arn:gcp:iam::123456789012:role/MyRole"),
				Entry("missing arn prefix", "aws:iam::123456789012:role/MyRole"),
				Entry("empty role name", "arn:aws:iam::123456789012:role/"),
				Entry("missing role name", "arn:aws:iam::123456789012:role"),
				Entry("unsupported partition", "arn:aws-other:iam::123456789012:role/MyRole"),
				Entry("invalid characters in role name", "arn:aws:iam::123456789012:role/My Role"),
			)

			It("should fail with invalid JSON in provider config", func() {
				workloadIdentity.Spec.TargetSystem.ProviderConfig.Raw = []byte(`invalid json`)

				_, err := validator.ValidateWorkloadIdentityConfig(workloadIdentity)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("failed to unmarshal AWS workload identity config")))
			})

			DescribeTable("fails with invalid provider config",
				func(providerConfig *runtime.RawExtension, errorSub string) {
					workloadIdentity.Spec.TargetSystem.ProviderConfig = providerConfig
					_, err := validator.ValidateWorkloadIdentityConfig(workloadIdentity)
					Expect(err).To(MatchError(ContainSubstring(errorSub)))
				},
				Entry("invalid JSON", &runtime.RawExtension{Raw: []byte(`invalid json`)}, "failed to unmarshal AWS workload identity config"),
				Entry("malformed JSON", &runtime.RawExtension{Raw: []byte(`{"roleARN": "arn:aws:iam::123456789012:role/MyRole"`)}, "failed to unmarshal AWS workload identity config"), // Missing closing brace
				Entry("nil provider config", nil, "providerConfig is missing"),
				Entry("empty provider config", &runtime.RawExtension{Raw: []byte(``)}, "failed to unmarshal AWS workload identity config"),
			)
		})
	})
})
