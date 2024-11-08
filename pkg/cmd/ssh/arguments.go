/*
SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package ssh

import (
	"fmt"
	"strings"

	"github.com/gardener/gardenctl-v2/internal/util"
)

type arguments struct {
	list []argument
}

var _ fmt.Stringer = (*arguments)(nil)

type argument struct {
	value               string
	shellEscapeDisabled bool
}

// String returns a shell escaped arguments string.
func (a *arguments) String() string {
	var sb strings.Builder

	for i, arg := range a.list {
		if i > 0 {
			sb.WriteString(" ")
		}

		value := arg.value
		if !arg.shellEscapeDisabled {
			value = util.ShellEscape(value)
		}

		sb.WriteString(value)
	}

	return sb.String()
}

func userKnownHostsFilesArgument(userKnownHostsFiles []string) *argument {
	if len(userKnownHostsFiles) == 0 {
		return nil
	}

	var escapedHostsFiles []string
	for _, path := range userKnownHostsFiles {
		escapedHostsFiles = append(escapedHostsFiles, util.ShellEscape(path))
	}

	userKnownHostsFilesValue := strings.Join(escapedHostsFiles, " ")

	return &argument{value: fmt.Sprintf("-oUserKnownHostsFile=%s", userKnownHostsFilesValue)}
}

func sshCommandArguments(
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
) arguments {
	bastionUserKnownHostsFilesArg := userKnownHostsFilesArgument(bastionUserKnownHostsFiles)
	nodeUserKnownHostsFilesArg := userKnownHostsFilesArgument(nodeUserKnownHostsFiles)

	proxyCmdArgs := sshProxyCmdArguments(
		bastionHost,
		bastionPort,
		sshPrivateKeyFile,
		bastionUserKnownHostsFilesArg,
		bastionStrictHostKeyChecking,
	)

	args := []argument{
		{value: "-oIdentitiesOnly=yes", shellEscapeDisabled: true},
		{value: fmt.Sprintf("-oStrictHostKeyChecking=%s", nodeStrictHostKeyChecking), shellEscapeDisabled: true},
	}

	if nodeUserKnownHostsFilesArg != nil {
		args = append(args, *nodeUserKnownHostsFilesArg)
	}

	for _, file := range nodePrivateKeyFiles {
		args = append(args, argument{value: fmt.Sprintf("-i%s", file)})
	}

	args = append(args, argument{value: fmt.Sprintf("-oProxyCommand=%s", proxyCmdArgs.String())})

	args = append(args, argument{value: fmt.Sprintf("%s@%s", user, nodeHostname)})

	return arguments{list: args}
}

func sshProxyCmdArguments(
	bastionHost string,
	bastionPort string,
	sshPrivateKeyFile PrivateKeyFile,
	userKnownHostsFileArg *argument,
	bastionStrictHostKeyChecking StrictHostKeyChecking,
) arguments {
	args := []argument{
		{value: "ssh", shellEscapeDisabled: true},
		{value: "-W%h:%p", shellEscapeDisabled: true},
		{value: fmt.Sprintf("-oStrictHostKeyChecking=%s", bastionStrictHostKeyChecking), shellEscapeDisabled: true},
	}

	if sshPrivateKeyFile != "" {
		args = append(args, argument{value: "-oIdentitiesOnly=yes", shellEscapeDisabled: true})
		args = append(args, argument{value: fmt.Sprintf("-i%s", sshPrivateKeyFile)})
	}

	if userKnownHostsFileArg != nil {
		args = append(args, *userKnownHostsFileArg)
	}

	args = append(args, argument{value: fmt.Sprintf("%s@%s", SSHBastionUsername, bastionHost)})

	if bastionPort != "" {
		args = append(args, argument{value: fmt.Sprintf("-p%s", bastionPort)})
	}

	return arguments{list: args}
}
