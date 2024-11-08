/*
SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package flags_test

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
	"k8s.io/utils/ptr"

	"github.com/gardener/gardenctl-v2/pkg/config"
)

var (
	gardenHomeDir string
	sessionDir    string
	configFile    string
	cfg           *config.Config
)

const (
	envSessionID     = "GCTL_SESSION_ID"
	envGardenHomeDir = "GCTL_HOME"
	configName       = "gardenctl-v2"
)

func init() {
	utilruntime.Must(gardencorev1beta1.AddToScheme(scheme.Scheme))
	utilruntime.Must(operationsv1alpha1.AddToScheme(scheme.Scheme))
}

func TestCommand(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Flags Test Suite")
}

var _ = BeforeSuite(func() {
	gardenHomeDir = makeTempGardenHomeDir()
	Expect(os.Setenv(envGardenHomeDir, gardenHomeDir)).To(Succeed())
	configFile = filepath.Join(gardenHomeDir, configName+".yaml")
	sessionID := uuid.New().String()
	Expect(os.Setenv(envSessionID, sessionID)).To(Succeed())
	sessionDir = filepath.Join(os.TempDir(), "garden", "sessions", sessionID)
	Expect(os.MkdirAll(sessionDir, os.ModePerm))
	cfg = &config.Config{
		Filename:       configFile,
		LinkKubeconfig: ptr.To(false),
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
	Expect(os.Unsetenv(envSessionID))
	Expect(os.Unsetenv(envGardenHomeDir))
	Expect(os.RemoveAll(gardenHomeDir)).To(Succeed())
	Expect(os.RemoveAll(sessionDir)).To(Succeed())
})

func makeTempGardenHomeDir() string {
	dir, err := os.MkdirTemp("", "garden-*")
	Expect(err).NotTo(HaveOccurred())

	return dir
}
