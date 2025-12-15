/*
SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package allowpattern_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardenctl-v2/pkg/provider/common/allowpattern"
)

// testStrictHTTPSContext provides a local validation context for tests with
// StrictHTTPS enabled and a generic set of fields that may use RegexValue.
func testStrictHTTPSContext() *allowpattern.ValidationContext {
	return &allowpattern.ValidationContext{
		AllowedRegexFields: map[string]bool{
			"record_id": true,
			"client_id": true,
			"email":     true,
			"audience":  true,
		},
		StrictHTTPS: true,
	}
}

var _ = Describe("Pattern", func() {
	Describe("Validate", func() {
		It("should require field to be set", func() {
			pattern := &allowpattern.Pattern{}
			err := pattern.ValidateWithContext(testStrictHTTPSContext())
			Expect(err).To(MatchError("field is required"))
		})

		Context("URI mode", func() {
			It("should validate a valid HTTPS URI", func() {
				pattern := &allowpattern.Pattern{
					Field: "endpoint",
					URI:   "https://api.example.com/token",
				}
				err := pattern.ValidateWithContext(testStrictHTTPSContext())
				Expect(err).NotTo(HaveOccurred())
			})

			It("should reject HTTP scheme", func() {
				pattern := &allowpattern.Pattern{
					Field: "endpoint",
					URI:   "http://api.example.com/token",
				}
				err := pattern.ValidateWithContext(testStrictHTTPSContext())
				Expect(err).To(MatchError(ContainSubstring("scheme must be one of {https}, got \"http\"")))
			})

			It("should reject URI with query parameters", func() {
				pattern := &allowpattern.Pattern{
					Field: "endpoint",
					URI:   "https://api.example.com/token?param=value",
				}
				err := pattern.ValidateWithContext(testStrictHTTPSContext())
				Expect(err).To(MatchError(ContainSubstring("must not contain query parameters")))
			})

			It("should reject URI with fragments", func() {
				pattern := &allowpattern.Pattern{
					Field: "endpoint",
					URI:   "https://api.example.com/token#fragment",
				}
				err := pattern.ValidateWithContext(testStrictHTTPSContext())
				Expect(err).To(MatchError(ContainSubstring("must not contain fragments")))
			})

			It("should reject URI with userinfo", func() {
				pattern := &allowpattern.Pattern{
					Field: "endpoint",
					URI:   "https://user:pass@api.example.com/token",
				}
				err := pattern.ValidateWithContext(testStrictHTTPSContext())
				Expect(err).To(MatchError(ContainSubstring("must not contain userinfo")))
			})

			It("should reject URI without hostname", func() {
				pattern := &allowpattern.Pattern{
					Field: "endpoint",
					URI:   "https:///token",
				}
				err := pattern.ValidateWithContext(testStrictHTTPSContext())
				Expect(err).To(MatchError(ContainSubstring("hostname is required")))
			})

			It("should reject invalid URI", func() {
				pattern := &allowpattern.Pattern{
					Field: "endpoint",
					URI:   "not-a-valid-uri",
				}
				err := pattern.ValidateWithContext(testStrictHTTPSContext())
				Expect(err).To(MatchError(ContainSubstring("scheme must be one of {https}")))
			})

			It("should reject URI combined with other fields", func() {
				pattern := &allowpattern.Pattern{
					Field: "endpoint",
					URI:   "https://api.example.com/token",
					Host:  ptr.To("api.example.com"),
				}
				err := pattern.ValidateWithContext(testStrictHTTPSContext())
				Expect(err).To(MatchError("uri cannot be used together with host, path, regexPath, or port for field endpoint"))
			})
		})

		Context("RegexValue mode", func() {
			It("should validate a valid pattern with regexValue for allowed field", func() {
				pattern := &allowpattern.Pattern{
					Field:      "record_id",
					RegexValue: ptr.To(`^[A-F0-9]{8}$`),
				}
				err := pattern.ValidateWithContext(testStrictHTTPSContext())
				Expect(err).NotTo(HaveOccurred())
			})

			It("should reject regexValue for disallowed field", func() {
				pattern := &allowpattern.Pattern{
					Field:      "endpoint",
					RegexValue: ptr.To(`^https://.*$`),
				}
				err := pattern.ValidateWithContext(testStrictHTTPSContext())
				Expect(err).To(MatchError(ContainSubstring("regexValue is not allowed for field endpoint, only allowed for:")))
				Expect(err).To(MatchError(ContainSubstring("email")))
				Expect(err).To(MatchError(ContainSubstring("audience")))
				Expect(err).To(MatchError(ContainSubstring("record_id")))
				Expect(err).To(MatchError(ContainSubstring("client_id")))
			})

			It("should reject regexValue for project_id (hardcoded validation only)", func() {
				pattern := &allowpattern.Pattern{
					Field:      "project_id",
					RegexValue: ptr.To(`^[a-z][a-z0-9-]{4,28}[a-z0-9]$`),
				}
				err := pattern.ValidateWithContext(testStrictHTTPSContext())
				Expect(err).To(MatchError(ContainSubstring("regexValue is not allowed for field project_id, only allowed for:")))
				Expect(err).To(MatchError(ContainSubstring("record_id")))
				Expect(err).To(MatchError(ContainSubstring("client_id")))
				Expect(err).To(MatchError(ContainSubstring("email")))
				Expect(err).To(MatchError(ContainSubstring("audience")))
			})

			It("should reject regexValue combined with other fields", func() {
				pattern := &allowpattern.Pattern{
					Field:      "record_id",
					RegexValue: ptr.To(`^[A-F0-9]{8}$`),
					Host:       ptr.To("example.com"),
				}
				err := pattern.ValidateWithContext(testStrictHTTPSContext())
				Expect(err).To(MatchError("regexValue cannot be used together with uri, host, path, regexPath, or port for field record_id"))
			})

			It("should reject empty regexValue", func() {
				pattern := &allowpattern.Pattern{
					Field:      "record_id",
					RegexValue: ptr.To(""),
				}
				err := pattern.ValidateWithContext(testStrictHTTPSContext())
				Expect(err).To(MatchError("regexValue must not be empty for field record_id"))
			})

			It("should reject invalid regex pattern", func() {
				pattern := &allowpattern.Pattern{
					Field:      "record_id",
					RegexValue: ptr.To("[invalid"),
				}
				err := pattern.ValidateWithContext(testStrictHTTPSContext())
				Expect(err).To(MatchError(ContainSubstring("invalid regexValue pattern for field record_id")))
			})

			DescribeTable("should validate all allowed fields",
				func(field string) {
					pattern := &allowpattern.Pattern{
						Field:      field,
						RegexValue: ptr.To(`^test$`),
					}
					err := pattern.ValidateWithContext(testStrictHTTPSContext())
					Expect(err).NotTo(HaveOccurred(), "field %s should be allowed", field)
				},
				Entry("record_id", "record_id"),
				Entry("client_id", "client_id"),
				Entry("email", "email"),
			)
		})

		Context("Non-URI mode", func() {
			It("should validate a valid pattern with host and path", func() {
				pattern := &allowpattern.Pattern{
					Field: "endpoint",
					Host:  ptr.To("api.example.com"),
					Path:  ptr.To("/token"),
				}
				err := pattern.ValidateWithContext(testStrictHTTPSContext())
				Expect(err).NotTo(HaveOccurred())
			})

			It("should validate a valid pattern with host and regex path", func() {
				pattern := &allowpattern.Pattern{
					Field:     "impersonation_url",
					Host:      ptr.To("iam.example.com"),
					RegexPath: ptr.To("^/v1/projects/-/serviceAccounts/[^/:]+:generateAccessToken$"),
				}
				err := pattern.ValidateWithContext(testStrictHTTPSContext())
				Expect(err).NotTo(HaveOccurred())
			})

			It("should require host when URI is not provided", func() {
				pattern := &allowpattern.Pattern{
					Field: "endpoint",
					Path:  ptr.To("/token"),
				}
				err := pattern.ValidateWithContext(testStrictHTTPSContext())
				Expect(err).To(MatchError("host is required when uri is not provided for field endpoint"))
			})

			It("should require either path or regexPath", func() {
				pattern := &allowpattern.Pattern{
					Field: "endpoint",
					Host:  ptr.To("api.example.com"),
				}
				err := pattern.ValidateWithContext(testStrictHTTPSContext())
				Expect(err).To(MatchError("either uri must be provided, or at least one of path or regexPath must be set for field endpoint"))
			})

			It("should reject both path and regexPath", func() {
				pattern := &allowpattern.Pattern{
					Field:     "endpoint",
					Host:      ptr.To("api.example.com"),
					Path:      ptr.To("/token"),
					RegexPath: ptr.To("^/token$"),
				}
				err := pattern.ValidateWithContext(testStrictHTTPSContext())
				Expect(err).To(MatchError("path and regexPath are mutually exclusive for field endpoint"))
			})

			It("should reject empty regexPath", func() {
				pattern := &allowpattern.Pattern{
					Field:     "endpoint",
					Host:      ptr.To("api.example.com"),
					RegexPath: ptr.To(""),
				}
				err := pattern.ValidateWithContext(testStrictHTTPSContext())
				Expect(err).To(MatchError("regexPath must not be empty for field endpoint"))
			})

			It("should reject invalid regex pattern", func() {
				pattern := &allowpattern.Pattern{
					Field:     "endpoint",
					Host:      ptr.To("api.example.com"),
					RegexPath: ptr.To("[invalid"),
				}
				err := pattern.ValidateWithContext(testStrictHTTPSContext())
				Expect(err).To(MatchError(ContainSubstring("invalid regex pattern for field endpoint")))
			})

			It("should validate port range", func() {
				pattern := &allowpattern.Pattern{
					Field: "endpoint",
					Host:  ptr.To("api.example.com"),
					Path:  ptr.To("/token"),
					Port:  ptr.To(443),
				}
				err := pattern.ValidateWithContext(testStrictHTTPSContext())
				Expect(err).NotTo(HaveOccurred())
			})

			It("should reject port below valid range", func() {
				pattern := &allowpattern.Pattern{
					Field: "endpoint",
					Host:  ptr.To("api.example.com"),
					Path:  ptr.To("/token"),
					Port:  ptr.To(0),
				}
				err := pattern.ValidateWithContext(testStrictHTTPSContext())
				Expect(err).To(MatchError("invalid value for field endpoint: port must be between 1 and 65535"))
			})

			It("should reject port above valid range", func() {
				pattern := &allowpattern.Pattern{
					Field: "endpoint",
					Host:  ptr.To("api.example.com"),
					Path:  ptr.To("/token"),
					Port:  ptr.To(65536),
				}
				err := pattern.ValidateWithContext(testStrictHTTPSContext())
				Expect(err).To(MatchError("invalid value for field endpoint: port must be between 1 and 65535"))
			})

			It("should enforce HTTPS when context StrictHTTPS is true (non-URI)", func() {
				pattern := &allowpattern.Pattern{
					Field:  "endpoint",
					Scheme: ptr.To("http"),
					Host:   ptr.To("api.example.com"),
					Path:   ptr.To("/token"),
				}
				err := pattern.ValidateWithContext(testStrictHTTPSContext())
				Expect(err).To(MatchError(ContainSubstring("scheme must be one of {https}, got \"http\"")))
			})

			It("should allow HTTP when context StrictHTTPS is false (non-URI)", func() {
				ctx := &allowpattern.ValidationContext{StrictHTTPS: false}
				pattern := &allowpattern.Pattern{
					Field:  "endpoint",
					Scheme: ptr.To("http"),
					Host:   ptr.To("api.example.com"),
					Path:   ptr.To("/token"),
				}
				err := pattern.ValidateWithContext(ctx)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should reject invalid schemes when StrictHTTPS is false (non-URI)", func() {
				ctx := &allowpattern.ValidationContext{StrictHTTPS: false}
				pattern := &allowpattern.Pattern{
					Field:  "endpoint",
					Scheme: ptr.To("ftp"),
					Host:   ptr.To("api.example.com"),
					Path:   ptr.To("/token"),
				}
				err := pattern.ValidateWithContext(ctx)
				Expect(err).To(MatchError(ContainSubstring("scheme must be one of {https, http}, got \"ftp\"")))
			})
		})
	})

	Describe("ValidateWithContext", func() {
		Context("Provider-specific RegexValue validation", func() {
			It("should allow regexValue for fields specified in context", func() {
				ctx := &allowpattern.ValidationContext{
					AllowedRegexFields: map[string]bool{
						"custom_id":  true,
						"user_field": true,
					},
					StrictHTTPS: true,
				}
				pattern := &allowpattern.Pattern{
					Field:      "custom_id",
					RegexValue: ptr.To(`^[a-zA-Z0-9_-]+$`),
				}
				err := pattern.ValidateWithContext(ctx)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should reject regexValue for fields not specified in context", func() {
				ctx := &allowpattern.ValidationContext{
					AllowedRegexFields: map[string]bool{
						"allowed_field": true,
					},
					StrictHTTPS: true,
				}
				pattern := &allowpattern.Pattern{
					Field:      "disallowed_field",
					RegexValue: ptr.To(`^test$`),
				}
				err := pattern.ValidateWithContext(ctx)
				Expect(err).To(MatchError(ContainSubstring("regexValue is not allowed for field disallowed_field, only allowed for: allowed_field")))
			})

			It("should error when context is nil", func() {
				pattern := &allowpattern.Pattern{
					Field:      "any_field",
					RegexValue: ptr.To(`^test$`),
				}
				err := pattern.ValidateWithContext(nil)
				Expect(err).To(MatchError("validation context is required"))
			})

			It("should reject regexValue with empty AllowedRegexFields", func() {
				ctx := &allowpattern.ValidationContext{
					AllowedRegexFields: map[string]bool{}, // Empty map
					StrictHTTPS:        true,
				}
				pattern := &allowpattern.Pattern{
					Field:      "any_field",
					RegexValue: ptr.To(`^test$`),
				}
				err := pattern.ValidateWithContext(ctx)
				Expect(err).To(MatchError("regexValue is not allowed for field any_field"))
			})
		})

		Context("Configurable HTTPS validation", func() {
			It("should enforce HTTPS when context StrictHTTPS is true", func() {
				ctx := &allowpattern.ValidationContext{
					StrictHTTPS: true,
				}
				pattern := &allowpattern.Pattern{
					Field: "authURL",
					URI:   "http://keystone.example.com:5000/v3",
				}
				err := pattern.ValidateWithContext(ctx)
				Expect(err).To(MatchError(ContainSubstring("scheme must be one of {https}, got \"http\"")))
			})

			It("should allow HTTP when context StrictHTTPS is false", func() {
				ctx := &allowpattern.ValidationContext{
					StrictHTTPS: false,
				}
				pattern := &allowpattern.Pattern{
					Field: "authURL",
					URI:   "http://keystone.example.com:5000/v3",
				}
				err := pattern.ValidateWithContext(ctx)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should allow HTTPS when context StrictHTTPS is false", func() {
				ctx := &allowpattern.ValidationContext{
					StrictHTTPS: false,
				}
				pattern := &allowpattern.Pattern{
					Field: "authURL",
					URI:   "https://keystone.example.com:5000/v3",
				}
				err := pattern.ValidateWithContext(ctx)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should reject invalid schemes when StrictHTTPS is false", func() {
				ctx := &allowpattern.ValidationContext{
					StrictHTTPS: false,
				}
				pattern := &allowpattern.Pattern{
					Field: "authURL",
					URI:   "ftp://keystone.example.com:5000/v3",
				}
				err := pattern.ValidateWithContext(ctx)
				Expect(err).To(MatchError(ContainSubstring("scheme must be one of {https, http}, got \"ftp\"")))
			})
		})

		Context("User-configurable fields validation", func() {
			It("should allow user-provided pattern when field is in AllowedUserConfigurableFields", func() {
				ctx := &allowpattern.ValidationContext{
					AllowedUserConfigurableFields: map[string]bool{
						"allowed_user_provided_field": true,
					},
					StrictHTTPS: true,
				}
				pattern := &allowpattern.Pattern{
					Field:          "allowed_user_provided_field",
					URI:            "https://api.example.com/v1",
					IsUserProvided: true,
				}
				err := pattern.ValidateWithContext(ctx)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should reject user-provided pattern when AllowedUserConfigurableFields is empty", func() {
				ctx := &allowpattern.ValidationContext{
					AllowedUserConfigurableFields: map[string]bool{},
					StrictHTTPS:                   true,
				}
				pattern := &allowpattern.Pattern{
					Field:          "disallowed_user_provided_field",
					URI:            "https://api.example.com/v1",
					IsUserProvided: true,
				}
				err := pattern.ValidateWithContext(ctx)
				Expect(err).To(MatchError("field disallowed_user_provided_field cannot be configured by users; no user-configurable fields are allowed for this provider"))
			})

			It("should reject user-provided pattern when AllowedUserConfigurableFields is nil", func() {
				ctx := &allowpattern.ValidationContext{
					AllowedUserConfigurableFields: nil,
					StrictHTTPS:                   true,
				}
				pattern := &allowpattern.Pattern{
					Field:          "disallowed_user_provided_field",
					URI:            "https://api.example.com/v1",
					IsUserProvided: true,
				}
				err := pattern.ValidateWithContext(ctx)
				Expect(err).To(MatchError("field disallowed_user_provided_field cannot be configured by users; no user-configurable fields are allowed for this provider"))
			})

			It("should reject user-provided pattern when field is not in AllowedUserConfigurableFields", func() {
				ctx := &allowpattern.ValidationContext{
					AllowedUserConfigurableFields: map[string]bool{
						"allowed_user_provided_field": true,
					},
					StrictHTTPS: true,
				}
				pattern := &allowpattern.Pattern{
					Field:          "disallowed_user_provided_field",
					URI:            "https://api.example.com/v1",
					IsUserProvided: true,
				}
				err := pattern.ValidateWithContext(ctx)
				Expect(err).To(MatchError("field disallowed_user_provided_field cannot be configured by users"))
			})

			It("should allow non-user-provided pattern regardless of AllowedUserConfigurableFields", func() {
				ctx := &allowpattern.ValidationContext{
					AllowedUserConfigurableFields: map[string]bool{
						"allowed_user_provided_field": true,
					},
					StrictHTTPS: true,
				}
				pattern := &allowpattern.Pattern{
					Field:          "disallowed_user_provided_field",
					URI:            "https://api.example.com/v1",
					IsUserProvided: false,
				}
				err := pattern.ValidateWithContext(ctx)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should allow non-user-provided pattern when AllowedUserConfigurableFields is empty", func() {
				ctx := &allowpattern.ValidationContext{
					AllowedUserConfigurableFields: map[string]bool{},
					StrictHTTPS:                   true,
				}
				pattern := &allowpattern.Pattern{
					Field:          "disallowed_user_provided_field",
					URI:            "https://api.example.com/v1",
					IsUserProvided: false,
				}
				err := pattern.ValidateWithContext(ctx)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("ToNormalizedPattern", func() {
		It("should normalize URI to host and path", func() {
			pattern := &allowpattern.Pattern{
				Field: "endpoint",
				URI:   "https://api.example.com/token",
			}
			normalized, err := pattern.ToNormalizedPattern()
			Expect(err).NotTo(HaveOccurred())
			Expect(normalized.Host).To(Equal(ptr.To("api.example.com")))
			Expect(normalized.Path).To(Equal(ptr.To("/token")))
			Expect(normalized.URI).To(BeEmpty())
			Expect(normalized.Port).To(BeNil())
		})

		It("should normalize URI with port", func() {
			pattern := &allowpattern.Pattern{
				Field: "endpoint",
				URI:   "https://api.example.com:8443/token",
			}
			normalized, err := pattern.ToNormalizedPattern()
			Expect(err).NotTo(HaveOccurred())
			Expect(normalized.Host).To(Equal(ptr.To("api.example.com")))
			Expect(normalized.Path).To(Equal(ptr.To("/token")))
			Expect(normalized.Port).To(Equal(ptr.To(8443)))
			Expect(normalized.URI).To(BeEmpty())
		})

		It("should preserve non-URI patterns unchanged", func() {
			pattern := &allowpattern.Pattern{
				Field: "endpoint",
				Host:  ptr.To("api.example.com"),
				Path:  ptr.To("/token"),
				Port:  ptr.To(443),
			}
			normalized, err := pattern.ToNormalizedPattern()
			Expect(err).NotTo(HaveOccurred())
			Expect(normalized.Host).To(Equal(ptr.To("api.example.com")))
			Expect(normalized.Path).To(Equal(ptr.To("/token")))
			Expect(normalized.Port).To(Equal(ptr.To(443)))
			Expect(normalized.URI).To(BeEmpty())
		})

		It("should handle invalid URI", func() {
			pattern := &allowpattern.Pattern{
				Field: "endpoint",
				URI:   "://invalid-uri",
			}
			_, err := pattern.ToNormalizedPattern()
			Expect(err).To(MatchError(ContainSubstring("failed to parse URI for field endpoint")))
		})

		It("should handle invalid port in URI", func() {
			pattern := &allowpattern.Pattern{
				Field: "endpoint",
				URI:   "https://api.example.com:invalid/token",
			}
			_, err := pattern.ToNormalizedPattern()
			Expect(err).To(MatchError(ContainSubstring("failed to parse URI for field endpoint")))
		})
	})

	Describe("String", func() {
		It("should return regex format for RegexValue patterns", func() {
			pattern := &allowpattern.Pattern{
				Field:      "record_id",
				RegexValue: ptr.To(`^[A-F0-9]{8}$`),
			}
			result := pattern.String()
			Expect(result).To(Equal("regexValue:^[A-F0-9]{8}$"))
		})

		It("should return uri format for URI patterns", func() {
			pattern := &allowpattern.Pattern{
				Field: "endpoint",
				URI:   "https://api.example.com/token",
			}
			result := pattern.String()
			Expect(result).To(Equal("uri:https://api.example.com/token"))
		})

		It("should return host format for host-only patterns", func() {
			pattern := &allowpattern.Pattern{
				Field: "domain",
				Host:  ptr.To("example.com"),
			}
			result := pattern.String()
			Expect(result).To(Equal("host:example.com"))
		})

		It("should return host,path format for host with path patterns", func() {
			pattern := &allowpattern.Pattern{
				Field: "client_cert_url",
				Host:  ptr.To("www.example.com"),
				Path:  ptr.To("/certs/{email}"),
			}
			result := pattern.String()
			Expect(result).To(Equal("host:www.example.com,path:/certs/{email}"))
		})

		It("should return host,regexPath format for host with regex path patterns", func() {
			pattern := &allowpattern.Pattern{
				Field:     "impersonation_url",
				Host:      ptr.To("iam.example.com"),
				RegexPath: ptr.To("^/v1/projects/-/serviceAccounts/[^/:]+:generateAccessToken$"),
			}
			result := pattern.String()
			Expect(result).To(Equal("host:iam.example.com,regexPath:^/v1/projects/-/serviceAccounts/[^/:]+:generateAccessToken$"))
		})

		It("should include port when set", func() {
			pattern := &allowpattern.Pattern{
				Field: "endpoint",
				Host:  ptr.To("api.example.com"),
				Path:  ptr.To("/token"),
				Port:  ptr.To(8443),
			}
			result := pattern.String()
			Expect(result).To(Equal("host:api.example.com,port:8443,path:/token"))
		})

		It("should include explicit scheme when provided", func() {
			pattern := &allowpattern.Pattern{
				Field:  "endpoint",
				Scheme: ptr.To("http"),
				Host:   ptr.To("example.com"),
				Path:   ptr.To("/token"),
			}
			result := pattern.String()
			Expect(result).To(Equal("scheme:http,host:example.com,path:/token"))
		})

		It("should return unknown for empty patterns", func() {
			pattern := &allowpattern.Pattern{
				Field: "test_field",
			}
			result := pattern.String()
			Expect(result).To(Equal("unknown"))
		})

		It("should include all present fields in order", func() {
			pattern := &allowpattern.Pattern{
				Field:      "all_fields",
				RegexValue: ptr.To(`^[0-9]+$`),
				URI:        "https://login.example.com/oauth2/auth",
				Scheme:     ptr.To("http"),
				Host:       ptr.To("example.com"),
				Port:       ptr.To(8443),
				Path:       ptr.To("/path"),
				RegexPath:  ptr.To("^/v1/regexPath$"),
			}

			result := pattern.String()
			Expect(result).To(Equal("regexValue:^[0-9]+$,uri:https://login.example.com/oauth2/auth,scheme:http,host:example.com,port:8443,path:/path,regexPath:^/v1/regexPath$"))
		})

		It("should handle audience regex pattern", func() {
			pattern := &allowpattern.Pattern{
				Field:      "audience",
				RegexValue: ptr.To(`^//iam\.example\.com/projects/[0-9]+/locations/[a-z0-9-]+/pools/[a-zA-Z0-9_-]+/providers/[a-zA-Z0-9_-]+$`),
			}
			result := pattern.String()
			Expect(result).To(Equal("regexValue:^//iam\\.example\\.com/projects/[0-9]+/locations/[a-z0-9-]+/pools/[a-zA-Z0-9_-]+/providers/[a-zA-Z0-9_-]+$"))
		})

		It("should handle client_id regex pattern", func() {
			pattern := &allowpattern.Pattern{
				Field:      "client_id",
				RegexValue: ptr.To(`^[0-9]{15,25}$`),
			}
			result := pattern.String()
			Expect(result).To(Equal("regexValue:^[0-9]{15,25}$"))
		})
	})

	Describe("UnmarshalJSON", func() {
		It("should set IsUserProvided=true when unmarshaling from JSON", func() {
			jsonData := `{"field": "endpoint", "uri": "https://api.example.com/token"}`
			var pattern allowpattern.Pattern
			err := pattern.UnmarshalJSON([]byte(jsonData))
			Expect(err).NotTo(HaveOccurred())
			Expect(pattern.Field).To(Equal("endpoint"))
			Expect(pattern.URI).To(Equal("https://api.example.com/token"))
			Expect(pattern.IsUserProvided).To(BeTrue())
		})

		It("should set IsUserProvided=true even if explicitly set to false in JSON", func() {
			// IsUserProvided has json:"-" tag, so it won't be deserialized from JSON
			// but if someone tries to include it, it should be ignored and set to true
			jsonData := `{"field": "endpoint", "uri": "https://api.example.com/token", "isUserProvided": false}`
			var pattern allowpattern.Pattern
			err := pattern.UnmarshalJSON([]byte(jsonData))
			Expect(err).NotTo(HaveOccurred())
			Expect(pattern.IsUserProvided).To(BeTrue())
		})

		It("should handle complex pattern with host and path", func() {
			jsonData := `{"field": "authURL", "host": "keystone.example.com", "path": "/v3"}`
			var pattern allowpattern.Pattern
			err := pattern.UnmarshalJSON([]byte(jsonData))
			Expect(err).NotTo(HaveOccurred())
			Expect(pattern.Field).To(Equal("authURL"))
			Expect(pattern.Host).To(Equal(ptr.To("keystone.example.com")))
			Expect(pattern.Path).To(Equal(ptr.To("/v3")))
			Expect(pattern.IsUserProvided).To(BeTrue())
		})
	})

	Describe("ParseAllowedPatterns", func() {
		var valCtx *allowpattern.ValidationContext

		BeforeEach(func() {
			// Create a custom validation context that allows user-defined test fields
			valCtx = &allowpattern.ValidationContext{
				AllowedUserConfigurableFields: map[string]bool{
					"test_field":    true,
					"another_field": true,
					"json_field":    true,
					"uri_field":     true,
				},
				StrictHTTPS: true,
			}
		})

		It("should parse valid JSON patterns", func() {
			jsonPatterns := []string{
				`{"field": "test_field", "host": "example.com", "path": "/test"}`,
				`{"field": "another_field", "uri": "https://example.com/path"}`,
			}

			patterns, err := allowpattern.ParseAllowedPatterns(valCtx, jsonPatterns, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(patterns).To(HaveLen(2))
			Expect(patterns[0].Field).To(Equal("test_field"))
			Expect(patterns[0].IsUserProvided).To(BeTrue())
			Expect(patterns[1].Field).To(Equal("another_field"))
			Expect(patterns[1].IsUserProvided).To(BeTrue())
		})

		It("should parse valid URI patterns", func() {
			uriPatterns := []string{
				"test_field=https://example.com/test",
				"another_field=https://example.org/path",
			}

			patterns, err := allowpattern.ParseAllowedPatterns(valCtx, nil, uriPatterns)
			Expect(err).NotTo(HaveOccurred())
			Expect(patterns).To(HaveLen(2))
			Expect(patterns[0].Field).To(Equal("test_field"))
			Expect(patterns[0].URI).To(Equal("https://example.com/test"))
			Expect(patterns[0].IsUserProvided).To(BeTrue())
			Expect(patterns[1].Field).To(Equal("another_field"))
			Expect(patterns[1].URI).To(Equal("https://example.org/path"))
			Expect(patterns[1].IsUserProvided).To(BeTrue())
		})

		It("should parse mixed JSON and URI patterns", func() {
			jsonPatterns := []string{
				`{"field": "json_field", "host": "example.com", "path": "/test"}`,
			}
			uriPatterns := []string{
				"uri_field=https://example.org/path",
			}

			patterns, err := allowpattern.ParseAllowedPatterns(valCtx, jsonPatterns, uriPatterns)
			Expect(err).NotTo(HaveOccurred())
			Expect(patterns).To(HaveLen(2))
			for i, pattern := range patterns {
				Expect(pattern.IsUserProvided).To(BeTrue(), "pattern at index %d should have IsUserProvided=true", i)
			}
		})

		It("should fail with invalid JSON patterns", func() {
			jsonPatterns := []string{
				`{"field": "test_field", "host": "example.com"`, // Missing closing brace
			}

			_, err := allowpattern.ParseAllowedPatterns(valCtx, jsonPatterns, nil)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("could not parse JSON pattern")))
		})

		It("should fail with invalid URI patterns", func() {
			uriPatterns := []string{
				"invalid-pattern-without-equals",
			}

			_, err := allowpattern.ParseAllowedPatterns(valCtx, nil, uriPatterns)
			Expect(err).To(MatchError("invalid URI pattern: invalid-pattern-without-equals"))
		})

		It("should fail with invalid pattern validation", func() {
			jsonPatterns := []string{
				`{"field": "", "host": "example.com"}`, // Empty field
			}

			_, err := allowpattern.ParseAllowedPatterns(valCtx, jsonPatterns, nil)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("field is required")))
		})
	})
})
