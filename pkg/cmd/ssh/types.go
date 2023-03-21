/*
SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package ssh

import (
	"fmt"

	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
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

var _ fmt.Stringer = (*PrivateKeyFile)(nil)

func (s *PrivateKeyFile) String() string {
	return string(*s)
}

// Address holds information about an IP address and hostname.
type Address struct {
	Hostname string `json:"hostname"`
	IP       string `json:"ip"`
}

var _ fmt.Stringer = &Address{}
func toAdress(ingress *corev1.LoadBalancerIngress) *Address {
	if ingress == nil {
		return nil
	}

	return &Address{
		Hostname: ingress.Hostname,
		IP:       ingress.IP,
	}
}

func (a *Address) String() string {
	switch {
	case a.Hostname != "" && a.IP != "":
		return fmt.Sprintf("%s (%s)", a.IP, a.Hostname)
	case a.Hostname != "":
		return a.Hostname
	default:
		return a.IP
	}
}
