/*
SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package ssh

import (
	"fmt"

	"github.com/spf13/pflag"
)

// StrictHostKeyChecking defines the type for strict host key checking options.
type StrictHostKeyChecking string

const (
	StrictHostKeyCheckingYes       StrictHostKeyChecking = "yes"
	StrictHostKeyCheckingAsk       StrictHostKeyChecking = "ask"
	StrictHostKeyCheckingAcceptNew StrictHostKeyChecking = "accept-new"
	StrictHostKeyCheckingNo        StrictHostKeyChecking = "no"
	StrictHostKeyCheckingOff       StrictHostKeyChecking = "off"
)

var (
	_ pflag.Value  = (*StrictHostKeyChecking)(nil)
	_ fmt.Stringer = (*StrictHostKeyChecking)(nil)
)

func (s *StrictHostKeyChecking) Set(value string) error {
	switch value {
	case string(StrictHostKeyCheckingYes),
		string(StrictHostKeyCheckingAsk),
		string(StrictHostKeyCheckingAcceptNew),
		string(StrictHostKeyCheckingNo),
		string(StrictHostKeyCheckingOff):
		*s = StrictHostKeyChecking(value)
		return nil
	default:
		return fmt.Errorf("invalid value %q for StrictHostKeyChecking. Valid options are 'yes', 'ask', 'accept-new', 'no', or 'off'", value)
	}
}

func (s *StrictHostKeyChecking) Type() string {
	return "string"
}

func (s *StrictHostKeyChecking) String() string {
	return string(*s)
}

type PublicKeyFile string

var (
	_ pflag.Value  = (*PublicKeyFile)(nil)
	_ fmt.Stringer = (*PublicKeyFile)(nil)
)

func (s *PublicKeyFile) Set(val string) error {
	*s = PublicKeyFile(val)

	return nil
}

func (s *PublicKeyFile) Type() string {
	return "string"
}

func (s *PublicKeyFile) String() string {
	return string(*s)
}

type PrivateKeyFile string

var (
	_ pflag.Value  = (*PrivateKeyFile)(nil)
	_ fmt.Stringer = (*PrivateKeyFile)(nil)
)

func (s *PrivateKeyFile) Set(val string) error {
	*s = PrivateKeyFile(val)

	return nil
}

func (s *PrivateKeyFile) Type() string {
	return "string"
}

func (s *PrivateKeyFile) String() string {
	return string(*s)
}
