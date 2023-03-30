/*
SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package gardenclient

import "k8s.io/apimachinery/pkg/runtime"

type ExecPluginConfig struct {
	execPluginConfig
}

func (e *ExecPluginConfig) ToRuntimeObject() runtime.Object {
	return &e.execPluginConfig
}
