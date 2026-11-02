//go:build unix

/*
SPDX-FileCopyrightText: Contributors to the Gardener project

SPDX-License-Identifier: Apache-2.0
*/

package util

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

func checkPrivate(dir string, info fs.FileInfo) error {
	uid, err := fileOwner(dir, info)
	if err != nil {
		return err
	}

	if uid != os.Geteuid() {
		return untrustedDirError(dir, fmt.Sprintf("it is owned by uid %d instead of the current user (uid %d)", uid, os.Geteuid()))
	}

	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		return untrustedDirError(dir, fmt.Sprintf("it has mode %#o, but must not be accessible by group or others", perm))
	}

	return nil
}

// checkTrustedParents verifies that no other user can replace dir or any of its parents.
// dir must not contain symlinks. Each directory must be owned by root or the current user,
// and must either not be writable by group or others, or have the sticky bit set (like /tmp).
func checkTrustedParents(dir string) error {
	for {
		info, err := os.Lstat(dir)
		if err != nil {
			return fmt.Errorf("failed to stat directory %s: %w", dir, err)
		}

		if info.Mode().Type() != fs.ModeDir {
			return untrustedParentError(dir, fmt.Sprintf("%s is not a directory", dir))
		}

		uid, err := fileOwner(dir, info)
		if err != nil {
			return err
		}

		if uid != 0 && uid != os.Geteuid() {
			return untrustedParentError(dir, fmt.Sprintf("%s is owned by uid %d", dir, uid))
		}

		if info.Mode().Perm()&0o022 != 0 && info.Mode()&fs.ModeSticky == 0 {
			return untrustedParentError(dir, fmt.Sprintf("%s is writable by group or others and does not have the sticky bit set", dir))
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return nil
		}

		dir = parent
	}
}

func fileOwner(path string, info fs.FileInfo) (int, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, fmt.Errorf("failed to determine owner of %s", path)
	}

	return int(stat.Uid), nil
}
