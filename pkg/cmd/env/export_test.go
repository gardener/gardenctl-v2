/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package env

import (
	"text/template"

	"github.com/gardener/gardenctl-v2/pkg/ac"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	corev1 "k8s.io/api/core/v1"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
)

var (
	ValidShells         = validShells
	ParseGCPCredentials = parseGCPCredentials
	GetKeyStoneURL      = getKeyStoneURL
	GetProviderCLI      = getProviderCLI
	GetTargetFlags      = getTargetFlags
	ParseFile           = parseFile
	NewTemplate         = newTemplate
)

type RCOptions struct {
	rcOptions
}

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

func (o *TestOptions) ExecTmpl(shoot *gardencorev1beta1.Shoot, secret *corev1.Secret, cloudProfile *gardencorev1beta1.CloudProfile, messages ...*ac.AccessRestrictionMessage) error {
	return execTmpl(&o.options, shoot, secret, cloudProfile, messages)
}

func (o *TestOptions) GenerateMetadata() map[string]interface{} {
	return generateMetadata(&o.options)
}

func (o *TestOptions) String() string {
	return o.out.String()
}

type TestTemplate interface {
	Template
	Delegate() *template.Template
}

var _ TestTemplate = &templateImpl{}
