/*
SPDX-FileCopyrightText: Contributors to the Gardener project

SPDX-License-Identifier: Apache-2.0
*/

package util

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// EnsurePrivateDir creates dir with mode 0700 if it does not exist and verifies that it can be trusted and used:
// it must be a directory (not a symlink), and on Unix it must be owned by the current user, must not be
// accessible by group or others, must be readable, writable and searchable by the owner, and all of its
// parent directories must be protected against other users.
// An existing directory that fails these checks is never modified; an error is returned instead.
//
// It returns the verified absolute path with all symlinks in the parent resolved. Callers must use the
// returned path, because the original path may traverse symlinks that could be changed after the check.
func EnsurePrivateDir(dir string) (string, error) {
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("failed to determine absolute path of %s: %w", dir, err)
	}

	parent, err := filepath.EvalSymlinks(filepath.Dir(absolute))
	if err != nil {
		return "", fmt.Errorf("failed to resolve directory %s: %w", filepath.Dir(absolute), err)
	}

	if err := checkTrustedParents(parent); err != nil {
		return "", err
	}

	dir = filepath.Join(parent, filepath.Base(absolute))

	if err := os.Mkdir(dir, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
		return "", fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	info, err := os.Lstat(dir)
	if err != nil {
		return "", fmt.Errorf("failed to stat directory %s: %w", dir, err)
	}

	if info.Mode().Type() != fs.ModeDir {
		return "", untrustedDirError(dir, "it is not a directory")
	}

	if err := checkPrivate(dir, info); err != nil {
		return "", err
	}

	return dir, nil
}

func untrustedDirError(dir, reason string) error {
	return fmt.Errorf("refusing to use %s: %s. It may have been created by another user. Remove it (or ask an administrator to remove it), or set TMPDIR to a directory that only you can write to, and try again", dir, reason)
}

func untrustedParentError(dir, reason string) error {
	return fmt.Errorf("refusing to use temporary directory below %s: %s. Set TMPDIR to a directory that only you can write to", dir, reason)
}
