/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package util

import (
	"strings"
	"unicode"
)

func FilterStringsByPrefix(prefix string, values []string) []string {
	if prefix == "" {
		return values
	}

	filtered := []string{}

	for _, item := range values {
		if strings.HasPrefix(item, prefix) {
			filtered = append(filtered, item)
		}
	}

	return filtered
}

// StripUnsafe remove non-printable characters from the string.
func StripUnsafe(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsPrint(r) {
			return r
		}

		return -1
	}, s)
}
