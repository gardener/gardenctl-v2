/*
SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package resolve

import (
	"github.com/gardener/gardenctl-v2/internal/util"
)

type TestOptions struct {
	options
	out *util.SafeBytesBuffer
}

func NewOptions(kind Kind) *TestOptions {
	streams, _, out, _ := util.NewTestIOStreams()

	return &TestOptions{
		options: *newOptions(streams, kind),
		out:     out,
	}
}

func (o *TestOptions) String() string {
	return o.out.String()
}
