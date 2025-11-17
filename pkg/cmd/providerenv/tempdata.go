/*
SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package providerenv

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// CanonicalTarget represents a fully resolved target used for deterministic file naming.
// This ensures that the same target (even if specified in different ways) always produces
// the same file names.
type CanonicalTarget struct {
	Garden    string
	Namespace string
	Shoot     string
}

// getDataDir returns the provider-env data directory path within the session directory.
func getDataDir(sessionDir string) string {
	return filepath.Join(sessionDir, "provider-env")
}

// computeFilePrefix creates a deterministic prefix from session ID and canonical target.
// This ensures the same target always produces the same files, preventing accumulation.
func computeFilePrefix(sessionID string, target CanonicalTarget) string {
	targetKey := fmt.Sprintf("%s|%s|%s", target.Garden, target.Namespace, target.Shoot)
	hash := sha256.Sum256([]byte(sessionID + "|" + targetKey))

	return hex.EncodeToString(hash[:8]) // Use first 8 bytes (16 hex chars)
}

// DataWriter is an interface for managing temporary credential data files.
type DataWriter interface {
	// WriteField writes a field value to a temporary file and returns its path.
	// For CleanupDataWriter, this is a no-op that returns an empty string.
	WriteField(field string, value string) (string, error)

	// GetAllFilePaths returns a map of all field names to their file paths.
	// For CleanupDataWriter, this returns an empty map.
	GetAllFilePaths() map[string]string

	// DataDirectory returns the directory path containing all temporary files.
	DataDirectory() string
}

// TempDataWriter manages temporary files for provider credentials.
type TempDataWriter struct {
	sessionDir string
	dataDir    string            // provider-env directory
	prefix     string            // deterministic prefix for this session+target
	files      map[string]string // field name -> filepath mapping
	dirCreated bool              // tracks if directory has been created
}

var _ DataWriter = &TempDataWriter{}

// CleanupDataWriter is used for unset operations. It cleans up any existing temporary files
// for the target without writing new ones. WriteField is a no-op and GetAllFilePaths returns
// an empty map, as templates don't need file paths during unset operations.
type CleanupDataWriter struct {
	dataDir string
	prefix  string
}

var _ DataWriter = &CleanupDataWriter{}

// NewTempDataWriter creates a new TempDataWriter with deterministic file naming.
// The file structure is: ${sessionDir}/provider-env/.${prefix}-${fieldname}.txt
// The prefix is derived from a hash of the session ID and canonical target, ensuring
// that the same target always produces the same files (avoiding accumulation of obsolete files).
// The directory is created lazily when the first file is written.
func NewTempDataWriter(sessionID, sessionDir string, target CanonicalTarget) (*TempDataWriter, error) {
	dataDir := getDataDir(sessionDir)
	prefix := computeFilePrefix(sessionID, target)

	return &TempDataWriter{
		sessionDir: sessionDir,
		dataDir:    dataDir,
		prefix:     prefix,
		files:      make(map[string]string),
		dirCreated: false,
	}, nil
}

// WriteField writes a field value to a temporary file and returns its path.
// Files are created with 0600 permissions (owner read/write only).
// The directory is created on the first call to WriteField (lazy initialization).
func (t *TempDataWriter) WriteField(field string, value string) (string, error) {
	// Create directory on first write (lazy initialization)
	if !t.dirCreated {
		if err := os.MkdirAll(t.dataDir, 0o700); err != nil {
			return "", fmt.Errorf("failed to create temporary data directory: %w", err)
		}

		t.dirCreated = true
	}

	filename := fmt.Sprintf("%s-%s.txt", t.prefix, field)
	filepath := filepath.Join(t.dataDir, filename)

	// Write file with restrictive permissions (owner read/write only)
	// This will overwrite any existing file from a previous run
	if err := os.WriteFile(filepath, []byte(value), 0o600); err != nil {
		return "", fmt.Errorf("failed to write field %q: %w", field, err)
	}

	t.files[field] = filepath

	return filepath, nil
}

// DataDirectory returns the directory path containing all temporary files.
func (t *TempDataWriter) DataDirectory() string {
	return t.dataDir
}

// GetFilePath returns the path for a previously written field, or empty string if not found.
func (t *TempDataWriter) GetFilePath(field string) string {
	return t.files[field]
}

// GetAllFilePaths returns a map of all field names to their file paths.
func (t *TempDataWriter) GetAllFilePaths() map[string]string {
	// Return a copy to prevent external modifications
	result := make(map[string]string, len(t.files))
	for k, v := range t.files {
		result[k] = v
	}

	return result
}

// NewCleanupDataWriter creates a new CleanupDataWriter for unset operations.
// It does not write new files, but provides a CleanupExisting() method to remove
// any leftover files from previous runs. Call CleanupExisting() explicitly to
// perform the cleanup.
func NewCleanupDataWriter(sessionID, sessionDir string, target CanonicalTarget) (*CleanupDataWriter, error) {
	dataDir := getDataDir(sessionDir)
	prefix := computeFilePrefix(sessionID, target)

	writer := &CleanupDataWriter{
		dataDir: dataDir,
		prefix:  prefix,
	}

	return writer, nil
}

// WriteField is a no-op for CleanupDataWriter. It returns an empty string and no error.
// This allows the calling code to use the same logic for both TempDataWriter and CleanupDataWriter.
func (c *CleanupDataWriter) WriteField(field string, value string) (string, error) {
	return "", nil
}

// GetAllFilePaths returns an empty map for CleanupDataWriter.
// Templates check .__meta.unset and don't reference .dataFiles during unset operations,
// so they don't need any file paths.
func (c *CleanupDataWriter) GetAllFilePaths() map[string]string {
	return make(map[string]string)
}

// DataDirectory returns the directory path that would contain temporary files.
func (c *CleanupDataWriter) DataDirectory() string {
	return c.dataDir
}

// CleanupExisting removes all temporary files with this writer's prefix.
// This should be called explicitly after creating the CleanupDataWriter to
// ensure leftover files from previous runs are removed.
func (c *CleanupDataWriter) CleanupExisting() error {
	return c.cleanup()
}

// cleanup removes all temporary files with this writer's prefix.
func (c *CleanupDataWriter) cleanup() error {
	if c.dataDir == "" || c.prefix == "" {
		return nil
	}

	// Use glob pattern to find all files with this prefix
	pattern := filepath.Join(c.dataDir, fmt.Sprintf("%s-*.txt", c.prefix))

	matches, err := filepath.Glob(pattern)
	if err != nil {
		// Ignore glob errors (e.g., directory doesn't exist)
		return nil
	}

	// Remove each file, ignoring only "not exist" errors
	for _, match := range matches {
		if err := os.Remove(match); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	return nil
}
