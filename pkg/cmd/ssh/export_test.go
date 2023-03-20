/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package ssh

import (
	"context"
	"os"
	"time"

	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

func SetWaitForSignal(f func(ctx context.Context, o *SSHOptions, shootClient client.Client, bastion *operationsv1alpha1.Bastion, nodeHostname string, nodePrivateKeyFiles []string, signalChan <-chan struct{}) error) {
	waitForSignal = f
}
