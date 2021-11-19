/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package cloudenv_test

import (
	"embed"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

//go:embed testdata
var testdata embed.FS
var gardenHomeDir string

func init() {
	utilruntime.Must(gardencorev1beta1.AddToScheme(scheme.Scheme))
}

func TestCloudEnvCommand(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CloudEnv Command Test Suite")
}

var _ = BeforeSuite(func() {
	gardenHomeDir = makeTempGardenHomeDir()
})

var _ = AfterSuite(func() {
	Expect(os.RemoveAll(gardenHomeDir)).To(Succeed())
})

func makeTempGardenHomeDir() string {
	dir, err := os.MkdirTemp(os.TempDir(), "garden-*")
	Expect(err).NotTo(HaveOccurred())
	Expect(os.Mkdir(filepath.Join(dir, "templates"), 0777)).NotTo(HaveOccurred())

	return dir
}

func readTestFile(filename string) string {
	data, err := testdata.ReadFile(filepath.Join("testdata", filename))
	Expect(err).NotTo(HaveOccurred())

	return string(data)
}

func writeTempFile(filename string, content string) {
	err := os.WriteFile(filepath.Join(gardenHomeDir, filename), []byte(content), 0777)
	Expect(err).NotTo(HaveOccurred())
}

func removeTempFile(filename string) {
	err := os.Remove(filepath.Join(gardenHomeDir, filename))
	if !os.IsNotExist(err) {
		Expect(err).NotTo(HaveOccurred())
	}
}
