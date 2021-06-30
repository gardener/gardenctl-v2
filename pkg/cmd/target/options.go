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
	"github.com/gardener/gardenctl-v2/pkg/target"

	"github.com/spf13/cobra"
)

// TargetKind is representing the type of things that can be targeted
// by this cobra command. While this may sound stuttery, the alternative
// of just calling it "Kind" is even worse, hence the nolint.
// nolint
type TargetKind string

const (
	TargetKindGarden  TargetKind = "garden"
	TargetKindProject TargetKind = "project"
	TargetKindSeed    TargetKind = "seed"
	TargetKindShoot   TargetKind = "shoot"
)

var (
	AllTargetKinds = []TargetKind{TargetKindGarden, TargetKindProject, TargetKindSeed, TargetKindShoot}
)

func validateKind(kind TargetKind) error {
	for _, k := range AllTargetKinds {
		if k == kind {
			return nil
		}
	}

	return fmt.Errorf("invalid target kind given, must be one of %v", AllTargetKinds)
}

// Options is a struct to support target command
type Options struct {
	base.Options

	// Kind is the target kind, for example "garden" or "seed"
	Kind TargetKind
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
		o.Kind = TargetKind(strings.TrimSpace(args[0]))
	}

	if len(args) > 1 {
		o.TargetName = strings.TrimSpace(args[1])
	}

	if o.Kind == "" {
		if targetProvider.ShootNameFlag != "" {
			o.Kind = TargetKindShoot
		} else if targetProvider.ProjectNameFlag != "" {
			o.Kind = TargetKindProject
		} else if targetProvider.SeedNameFlag != "" {
			o.Kind = TargetKindSeed
		} else if targetProvider.GardenNameFlag != "" {
			o.Kind = TargetKindGarden
		}
	}

	if o.TargetName == "" {
		switch o.Kind {
		case TargetKindGarden:
			o.TargetName = targetProvider.GardenNameFlag
		case TargetKindProject:
			o.TargetName = targetProvider.ProjectNameFlag
		case TargetKindSeed:
			o.TargetName = targetProvider.SeedNameFlag
		case TargetKindShoot:
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
