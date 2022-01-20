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

var gardenHomeDir string

func TestCommand(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Config Command Test Suite")
}

var _ = BeforeSuite(func() {
	var err error

	gardenHomeDir, err = os.MkdirTemp("", "gctlv2-*")
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	Expect(os.RemoveAll(gardenHomeDir)).To(Succeed())
})
