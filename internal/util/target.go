/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package util

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/util/sets"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"

	"github.com/gardener/gardenctl-v2/internal/gardenclient"
	"github.com/gardener/gardenctl-v2/pkg/target"
)

// ShootForTarget returns the targeted shoot, if a shoot cluster is targeted and exists otherwise an error.
func ShootForTarget(ctx context.Context, gardenClient gardenclient.Client, t target.Target) (*gardencorev1beta1.Shoot, error) {
	return gardenClient.FindShoot(ctx, t.AsListOption())
}

// ShootNamesForTarget returns all possible shoots for a given target.
func ShootNamesForTarget(ctx context.Context, manager target.Manager) ([]string, error) {
	t, err := manager.CurrentTarget()
	if err != nil {
		return nil, err
	}

	gardenClient, err := manager.GardenClient(t.GardenName())
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client for garden cluster %q: %w", t.GardenName(), err)
	}

	shootList, err := gardenClient.ListShoots(ctx, t.WithShootName("").AsListOption())
	if err != nil {
		return nil, err
	}

	names := sets.NewString()
	for _, shoot := range shootList.Items {
		names.Insert(shoot.Name)
	}

	return names.List(), nil
}

// SeedForTarget returns the targeted seed, if a seed is targeted and exists otherwise an error.
func SeedForTarget(ctx context.Context, gardenClient gardenclient.Client, t target.Target) (*gardencorev1beta1.Seed, error) {
	name := t.SeedName()
	if name == "" {
		return nil, errors.New("no seed targeted")
	}

	return gardenClient.GetSeed(ctx, name)
}

// SeedNamesForTarget returns all possible seeds for a given target. The
// target must at least point to a garden.
func SeedNamesForTarget(ctx context.Context, manager target.Manager) ([]string, error) {
	t, err := manager.CurrentTarget()
	if err != nil {
		return nil, err
	}

	gardenClient, err := manager.GardenClient(t.GardenName())
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client for garden cluster %q: %w", t.GardenName(), err)
	}

	seedList, err := gardenClient.ListSeeds(ctx)
	if err != nil {
		return nil, err
	}

	names := sets.NewString()
	for _, seed := range seedList.Items {
		names.Insert(seed.Name)
	}

	return names.List(), nil
}

// ProjectForTarget returns the targeted project, if a project is targeted and exists otherwise an error.
func ProjectForTarget(ctx context.Context, gardenClient gardenclient.Client, t target.Target) (*gardencorev1beta1.Project, error) {
	name := t.ProjectName()
	if name == "" {
		return nil, errors.New("no project targeted")
	}

	return gardenClient.GetProject(ctx, name)
}

// ProjectNamesForTarget returns all projects for the targeted garden.
// target must at least point to a garden.
func ProjectNamesForTarget(ctx context.Context, manager target.Manager) ([]string, error) {
	t, err := manager.CurrentTarget()
	if err != nil {
		return nil, err
	}
	
	gardenClient, err := manager.GardenClient(t.GardenName())
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client for garden cluster %q: %w", t.GardenName(), err)
	}

	projectList, err := gardenClient.ListProjects(ctx)
	if err != nil {
		return nil, err
	}

	names := sets.NewString()
	for _, project := range projectList.Items {
		names.Insert(project.Name)
	}

	return names.List(), nil
}

// GardenNames returns all names of configured Gardens
func GardenNames(manager target.Manager) ([]string, error) {
	config := manager.Configuration()
	if config == nil {
		return nil, errors.New("could not get configuration")
	}

	names := sets.NewString()
	for _, garden := range config.Gardens {
		names.Insert(garden.Name)
	}

	return names.List(), nil
}
