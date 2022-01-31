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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
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
	gardenKubeconfig string
)

var _ = BeforeSuite(func() {
	ctx, cancel = context.WithCancel(context.TODO())

	dir, err := os.MkdirTemp("", "garden-*")
	Expect(err).NotTo(HaveOccurred())
	gardenHomeDir = dir
	gardenKubeconfig = filepath.Join(gardenHomeDir, "kubeconfig.yaml")
	data := createTestKubeconfig(gardenName)
	Expect(os.WriteFile(gardenKubeconfig, data, 0600)).To(Succeed())
}, 60)

var _ = AfterSuite(func() {
	cancel()
	Expect(os.RemoveAll(gardenHomeDir)).To(Succeed())
}, 5)

func createTestKubeconfig(name string) []byte {
	config := clientcmdapi.NewConfig()
	config.Clusters["cluster"] = &clientcmdapi.Cluster{
		Server:                "https://kubernetes:6443/",
		InsecureSkipTLSVerify: true,
	}
	config.AuthInfos["user"] = &clientcmdapi.AuthInfo{
		Token: "token",
	}
	config.Contexts[name] = &clientcmdapi.Context{
		Namespace: "default",
		AuthInfo:  "user",
		Cluster:   "cluster",
	}
	config.CurrentContext = name
	data, err := clientcmd.Write(*config)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	return data
}
