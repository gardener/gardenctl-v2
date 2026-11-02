//go:build unix

/*
SPDX-FileCopyrightText: Contributors to the Gardener project

SPDX-License-Identifier: Apache-2.0
*/

package util_test

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardenctl-v2/internal/util"
)

var _ = Describe("EnsurePrivateDir", func() {
	var parent, dir string

	BeforeEach(func() {
		var err error

		parent, err = filepath.EvalSymlinks(GinkgoT().TempDir())
		Expect(err).NotTo(HaveOccurred())

		dir = filepath.Join(parent, "garden")
	})

	expectMode := func(path string, mode os.FileMode) {
		GinkgoHelper()

		info, err := os.Lstat(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(info.Mode().Perm()).To(Equal(mode))
	}

	expectRejected := func(path, reason string) {
		GinkgoHelper()

		_, err := util.EnsurePrivateDir(path)
		Expect(err).To(MatchError(ContainSubstring(reason)))
	}

	It("should create a missing directory with mode 0700", func() {
		Expect(util.EnsurePrivateDir(dir)).To(Equal(dir))
		expectMode(dir, 0o700)
	})

	It("should accept an existing private directory", func() {
		Expect(os.Mkdir(dir, 0o700)).To(Succeed())
		Expect(util.EnsurePrivateDir(dir)).To(Equal(dir))
	})

	It("should reject an existing directory accessible by others without changing it", func() {
		Expect(os.Mkdir(dir, 0o700)).To(Succeed())
		Expect(os.Chmod(dir, 0o777)).To(Succeed())

		expectRejected(dir, "must not be accessible by group or others")
		expectMode(dir, 0o777)
	})

	It("should reject a symlink to a private directory", func() {
		target := filepath.Join(parent, "target")
		Expect(os.Mkdir(target, 0o700)).To(Succeed())
		Expect(os.Symlink(target, dir)).To(Succeed())

		expectRejected(dir, "it is not a directory")
	})

	It("should reject a dangling symlink", func() {
		Expect(os.Symlink(filepath.Join(parent, "missing"), dir)).To(Succeed())

		expectRejected(dir, "it is not a directory")
	})

	It("should reject a regular file", func() {
		Expect(os.WriteFile(dir, nil, 0o600)).To(Succeed())

		expectRejected(dir, "it is not a directory")
	})

	It("should reject a parent that is writable by others and not sticky", func() {
		Expect(os.Chmod(parent, 0o777)).To(Succeed())

		expectRejected(dir, "does not have the sticky bit set")
		Expect(dir).NotTo(BeADirectory())
	})

	It("should accept a sticky parent that is writable by others", func() {
		Expect(os.Chmod(parent, 0o777|os.ModeSticky)).To(Succeed())

		Expect(util.EnsurePrivateDir(dir)).To(Equal(dir))
	})

	It("should reject a parent that cannot be resolved", func() {
		Expect(os.Symlink(filepath.Join(parent, "missing"), filepath.Join(parent, "dangling"))).To(Succeed())

		expectRejected(filepath.Join(parent, "dangling", "garden"), "failed to resolve directory")
	})

	It("should return the resolved path for relative and nested parent symlinks", func() {
		safe := filepath.Join(parent, "safe")
		Expect(os.Mkdir(safe, 0o700)).To(Succeed())
		Expect(os.Symlink("safe", filepath.Join(parent, "link1"))).To(Succeed())
		Expect(os.Symlink(filepath.Join(parent, "link1"), filepath.Join(parent, "link2"))).To(Succeed())

		Expect(util.EnsurePrivateDir(filepath.Join(parent, "link2", "garden"))).To(Equal(filepath.Join(safe, "garden")))
		Expect(util.EnsurePrivateDir(filepath.Join(parent, "link2", "..", "safe", "garden"))).To(Equal(filepath.Join(safe, "garden")))
	})

	It("should not be affected by repointing a parent symlink located in a directory writable by others", func() {
		safe := filepath.Join(parent, "safe")
		Expect(os.Mkdir(safe, 0o700)).To(Succeed())

		shared := filepath.Join(parent, "shared")
		Expect(os.Mkdir(shared, 0o700)).To(Succeed())
		Expect(os.Chmod(shared, 0o777)).To(Succeed())

		alias := filepath.Join(shared, "tmp")
		Expect(os.Symlink(safe, alias)).To(Succeed())

		verified, err := util.EnsurePrivateDir(filepath.Join(alias, "garden"))
		Expect(err).NotTo(HaveOccurred())
		Expect(verified).To(Equal(filepath.Join(safe, "garden")))

		evil := filepath.Join(parent, "evil")
		Expect(os.Mkdir(evil, 0o777)).To(Succeed())
		Expect(os.Remove(alias)).To(Succeed())
		Expect(os.Symlink(evil, alias)).To(Succeed())

		Expect(os.WriteFile(filepath.Join(verified, "secret"), []byte("s3cr3t"), 0o600)).To(Succeed())
		Expect(filepath.Join(safe, "garden", "secret")).To(BeARegularFile())
		Expect(filepath.Join(evil, "garden")).NotTo(BeAnExistingFile())
	})

	It("should return an absolute path for a relative input", func() {
		wd, err := os.Getwd()
		Expect(err).NotTo(HaveOccurred())
		Expect(os.Chdir(parent)).To(Succeed())
		DeferCleanup(os.Chdir, wd)

		Expect(util.EnsurePrivateDir("garden")).To(Equal(dir))
	})
})

var _ = Describe("Factory temp directory", func() {
	var factory *util.FactoryImpl

	BeforeEach(func() {
		GinkgoT().Setenv("GCTL_SESSION_ID", "test-session")

		factory = util.NewFactoryImpl()
	})

	It("should default to a per-user directory", func() {
		Expect(factory.GardenTempDir()).To(Equal(filepath.Join(os.TempDir(), fmt.Sprintf("garden-%d", os.Geteuid()))))
	})

	It("should refuse to create a manager in a temp directory accessible by others", func() {
		tempDir := filepath.Join(GinkgoT().TempDir(), "garden")
		Expect(os.Mkdir(tempDir, 0o700)).To(Succeed())
		Expect(os.Chmod(tempDir, 0o777)).To(Succeed())

		factory.GardenTempDirectory = tempDir

		_, err := factory.Manager()
		Expect(err).To(MatchError(ContainSubstring("refusing to use")))
		Expect(filepath.Join(tempDir, "sessions")).NotTo(BeADirectory())
	})

	It("should place the session directory below the resolved temp directory", func() {
		parent, err := filepath.EvalSymlinks(GinkgoT().TempDir())
		Expect(err).NotTo(HaveOccurred())

		safe := filepath.Join(parent, "safe")
		Expect(os.Mkdir(safe, 0o700)).To(Succeed())
		Expect(os.Symlink(safe, filepath.Join(parent, "alias"))).To(Succeed())

		factory.GardenTempDirectory = filepath.Join(parent, "alias", "garden")

		manager, err := factory.Manager()
		Expect(err).NotTo(HaveOccurred())
		Expect(manager.SessionDir()).To(Equal(filepath.Join(safe, "garden", "sessions", "test-session")))
		Expect(manager.SessionDir()).To(BeADirectory())
	})
})
