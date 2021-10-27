/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package cloudenv

var (
	ValidShells = validShells
	Aliases     = aliases
)

func DetectShell(goos string) string {
	return detectShell(goos)
}
