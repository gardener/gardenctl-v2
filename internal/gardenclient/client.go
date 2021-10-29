/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package gardenclient

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// Client returns a new client with functions to get Gardener and Kubernetes resources
type Client interface {
	// GetProject returns a Gardener project resource by name
	GetProject(ctx context.Context, projectName string) (*gardencorev1beta1.Project, error)
	// GetProjectByNamespace returns a Gardener project resource by namespace
	GetProjectByNamespace(ctx context.Context, namespaceName string) (*gardencorev1beta1.Project, error)
	// ListProjects returns all Gardener project resources
	ListProjects(ctx context.Context) ([]gardencorev1beta1.Project, error)

	// GetSeed returns a Gardener seed resource by name
	GetSeed(ctx context.Context, seedName string) (*gardencorev1beta1.Seed, error)
	// ListSeeds returns all Gardener seed resources
	ListSeeds(ctx context.Context) ([]gardencorev1beta1.Seed, error)

	// GetShoot returns a Gardener shoot resource in a namespace by name
	GetShoot(ctx context.Context, namespaceName string, shootName string) (*gardencorev1beta1.Shoot, error)
	// GetShootBySeed returns a Gardener shoot resource by name
	// An optional seedName can be provided to filter by ShootSeedName field selector
	GetShootBySeed(ctx context.Context, seedName string, shootName string) (*gardencorev1beta1.Shoot, error)
	// FindShoot tries to get a shoot via project (namespace) first, if not provided it tries to find the shot
	// with an (optional) seedname
	FindShoot(ctx context.Context, shootName string, projectName string, seedName string) (*gardencorev1beta1.Shoot, error)

	// ListShoots returns all Gardener shoot resources, filtered by a list option
	ListShoots(ctx context.Context, listOpt client.ListOption) ([]gardencorev1beta1.Shoot, error)

	// GetNamespace returns a Kubernetes namespace resource by name
	GetNamespace(ctx context.Context, namespaceName string) (*corev1.Namespace, error)
	// GetSecret returns a Kubernetes namespace resource by name
	GetSecret(ctx context.Context, namespaceName string, seedName string) (*corev1.Secret, error)

	// GetRuntimeClient returns the underlying kubernetes runtime client
	// TODO: Remove this when we switched all APIs to the new gardenclient
	RuntimeClient() client.Client
}

type clientImpl struct {
	c client.Client
}

// NewGardenClient returns a new gardenclient
func NewGardenClient(client client.Client) Client {
	return &clientImpl{
		c: client,
	}
}

var _ Client = &clientImpl{}

func (g *clientImpl) GetProject(ctx context.Context, projectName string) (*gardencorev1beta1.Project, error) {
	project := &gardencorev1beta1.Project{}
	key := types.NamespacedName{Name: projectName}

	if err := g.c.Get(ctx, key, project); err != nil {
		return nil, fmt.Errorf("failed to get project %v: %w", key, err)
	}

	return project, nil
}

func (g *clientImpl) GetProjectByNamespace(ctx context.Context, namespaceName string) (*gardencorev1beta1.Project, error) {
	projectList := &gardencorev1beta1.ProjectList{}

	// ctrl-runtime doesn't support FieldSelectors in fake clients
	// ( https://github.com/kubernetes-sigs/controller-runtime/issues/1376 )
	// yet, which affects the unit tests. To ensure proper filtering,
	// the shootList (and projectList later on) are filtered again.
	// In production this does not hurt much, as the FieldSelector is
	// already applied, and in tests very few objects exist anyway.
	if err := g.c.List(ctx, projectList, client.MatchingFields{gardencore.ProjectNamespace: namespaceName}); err != nil {
		return nil, fmt.Errorf("failed to fetch project by namespace: %v", err)
	}

	// if filtering by seed, ignore shoot's whose seed name doesn't match
	// (if ctrl-runntime supported FieldSelectors in tests, this if statement could go away)
	matchingProjects := []*gardencorev1beta1.Project{}

	for i, project := range projectList.Items {
		if project.Spec.Namespace != nil && *project.Spec.Namespace == namespaceName {
			matchingProjects = append(matchingProjects, &projectList.Items[i])
		}
	}

	if len(matchingProjects) == 0 {
		return nil, errors.New("failed to fetch project by namespace")
	}

	return matchingProjects[0], nil
}

func (g *clientImpl) ListProjects(ctx context.Context) ([]gardencorev1beta1.Project, error) {
	projectList := &gardencorev1beta1.ProjectList{}
	if err := g.c.List(ctx, projectList); err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}

	return projectList.Items, nil
}

func (g *clientImpl) GetSeed(ctx context.Context, seedName string) (*gardencorev1beta1.Seed, error) {
	seed := &gardencorev1beta1.Seed{}
	key := types.NamespacedName{Name: seedName}

	if err := g.c.Get(ctx, key, seed); err != nil {
		return nil, fmt.Errorf("failed to get seed %v: %w", key, err)
	}

	return seed, nil
}

func (g *clientImpl) ListSeeds(ctx context.Context) ([]gardencorev1beta1.Seed, error) {
	seedList := &gardencorev1beta1.SeedList{}
	if err := g.c.List(ctx, seedList); err != nil {
		return nil, fmt.Errorf("failed to list seeds: %w", err)
	}

	return seedList.Items, nil
}

func (g *clientImpl) GetShoot(ctx context.Context, namespaceName string, shootName string) (*gardencorev1beta1.Shoot, error) {
	shoot := &gardencorev1beta1.Shoot{}
	key := types.NamespacedName{Name: shootName, Namespace: namespaceName}

	if err := g.c.Get(ctx, key, shoot); err != nil {
		return nil, fmt.Errorf("failed to get shoot %v: %w", key, err)
	}

	return shoot, nil
}

func (g *clientImpl) GetShootBySeed(ctx context.Context, seedName string, shootName string) (*gardencorev1beta1.Shoot, error) {
	// list all shoots, filter by their name and possibly spec.seedName (if seed is set)
	shootList := gardencorev1beta1.ShootList{}
	listOpts := []client.ListOption{}

	if seedName != "" {
		// ctrl-runtime doesn't support FieldSelectors in fake clients
		// ( https://github.com/kubernetes-sigs/controller-runtime/issues/1376 )
		// yet, which affects the unit tests. To ensure proper filtering,
		// the shootList (and projectList later on) are filtered again.
		// In production this does not hurt much, as the FieldSelector is
		// already applied, and in tests very few objects exist anyway.
		listOpts = append(listOpts, client.MatchingFields{gardencore.ShootSeedName: seedName})
	}

	if err := g.c.List(ctx, &shootList, listOpts...); err != nil {
		return nil, fmt.Errorf("failed to list shoot clusters: %w", err)
	}

	// filter found shoots
	matchingShoots := []*gardencorev1beta1.Shoot{}

	for i, s := range shootList.Items {
		if s.Name != shootName {
			continue
		}

		// if filtering by seed, ignore shoot's whose seed name doesn't match
		// (if ctrl-runntime supported FieldSelectors in tests, this if statement could go away)
		if seedName != "" && (s.Spec.SeedName == nil || *s.Spec.SeedName != seedName) {
			continue
		}

		matchingShoots = append(matchingShoots, &shootList.Items[i])
	}

	if len(matchingShoots) == 0 {
		return nil, fmt.Errorf("no shoot named %q exists", shootName)
	}

	if len(matchingShoots) > 1 {
		return nil, fmt.Errorf("there are multiple shoots named %q on this garden, please target a project or seed to make your choice unambiguous", shootName)
	}

	return matchingShoots[0], nil
}

func (g *clientImpl) FindShoot(ctx context.Context, shootName string, projectName string, seedName string) (*gardencorev1beta1.Shoot, error) {
	if projectName != "" {
		// project name set, get shoot within project namespace
		project, err := g.GetProject(ctx, projectName)
		if err != nil {
			return nil, err
		}

		return g.GetShoot(ctx, *project.Spec.Namespace, shootName)
	}

	return g.GetShootBySeed(ctx, seedName, shootName)
}

func (g *clientImpl) ListShoots(ctx context.Context, listOpt client.ListOption) ([]gardencorev1beta1.Shoot, error) {
	shootList := &gardencorev1beta1.ShootList{}
	if err := g.c.List(ctx, shootList, listOpt); err != nil {
		return nil, fmt.Errorf("failed to list shoots with list option %q: %w", listOpt, err)
	}

	return shootList.Items, nil
}

func (g *clientImpl) GetNamespace(ctx context.Context, namespaceName string) (*corev1.Namespace, error) {
	namespace := &corev1.Namespace{}
	key := types.NamespacedName{Name: namespaceName}

	if err := g.c.Get(ctx, key, namespace); err != nil {
		return nil, fmt.Errorf("failed to get namespace %v: %w", key, err)
	}

	return namespace, nil
}

func (g *clientImpl) GetSecret(ctx context.Context, namespaceName string, seedName string) (*corev1.Secret, error) {
	secret := corev1.Secret{}
	key := types.NamespacedName{Name: seedName, Namespace: namespaceName}

	if err := g.c.Get(ctx, key, &secret); err != nil {
		return nil, fmt.Errorf("failed to get secret %v: %w", key, err)
	}

	return &secret, nil
}

func (g *clientImpl) RuntimeClient() client.Client {
	return g.c
}
