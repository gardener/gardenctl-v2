/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package ssh

import (
	"context"
	"os"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/gardener/gardenctl-v2/internal/util"
)

func SetBastionAvailabilityChecker(f func(hostname string, port string, privateKey []byte, hostKeyCallback ssh.HostKeyCallback) error) {
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

// SetRSAKeyBitsForTest sets the RSA key size to use for testing (smaller keys generate faster).
func SetRSAKeyBitsForTest(bits int) {
	rsaKeyBits = bits
}

type TestArguments struct {
	arguments
}

func SSHCommandArguments(
	bastionHost string,
	bastionPort string,
	sshPrivateKeyFile PrivateKeyFile,
	bastionUserKnownHostsFiles []string,
	bastionStrictHostKeyChecking StrictHostKeyChecking,
	nodeUserKnownHostsFiles []string,
	nodeStrictHostKeyChecking StrictHostKeyChecking,
	nodeHostname string,
	nodePrivateKeyFiles []PrivateKeyFile,
	user string,
) TestArguments {
	return TestArguments{
		sshCommandArguments(
			bastionHost,
			bastionPort,
			sshPrivateKeyFile,
			bastionUserKnownHostsFiles,
			bastionStrictHostKeyChecking,
			nodeUserKnownHostsFiles,
			nodeStrictHostKeyChecking,
			nodeHostname,
			nodePrivateKeyFiles,
			user,
		),
	}
}
