//go:build !unix

/*
SPDX-FileCopyrightText: Contributors to the Gardener project

SPDX-License-Identifier: Apache-2.0
*/

package util

import "io/fs"

// On Windows the temporary directory is per user and protected by ACLs. Go synthesizes the
// permission bits from file attributes, so they cannot be used to verify privacy.
func checkPrivate(string, fs.FileInfo) error {
	return nil
}

func checkTrustedParents(string) error {
	return nil
}
