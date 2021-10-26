/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package garden

import (
	"context"
	"fmt"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	controllerruntime "sigs.k8s.io/controller-runtime/pkg/client"
)

// Client is a interface which provides simple access methods to a garden cluster
type Client interface {
	GetProject(ctx context.Context, name string) (*gardencorev1beta1.Project, error)
	GetShoot(ctx context.Context, namespace, name string) (*gardencorev1beta1.Shoot, error)
	GetShootByProjectAndName(ctx context.Context, projectName, name string) (*gardencorev1beta1.Shoot, error)
	FindShootBySeedAndName(ctx context.Context, seedName, name string) (*gardencorev1beta1.Shoot, error)
	GetSecretBinding(ctx context.Context, namespace, name string) (*gardencorev1beta1.SecretBinding, error)
	GetSecret(ctx context.Context, namespace, name string) (*v1.Secret, error)
	GetSecretBySecretBinding(ctx context.Context, namespace, name string) (*v1.Secret, error)
}

type clientImpl struct {
	name   string
	client controllerruntime.Client
}

var _ Client = &clientImpl{}

// NewClient create a Client instance for a given garden cluster
func NewClient(client controllerruntime.Client, gardenName string) Client {
	return &clientImpl{
		name:   gardenName,
		client: client,
	}
}

// GetProject returns a project by name
func (g *clientImpl) GetProject(ctx context.Context, name string) (*gardencorev1beta1.Project, error) {
	project := &gardencorev1beta1.Project{}

	if err := g.client.Get(ctx, types.NamespacedName{Name: name}, project); err != nil {
		return nil, fmt.Errorf("failed to get project '%s': %w", name, err)
	}

	return project, nil
}

// GetShoot returns a shoot by namespace and name
func (g *clientImpl) GetShoot(ctx context.Context, namespace, name string) (*gardencorev1beta1.Shoot, error) {
	shoot := &gardencorev1beta1.Shoot{}

	if err := g.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, shoot); err != nil {
		return nil, fmt.Errorf("failed to get shoot '%s/%s': %w", namespace, name, err)
	}

	return shoot, nil
}

// GetShootByProjectAndName returns a shoot by name for the given project
func (g *clientImpl) GetShootByProjectAndName(ctx context.Context, projectName, name string) (*gardencorev1beta1.Shoot, error) {
	project, err := g.GetProject(ctx, projectName)
	if err != nil {
		return nil, err
	}

	shootNamespace := *project.Spec.Namespace
	shoot, err := g.GetShoot(ctx, shootNamespace, name)

	if err != nil {
		return nil, err
	}

	return shoot, nil
}

// FindShootBySeedAndName tries to find a shoot on a seed with a given name.
// If no shoot is found or multiple shoots are found it returns an error.
func (g *clientImpl) FindShootBySeedAndName(ctx context.Context, seedName, name string) (*gardencorev1beta1.Shoot, error) {
	shootList := &gardencorev1beta1.ShootList{}
	labels := controllerruntime.MatchingLabels{"metadata.name": name}
	fields := controllerruntime.MatchingFields{gardencore.ShootSeedName: seedName}

	if err := g.client.List(ctx, shootList, labels, fields); err != nil {
		return nil, fmt.Errorf("failed to list shoots with name '%s' and seed '%s': %w", name, seedName, err)
	}

	if len(shootList.Items) > 1 {
		return nil, fmt.Errorf("found more than one shoot with name '%s' and seed '%s'", name, seedName)
	} else if len(shootList.Items) < 1 {
		return nil, fmt.Errorf("found no shoot with name '%s' and seed '%s'", name, seedName)
	}

	return &shootList.Items[0], nil
}

// GetSecretBinding returns a secretBinding by namespace and name
func (g *clientImpl) GetSecretBinding(ctx context.Context, namespace, name string) (*gardencorev1beta1.SecretBinding, error) {
	secretBinding := &gardencorev1beta1.SecretBinding{}

	if err := g.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, secretBinding); err != nil {
		return nil, fmt.Errorf("failed to get secretBinding '%s/%s': %w", namespace, name, err)
	}

	return secretBinding, nil
}

// GetSecret returns a secret by namespace and name
func (g *clientImpl) GetSecret(ctx context.Context, namespace, name string) (*v1.Secret, error) {
	secret := &v1.Secret{}

	if err := g.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, secret); err != nil {
		return nil, fmt.Errorf("failed to get secret '%s/%s': %w", namespace, name, err)
	}

	return secret, nil
}

// GetSecret returns a secret by namespace and name of a secretBinding
func (g *clientImpl) GetSecretBySecretBinding(ctx context.Context, namespace, name string) (*v1.Secret, error) {
	secretBinding, err := g.GetSecretBinding(ctx, namespace, name)
	if err != nil {
		return nil, err
	}

	secretRef := secretBinding.SecretRef

	secret, err := g.GetSecret(ctx, secretRef.Namespace, secretRef.Name)
	if err != nil {
		return nil, err
	}

	return secret, nil
}
