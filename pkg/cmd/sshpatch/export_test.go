/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package sshpatch

import (
	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
	"github.com/gardener/gardenctl-v2/pkg/cmd/ssh"
)

type TestOptions struct {
	options
	Out *util.SafeBytesBuffer
}

func NewTestOptions() *TestOptions {
	streams, _, out, _ := util.NewTestIOStreams()

	return &TestOptions{
		options: options{
			AccessConfig: ssh.AccessConfig{},
			Options: base.Options{
				IOStreams: streams,
			},
		},
		Out: out,
	}
}
