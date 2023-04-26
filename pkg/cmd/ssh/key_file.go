/*
SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package ssh

import (
	"fmt"

	"github.com/spf13/pflag"
)

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
