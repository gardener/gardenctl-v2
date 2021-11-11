/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package cloudenv

import (
	"text/template"

	corev1 "k8s.io/api/core/v1"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

var (
	ValidShells       = validShells
	Aliases           = aliases
	MatchAll          = matchAll
	ParseCredentials  = parseCredentials
	GenerateUsageHint = generateUsageHint
	DetectShell       = detectShell
	ValidateShell     = validateShell
)

func NewTestOptions() *TestOptions {
	out := &util.SafeBytesBuffer{}
	streams := util.IOStreams{Out: out}
	baseOptions := base.Options{IOStreams: streams}

	return &TestOptions{
		cmdOptions: cmdOptions{Options: baseOptions},
		out:        out,
	}
}

type TestOptions struct {
	cmdOptions
	out *util.SafeBytesBuffer
}

func (o *TestOptions) ExecTmpl(shoot *gardencorev1beta1.Shoot, secret *corev1.Secret) error {
	return o.cmdOptions.execTmpl(shoot, secret)
}

func (o *TestOptions) ParseTmpl(name string) (*template.Template, error) {
	return o.cmdOptions.parseTmpl(name)
}

func (o *TestOptions) Out() string {
	return o.out.String()
}
