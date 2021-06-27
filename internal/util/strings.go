/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package util

import "strings"

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
