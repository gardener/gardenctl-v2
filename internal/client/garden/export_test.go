/*
SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package garden

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type ExecPluginConfig struct {
	execPluginConfig
}

func (e *ExecPluginConfig) ToRuntimeObject() runtime.Object {
	return &e.execPluginConfig
}

func ValidateObjectMetadata(obj metav1.Object) error {
	return validateObjectMetadata(obj)
}

func ValidateBastionIngress(ingress *corev1.LoadBalancerIngress) error {
	return validateBastionIngress(ingress)
}
