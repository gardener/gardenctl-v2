/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package util

import (
	"context"
	"errors"
	"fmt"

	"github.com/gardener/gardenctl-v2/pkg/target"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ShootForTarget(ctx context.Context, gardenClient client.Client, t target.Target) (*gardencorev1beta1.Shoot, error) {
	shoot := &gardencorev1beta1.Shoot{}

	// If a shoot is targeted via a project, we fetch it based on the project's namespace.
	// If the target uses a seed, _all_ shoots in the garden are filtered
	// for shoots with matching seed and name.
	// If neither project nor seed are given, _all_ shoots in the garden are filtered by
	// their name.
	// It's an error if no or multiple matching shoots are found.

	if t.ProjectName() != "" {
		return shootForTargetViaProject(ctx, gardenClient, t)
	}

	// list all shoots, filter by their name and possibly spec.seedName (if seed is set)
	shootList := gardencorev1beta1.ShootList{}
	if err := gardenClient.List(ctx, &shootList, &client.ListOptions{}); err != nil {
		return nil, fmt.Errorf("failed to list shoot clusters: %w", err)
	}

	var (
		seed *gardencorev1beta1.Seed
		err  error
	)

	if t.SeedName() != "" {
		seed, err = SeedForTarget(ctx, gardenClient, t)
		if err != nil {
			return nil, fmt.Errorf("invalid seed %q: %v", t.SeedName(), err)
		}
	}

	// filter found shoots
	matchingShoots := []*gardencorev1beta1.Shoot{}
	for i, s := range shootList.Items {
		if s.Name != t.ShootName() {
			continue
		}

		// if filtering by seed, ignore shoot's where seed name doesn't match
		if seed != nil && (s.Spec.SeedName == nil || *s.Spec.SeedName != seed.Name) {
			continue
		}

		matchingShoots = append(matchingShoots, &shootList.Items[i])
	}

	if len(matchingShoots) == 0 {
		return nil, fmt.Errorf("invalid shoot %q: not found", t.ShootName())
	}

	if len(matchingShoots) > 1 {
		return nil, fmt.Errorf("there are multiple shoots named %q on this garden, please target a project or seed to make your choice unambiguous", t.ShootName())
	}

	shoot = matchingShoots[0]

	return shoot, nil
}

func shootForTargetViaProject(ctx context.Context, gardenClient client.Client, t target.Target) (*gardencorev1beta1.Shoot, error) {
	project, err := ProjectForTarget(ctx, gardenClient, t)
	if err != nil {
		return nil, fmt.Errorf("invalid project %q: %v", t.ProjectName(), err)
	}

	if project.Spec.Namespace == nil || *project.Spec.Namespace == "" {
		return nil, fmt.Errorf("project %q has not yet been fully created", t.ProjectName())
	}

	// fetch shoot from project namespace
	shoot := &gardencorev1beta1.Shoot{}
	key := types.NamespacedName{Name: t.ShootName(), Namespace: *project.Spec.Namespace}

	if err := gardenClient.Get(ctx, key, shoot); err != nil {
		return nil, fmt.Errorf("invalid shoot %q: %w", key.Name, err)
	}

	return shoot, nil
}

func SeedForTarget(ctx context.Context, gardenClient client.Client, t target.Target) (*gardencorev1beta1.Seed, error) {
	name := t.SeedName()
	if name == "" {
		return nil, errors.New("no seed targeted")
	}

	seed := &gardencorev1beta1.Seed{}
	key := types.NamespacedName{Name: name}

	if err := gardenClient.Get(ctx, key, seed); err != nil {
		return nil, err
	}

	return seed, nil
}

func ProjectForTarget(ctx context.Context, gardenClient client.Client, t target.Target) (*gardencorev1beta1.Project, error) {
	name := t.ProjectName()
	if name == "" {
		return nil, errors.New("no project targeted")
	}

	project := &gardencorev1beta1.Project{}
	key := types.NamespacedName{Name: name}

	if err := gardenClient.Get(ctx, key, project); err != nil {
		return nil, err
	}

	return project, nil
}
