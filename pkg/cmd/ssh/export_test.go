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
)

func SetBastionAvailabilityChecker(f func(hostname string, port string, privateKey []byte) error) {
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

func SetExecCommand(f func(ctx context.Context, command string, args []string, ioStreams util.IOStreams) error) {
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

func SetWaitForSignal(f func(ctx context.Context, o *SSHOptions, signalChan <-chan struct{})) {
	waitForSignal = f
}

type TestArguments struct {
	arguments
}

func SSHCommandArguments(
	bastionHost string,
	bastionPort string,
	sshPrivateKeyFile PrivateKeyFile,
	bastionUserKnownHostsFiles []string,
	nodeHostname string,
	nodePrivateKeyFiles []PrivateKeyFile,
) TestArguments {
	return TestArguments{
		sshCommandArguments(
			bastionHost,
			bastionPort,
			sshPrivateKeyFile,
			bastionUserKnownHostsFiles,
			nodeHostname,
			nodePrivateKeyFiles,
		),
	}
}
