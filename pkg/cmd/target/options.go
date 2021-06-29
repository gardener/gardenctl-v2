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
func (o *Options) Complete(f util.Factory, cmd *cobra.Command, args []string) error {
	manager, err := f.Manager()
	if err != nil {
		return err
	}

	if len(args) > 0 {
		o.Kind = TargetKind(strings.TrimSpace(args[0]))
	}

	if len(args) > 1 {
		o.TargetName = strings.TrimSpace(args[1])
	}

	// as loading from file is disabled, this target will only represent whatever
	// CLI flags the user has specified (--garden, --project etc.)
	currentTarget, err := manager.CurrentTarget()
	if err != nil {
		return err
	}

	// in case the user didn't specify all arguments, we fake them
	// by looking at the CLI flags instead. This allows the user to do a
	// "gardenctl target --shoot foo" and still have it working; however
	// they must be specifying either arguments or flags, just calling
	// this command without anything results in an error.
	if currentTarget != nil {
		if o.Kind == "" {
			if currentTarget.ShootName() != "" {
				o.Kind = TargetKindShoot
			} else if currentTarget.ProjectName() != "" {
				o.Kind = TargetKindProject
			} else if currentTarget.SeedName() != "" {
				o.Kind = TargetKindSeed
			} else if currentTarget.GardenName() != "" {
				o.Kind = TargetKindGarden
			}
		}

		if o.TargetName == "" {
			switch o.Kind {
			case TargetKindGarden:
				o.TargetName = currentTarget.GardenName()
			case TargetKindProject:
				o.TargetName = currentTarget.ProjectName()
			case TargetKindSeed:
				o.TargetName = currentTarget.SeedName()
			case TargetKindShoot:
				o.TargetName = currentTarget.ShootName()
			}
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
