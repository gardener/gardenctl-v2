/*
SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package providerenv_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	providerenv "github.com/gardener/gardenctl-v2/pkg/cmd/providerenv"
)

var _ = Describe("TempDataWriter", func() {
	var (
		tempDir         string
		sessionDir      string
		sessionID       string
		writer          *providerenv.TempDataWriter
		err             error
		canonicalTarget providerenv.CanonicalTarget
	)

	BeforeEach(func() {
		// Create a temporary directory for testing
		tempDir, err = os.MkdirTemp("", "gardenctl-test-*")
		Expect(err).NotTo(HaveOccurred())
		sessionID = "test-session-id"
		sessionDir = filepath.Join(tempDir, "sessions", "test-session")
		err = os.MkdirAll(sessionDir, 0o700)
		Expect(err).NotTo(HaveOccurred())

		// Create a canonical target for testing
		canonicalTarget = providerenv.CanonicalTarget{
			Garden:    "test-garden",
			Namespace: "garden-test-project",
			Shoot:     "test-shoot",
		}
	})

	AfterEach(func() {
		// Clean up the temporary directory
		if tempDir != "" {
			os.RemoveAll(tempDir)
		}
	})

	Describe("NewTempDataWriter", func() {
		It("should create a new TempDataWriter with deterministic prefix", func() {
			writer, err = providerenv.NewTempDataWriter(sessionID, sessionDir, canonicalTarget)
			Expect(err).NotTo(HaveOccurred())
			Expect(writer).NotTo(BeNil())
			Expect(writer.DataDirectory()).To(ContainSubstring("provider-env"))

			// Check that the directory was NOT created yet (lazy initialization)
			_, err = os.Stat(writer.DataDirectory())
			Expect(os.IsNotExist(err)).To(BeTrue(), "directory should not exist until first write")
		})

		It("should create the same prefix for the same target", func() {
			writer1, err := providerenv.NewTempDataWriter(sessionID, sessionDir, canonicalTarget)
			Expect(err).NotTo(HaveOccurred())

			writer2, err := providerenv.NewTempDataWriter(sessionID, sessionDir, canonicalTarget)
			Expect(err).NotTo(HaveOccurred())

			// Should have same data directory and prefix (deterministic)
			Expect(writer1.DataDirectory()).To(Equal(writer2.DataDirectory()))
			Expect(writer1.GetPrefix()).To(Equal(writer2.GetPrefix()))
		})

		It("should create different prefixes for different targets", func() {
			target1 := providerenv.CanonicalTarget{Garden: "garden1", Namespace: "garden-project1", Shoot: "shoot1"}
			target2 := providerenv.CanonicalTarget{Garden: "garden2", Namespace: "garden-project2", Shoot: "shoot2"}

			writer1, err := providerenv.NewTempDataWriter(sessionID, sessionDir, target1)
			Expect(err).NotTo(HaveOccurred())

			writer2, err := providerenv.NewTempDataWriter(sessionID, sessionDir, target2)
			Expect(err).NotTo(HaveOccurred())

			// Different targets should have different prefixes
			Expect(writer1.GetPrefix()).NotTo(Equal(writer2.GetPrefix()))
		})

		It("should create different prefixes for different sessionIDs with same target", func() {
			sessionID1 := "session1"
			sessionID2 := "session2"

			writer1, err := providerenv.NewTempDataWriter(sessionID1, sessionDir, canonicalTarget)
			Expect(err).NotTo(HaveOccurred())

			writer2, err := providerenv.NewTempDataWriter(sessionID2, sessionDir, canonicalTarget)
			Expect(err).NotTo(HaveOccurred())

			// Different sessionIDs should have different prefixes
			Expect(writer1.GetPrefix()).NotTo(Equal(writer2.GetPrefix()))
		})
	})

	Describe("WriteField", func() {
		BeforeEach(func() {
			writer, err = providerenv.NewTempDataWriter(sessionID, sessionDir, canonicalTarget)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should write multiple fields to separate files with correct content and permissions", func() {
			// Write first field with simple value
			path1, err := writer.WriteField("field1", "value1")
			Expect(err).NotTo(HaveOccurred())
			Expect(path1).To(ContainSubstring("field1.txt"))
			Expect(path1).To(ContainSubstring(writer.GetPrefix()))

			// Write second field with special characters
			specialValue := "value with\nnewlines\tand\n$special 'chars' \"quotes\""
			path2, err := writer.WriteField("field2", specialValue)
			Expect(err).NotTo(HaveOccurred())
			Expect(path2).To(ContainSubstring("field2.txt"))

			// Verify paths are different
			Expect(path1).NotTo(Equal(path2))

			// Verify first field content
			content1, err := os.ReadFile(path1)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content1)).To(Equal("value1"))

			// Verify second field content (including special characters)
			content2, err := os.ReadFile(path2)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content2)).To(Equal(specialValue))

			// Verify file permissions (owner read/write only)
			info1, err := os.Stat(path1)
			Expect(err).NotTo(HaveOccurred())
			Expect(info1.Mode().Perm()).To(Equal(os.FileMode(0o600)))

			info2, err := os.Stat(path2)
			Expect(err).NotTo(HaveOccurred())
			Expect(info2.Mode().Perm()).To(Equal(os.FileMode(0o600)))
		})

		It("should track written files", func() {
			_, err := writer.WriteField("field1", "value1")
			Expect(err).NotTo(HaveOccurred())
			_, err = writer.WriteField("field2", "value2")
			Expect(err).NotTo(HaveOccurred())

			Expect(writer.GetFilePath("field1")).To(ContainSubstring("field1.txt"))
			Expect(writer.GetFilePath("field2")).To(ContainSubstring("field2.txt"))
			Expect(writer.GetFilePath("nonexistent")).To(BeEmpty())
		})

		It("should fail when directory cannot be created", func() {
			// Create a read-only parent directory
			readOnlyDir := filepath.Join(tempDir, "readonly")
			err := os.MkdirAll(readOnlyDir, 0o500)
			Expect(err).NotTo(HaveOccurred())

			// Create writer with unwritable path
			writer, err := providerenv.NewTempDataWriter(sessionID, filepath.Join(readOnlyDir, "subdir"), canonicalTarget)
			Expect(err).NotTo(HaveOccurred())

			// WriteField should fail when trying to create the directory
			_, err = writer.WriteField("testfield", "testvalue")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to create temporary data directory"))
		})
	})

	Describe("GetAllFilePaths", func() {
		BeforeEach(func() {
			writer, err = providerenv.NewTempDataWriter(sessionID, sessionDir, canonicalTarget)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return all written file paths", func() {
			_, err := writer.WriteField("field1", "value1")
			Expect(err).NotTo(HaveOccurred())
			_, err = writer.WriteField("field2", "value2")
			Expect(err).NotTo(HaveOccurred())
			_, err = writer.WriteField("field3", "value3")
			Expect(err).NotTo(HaveOccurred())

			allPaths := writer.GetAllFilePaths()
			Expect(allPaths).To(HaveLen(3))
			Expect(allPaths["field1"]).To(ContainSubstring("field1.txt"))
			Expect(allPaths["field2"]).To(ContainSubstring("field2.txt"))
			Expect(allPaths["field3"]).To(ContainSubstring("field3.txt"))
		})

		It("should return a copy of the file map", func() {
			_, err := writer.WriteField("field1", "value1")
			Expect(err).NotTo(HaveOccurred())

			allPaths := writer.GetAllFilePaths()
			// Modify the returned map
			allPaths["field2"] = "fake-path"

			// Verify the internal map was not modified
			Expect(writer.GetFilePath("field2")).To(BeEmpty())
			Expect(writer.GetAllFilePaths()).To(HaveLen(1))
		})
	})
})

var _ = Describe("CleanupDataWriter", func() {
	var (
		tempDir         string
		sessionDir      string
		sessionID       string
		cleanupWriter   *providerenv.CleanupDataWriter
		err             error
		canonicalTarget providerenv.CanonicalTarget
	)

	BeforeEach(func() {
		// Create a temporary directory for testing
		tempDir, err = os.MkdirTemp("", "gardenctl-test-*")
		Expect(err).NotTo(HaveOccurred())
		sessionID = "test-session-id"
		sessionDir = filepath.Join(tempDir, "sessions", "test-session")
		err = os.MkdirAll(sessionDir, 0o700)
		Expect(err).NotTo(HaveOccurred())

		// Create a canonical target for testing
		canonicalTarget = providerenv.CanonicalTarget{
			Garden:    "test-garden",
			Namespace: "garden-test-project",
			Shoot:     "test-shoot",
		}
	})

	AfterEach(func() {
		// Clean up the temporary directory
		if tempDir != "" {
			os.RemoveAll(tempDir)
		}
	})

	Describe("NewCleanupDataWriter", func() {
		It("should create a new CleanupDataWriter", func() {
			cleanupWriter, err = providerenv.NewCleanupDataWriter(sessionID, sessionDir, canonicalTarget)
			Expect(err).NotTo(HaveOccurred())
			Expect(cleanupWriter).NotTo(BeNil())
			Expect(cleanupWriter.DataDirectory()).To(ContainSubstring("provider-env"))
		})

		It("should clean up existing files when CleanupExisting is called", func() {
			// First, create some temp files using TempDataWriter
			tempWriter, err := providerenv.NewTempDataWriter(sessionID, sessionDir, canonicalTarget)
			Expect(err).NotTo(HaveOccurred())

			path1, err := tempWriter.WriteField("field1", "value1")
			Expect(err).NotTo(HaveOccurred())
			path2, err := tempWriter.WriteField("field2", "value2")
			Expect(err).NotTo(HaveOccurred())

			// Verify files exist
			_, err = os.Stat(path1)
			Expect(err).NotTo(HaveOccurred())
			_, err = os.Stat(path2)
			Expect(err).NotTo(HaveOccurred())

			// Create CleanupDataWriter and call CleanupExisting
			cleanupWriter, err = providerenv.NewCleanupDataWriter(sessionID, sessionDir, canonicalTarget)
			Expect(err).NotTo(HaveOccurred())
			err = cleanupWriter.CleanupExisting()
			Expect(err).NotTo(HaveOccurred())

			// Verify files are gone
			_, err = os.Stat(path1)
			Expect(os.IsNotExist(err)).To(BeTrue())
			_, err = os.Stat(path2)
			Expect(os.IsNotExist(err)).To(BeTrue())
		})

		It("should not fail when no files exist", func() {
			// Create CleanupDataWriter when no files exist
			cleanupWriter, err = providerenv.NewCleanupDataWriter(sessionID, sessionDir, canonicalTarget)
			Expect(err).NotTo(HaveOccurred())
			Expect(cleanupWriter).NotTo(BeNil())

			// CleanupExisting should not fail even when no files exist
			err = cleanupWriter.CleanupExisting()
			Expect(err).NotTo(HaveOccurred())
		})

		It("should use the same prefix as TempDataWriter for the same target", func() {
			tempWriter, err := providerenv.NewTempDataWriter(sessionID, sessionDir, canonicalTarget)
			Expect(err).NotTo(HaveOccurred())

			cleanupWriter, err := providerenv.NewCleanupDataWriter(sessionID, sessionDir, canonicalTarget)
			Expect(err).NotTo(HaveOccurred())

			// Should have same prefix (so they clean up the same files)
			Expect(cleanupWriter.GetPrefix()).To(Equal(tempWriter.GetPrefix()))
		})
	})

	Describe("WriteField", func() {
		BeforeEach(func() {
			cleanupWriter, err = providerenv.NewCleanupDataWriter(sessionID, sessionDir, canonicalTarget)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should be a no-op and return empty string", func() {
			path, err := cleanupWriter.WriteField("testfield", "testvalue")
			Expect(err).NotTo(HaveOccurred())
			Expect(path).To(BeEmpty())

			// Verify no file was created
			dataDir := cleanupWriter.DataDirectory()
			if _, err := os.Stat(dataDir); err == nil {
				// If directory exists, verify it's empty
				entries, err := os.ReadDir(dataDir)
				Expect(err).NotTo(HaveOccurred())
				Expect(entries).To(BeEmpty())
			}
		})

		It("should not create any files when called multiple times", func() {
			_, err := cleanupWriter.WriteField("field1", "value1")
			Expect(err).NotTo(HaveOccurred())
			_, err = cleanupWriter.WriteField("field2", "value2")
			Expect(err).NotTo(HaveOccurred())

			// Verify no files were created
			dataDir := cleanupWriter.DataDirectory()
			if _, err := os.Stat(dataDir); err == nil {
				entries, err := os.ReadDir(dataDir)
				Expect(err).NotTo(HaveOccurred())
				Expect(entries).To(BeEmpty())
			}
		})
	})

	Describe("GetAllFilePaths", func() {
		BeforeEach(func() {
			cleanupWriter, err = providerenv.NewCleanupDataWriter(sessionID, sessionDir, canonicalTarget)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an empty map", func() {
			allPaths := cleanupWriter.GetAllFilePaths()
			Expect(allPaths).NotTo(BeNil())
			Expect(allPaths).To(BeEmpty())
		})

		It("should return an empty map even after WriteField calls", func() {
			_, err := cleanupWriter.WriteField("field1", "value1")
			Expect(err).NotTo(HaveOccurred())
			_, err = cleanupWriter.WriteField("field2", "value2")
			Expect(err).NotTo(HaveOccurred())

			allPaths := cleanupWriter.GetAllFilePaths()
			Expect(allPaths).NotTo(BeNil())
			Expect(allPaths).To(BeEmpty())
		})
	})

	Describe("CleanupExisting", func() {
		BeforeEach(func() {
			cleanupWriter, err = providerenv.NewCleanupDataWriter(sessionID, sessionDir, canonicalTarget)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should remove existing files", func() {
			// Create some temp files first
			tempWriter, err := providerenv.NewTempDataWriter(sessionID, sessionDir, canonicalTarget)
			Expect(err).NotTo(HaveOccurred())
			path1, err := tempWriter.WriteField("field1", "value1")
			Expect(err).NotTo(HaveOccurred())
			path2, err := tempWriter.WriteField("field2", "value2")
			Expect(err).NotTo(HaveOccurred())

			// Verify files exist
			Expect(path1).To(BeARegularFile())
			Expect(path2).To(BeARegularFile())

			// Call CleanupExisting
			err = cleanupWriter.CleanupExisting()
			Expect(err).NotTo(HaveOccurred())

			// Verify files are gone
			Expect(path1).NotTo(BeAnExistingFile())
			Expect(path2).NotTo(BeAnExistingFile())
		})

		It("should not fail when no files exist", func() {
			err := cleanupWriter.CleanupExisting()
			Expect(err).NotTo(HaveOccurred())
		})

		It("should be idempotent", func() {
			// Create some temp files
			tempWriter, err := providerenv.NewTempDataWriter(sessionID, sessionDir, canonicalTarget)
			Expect(err).NotTo(HaveOccurred())
			path1, err := tempWriter.WriteField("field1", "value1")
			Expect(err).NotTo(HaveOccurred())

			// Call CleanupExisting multiple times
			err = cleanupWriter.CleanupExisting()
			Expect(err).NotTo(HaveOccurred())
			Expect(path1).NotTo(BeAnExistingFile())

			err = cleanupWriter.CleanupExisting()
			Expect(err).NotTo(HaveOccurred())

			err = cleanupWriter.CleanupExisting()
			Expect(err).NotTo(HaveOccurred())
		})

		It("should only clean up files for the same target, not other targets", func() {
			target1 := providerenv.CanonicalTarget{Garden: "garden1", Namespace: "garden-project1", Shoot: "shoot1"}
			target2 := providerenv.CanonicalTarget{Garden: "garden2", Namespace: "garden-project2", Shoot: "shoot2"}

			// Create files for target1
			writer1, err := providerenv.NewTempDataWriter(sessionID, sessionDir, target1)
			Expect(err).NotTo(HaveOccurred())
			path1, _ := writer1.WriteField("field1", "value1")

			// Create files for target2
			writer2, err := providerenv.NewTempDataWriter(sessionID, sessionDir, target2)
			Expect(err).NotTo(HaveOccurred())
			path2, _ := writer2.WriteField("field2", "value2")

			// Both files should exist
			Expect(path1).To(BeARegularFile())
			Expect(path2).To(BeARegularFile())

			// Create CleanupDataWriter for target1 only
			cleanupWriter1, err := providerenv.NewCleanupDataWriter(sessionID, sessionDir, target1)
			Expect(err).NotTo(HaveOccurred())
			err = cleanupWriter1.CleanupExisting()
			Expect(err).NotTo(HaveOccurred())

			// target1 files should be gone, target2 files should remain
			Expect(path1).NotTo(BeAnExistingFile())
			Expect(path2).To(BeARegularFile())
		})
	})
})
