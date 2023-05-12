/*
SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package providerenv

import (
	"text/template"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	corev1 "k8s.io/api/core/v1"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/ac"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
	"github.com/gardener/gardenctl-v2/pkg/env"
)

var (
	ParseGCPCredentials = parseGCPCredentials
	GetKeyStoneURL      = getKeyStoneURL
	GetProviderCLI      = getProviderCLI
	GetTargetFlags      = getTargetFlags
)

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

func (o *TestOptions) PrintProviderEnv(shoot *gardencorev1beta1.Shoot, secret *corev1.Secret, cloudProfile *gardencorev1beta1.CloudProfile, messages ...*ac.AccessRestrictionMessage) error {
	return printProviderEnv(&o.options, shoot, secret, cloudProfile, messages)
}

func (o *TestOptions) GenerateMetadata(cli string) map[string]interface{} {
	return generateMetadata(&o.options, cli)
}

func (o *TestOptions) String() string {
	return o.out.String()
}

type TestTemplate interface {
	env.Template
	Delegate() *template.Template
}
