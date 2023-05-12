/*
SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package plugins_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gardener/gardenctl-v2/pkg/plugins"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPlugins(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Plugins Package Test Suite")
}

var _ = Describe("plugins", func() {
	It("should return an error when plugins loading fake example.so", func() {
		pluginsPath, err := os.MkdirTemp("", "plugins")
		Expect(err).NotTo(HaveOccurred())

		filename := filepath.Join(pluginsPath, "example.so")
		_, err = os.Create(filename)
		Expect(err).NotTo(HaveOccurred())

		_, err = plugins.LoadPlugins(pluginsPath)
		Expect(err).To(MatchError(MatchRegexp("^plugin open error or invalid file ")))
		Expect(os.RemoveAll(pluginsPath)).To(Succeed())
	})
})
