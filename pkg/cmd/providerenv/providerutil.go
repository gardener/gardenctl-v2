/*
SPDX-FileCopyrightText: Contributors to the Gardener project

SPDX-License-Identifier: Apache-2.0
*/

package providerenv

import (
	"errors"
	"os"
	"path/filepath"
)

// writeOrRemoveFile writes contents to the specified path, or removes the file if unset is true.
func writeOrRemoveFile(unset bool, path string, contents []byte) error {
	if unset {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}

		return nil
	}

	return writePrivateFile(path, contents)
}

// writePrivateFile replaces path with a new file with mode 0600 containing contents.
// It never writes through an existing file or symlink at path, so the mode and owner
// of a pre-existing file cannot leak the contents.
func writePrivateFile(path string, contents []byte) (err error) {
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			_ = os.Remove(f.Name())
		}
	}()

	if _, err = f.Write(contents); err != nil {
		return errors.Join(err, f.Close())
	}

	if err = f.Close(); err != nil {
		return err
	}

	return os.Rename(f.Name(), path)
}
