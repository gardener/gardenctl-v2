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
	if len(args) != 2 {
		return errors.New("expected exactly 2 arguments")
	}

	o.Kind = TargetKind(strings.TrimSpace(args[0]))
	o.TargetName = strings.TrimSpace(args[1])

	return nil
}

// Validate validates the provided options
func (o *Options) Validate() error {
	if err := validateKind(o.Kind); err != nil {
		return err
	}

	if len(o.TargetName) == 0 {
		return errors.New("target name must not be empty")
	}

	return nil
}
