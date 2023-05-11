/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package providerenv_test

import (
	"os"
	"path/filepath"
	"testing"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/gardener/gardenctl-v2/pkg/env/testdata"
)

var (
	gardenHomeDir string
	sessionDir    string
)

func init() {
	utilruntime.Must(gardencorev1beta1.AddToScheme(scheme.Scheme))
	utilruntime.Must(seedmanagementv1alpha1.AddToScheme(scheme.Scheme))
}

func TestProviderEnvCommand(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ProviderEnv Command Test Suite")
}

var _ = BeforeSuite(func() {
	gardenHomeDir = makeTempGardenHomeDir()
	sessionDir = filepath.Join(gardenHomeDir, "sessions")
})

var _ = AfterSuite(func() {
	Expect(os.RemoveAll(gardenHomeDir)).To(Succeed())
})

func makeTempGardenHomeDir() string {
	dir, err := os.MkdirTemp("", "garden-*")
	Expect(err).NotTo(HaveOccurred())
	Expect(os.Mkdir(filepath.Join(dir, "templates"), 0o777)).NotTo(HaveOccurred())

	return dir
}

// The separator in the filename must be a forward slash, even on Windows systems.
// see https://pkg.go.dev/embed#hdr-Directives
func readTestFile(filename string) string {
	data, err := testdata.FS.ReadFile(filename)
	Expect(err).NotTo(HaveOccurred())

	return string(data)
}

func writeTempFile(filename string, content string) {
	err := os.WriteFile(filepath.Join(gardenHomeDir, filename), []byte(content), 0o777)
	Expect(err).NotTo(HaveOccurred())
}

func removeTempFile(filename string) {
	err := os.Remove(filepath.Join(gardenHomeDir, filename))
	if !os.IsNotExist(err) {
		Expect(err).NotTo(HaveOccurred())
	}
}
