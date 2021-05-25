/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/
package main

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"

	"github.com/gardener/gardenctl-v2/pkg/cmd/root"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
)

func main() {
	utilruntime.Must(gardencorev1beta1.AddToScheme(scheme.Scheme))

	root.Execute()
}
