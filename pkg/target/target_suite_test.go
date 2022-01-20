/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package target_test

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

func init() {
	utilruntime.Must(gardencorev1beta1.AddToScheme(scheme.Scheme))
}

func TestTarget(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Target Package Test Suite")
}

var (
	ctx    context.Context
	cancel context.CancelFunc
)

var _ = BeforeSuite(func() {
	ctx, cancel = context.WithCancel(context.TODO())

}, 60)

var _ = AfterSuite(func() {
	cancel()
}, 5)

func createTestKubeconfig(name string) []byte {
	config := clientcmdapi.NewConfig()
	config.CurrentContext = name
	data, err := clientcmd.Write(*config)
	Expect(err).NotTo(HaveOccurred())

	return data
}
