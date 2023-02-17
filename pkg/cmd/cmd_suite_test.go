/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package cmd_test

import (
	"os"
	"path/filepath"
	"testing"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardenctl-v2/pkg/cmd"
	"github.com/gardener/gardenctl-v2/pkg/config"
)

var (
	gardenHomeDir string
	sessionDir    string
	configFile    string
	cfg           *config.Config
)

func init() {
	utilruntime.Must(gardencorev1beta1.AddToScheme(scheme.Scheme))
	utilruntime.Must(operationsv1alpha1.AddToScheme(scheme.Scheme))
}

func TestGardenctlCommand(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Gardenctl Command Test Suite")
}

var _ = BeforeSuite(func() {
	gardenHomeDir = makeTempGardenHomeDir()
	Expect(os.Setenv(cmd.EnvGardenHomeDir, gardenHomeDir)).To(Succeed())
	configFile = filepath.Join(gardenHomeDir, cmd.ConfigName+".yaml")
	sessionID := uuid.New().String()
	Expect(os.Setenv(cmd.EnvSessionID, sessionID)).To(Succeed())
	sessionDir = filepath.Join(os.TempDir(), "garden", sessionID)
	Expect(os.MkdirAll(sessionDir, os.ModePerm))
	cfg = &config.Config{
		Filename:       configFile,
		LinkKubeconfig: pointer.Bool(false),
		Gardens: []config.Garden{{
			Name:       "foo",
			Kubeconfig: "/not/a/real/garden-foo/kubeconfig",
		}, {
			Name:       "bar",
			Kubeconfig: "/not/a/real/garden-bar/kubeconfig",
		}},
	}
	Expect(cfg.Save()).To(Succeed())
})

var _ = AfterSuite(func() {
	Expect(os.Unsetenv(cmd.EnvSessionID))
	Expect(os.Unsetenv(cmd.EnvGardenHomeDir))
	Expect(os.RemoveAll(gardenHomeDir)).To(Succeed())
	Expect(os.RemoveAll(sessionDir)).To(Succeed())
})

func makeTempGardenHomeDir() string {
	dir, err := os.MkdirTemp("", "garden-*")
	Expect(err).NotTo(HaveOccurred())

	return dir
}
