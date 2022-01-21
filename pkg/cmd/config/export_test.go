/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package config

import (
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
)

type TestOptions struct {
	options
}

func NewOptions(cmd string) *TestOptions {
	return &TestOptions{
		options: options{
			Options: base.Options{},
			Command: cmd,
		},
	}
}
