/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package sshpatch

import (
	"context"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
	"github.com/gardener/gardenctl-v2/pkg/cmd/ssh"
	"github.com/gardener/gardenctl-v2/pkg/target"
)

type TestUserBastionListPatcherImpl struct {
	userBastionListPatcherImpl
}

func NewTestUserBastionPatchLister(manager target.Manager) *TestUserBastionListPatcherImpl {
	target, _ := manager.CurrentTarget()
	gc, _ := manager.GardenClient(target.GardenName())
	clientConfig, _ := manager.ClientConfig(context.Background(), target)

	return &TestUserBastionListPatcherImpl{
		userBastionListPatcherImpl: userBastionListPatcherImpl{
			gardenClient: gc,
			target:       target,
			clientConfig: clientConfig,
		},
	}
}

type TestOptions struct {
	options
	Out     *util.SafeBytesBuffer
	Streams util.IOStreams
}

func NewTestOptions() *TestOptions {
	streams, _, out, _ := util.NewTestIOStreams()

	return &TestOptions{
		options: options{
			BaseOptions: ssh.BaseOptions{
				Options: base.Options{
					IOStreams: streams,
				},
			},
		},
		Out:     out,
		Streams: streams,
	}
}

//nolint:revive
func NewTestCompletions() *completions {
	return newCompletions()
}
