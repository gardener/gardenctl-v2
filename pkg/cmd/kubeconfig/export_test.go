/*
SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package kubeconfig

import (
	"github.com/gardener/gardenctl-v2/internal/util"
)

type TestOptions struct {
	options
	out *util.SafeBytesBuffer
}

func NewOptions() *TestOptions {
	streams, _, out, _ := util.NewTestIOStreams()

	return &TestOptions{
		options: *newOptions(streams),
		out:     out,
	}
}

func (o *TestOptions) String() string {
	return o.out.String()
}
