/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/
package target

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
)

// KubeconfigCache can read and save kubeconfigs.
type KubeconfigCache interface {
	// Read reads the kubeconfig for a given target and returns it
	// as YAML.
	Read(t Target) ([]byte, error)
	// Write takes a target and the kubeconfig as YAML and saves it.
	Write(t Target, kubeconfig []byte) error
}

type fsKubeconfigCache struct {
	baseDirectory string
}

var _ KubeconfigCache = &fsKubeconfigCache{}

// NewFilesystemKubeconfigCache returns a new KubeconfigCache that
// reads and writes using the local filesystem.
func NewFilesystemKubeconfigCache(baseDirectory string) KubeconfigCache {
	return &fsKubeconfigCache{
		baseDirectory: baseDirectory,
	}
}

// Read reads the kubeconfig for a given target and returns it
// as YAML.
func (c *fsKubeconfigCache) Read(t Target) ([]byte, error) {
	filename, err := c.filename(t)
	if err != nil {
		return nil, err
	}

	return ioutil.ReadFile(filename)
}

// Write takes a target and the kubeconfig as YAML and saves it.
func (c *fsKubeconfigCache) Write(t Target, kubeconfig []byte) error {
	filename, err := c.filename(t)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
		return err
	}

	return ioutil.WriteFile(filename, kubeconfig, 0600)
}

func (c *fsKubeconfigCache) filename(t Target) (string, error) {
	if t.GardenIdentity() == "" {
		return "", ErrNoGardenTargeted
	}

	directory := filepath.Join(c.baseDirectory, t.GardenIdentity())

	if t.ShootName() != "" {
		if t.ProjectName() != "" {
			directory = filepath.Join(directory, "projects", t.ProjectName(), "shoots", t.ShootName())
		} else if t.SeedName() != "" {
			directory = filepath.Join(directory, "seeds", t.SeedName(), "shoots", t.ShootName())
		} else {
			return "", errors.New("shoot targets need either project or seed name specified")
		}
	} else if t.SeedName() != "" {
		directory = filepath.Join(directory, "seeds", t.SeedName())
	}

	return filepath.Join(directory, "kubeconfig"), nil
}
