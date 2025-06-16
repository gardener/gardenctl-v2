/*
SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package providerenv

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// EncodedClientEmailPlaceholder is a placeholder string used in URL patterns to be replaced with the URL-encoded client email from the service account credentials.
const EncodedClientEmailPlaceholder = "{encoded_client_email}"

// defaultAllowedPatterns returns the default allowed patterns for GCP service account fields.
func defaultAllowedPatterns() map[string][]string {
	return map[string][]string{
		"universe_domain": {
			"googleapis.com",
		},
		"token_uri": {
			"https://accounts.google.com/o/oauth2/token",
			"https://oauth2.googleapis.com/token",
		},
		"auth_uri": {
			"https://accounts.google.com/o/oauth2/auth",
		},
		"auth_provider_x509_cert_url": {
			"https://www.googleapis.com/oauth2/v1/certs",
		},
		"client_x509_cert_url": {
			"https://www.googleapis.com/robot/v1/metadata/x509/" + EncodedClientEmailPlaceholder,
		},
	}
}

// parseAllowedPatterns parses the allowed patterns into a map of field to allowed values.
func parseAllowedPatterns(allowedPatterns []string) (map[string][]string, error) {
	patterns := make(map[string][]string)

	for _, pattern := range allowedPatterns {
		parts := strings.SplitN(pattern, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid pattern: %s", pattern)
		}

		field, value := parts[0], parts[1]
		patterns[field] = append(patterns[field], value)
	}

	return patterns, nil
}

// validateAndParseGCPServiceAccount parses and validates a GCP service account JSON from a Kubernetes secret.
// It populates the provided credentials map and returns the original JSON bytes.
func validateAndParseGCPServiceAccount(secret *corev1.Secret, credentials *map[string]interface{}, allowedPatterns map[string][]string) ([]byte, error) {
	serviceaccountJSON, ok := secret.Data["serviceaccount.json"]
	if !ok || serviceaccountJSON == nil {
		return nil, fmt.Errorf("no \"serviceaccount.json\" data in Secret %q", secret.Name)
	}

	err := json.Unmarshal(serviceaccountJSON, credentials)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal service account JSON: %w", err)
	}

	for key, value := range *credentials {
		if _, ok := value.(string); !ok {
			return nil, fmt.Errorf("field %s is not a string", key)
		}
	}

	typ, exists := (*credentials)["type"]
	if !exists {
		return nil, errors.New("type field is missing")
	}

	if typ.(string) != "service_account" {
		return nil, errors.New("type must be 'service_account'")
	}

	allowedFields := map[string]bool{
		"type":                        true,
		"project_id":                  true,
		"private_key_id":              true,
		"private_key":                 true,
		"client_email":                true,
		"client_id":                   true,
		"auth_uri":                    true,
		"token_uri":                   true,
		"auth_provider_x509_cert_url": true,
		"client_x509_cert_url":        true,
		"universe_domain":             true,
	}
	for key := range *credentials {
		if !allowedFields[key] {
			return nil, fmt.Errorf("disallowed field found: %s", key)
		}
	}

	if universeDomain, exists := (*credentials)["universe_domain"]; exists {
		universeDomainStr := universeDomain.(string)
		if !slices.Contains(allowedPatterns["universe_domain"], universeDomainStr) {
			return nil, fmt.Errorf("untrusted universe_domain: %s", universeDomainStr)
		}
	}

	uriFields := []string{
		"auth_uri",
		"token_uri",
		"auth_provider_x509_cert_url",
		"client_x509_cert_url",
	}
	for _, field := range uriFields {
		val, exists := (*credentials)[field]
		if !exists {
			continue
		}

		uri := val.(string)
		if err := validateURI(field, uri, allowedPatterns[field], *credentials); err != nil {
			return nil, err
		}
	}

	return json.Marshal(credentials)
}

func validateURI(field, uri string, patterns []string, credentials map[string]interface{}) error {
	u, err := url.Parse(uri)
	if err != nil {
		return fmt.Errorf("invalid URI in %s: %w", field, err)
	}

	if u.Scheme != "https" {
		return fmt.Errorf("URI in %s must use https scheme", field)
	}

	for _, pattern := range patterns {
		parsedPattern, err := url.Parse(pattern)
		if err != nil {
			return fmt.Errorf("invalid pattern for %s: %w", field, err)
		}

		expectedURI := pattern

		if field == "client_x509_cert_url" && strings.Contains(pattern, EncodedClientEmailPlaceholder) {
			clientEmail, ok := credentials["client_email"].(string)
			if !ok {
				return fmt.Errorf("client_email is missing or not a string for %s with pattern requiring %s", field, EncodedClientEmailPlaceholder)
			}

			originalHost := parsedPattern.Host
			encodedClientEmail := url.QueryEscape(clientEmail)
			expectedURI = strings.Replace(pattern, EncodedClientEmailPlaceholder, encodedClientEmail, 1)

			parsedExpected, err := url.Parse(expectedURI)
			if err != nil {
				return fmt.Errorf("invalid URI after placeholder replacement for %s: %w", field, err)
			}

			// Ensure hostname didn't change after replacing {encoded_client_email}.
			if parsedExpected.Host != originalHost {
				return fmt.Errorf("unexpected hostname change from %s to %s for %s after placeholder replacement", originalHost, parsedExpected.Host, field)
			}
		}

		if uri == expectedURI {
			return nil
		}
	}

	return fmt.Errorf("URI for %s (%s) does not match any allowed patterns", field, uri)
}
