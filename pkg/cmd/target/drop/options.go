/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package drop

import (
	"strings"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
	commonTarget "github.com/gardener/gardenctl-v2/pkg/cmd/common/target"
	"github.com/gardener/gardenctl-v2/pkg/target"

	"github.com/spf13/cobra"
)

// Options is a struct to support drop command
type Options struct {
	base.Options

	// Kind is the target kind, for example "garden" or "seed"
	Kind commonTarget.TargetKind
}

// NewOptions returns initialized Options
func NewOptions(ioStreams util.IOStreams) *Options {
	return &Options{
		Options: base.Options{
			IOStreams: ioStreams,
		},
	}
}

// Complete adapts from the command line args to the data required.
func (o *Options) Complete(f util.Factory, cmd *cobra.Command, args []string, targetProvider *target.DynamicTargetProvider) error {
	if len(args) > 0 {
		o.Kind = commonTarget.TargetKind(strings.TrimSpace(args[0]))
	}

	return nil
}

// Validate validates the provided options
func (o *Options) Validate() error {
	if err := commonTarget.ValidateKind(o.Kind); err != nil {
		return err
	}

	return nil
}
