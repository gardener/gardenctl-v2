/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package target

import (
	"context"
	"fmt"
	"strings"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/target"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"k8s.io/apimachinery/pkg/util/sets"
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
	var result sets.String

	switch kind {
	case TargetKindGarden:
		result, err = getGardenArguments(manager)
	case TargetKindProject:
		result, err = getProjectArguments(ctx, manager, currentTarget)
	case TargetKindSeed:
		result, err = getSeedArguments(ctx, manager, currentTarget)
	case TargetKindShoot:
		result, err = getShootArguments(ctx, manager, currentTarget)
	}

	return result.List(), nil
}

func getGardenArguments(manager target.Manager) (sets.String, error) {
	names := sets.NewString()
	for _, garden := range manager.Configuration().Gardens {
		names.Insert(garden.Name)
	}

	return names, nil
}

func getProjectArguments(ctx context.Context, manager target.Manager, t target.Target) (sets.String, error) {
	gardenClient, err := manager.GardenClient(t)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client for garden cluster %q: %w", t.GardenName(), err)
	}

	projectList := &gardencorev1beta1.ProjectList{}
	if err := gardenClient.List(ctx, projectList); err != nil {
		return nil, fmt.Errorf("failed to list projects on garden cluster %q: %w", t.GardenName(), err)
	}

	names := sets.NewString()
	for _, project := range projectList.Items {
		names.Insert(project.Name)
	}

	return names, nil
}

func getSeedArguments(ctx context.Context, manager target.Manager, t target.Target) (sets.String, error) {
	gardenClient, err := manager.GardenClient(t)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client for garden cluster %q: %w", t.GardenName(), err)
	}

	seedList := &gardencorev1beta1.SeedList{}
	if err := gardenClient.List(ctx, seedList); err != nil {
		return nil, fmt.Errorf("failed to list seeds on garden cluster %q: %w", t.GardenName(), err)
	}

	names := sets.NewString()
	for _, seed := range seedList.Items {
		names.Insert(seed.Name)
	}

	return names, nil
}

func getShootArguments(ctx context.Context, manager target.Manager, t target.Target) (sets.String, error) {
	gardenClient, err := manager.GardenClient(t)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client for garden cluster %q: %w", t.GardenName(), err)
	}

	var listOpt client.ListOption

	if t.ProjectName() != "" {
		project, err := util.ProjectForTarget(ctx, gardenClient, t)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch project: %w", err)
		}

		if project.Spec.Namespace == nil {
			return nil, nil
		}

		listOpt = &client.ListOptions{Namespace: *project.Spec.Namespace}
	} else if t.SeedName() != "" {
		listOpt = client.MatchingFields{gardencore.ShootSeedName: t.SeedName()}
	} else {
		return nil, nil
	}

	shootList := &gardencorev1beta1.ShootList{}
	if err := gardenClient.List(ctx, shootList, listOpt); err != nil {
		return nil, fmt.Errorf("failed to list shoots on garden cluster %q: %w", t.GardenName(), err)
	}

	names := sets.NewString()
	for _, shoot := range shootList.Items {
		names.Insert(shoot.Name)
	}

	return names, nil
}
