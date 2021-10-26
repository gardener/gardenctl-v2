/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package util

import (
	"context"
	"errors"
	"fmt"

	"github.com/gardener/gardenctl-v2/internal/gardenclient"

	"github.com/gardener/gardenctl-v2/pkg/target"

	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// ShootForTarget returns the targeted shoot, if a shoot cluster is targeted,
// otherwise an error.
func ShootForTarget(ctx context.Context, gardenClient gardenclient.Client, t target.Target) (*gardencorev1beta1.Shoot, error) {
	if t.ProjectName() != "" {
		return shootForTargetViaProject(ctx, gardenClient, t)
	} else if t.SeedName() != "" {
		return shootForTargetViaSeed(ctx, gardenClient, t)
	}

	return nil, errors.New("invalid target, must have either project or seed specified for targeting a shoot")
}

func shootForTargetViaProject(ctx context.Context, gardenClient gardenclient.Client, t target.Target) (*gardencorev1beta1.Shoot, error) {
	project, err := ProjectForTarget(ctx, gardenClient, t)
	if err != nil {
		return nil, fmt.Errorf("invalid project %q: %v", t.ProjectName(), err)
	}

	if project.Spec.Namespace == nil || *project.Spec.Namespace == "" {
		return nil, fmt.Errorf("project %q has not yet been fully created", t.ProjectName())
	}

	// fetch shoot from project namespace
	shoot, err := gardenClient.GetShoot(ctx, *project.Spec.Namespace, t.ShootName())
	if err != nil {
		return nil, err
	}

	return shoot, nil
}

func shootForTargetViaSeed(ctx context.Context, gardenClient gardenclient.Client, t target.Target) (*gardencorev1beta1.Shoot, error) {
	seed, err := SeedForTarget(ctx, gardenClient, t)
	if err != nil {
		return nil, fmt.Errorf("invalid seed %q: %v", t.SeedName(), err)
	}

	shoot, err := gardenClient.GetShootBySeed(ctx, seed.Name, t.ShootName())
	if err != nil {
		return nil, err
	}

	return shoot, nil
}

// ShootsForTarget returns all possible shoots for a given target. The
// target must either target a project or a seed (both including a garden).
func ShootsForTarget(ctx context.Context, gardenClient gardenclient.Client, t target.Target) ([]gardencorev1beta1.Shoot, error) {
	var listOpt client.ListOption

	if t.ProjectName() != "" {
		project, err := gardenClient.GetProject(ctx, t.ProjectName())
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
		// list all
		listOpt = &client.ListOptions{Namespace: ""}
	}

	shootItems, err := gardenClient.GetShoots(ctx, listOpt)
	if err != nil {
		return nil, err
	}

	return shootItems, nil
}

// ShootNamesForTarget returns all possible shoots for a given target. The
// target must either target a project or a seed (both including a garden).
func ShootNamesForTarget(ctx context.Context, manager target.Manager, t target.Target) ([]string, error) {
	gardenClient, err := manager.Configuration().GardenClientForGarden(t.GardenName())
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client for garden cluster %q: %w", t.GardenName(), err)
	}

	shoots, err := ShootsForTarget(ctx, gardenClient, t)
	if err != nil {
		return nil, err
	}

	names := sets.NewString()
	for _, shoot := range shoots {
		names.Insert(shoot.Name)
	}

	return names.List(), nil
}

func SeedForTarget(ctx context.Context, gardenClient gardenclient.Client, t target.Target) (*gardencorev1beta1.Seed, error) {
	name := t.SeedName()
	if name == "" {
		return nil, errors.New("no seed targeted")
	}

	seed, err := gardenClient.GetSeed(ctx, name)
	if err != nil {
		return nil, err
	}

	return seed, nil
}

// SeedNamesForTarget returns all possible shoots for a given target. The
// target must at least point to a garden.
func SeedNamesForTarget(ctx context.Context, manager target.Manager, t target.Target) ([]string, error) {
	gardenClient, err := manager.Configuration().GardenClientForGarden(t.GardenName())
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client for garden cluster %q: %w", t.GardenName(), err)
	}

	seedItems, err := gardenClient.GetSeeds(ctx)
	if err != nil {
		return nil, err
	}

	names := sets.NewString()
	for _, seed := range seedItems {
		names.Insert(seed.Name)
	}

	return names.List(), nil
}

func ProjectForTarget(ctx context.Context, gardenClient gardenclient.Client, t target.Target) (*gardencorev1beta1.Project, error) {
	name := t.ProjectName()
	if name == "" {
		return nil, errors.New("no project targeted")
	}

	project, err := gardenClient.GetProject(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch project: %w", err)
	}

	return project, nil
}

// ProjectNamesForTarget returns all possible shoots for a given target. The
// target must at least point to a garden.
func ProjectNamesForTarget(ctx context.Context, manager target.Manager, t target.Target) ([]string, error) {
	gardenClient, err := manager.Configuration().GardenClientForGarden(t.GardenName())
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client for garden cluster %q: %w", t.GardenName(), err)
	}

	projectItems, err := gardenClient.GetProjects(ctx)
	if err != nil {
		return nil, err
	}

	names := sets.NewString()
	for _, project := range projectItems {
		names.Insert(project.Name)
	}

	return names.List(), nil
}

func GardenNames(manager target.Manager) ([]string, error) {
	names := sets.NewString()
	for _, garden := range manager.Configuration().Gardens {
		names.Insert(garden.Name)
	}

	return names.List(), nil
}
