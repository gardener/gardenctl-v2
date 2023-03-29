/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package target_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/gardener/gardenctl-v2/internal/fake"
)

func init() {
	utilruntime.Must(gardencorev1beta1.AddToScheme(scheme.Scheme))
}

func TestTarget(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Target Package Test Suite")
}

const gardenName = "testgarden"

var (
	ctx              context.Context
	cancel           context.CancelFunc
	gardenHomeDir    string
	sessionDir       string
	gardenKubeconfig string
)

var _ = BeforeSuite(func() {
	ctx, cancel = context.WithCancel(context.TODO())

	dir, err := os.MkdirTemp("", "garden-*")
	Expect(err).NotTo(HaveOccurred())
	sessionDir = filepath.Join(dir, uuid.New().String())
	Expect(os.MkdirAll(sessionDir, 0o700)).To(Succeed())
	gardenHomeDir = dir
	gardenKubeconfig = filepath.Join(gardenHomeDir, "kubeconfig.yaml")
	data, err := fake.NewConfigData(gardenName)
	Expect(err).NotTo(HaveOccurred())
	Expect(os.WriteFile(gardenKubeconfig, data, 0o600)).To(Succeed())
})

var _ = AfterSuite(func() {
	cancel()
	Expect(os.RemoveAll(gardenHomeDir)).To(Succeed())
})
