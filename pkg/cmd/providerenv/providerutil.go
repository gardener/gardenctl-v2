/*
SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package providerenv

import "os"

// writeOrRemoveFile writes contents to the specified path, or removes the file if unset is true.
func writeOrRemoveFile(unset bool, path string, contents []byte) error {
	if unset {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}

		return nil
	}

	return os.WriteFile(path, contents, 0o600)
}
