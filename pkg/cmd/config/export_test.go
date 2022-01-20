/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package config

import (
	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
)

type TestOptions struct {
	options
	out *util.SafeBytesBuffer
}

func NewOptions(cmd string) *TestOptions {
	streams, _, out, _ := util.NewTestIOStreams()

	return &TestOptions{
		options: options{
			Options: base.Options{
				IOStreams: streams,
			},
			Command: cmd,
		},
		out: out,
	}
}

func (o *TestOptions) String() string {
	return o.out.String()
}
