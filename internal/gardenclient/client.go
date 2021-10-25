/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package gardenclient

import (
	"context"
	"errors"
	"fmt"
	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Client returns a new client with functions to get Gardener and Kubernetes resources
type Client interface {
	// GetProjectByName returns a Gardener project resource by name
	GetProjectByName(ctx context.Context, projectName string) (*gardencorev1beta1.Project, error)
	// GetProjectByNamespace returns a Gardener project resource by namespace
	GetProjectByNamespace(ctx context.Context, namespaceName string) (*gardencorev1beta1.Project, error)
	// GetProjects returns all Gardener project resources
	GetProjects(ctx context.Context) ([]gardencorev1beta1.Project, error)

	// GetSeedByName returns a Gardener seed resource by name
	GetSeedByName(ctx context.Context, seedName string) (*gardencorev1beta1.Seed, error)
	// GetSeeds returns all Gardener seed resources
	GetSeeds(ctx context.Context) ([]gardencorev1beta1.Seed, error)

	// GetShootByNamespaceAndName returns a Gardener shoot resource in a namespace by name
	GetShootByNamespaceAndName(ctx context.Context, namespaceName string, shootName string) (*gardencorev1beta1.Shoot, error)
	// GetShootBySeedAndName returns a Gardener shoot resource by name
	// An optional seedName can be provided to filter by ShootSeedName field selector
	GetShootBySeedAndName(ctx context.Context, seedName string, shootName string) (*gardencorev1beta1.Shoot, error)
	// GetShoots returns all Gardener shoot resources, filtered by a list option
	GetShoots(ctx context.Context, listOpt client.ListOption) ([]gardencorev1beta1.Shoot, error)
	// GetNamespaceByName returns a Kubernetes namespace resource by name

	GetNamespaceByName(ctx context.Context, namespaceName string) (*corev1.Namespace, error)
	// GetSecretByNamespaceAndName returns a Kubernetes namespace resource by name

	GetSecretByNamespaceAndName(ctx context.Context, namespaceName string, seedName string) (*corev1.Secret, error)
	// GetNativeClient returns the underlying kubernetes runtime client

	// TODO: Remove this when we switched all APIs to the new gardenclient
	GetNativeClient() client.Client
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

func (g *clientImpl) GetProjectByName(ctx context.Context, projectName string) (*gardencorev1beta1.Project, error) {
	project := &gardencorev1beta1.Project{}
	key := types.NamespacedName{Name: projectName}

	if err := g.c.Get(ctx, key, project); err != nil {
		return nil, err
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

func (g *clientImpl) GetProjects(ctx context.Context) ([]gardencorev1beta1.Project, error) {
	projectList := &gardencorev1beta1.ProjectList{}
	if err := g.c.List(ctx, projectList); err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}

	return projectList.Items, nil
}

func (g *clientImpl) GetSeedByName(ctx context.Context, seedName string) (*gardencorev1beta1.Seed, error) {
	seed := &gardencorev1beta1.Seed{}
	key := types.NamespacedName{Name: seedName}

	if err := g.c.Get(ctx, key, seed); err != nil {
		return nil, err
	}

	return seed, nil
}

func (g *clientImpl) GetSeeds(ctx context.Context) ([]gardencorev1beta1.Seed, error) {
	seedList := &gardencorev1beta1.SeedList{}
	if err := g.c.List(ctx, seedList); err != nil {
		return nil, fmt.Errorf("failed to list seeds: %w", err)
	}

	return seedList.Items, nil
}

func (g *clientImpl) GetShootByNamespaceAndName(ctx context.Context, namespaceName string, shootName string) (*gardencorev1beta1.Shoot, error) {
	shoot := &gardencorev1beta1.Shoot{}
	key := types.NamespacedName{Name: shootName, Namespace: namespaceName}

	if err := g.c.Get(ctx, key, shoot); err != nil {
		return nil, fmt.Errorf("failed to get shoot %v: %w", key, err)
	}

	return shoot, nil
}

func (g *clientImpl) GetShootBySeedAndName(ctx context.Context, seedName string, shootName string) (*gardencorev1beta1.Shoot, error) {
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

func (g *clientImpl) GetShoots(ctx context.Context, listOpt client.ListOption) ([]gardencorev1beta1.Shoot, error) {
	shootList := &gardencorev1beta1.ShootList{}
	if err := g.c.List(ctx, shootList, listOpt); err != nil {
		return nil, fmt.Errorf("failed to list shoots with list option %q: %w", listOpt, err)
	}

	return shootList.Items, nil
}

func (g *clientImpl) GetNamespaceByName(ctx context.Context, namespaceName string) (*corev1.Namespace, error) {
	namespace := &corev1.Namespace{}
	key := types.NamespacedName{Name: namespaceName}

	if err := g.c.Get(ctx, key, namespace); err != nil {
		return nil, err
	}

	return namespace, nil
}

func (g *clientImpl) GetSecretByNamespaceAndName(ctx context.Context, namespaceName string, seedName string) (*corev1.Secret, error) {
	secret := corev1.Secret{}
	key := types.NamespacedName{Name: seedName, Namespace: namespaceName}

	if err := g.c.Get(ctx, key, &secret); err != nil {
		return nil, err
	}

	return &secret, nil
}

func (g *clientImpl) GetNativeClient() client.Client {
	return g.c
}
