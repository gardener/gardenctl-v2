/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package target

import (
	"strings"

	"github.com/gardener/gardenctl-v2/internal/util"
)

func validArgsFunction(f util.Factory, o *Options, args []string, toComplete string) ([]string, error) {
	if len(args) == 0 {
		return []string{
			string(TargetKindGarden),
			string(TargetKindProject),
			string(TargetKindSeed),
			string(TargetKindShoot),
		}, nil
	}

	kind := TargetKind(strings.TrimSpace(args[0]))
	if err := validateKind(kind); err != nil {
		return nil, err
	}

	manager, err := f.Manager()
	if err != nil {
		return nil, err
	}

	// NB: this uses the DynamicTargetProvider from the root cmd and
	// is therefore aware of flags like --garden; the goal here is to
	// allow the user to type "gardenctl target --garden [tab][select] --project [tab][select] shoot [tab][select]"
	currentTarget, err := manager.CurrentTarget()
	if err != nil {
		return nil, err
	}

	ctx := f.Context()

	var result []string

	switch kind {
	case TargetKindGarden:
		result, err = util.GardenNames(manager)
	case TargetKindProject:
		result, err = util.ProjectNamesForTarget(ctx, manager, currentTarget)
	case TargetKindSeed:
		result, err = util.SeedNamesForTarget(ctx, manager, currentTarget)
	case TargetKindShoot:
		result, err = util.ShootNamesForTarget(ctx, manager, currentTarget)
	}

	return result, err
}
