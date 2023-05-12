/*
SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package kubectlenv

import (
	"text/template"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
	"github.com/gardener/gardenctl-v2/pkg/env"
)

var GetTargetFlags = getTargetFlags

type TestOptions struct {
	options
	out *util.SafeBytesBuffer
}

func NewOptions() *TestOptions {
	streams, _, out, _ := util.NewTestIOStreams()

	return &TestOptions{
		options: options{
			Options: base.Options{
				IOStreams: streams,
			},
		},
		out: out,
	}
}

func (o *TestOptions) GenerateMetadata() map[string]interface{} {
	return generateMetadata(&o.options)
}

func (o *TestOptions) String() string {
	return o.out.String()
}

type TestTemplate interface {
	env.Template
	Delegate() *template.Template
}
