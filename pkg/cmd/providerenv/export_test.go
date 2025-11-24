/*
SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package providerenv

import (
	"context"
	"text/template"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	corev1 "k8s.io/api/core/v1"

	clientgarden "github.com/gardener/gardenctl-v2/internal/client/garden"
	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/ac"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
	"github.com/gardener/gardenctl-v2/pkg/env"
)

var (
	GetKeyStoneURL = getKeyStoneURL
	GetProviderCLI = getProviderCLI
	GetTargetFlags = getTargetFlags
)

// GetPrefix returns the prefix field from TempDataWriter for testing.
func (t *TempDataWriter) GetPrefix() string {
	return t.prefix
}

// GetPrefix returns the prefix field from CleanupDataWriter for testing.
func (c *CleanupDataWriter) GetPrefix() string {
	return c.prefix
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

func (o *TestOptions) PrintProviderEnv(ctx context.Context, client clientgarden.Client, shoot *gardencorev1beta1.Shoot, credentialsRef corev1.ObjectReference, cloudProfile *clientgarden.CloudProfileUnion, messages ac.AccessRestrictionMessages) error {
	return printProviderEnv(&o.options, ctx, client, shoot, credentialsRef, cloudProfile, messages)
}

func (o *TestOptions) GenerateMetadata(cli string, credentialKind string) map[string]interface{} {
	return generateMetadata(&o.options, cli, credentialKind)
}

func (o *TestOptions) String() string {
	return o.out.String()
}

type TestTemplate interface {
	env.Template
	Delegate() *template.Template
}
