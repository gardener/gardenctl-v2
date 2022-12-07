/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package target

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/util/sets"
)

// TODO: check if we really want to pass the manager and always use the current target
//
//	In the latter case it would be required that the caller already passes the gardenClient for the
//	correct garden. So eventually manager + target is also an option?
//
// TODO: check if we can somehow group those functions on a struct/interface?

// ShootNamesForTarget returns all shoots for the current target.
func ShootNamesForTarget(ctx context.Context, manager Manager) ([]string, error) {
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

// SeedNamesForTarget returns all seeds for the current target. The
// target must at least point to a garden.
func SeedNamesForTarget(ctx context.Context, manager Manager) ([]string, error) {
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

// ProjectNamesForTarget returns all projects for the currently targeted garden.
// target must at least point to a garden.
func ProjectNamesForTarget(ctx context.Context, manager Manager) ([]string, error) {
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

// GardenNames returns all names of configured Gardens.
func GardenNames(manager Manager) ([]string, error) {
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
