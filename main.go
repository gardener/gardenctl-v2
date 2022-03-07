/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/
package main

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/gardener/gardenctl-v2/pkg/cmd"
)

func main() {
	utilruntime.Must(gardencorev1beta1.AddToScheme(scheme.Scheme))
	utilruntime.Must(operationsv1alpha1.AddToScheme(scheme.Scheme))
	utilruntime.Must(seedmanagementv1alpha1.AddToScheme(scheme.Scheme))

	cmd.Execute()
}
