/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package ssh

import (
	"context"
	"os"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"golang.org/x/crypto/ssh"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/config"
	"github.com/gardener/gardenctl-v2/pkg/target"
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

func SetExecSSHCommand(f func(ctx context.Context, args []string, ioStreams util.IOStreams) error) {
	execSSHCommand = f
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

var (
	ValidateSSHPrivateKey = validateSSHPrivateKey
	ValidateSSHPublicKey  = validateSSHPublicKey
	ValidateHost          = validateHost
)

// CheckAccessRestrictions exposes the unexported checkAccessRestrictions method for tests.
func (o *SSHOptions) CheckAccessRestrictions(cfg *config.Config, gardenName string, tf target.TargetFlags, shoot *gardencorev1beta1.Shoot) (bool, error) {
	return o.checkAccessRestrictions(cfg, gardenName, tf, shoot)
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
	shellEscapeFn func(values ...interface{}) string,
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
			shellEscapeFn,
		),
	}
}
