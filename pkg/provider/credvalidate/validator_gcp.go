/*
SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package credvalidate

import (
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardenctl-v2/pkg/provider/common/allowpattern"
	"github.com/gardener/gardenctl-v2/pkg/provider/common/credvalidate"
)

// ClientEmailPlaceholder is a placeholder string used in URL patterns to be replaced with the client email from the service account credentials.
const ClientEmailPlaceholder = "{client_email}"

var projectIDRegexp = regexp.MustCompile(`^(?P<project>[a-z][a-z0-9-]{4,28}[a-z0-9])$`)

// GCPValidator implements the common Validator interface for GCP.
type GCPValidator struct {
	*credvalidate.BaseValidator
}

var _ credvalidate.Validator = &GCPValidator{}

// NewGCPValidator creates a new GCP validator with the provided context and allowed patterns.
func NewGCPValidator(ctx context.Context, allowedPatterns []allowpattern.Pattern) *GCPValidator {
	return &GCPValidator{
		BaseValidator: credvalidate.NewBaseValidator(ctx, allowedPatterns),
	}
}

// ValidateSecret validates GCP credentials from a Kubernetes secret.
func (v *GCPValidator) ValidateSecret(secret *corev1.Secret) (map[string]interface{}, error) {
	fields := credvalidate.FlatCoerceBytesToStringsMap(secret.Data)

	registry := map[string]credvalidate.FieldRule{
		"serviceaccount.json": {Required: true, Validator: validateServiceAccountJSON, NonSensitive: false},
	}

	return v.ValidateWithRegistry(fields, registry, credvalidate.Permissive)
}

// GetGCPValidationContext returns the validation context for GCP patterns.
func GetGCPValidationContext() *allowpattern.ValidationContext {
	return &allowpattern.ValidationContext{
		AllowedRegexFields: map[string]bool{
			"private_key_id": true,
			"client_id":      true,
			"client_email":   true,
		},
		StrictHTTPS: true,
	}
}

// DefaultGCPAllowedPatterns returns the default allowed patterns for GCP credential fields.
func DefaultGCPAllowedPatterns() []allowpattern.Pattern {
	return []allowpattern.Pattern{
		// Value regex patterns for fields that support regex validation
		{
			Field:      "private_key_id",
			RegexValue: ptr.To(`^[a-fA-F0-9]{40}$`),
		},
		{
			Field:      "client_id",
			RegexValue: ptr.To(`^[0-9]{15,25}$`),
		},
		{
			Field:      "client_email",
			RegexValue: ptr.To(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.iam\.gserviceaccount\.com$`),
		},
		{
			Field:      "client_email",
			RegexValue: ptr.To(`^[0-9]+-compute@developer\.gserviceaccount\.com$`),
		},
		// URI and domain patterns for other fields
		{
			Field: "universe_domain",
			Host:  ptr.To("googleapis.com"),
			Path:  ptr.To(""),
		},
		{
			Field: "token_uri",
			URI:   "https://accounts.google.com/o/oauth2/token",
		},
		{
			Field: "token_uri",
			URI:   "https://oauth2.googleapis.com/token",
		},
		{
			Field: "auth_uri",
			URI:   "https://accounts.google.com/o/oauth2/auth",
		},
		{
			Field: "auth_provider_x509_cert_url",
			URI:   "https://www.googleapis.com/oauth2/v1/certs",
		},
		{
			Field: "client_x509_cert_url",
			Host:  ptr.To("www.googleapis.com"),
			Path:  ptr.To("/robot/v1/metadata/x509/" + ClientEmailPlaceholder),
		},
	}
}

// Field-specific validator functions that implement the credvalidate.FieldValidator interface.

// validateServiceAccountJSON validates the nested serviceaccount.json field for service account credentials.
// This is a nested sub-validator that handles the service account JSON content fields.
func validateServiceAccountJSON(baseValidator *credvalidate.BaseValidator, field string, val any, allFields map[string]any, nonSensitive bool) error {
	// The serviceaccount.json should be a string containing JSON content
	jsonData, ok := val.(string)
	if !ok {
		return credvalidate.NewFieldError(field, "field value must be a string", nil, nonSensitive)
	}

	fields := make(map[string]interface{})
	if err := json.Unmarshal([]byte(jsonData), &fields); err != nil {
		return fmt.Errorf("failed to unmarshal service account JSON: %w", err)
	}

	serviceAccountRegistry := map[string]credvalidate.FieldRule{
		"type":                        {Required: true, Validator: validateServiceAccountType, NonSensitive: true},
		"project_id":                  {Required: true, Validator: validateProjectID, NonSensitive: true},
		"private_key_id":              {Required: false, Validator: credvalidate.ValidateStringWithPattern(credvalidate.MatchRegexValuePattern), NonSensitive: true},
		"private_key":                 {Required: false, Validator: validatePrivateKey, NonSensitive: false},
		"client_email":                {Required: false, Validator: credvalidate.ValidateStringWithPattern(credvalidate.MatchRegexValuePattern), NonSensitive: false},
		"client_id":                   {Required: false, Validator: credvalidate.ValidateStringWithPattern(credvalidate.MatchRegexValuePattern), NonSensitive: false},
		"auth_uri":                    {Required: false, Validator: credvalidate.ValidateStringWithPattern(credvalidate.MatchURIPattern), NonSensitive: true},
		"token_uri":                   {Required: false, Validator: credvalidate.ValidateStringWithPattern(credvalidate.MatchURIPattern), NonSensitive: true},
		"auth_provider_x509_cert_url": {Required: false, Validator: credvalidate.ValidateStringWithPattern(credvalidate.MatchURIPattern), NonSensitive: true},
		"client_x509_cert_url":        {Required: false, Validator: credvalidate.ValidateStringWithPattern(matchClientX509CertURLPattern), NonSensitive: true},
		"universe_domain":             {Required: false, Validator: credvalidate.ValidateStringWithPattern(matchDomainPattern), NonSensitive: true},
	}

	// Validate the nested service account JSON using the base validator
	return baseValidator.ValidateNestedFieldsStrict(fields, serviceAccountRegistry)
}

var _ credvalidate.FieldValidator = validateServiceAccountJSON

// validateServiceAccountType validates the type field for service accounts.
func validateServiceAccountType(v *credvalidate.BaseValidator, field string, val any, allFields map[string]any, nonSensitive bool) error {
	str, ok := val.(string)
	if !ok {
		return credvalidate.NewFieldError(field, "field value must be a string", nil, nonSensitive)
	}

	if str != "service_account" {
		return credvalidate.NewFieldErrorWithValue(field, "type must be 'service_account'", str, nil, nonSensitive)
	}

	return nil
}

var _ credvalidate.FieldValidator = validateServiceAccountType

// validateProjectID validates the project_id field format.
func validateProjectID(v *credvalidate.BaseValidator, field string, val any, allFields map[string]any, nonSensitive bool) error {
	str, ok := val.(string)

	if !ok {
		return credvalidate.NewFieldError(field, "field value must be a string", nil, nonSensitive)
	}

	if !projectIDRegexp.MatchString(str) {
		return credvalidate.NewFieldErrorWithValue(field, "field does not match the expected format", str, nil, nonSensitive)
	}

	return nil
}

var _ credvalidate.FieldValidator = validateProjectID

// validatePrivateKey ensures the service account JSON "private_key":
// - is exactly one PEM block (no extra data before/after)
// - is PKCS#8 ("BEGIN PRIVATE KEY") with no headers
// - parses as RSA and passes rsa.PrivateKey.Validate().
func validatePrivateKey(_ *credvalidate.BaseValidator, field string, val any, _ map[string]any, nonSensitive bool) error {
	s, ok := val.(string)
	if !ok {
		return credvalidate.NewFieldError(field, "field value must be a string", nil, nonSensitive)
	}

	// No leading junk: must start with a PEM BEGIN line.
	if !strings.HasPrefix(s, "-----BEGIN ") {
		return credvalidate.NewFieldError(field, "field value must start with a PEM BEGIN line", nil, nonSensitive)
	}

	block, rest := pem.Decode([]byte(s))
	if block == nil {
		return credvalidate.NewFieldError(field, "field value must be a valid PEM-encoded private key", nil, nonSensitive)
	}

	// No trailing junk after the END line.
	if len(bytes.TrimSpace(rest)) != 0 {
		return credvalidate.NewFieldError(field, "field value must contain exactly one PEM block (unexpected data after END line)", nil, nonSensitive)
	}

	if block.Type != "PRIVATE KEY" {
		return credvalidate.NewFieldError(field, "field value must be a PKCS#8 PEM block (BEGIN PRIVATE KEY)", nil, nonSensitive)
	}

	if len(block.Headers) != 0 {
		return credvalidate.NewFieldError(field, "field value must not include PEM headers", nil, nonSensitive)
	}

	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return credvalidate.NewFieldError(field, "field value cannot be parsed", err, nonSensitive)
	}

	rsaKey, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return credvalidate.NewFieldError(field, "field value must be an RSA private key", nil, nonSensitive)
	}

	if err := rsaKey.Validate(); err != nil {
		return credvalidate.NewFieldError(field, "RSA key validation failed", err, nonSensitive)
	}

	return nil
}

var _ credvalidate.FieldValidator = validatePrivateKey

// Pattern matcher functions that implement the credvalidate.PatternMatcher interface.

// matchDomainPattern validates a domain field against a normalized pattern.
func matchDomainPattern(domain string, pattern allowpattern.Pattern, credentials map[string]interface{}, nonSensitive bool) error {
	field := pattern.Field

	// Pattern sanity: domain-type fields must not specify URI/path/port, except Path may be an empty string to represent no path.
	if pattern.URI != "" || (pattern.Path != nil && *pattern.Path != "") || pattern.RegexPath != nil || pattern.Port != nil {
		return credvalidate.NewFieldError(field, "domain patterns must not specify URI, path, or port", nil, nonSensitive)
	}

	if pattern.Host == nil || *pattern.Host == "" {
		return credvalidate.NewFieldError(field, "allowed domain (Host) must be set", nil, nonSensitive)
	}

	// For universe_domain, we expect just the domain name, not a full URI
	if domain != *pattern.Host {
		return credvalidate.NewPatternMismatchErrorWithValues(pattern.Field, "domain does not match allowed domain", domain, *pattern.Host, nonSensitive)
	}

	return nil
}

var _ credvalidate.PatternMatcher = matchDomainPattern

// expandClientEmailPlaceholder returns the pattern value with a single
// ClientEmailPlaceholder ("{client_email}") occurrence in Path expanded,
// if present. RegexPath is never modified.
// The email is encoded using url.PathEscape for safe path-segment substitution.
func expandClientEmailPlaceholder(p allowpattern.Pattern, credentials map[string]interface{}) (allowpattern.Pattern, error) {
	if p.Path == nil || !strings.Contains(*p.Path, ClientEmailPlaceholder) {
		// Nothing to do
		return p, nil
	}

	email, ok := credentials["client_email"].(string)
	if !ok {
		return p, credvalidate.NewPatternMismatchError(p.Field, fmt.Sprintf("client_email required for pattern with %s", ClientEmailPlaceholder))
	}

	occ := strings.Count(*p.Path, ClientEmailPlaceholder)
	if occ > 1 {
		return p, credvalidate.NewFieldError(p.Field, fmt.Sprintf("invalid pattern: multiple %s placeholder occurrences in Path", ClientEmailPlaceholder), nil, true)
	}

	encoded := url.PathEscape(email)
	patched := strings.Replace(*p.Path, ClientEmailPlaceholder, encoded, 1)
	// p is by value; reassigning pointer fields is local to this call
	p.Path = &patched

	return p, nil
}

// matchClientX509CertURLPattern validates uri against pattern for a
// ClientX509CertURL field. If Pattern.Path contains ClientEmailPlaceholder,
// it is expanded using credentials["client_email"] (path-segment encoded)
// before delegating to MatchURIPattern. RegexPath is not subject to placeholder
// expansion. Host, port, and scheme checks follow MatchURIPattern semantics.
func matchClientX509CertURLPattern(uri string, pattern allowpattern.Pattern, credentials map[string]interface{}, nonSensitive bool) error {
	patched, err := expandClientEmailPlaceholder(pattern, credentials)
	if err != nil {
		return err
	}

	return credvalidate.MatchURIPattern(uri, patched, credentials, nonSensitive)
}

var _ credvalidate.PatternMatcher = matchClientX509CertURLPattern
