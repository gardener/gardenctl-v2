/*
SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package util_test

import (
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardenctl-v2/internal/util"
)

var _ = Describe("Factory", func() {
	Describe("GetSessionID", func() {
		var (
			factory                   *util.FactoryImpl
			prevSessionID, prevTermID string
			hadSessionID, hadTermID   bool
		)

		BeforeEach(func() {
			factory = util.NewFactoryImpl()

			// Preserve original environment variables
			prevSessionID, hadSessionID = os.LookupEnv("GCTL_SESSION_ID")
			prevTermID, hadTermID = os.LookupEnv("TERM_SESSION_ID")
		})

		AfterEach(func() {
			// Restore original environment variables
			if hadSessionID {
				_ = os.Setenv("GCTL_SESSION_ID", prevSessionID)
			} else {
				_ = os.Unsetenv("GCTL_SESSION_ID")
			}

			if hadTermID {
				_ = os.Setenv("TERM_SESSION_ID", prevTermID)
			} else {
				_ = os.Unsetenv("TERM_SESSION_ID")
			}
		})

		Context("when GCTL_SESSION_ID is set", func() {
			Context("success cases", func() {
				type testCase struct {
					name           string
					sessionID      string
					expectedResult string
				}

				DescribeTable("should return valid session ID",
					func(tc testCase) {
						Expect(os.Setenv("GCTL_SESSION_ID", tc.sessionID)).To(Succeed())

						sid, err := factory.GetSessionID()
						Expect(err).NotTo(HaveOccurred())
						Expect(sid).To(Equal(tc.expectedResult))
					},
					Entry("valid session ID with dashes", testCase{
						name:           "valid with dashes",
						sessionID:      "test-session-123",
						expectedResult: "test-session-123",
					}),
					Entry("valid session ID with underscores", testCase{
						name:           "valid with underscores",
						sessionID:      "test_session_123",
						expectedResult: "test_session_123",
					}),
					Entry("valid session ID with maximum length (128 chars)", testCase{
						name:           "valid with maximum length",
						sessionID:      "a" + strings.Repeat("1234567890", 12) + "1234567",
						expectedResult: "a" + strings.Repeat("1234567890", 12) + "1234567",
					}),
				)
			})

			Context("failure cases", func() {
				type testCase struct {
					name      string
					sessionID string
				}

				DescribeTable("should return an error when session ID is invalid",
					func(tc testCase) {
						Expect(os.Setenv("GCTL_SESSION_ID", tc.sessionID)).To(Succeed())

						_, err := factory.GetSessionID()
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("environment variable GCTL_SESSION_ID must only contain alphanumeric characters, underscore and dash"))
					},
					Entry("invalid session ID with special characters", testCase{
						name:      "invalid with special characters",
						sessionID: "test@session",
					}),
					Entry("invalid session ID that is too long (129 chars)", testCase{
						name:      "invalid too long",
						sessionID: "a" + strings.Repeat("1234567890", 12) + "12345678",
					}),
					Entry("invalid empty session ID", testCase{
						name:      "invalid empty",
						sessionID: "",
					}),
					Entry("invalid session ID with spaces", testCase{
						name:      "invalid with spaces",
						sessionID: "test session",
					}),
				)
			})
		})

		Context("when GCTL_SESSION_ID is not set but TERM_SESSION_ID is set", func() {
			BeforeEach(func() {
				Expect(os.Unsetenv("GCTL_SESSION_ID")).To(Succeed())
			})

			Context("success cases", func() {
				type testCase struct {
					name           string
					termSessionID  string
					expectedResult string
				}

				DescribeTable("should extract and return UUID from TERM_SESSION_ID",
					func(tc testCase) {
						Expect(os.Setenv("TERM_SESSION_ID", tc.termSessionID)).To(Succeed())

						sid, err := factory.GetSessionID()
						Expect(err).NotTo(HaveOccurred())
						Expect(sid).To(Equal(tc.expectedResult))
					},
					Entry("valid TERM_SESSION_ID with prefix", testCase{
						name:           "valid with prefix",
						termSessionID:  "w0:12345678-1234-4567-89ab-123456789012",
						expectedResult: "12345678-1234-4567-89ab-123456789012",
					}),
					Entry("valid TERM_SESSION_ID with prefix and suffix", testCase{
						name:           "valid with prefix and suffix",
						termSessionID:  "w0:12345678-1234-4567-89ab-123456789012:some-suffix",
						expectedResult: "12345678-1234-4567-89ab-123456789012",
					}),
					Entry("valid TERM_SESSION_ID with UUID only", testCase{
						name:           "valid UUID only",
						termSessionID:  "a1b2c3d4-5678-4abc-9def-123456789012",
						expectedResult: "a1b2c3d4-5678-4abc-9def-123456789012",
					}),
					Entry("valid TERM_SESSION_ID with uppercase UUID", testCase{
						name:           "valid uppercase UUID",
						termSessionID:  "w0:12345678-1234-4567-89AB-123456789012:some-suffix",
						expectedResult: "12345678-1234-4567-89ab-123456789012",
					}),
					Entry("valid TERM_SESSION_ID with mixed case UUID", testCase{
						name:           "valid mixed case UUID",
						termSessionID:  "w0:ABCDEF12-3456-4789-9ABC-DEF123456789:some-suffix",
						expectedResult: "abcdef12-3456-4789-9abc-def123456789",
					}),
					Entry("valid TERM_SESSION_ID with UUID in the middle", testCase{
						name:           "valid UUID in the middle",
						termSessionID:  "foo:bar:baz:ABCDEF12-3456-4789-9ABC-DEF123456789:some-suffix",
						expectedResult: "abcdef12-3456-4789-9abc-def123456789",
					}),
				)
			})

			Context("failure cases", func() {
				type testCase struct {
					name          string
					termSessionID string
				}

				DescribeTable("should return an error when TERM_SESSION_ID is invalid",
					func(tc testCase) {
						Expect(os.Setenv("TERM_SESSION_ID", tc.termSessionID)).To(Succeed())

						_, err := factory.GetSessionID()
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("environment variable GCTL_SESSION_ID is required"))
					},
					Entry("invalid TERM_SESSION_ID without UUID", testCase{
						name:          "invalid without UUID",
						termSessionID: "invalid-session-id",
					}),
					Entry("invalid TERM_SESSION_ID without UUID in the middle", testCase{
						name:          "invalid without UUID in the middle",
						termSessionID: "w0:no-uid:some-suffix",
					}),
					Entry("invalid TERM_SESSION_ID with wrong UUID version", testCase{
						name:          "invalid UUID version",
						termSessionID: "12345678-1234-3567-89ab-123456789012",
					}),
					Entry("invalid empty TERM_SESSION_ID", testCase{
						name:          "invalid empty",
						termSessionID: "",
					}),
				)
			})
		})

		Context("when neither GCTL_SESSION_ID nor TERM_SESSION_ID is set", func() {
			BeforeEach(func() {
				Expect(os.Unsetenv("GCTL_SESSION_ID")).To(Succeed())
				Expect(os.Unsetenv("TERM_SESSION_ID")).To(Succeed())
			})

			It("should return an error", func() {
				_, err := factory.GetSessionID()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("environment variable GCTL_SESSION_ID is required"))
			})
		})

		Context("when both GCTL_SESSION_ID and TERM_SESSION_ID are set", func() {
			It("should prioritize GCTL_SESSION_ID over TERM_SESSION_ID", func() {
				Expect(os.Setenv("GCTL_SESSION_ID", "priority-session")).To(Succeed())
				Expect(os.Setenv("TERM_SESSION_ID", "w0:12345678-1234-4567-89ab-123456789012:suffix")).To(Succeed())

				sid, err := factory.GetSessionID()
				Expect(err).NotTo(HaveOccurred())
				Expect(sid).To(Equal("priority-session"))
			})

			It("should not fall back to TERM_SESSION_ID when GCTL_SESSION_ID is invalid", func() {
				Expect(os.Setenv("GCTL_SESSION_ID", "invalid@session")).To(Succeed())
				Expect(os.Setenv("TERM_SESSION_ID", "w0:12345678-1234-4567-89ab-123456789012:suffix")).To(Succeed())

				_, err := factory.GetSessionID()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("environment variable GCTL_SESSION_ID must only contain alphanumeric characters"))
			})
		})
	})
})
