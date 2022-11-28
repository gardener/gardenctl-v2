/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package ssh

import (
	"context"
	"os"
	"time"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
)

func SetBastionAvailabilityChecker(f func(hostname string, privateKey []byte) error) {
	bastionAvailabilityChecker = f
}

func SetTempFileCreator(f func() (*os.File, error)) {
	tempFileCreator = f
}

func SetBastionNameProvider(f func() (string, error)) {
	bastionNameProvider = f
}

func SetCreateSignalChannel(f func() chan os.Signal) {
	createSignalChannel = f
}

func SetExecCommand(f func(ctx context.Context, command string, args []string, o *SSHOptions) error) {
	execCommand = f
}

func SetPollBastionStatusInterval(d time.Duration) {
	pollBastionStatusInterval = d
}

func SetKeepAliveInterval(d time.Duration) {
	keepAliveIntervalMutex.Lock()
	defer keepAliveIntervalMutex.Unlock()

	keepAliveInterval = d
}

type TestSSHPatchUtils struct {
	sshPatchUtils
}

func NewTestSSHPatchUtils() *TestSSHPatchUtils {
	return &TestSSHPatchUtils{
		sshPatchUtils: &sshPatchUtilsImpl{},
	}
}

type TestSSHPatchOptions struct {
	sshPatchOptions
	Out     *util.SafeBytesBuffer
	Streams util.IOStreams
}

func NewTestSSHPatchOptions() *TestSSHPatchOptions {
	streams, _, out, _ := util.NewTestIOStreams()

	return &TestSSHPatchOptions{
		sshPatchOptions: sshPatchOptions{
			sshBaseOptions: sshBaseOptions{
				Options: base.Options{
					IOStreams: streams,
				},
			},
			Utils: &sshPatchUtilsImpl{},
		},
		Out:     out,
		Streams: streams,
	}
}

type TestSSHPatchCompletions struct {
	sshPatchCompletions
}

func NewTestSSHPatchCompletions() *TestSSHPatchCompletions {
	return &TestSSHPatchCompletions{
		sshPatchCompletions: sshPatchCompletions{
			Utils: &sshPatchUtilsImpl{},
		},
	}
}
