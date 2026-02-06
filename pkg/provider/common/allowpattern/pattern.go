/*
SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package allowpattern

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// ValidationContext holds provider-specific validation rules that are not exposed in configuration.
type ValidationContext struct {
	// AllowedRegexFields specifies which fields are allowed to use RegexValue validation
	AllowedRegexFields map[string]bool
	// StrictHTTPS controls the default HTTPS behavior for this provider context
	StrictHTTPS bool
	// AllowedUserConfigurableFields specifies which fields can be configured by users (via config or flags).
	// If nil or empty, no user-provided patterns are allowed. If set, only fields in this list can be configured by users.
	// Default patterns (IsUserProvided=false) are not subject to this restriction.
	AllowedUserConfigurableFields map[string]bool
}

// Pattern represents a pattern for validating credential fields across different cloud providers.
type Pattern struct {
	// Field is the name of the credential field to validate (e.g., "universe_domain", "token_uri")
	Field string `json:"field"`
	// Host is the allowed hostname for the field.
	// Optional when using regexValue, or when using URI. Required when not using URI or regexValue.
	Host *string `json:"host,omitempty"`
	// Port is the allowed port for the field (optional; if set, requires exact match)
	Port *int `json:"port,omitempty"`
	// Path is the allowed path for the field. Use RegexPath for pattern matching. Placeholders such as {client_email} are substituted before comparison.
	Path *string `json:"path,omitempty"`
	// URI is an alternative to Host+Path for simplicity
	URI string `json:"uri,omitempty"`
	// RegexPath is a regex pattern for path validation (mutually exclusive with Path)
	RegexPath *string `json:"regexPath,omitempty"`
	// RegexValue is a regex pattern for validating the field value itself (not URI components)
	// This is only allowed if the field is explicitly configured in the ValidationContext.AllowedRegexFields
	RegexValue *string `json:"regexValue,omitempty"`
	// Scheme is the allowed scheme for this pattern when URI is not provided (defaults to https).
	// Valid values: "https" or "http". When URI is provided, the scheme is derived from the URI.
	Scheme *string `json:"scheme,omitempty"`
	// IsUserProvided indicates whether this pattern comes from user configuration (config file or flags).
	// When true, the pattern field must be explicitly listed in AllowedUserConfigurableFields.
	// This field is not serialized to/from JSON.
	IsUserProvided bool `json:"-"`
}

// ParseAllowedPatterns parses the allowed patterns from JSON and URI formats.
// All patterns parsed by this function are marked as user-provided (IsUserProvided=true)
// since they come from command-line flags.
func ParseAllowedPatterns(ctx *ValidationContext, jsonPatterns, uriPatterns []string) ([]Pattern, error) {
	var patterns []Pattern

	// Parse JSON patterns
	for _, pattern := range jsonPatterns {
		var p Pattern
		if err := json.Unmarshal([]byte(pattern), &p); err != nil {
			return nil, fmt.Errorf("could not parse JSON pattern %s: %w", pattern, err)
		}

		p.IsUserProvided = true

		if err := p.ValidateWithContext(ctx); err != nil {
			return nil, fmt.Errorf("validation failed for JSON pattern %s: %w", pattern, err)
		}

		patterns = append(patterns, p)
	}

	// Parse URI patterns
	for _, pattern := range uriPatterns {
		parts := strings.SplitN(pattern, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid URI pattern: %s", pattern)
		}

		field, uri := parts[0], parts[1]

		p := Pattern{
			Field:          field,
			URI:            uri,
			IsUserProvided: true,
		}
		if err := p.ValidateWithContext(ctx); err != nil {
			return nil, fmt.Errorf("invalid URI pattern %s: %w", pattern, err)
		}

		patterns = append(patterns, p)
	}

	return patterns, nil
}

// ValidateWithContext validates the Pattern configuration with provider-specific context.
func (p *Pattern) ValidateWithContext(ctx *ValidationContext) error {
	if ctx == nil {
		return fmt.Errorf("validation context is required")
	}

	if p.Field == "" {
		return fmt.Errorf("field is required")
	}

	if p.IsUserProvided {
		if len(ctx.AllowedUserConfigurableFields) == 0 {
			return fmt.Errorf("field %s cannot be configured by users; no user-configurable fields are allowed for this provider", p.Field)
		}

		if !ctx.AllowedUserConfigurableFields[p.Field] {
			return fmt.Errorf("field %s cannot be configured by users", p.Field)
		}
	}

	// --- RegexValue mode: RegexValue must stand alone
	if p.RegexValue != nil {
		// Check against context-provided allowed fields
		if len(ctx.AllowedRegexFields) == 0 {
			return fmt.Errorf("regexValue is not allowed for field %s", p.Field)
		}

		if _, ok := ctx.AllowedRegexFields[p.Field]; !ok {
			allowedList := make([]string, 0, len(ctx.AllowedRegexFields))
			for field := range ctx.AllowedRegexFields {
				allowedList = append(allowedList, field)
			}

			return fmt.Errorf("regexValue is not allowed for field %s, only allowed for: %s", p.Field, strings.Join(allowedList, ", "))
		}

		if p.URI != "" || p.Host != nil || p.Path != nil || p.RegexPath != nil || p.Port != nil {
			return fmt.Errorf("regexValue cannot be used together with uri, host, path, regexPath, or port for field %s", p.Field)
		}

		if *p.RegexValue == "" {
			return fmt.Errorf("regexValue must not be empty for field %s", p.Field)
		}

		if _, err := regexp.Compile(*p.RegexValue); err != nil {
			return fmt.Errorf("invalid regexValue pattern for field %s: %w", p.Field, err)
		}

		return nil
	}

	// --- URI mode: URI must stand alone
	if p.URI != "" {
		if p.Host != nil || p.Path != nil || p.RegexPath != nil || p.Port != nil {
			return fmt.Errorf("uri cannot be used together with host, path, regexPath, or port for field %s", p.Field)
		}

		// Validate the endpoint URL against scheme/host/port and URL hygiene rules
		if _, err := ParseAndValidateEndpointURL(p.URI, ctx.StrictHTTPS); err != nil {
			return fmt.Errorf("invalid value for field %s: %w", p.Field, err)
		}

		return nil
	}

	// --- Non-URI mode
	if p.Host == nil || *p.Host == "" {
		return fmt.Errorf("host is required when uri is not provided for field %s", p.Field)
	}

	scheme := "https"
	if p.Scheme != nil && *p.Scheme != "" {
		scheme = *p.Scheme
	}

	if err := validateSchemeHostPort(scheme, *p.Host, p.Port, ctx.StrictHTTPS); err != nil {
		return fmt.Errorf("invalid value for field %s: %w", p.Field, err)
	}

	if p.Path == nil && p.RegexPath == nil {
		return fmt.Errorf("either uri must be provided, or at least one of path or regexPath must be set for field %s", p.Field)
	}

	if p.Path != nil && p.RegexPath != nil {
		return fmt.Errorf("path and regexPath are mutually exclusive for field %s", p.Field)
	}

	if p.RegexPath != nil {
		if *p.RegexPath == "" {
			return fmt.Errorf("regexPath must not be empty for field %s", p.Field)
		}

		if _, err := regexp.Compile(*p.RegexPath); err != nil {
			return fmt.Errorf("invalid regex pattern for field %s: %w", p.Field, err)
		}
	}

	return nil
}

// ParseAndValidateEndpointURL parses and validates a URL string against allowed
// scheme/host/port and URL hygiene rules with configurable HTTPS strictness.
// Returns the parsed URL on success for callers that need components.
func ParseAndValidateEndpointURL(rawURL string, strictHTTPS bool) (*url.URL, error) {
	parsedURI, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URI")
	}

	if err := validateSchemeHostPort(parsedURI.Scheme, parsedURI.Hostname(), parseOptionalPort(parsedURI.Port()), strictHTTPS); err != nil {
		return nil, err
	}
	// Disallow userinfo (username:password)
	if parsedURI.User != nil {
		return nil, fmt.Errorf("must not contain userinfo")
	}
	// Disallow query parameters
	if parsedURI.RawQuery != "" {
		return nil, fmt.Errorf("must not contain query parameters")
	}
	// Disallow fragments
	if parsedURI.Fragment != "" {
		return nil, fmt.Errorf("must not contain fragments")
	}

	return parsedURI, nil
}

// validateSchemeHostPort validates scheme, host and optional port according to strictHTTPS and format rules.
func validateSchemeHostPort(scheme string, host string, port *int, strictHTTPS bool) error {
	allowedSchemes := []string{"https"}
	if !strictHTTPS {
		allowedSchemes = append(allowedSchemes, "http")
	}

	if !slices.Contains(allowedSchemes, scheme) {
		return fmt.Errorf("scheme must be one of {%s}, got %q", strings.Join(allowedSchemes, ", "), scheme)
	}

	if host == "" {
		return fmt.Errorf("hostname is required")
	}

	if port != nil {
		if *port < 1 || *port > 65535 {
			return fmt.Errorf("port must be between 1 and 65535")
		}
	}

	return nil
}

// parseOptionalPort converts a port string to *int; returns nil when empty or invalid.
func parseOptionalPort(portStr string) *int {
	if portStr == "" {
		return nil
	}

	if port, err := strconv.Atoi(portStr); err == nil {
		return &port
	}

	return nil
}

// UnmarshalJSON implements custom JSON unmarshaling for Pattern.
// It sets IsUserProvided to true for patterns loaded from configuration files.
func (p *Pattern) UnmarshalJSON(data []byte) error {
	// Define a temporary type to avoid recursion
	type PatternAlias Pattern

	aux := &PatternAlias{}

	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	*p = Pattern(*aux)
	// Mark as user-provided since it comes from config file
	p.IsUserProvided = true

	return nil
}

// String returns a string representation of the pattern type for logging and debugging purposes.
// This implements the fmt.Stringer interface.
func (p *Pattern) String() string {
	var parts []string

	if p.RegexValue != nil {
		parts = append(parts, fmt.Sprintf("regexValue:%s", *p.RegexValue))
	}

	if p.URI != "" {
		parts = append(parts, fmt.Sprintf("uri:%s", p.URI))
	}

	if p.Scheme != nil {
		parts = append(parts, fmt.Sprintf("scheme:%s", *p.Scheme))
	}

	if p.Host != nil {
		parts = append(parts, fmt.Sprintf("host:%s", *p.Host))
	}

	if p.Port != nil {
		parts = append(parts, fmt.Sprintf("port:%d", *p.Port))
	}

	if p.Path != nil {
		parts = append(parts, fmt.Sprintf("path:%s", *p.Path))
	}

	if p.RegexPath != nil {
		parts = append(parts, fmt.Sprintf("regexPath:%s", *p.RegexPath))
	}

	if len(parts) == 0 {
		return "unknown"
	}

	return strings.Join(parts, ",")
}
