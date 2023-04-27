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

func sshCommandArguments(bastionAddress string, sshPrivateKeyFile PrivateKeyFile, nodeHostname string, nodePrivateKeyFiles []PrivateKeyFile) arguments {
	proxyCmdArgs := sshProxyCmdArguments(bastionAddress, sshPrivateKeyFile)

	args := []argument{
		{value: "-oStrictHostKeyChecking=no", shellEscapeDisabled: true},
		{value: "-oIdentitiesOnly=yes", shellEscapeDisabled: true},
	}

	for _, file := range nodePrivateKeyFiles {
		args = append(args, argument{value: fmt.Sprintf("-i%s", file)})
	}

	args = append(args, argument{value: fmt.Sprintf("-oProxyCommand=%s", proxyCmdArgs.String())})

	args = append(args, argument{value: fmt.Sprintf("%s@%s", SSHNodeUsername, nodeHostname)})

	return arguments{list: args}
}

func sshProxyCmdArguments(bastionAddress string, sshPrivateKeyFile PrivateKeyFile) arguments {
	args := []argument{
		{value: "ssh", shellEscapeDisabled: true},
		{value: "-W%h:%p", shellEscapeDisabled: true},
		{value: "-oStrictHostKeyChecking=no", shellEscapeDisabled: true},
	}

	if sshPrivateKeyFile != "" {
		args = append(args, argument{value: "-oIdentitiesOnly=yes", shellEscapeDisabled: true})
		args = append(args, argument{value: fmt.Sprintf("-i%s", sshPrivateKeyFile)})
	}

	args = append(args, argument{value: fmt.Sprintf("%s@%s", SSHBastionUsername, bastionAddress)})

	return arguments{list: args}
}
