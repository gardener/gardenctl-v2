/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package env

import (
	"fmt"
)

// Shell represents the type of shell.
type Shell string

const (
	bash       Shell = "bash"
	zsh        Shell = "zsh"
	fish       Shell = "fish"
	powershell Shell = "powershell"
)

// ValidShells returns a list of supported shell types.
func ValidShells() []Shell {
	return []Shell{bash, zsh, fish, powershell}
}

// EvalCommand returns the script that evaluates the given command.
func (s Shell) EvalCommand(cmd string) string {
	var format string

	switch s {
	case fish:
		format = "eval (%s)"
	case powershell:
		// Invoke-Expression cannot execute multi-line functions!!!
		format = "& %s | Invoke-Expression"
	default:
		format = "eval \"$(%s)\""
	}

	return fmt.Sprintf(format, cmd)
}

// Prompt returns the typical prompt for a given os.
func (s Shell) Prompt(goos string) string {
	switch s {
	case powershell:
		if goos == "windows" {
			return "PS C:\\> "
		}

		return "PS /> "
	default:
		return "$ "
	}
}

// Config returns the typical config file for the shell.
func (s Shell) Config() string {
	switch s {
	case bash:
		return "~/.bashrc"
	case zsh:
		return "~/.zshrc"
	case fish:
		return "~/.config/fish/config.fish"
	case powershell:
		return "$profile"
	default:
		return ""
	}
}

// Validate checks if the shell is valid.
func (s Shell) Validate() error {
	for _, shell := range ValidShells() {
		if s == shell {
			return nil
		}
	}

	return fmt.Errorf("invalid shell given, must be one of %v", ValidShells())
}
