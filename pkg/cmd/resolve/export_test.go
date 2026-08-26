/*
SPDX-FileCopyrightText: Copyright Contributors to the Gardener project

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
