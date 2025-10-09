/*
SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package credvalidate

import (
	"fmt"
	"regexp"

	"github.com/gardener/gardenctl-v2/pkg/provider/common/allowpattern"
)

// Pattern matcher functions that implement the credvalidate.PatternMatcher interface.

// MatchRegexValuePattern validates a field value directly against a regex pattern.
func MatchRegexValuePattern(value string, pattern allowpattern.Pattern, credentials map[string]interface{}, nonSensitive bool) error {
	if pattern.RegexValue == nil {
		return NewFieldError(pattern.Field, "pattern does not have regexValue set", nil, nonSensitive)
	}

	matched, err := regexp.MatchString(*pattern.RegexValue, value)
	if err != nil {
		return NewFieldError(pattern.Field, "invalid regex pattern", err, nonSensitive)
	}

	if !matched {
		return NewPatternMismatchErrorWithValues(pattern.Field, "does not match regex pattern", value, *pattern.RegexValue, nonSensitive)
	}

	return nil
}

var _ PatternMatcher = MatchRegexValuePattern

// MatchURIPattern validates a URI field against a normalized pattern.
func MatchURIPattern(uri string, pattern allowpattern.Pattern, credentials map[string]interface{}, nonSensitive bool) error {
	field := pattern.Field

	// URL hygiene checks and generic scheme allowlist (https/http)
	// Do not enforce exact scheme here to allow treating scheme mismatch as a pattern mismatch.
	parsedURI, err := allowpattern.ParseAndValidateEndpointURL(uri, false /* allow https and http */)
	if err != nil {
		return NewFieldError(field, "failed to validate URI", err, nonSensitive)
	}

	// Enforce exact scheme equality against normalized pattern's Scheme to classify mismatches correctly
	expectedScheme := "https"
	if pattern.Scheme != nil && *pattern.Scheme != "" {
		expectedScheme = *pattern.Scheme
	}

	if parsedURI.Scheme != expectedScheme {
		return NewPatternMismatchErrorWithValues(field, "scheme does not match allowed scheme", parsedURI.Scheme, expectedScheme, nonSensitive)
	}

	if pattern.Host == nil {
		// Should not be reached: patterns must be validated/normalized before matching
		return NewFieldError(field, "pattern does not specify an allowed host", nil, nonSensitive)
	}

	if parsedURI.Hostname() != *pattern.Host {
		return NewPatternMismatchErrorWithValues(field, "host does not match allowed host", parsedURI.Hostname(), *pattern.Host, nonSensitive)
	}

	port := parsedURI.Port()
	if pattern.Port != nil {
		if port != fmt.Sprintf("%d", *pattern.Port) {
			return NewPatternMismatchErrorWithValues(field, "port does not match allowed port", port, fmt.Sprintf("%d", *pattern.Port), nonSensitive)
		}
	} else {
		if port != "" {
			return NewPatternMismatchErrorWithValues(field, "port does not match allowed port", port, "(none)", nonSensitive)
		}
	}

	p := parsedURI.Path
	if pattern.Path != nil {
		if p != *pattern.Path {
			return NewPatternMismatchErrorWithValues(field, "path does not match allowed path", p, *pattern.Path, nonSensitive)
		}
	} else if pattern.RegexPath != nil {
		matched, err := regexp.MatchString(*pattern.RegexPath, p)
		if err != nil {
			return NewFieldError(field, "invalid regex pattern", err, nonSensitive)
		}

		if !matched {
			return NewPatternMismatchErrorWithValues(field, "path does not match regex pattern", p, *pattern.RegexPath, nonSensitive)
		}
	}

	return nil
}

var _ PatternMatcher = MatchURIPattern
