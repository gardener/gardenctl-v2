/*
SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package allowpattern

import (
	"fmt"
	"net/url"
	"strconv"

	"k8s.io/utils/ptr"
)

// ToNormalizedPattern returns a copy of the pattern with URI fields parsed into their components (scheme, host, port, path), and the URI field cleared.
func (p *Pattern) ToNormalizedPattern() (*Pattern, error) {
	normalized := *p

	// If URI is provided, parse it to extract host and path
	if p.URI != "" {
		parsedURI, err := url.Parse(p.URI)
		if err != nil {
			return nil, fmt.Errorf("failed to parse URI for field %s: %w", p.Field, err)
		}

		normalized.Scheme = ptr.To(parsedURI.Scheme)

		normalized.Host = ptr.To(parsedURI.Hostname())
		// Set allowed port if explicitly provided in the URI
		if portStr := parsedURI.Port(); portStr != "" {
			if portInt, err := strconv.Atoi(portStr); err == nil {
				normalized.Port = &portInt
			} else {
				return nil, fmt.Errorf("failed to parse port for field %s: %w", p.Field, err)
			}
		} else {
			normalized.Port = nil
		}

		normalized.Path = ptr.To(parsedURI.Path)
		// Clear URI after normalization
		normalized.URI = ""
	}

	return &normalized, nil
}
