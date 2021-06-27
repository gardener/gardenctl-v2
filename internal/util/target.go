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

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ShootForTarget returns the targeted shoot, if a shoot cluster is targeted,
// otherwise an error.
func ShootForTarget(ctx context.Context, gardenClient client.Client, t target.Target) (*gardencorev1beta1.Shoot, error) {
	if t.ProjectName() != "" {
		return shootForTargetViaProject(ctx, gardenClient, t)
	} else if t.SeedName() != "" {
		return shootForTargetViaSeed(ctx, gardenClient, t)
	}

	return nil, errors.New("invalid target, must have either project or seed specified for targeting a shoot")
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

func shootForTargetViaSeed(ctx context.Context, gardenClient client.Client, t target.Target) (*gardencorev1beta1.Shoot, error) {
	seed, err := SeedForTarget(ctx, gardenClient, t)
	if err != nil {
		return nil, fmt.Errorf("invalid seed %q: %v", t.SeedName(), err)
	}

	// list all shoots, filter by their name and possibly spec.seedName (if seed is set)
	shootList := gardencorev1beta1.ShootList{}
	if err := gardenClient.List(ctx, &shootList, &client.ListOptions{}); err != nil {
		return nil, fmt.Errorf("failed to list shoot clusters: %w", err)
	}

	// filter found shoots
	matchingShoots := []*gardencorev1beta1.Shoot{}

	for i, s := range shootList.Items {
		if s.Name != t.ShootName() {
			continue
		}

		// ignore shootss where seed name doesn't match
		if s.Spec.SeedName == nil || *s.Spec.SeedName != seed.Name {
			continue
		}

		matchingShoots = append(matchingShoots, &shootList.Items[i])
	}

	if len(matchingShoots) == 0 {
		return nil, fmt.Errorf("invalid shoot %q: not found", t.ShootName())
	}

	if len(matchingShoots) > 1 {
		return nil, fmt.Errorf("there are multiple shoots named %q on this garden, please target using a project to make your choice unambiguous", t.ShootName())
	}

	return matchingShoots[0], nil
}

// ShootsForTarget returns all possible shoots for a given target. The
// target must either target a project or a seed (both including a garden).
func ShootsForTarget(ctx context.Context, gardenClient client.Client, t target.Target) ([]gardencorev1beta1.Shoot, error) {
	var listOpt client.ListOption

	if t.ProjectName() != "" {
		project, err := ProjectForTarget(ctx, gardenClient, t)
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
		return nil, errors.New("invalid target, must have either project or seed specified for targeting a shoot")
	}

	shootList := &gardencorev1beta1.ShootList{}
	if err := gardenClient.List(ctx, shootList, listOpt); err != nil {
		return nil, fmt.Errorf("failed to list shoots on garden cluster %q: %w", t.GardenName(), err)
	}

	return shootList.Items, nil
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
