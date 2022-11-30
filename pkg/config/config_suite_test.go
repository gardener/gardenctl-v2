/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package config_test

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestConfiguration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Config Test Suite")
}

var gardenHomeDir string

var _ = BeforeSuite(func() {
	dir, err := os.MkdirTemp("", "garden-*")
	Expect(err).NotTo(HaveOccurred())
	gardenHomeDir = dir
})

var _ = AfterSuite(func() {
	Expect(os.RemoveAll(gardenHomeDir)).To(Succeed())
})
