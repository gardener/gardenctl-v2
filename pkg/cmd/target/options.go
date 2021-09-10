/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package target

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
	commonTarget "github.com/gardener/gardenctl-v2/pkg/cmd/common/target"
	"github.com/gardener/gardenctl-v2/pkg/target"

	"github.com/spf13/cobra"
)

func validateKind(kind commonTarget.TargetKind) error {
	for _, k := range commonTarget.AllTargetKinds {
		if k == kind {
			return nil
		}
	}

	return fmt.Errorf("invalid target kind given, must be one of %v", commonTarget.AllTargetKinds)
}

// Options is a struct to support target command
type Options struct {
	base.Options

	// Kind is the target kind, for example "garden" or "seed"
	Kind commonTarget.TargetKind
	// TargetName is the object name of the targeted kind
	TargetName string
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

	if len(args) > 1 {
		o.TargetName = strings.TrimSpace(args[1])
	}

	if o.Kind == "" {
		if targetProvider.ShootNameFlag != "" {
			o.Kind = commonTarget.TargetKindShoot
		} else if targetProvider.ProjectNameFlag != "" {
			o.Kind = commonTarget.TargetKindProject
		} else if targetProvider.SeedNameFlag != "" {
			o.Kind = commonTarget.TargetKindSeed
		} else if targetProvider.GardenNameFlag != "" {
			o.Kind = commonTarget.TargetKindGarden
		}
	}

	if o.TargetName == "" {
		switch o.Kind {
		case commonTarget.TargetKindGarden:
			o.TargetName = targetProvider.GardenNameFlag
		case commonTarget.TargetKindProject:
			o.TargetName = targetProvider.ProjectNameFlag
		case commonTarget.TargetKindSeed:
			o.TargetName = targetProvider.SeedNameFlag
		case commonTarget.TargetKindShoot:
			o.TargetName = targetProvider.ShootNameFlag
		}
	}

	return nil
}

// Validate validates the provided options
func (o *Options) Validate() error {
	// reject flag/arg-less invocations
	if o.Kind == "" || o.TargetName == "" {
		return errors.New("no target specified")
	}

	if err := validateKind(o.Kind); err != nil {
		return err
	}

	return nil
}
