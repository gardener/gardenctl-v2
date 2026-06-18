/*
SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package target

import (
	"context"
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	corev1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"

	clientgarden "github.com/gardener/gardenctl-v2/internal/client/garden"
)

// Resolver resolves targets that require garden cluster lookups.
type Resolver struct {
	gardenClient clientgarden.Client
}

// NewResolver returns a resolver using the given garden client.
func NewResolver(gardenClient clientgarden.Client) *Resolver {
	return &Resolver{gardenClient: gardenClient}
}

// ResolveShootTarget resolves seed and control-plane targets to the underlying
// shoot target. If neither case applies, it returns t when t already targets a shoot.
func (r *Resolver) ResolveShootTarget(ctx context.Context, t Target) (Target, error) {
	if t.ShootName() == "" && t.SeedName() != "" {
		return r.resolveManagedSeedTarget(ctx, t, t.SeedName())
	}

	if t.ShootName() == "" {
		return nil, ErrNoShootTargeted
	}

	if t.ControlPlane() {
		workloadShoot, err := r.FindShoot(ctx, t.WithControlPlane(false))
		if err != nil {
			return nil, err
		}

		if workloadShoot.Spec.SeedName == nil || *workloadShoot.Spec.SeedName == "" {
			return nil, fmt.Errorf("no seed assigned to shoot %s/%s", workloadShoot.Namespace, workloadShoot.Name)
		}

		return r.resolveManagedSeedTarget(ctx, t, *workloadShoot.Spec.SeedName)
	}

	return t, nil
}

// ResolveShoot resolves and returns the effective shoot targeted by t.
func (r *Resolver) ResolveShoot(ctx context.Context, t Target) (*gardencorev1beta1.Shoot, error) {
	resolvedTarget, err := r.ResolveShootTarget(ctx, t)
	if err != nil {
		return nil, err
	}

	shoot, err := r.FindShoot(ctx, resolvedTarget)
	if err != nil {
		return nil, err
	}

	return shoot, nil
}

// FindShoot returns the shoot identified by the given target.
func (r *Resolver) FindShoot(ctx context.Context, t Target) (*gardencorev1beta1.Shoot, error) {
	return r.gardenClient.FindShoot(ctx, t.AsListOption())
}

func (r *Resolver) resolveManagedSeedTarget(ctx context.Context, t Target, seedName string) (Target, error) {
	shootOfManagedSeed, err := r.gardenClient.GetShootOfManagedSeed(ctx, seedName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("seed %q is not a managed seed: %w", seedName, err)
		}

		return nil, err
	}

	klog.FromContext(ctx).V(1).Info("resolved managed seed to referred shoot",
		"seed", seedName,
		"shoot", klog.KRef(corev1beta1constants.GardenNamespace, shootOfManagedSeed.Name))

	return t.
		WithProjectName(corev1beta1constants.GardenNamespace).
		WithSeedName("").
		WithShootName(shootOfManagedSeed.Name).
		WithControlPlane(false), nil
}

func getProjectNamespace(ctx context.Context, client clientgarden.Client, name string) (*string, error) {
	project, err := client.GetProject(ctx, name)
	if err != nil {
		return nil, err
	}

	if project.Spec.Namespace == nil || *project.Spec.Namespace == "" {
		return nil, fmt.Errorf("project %q has not yet been assigned to a namespace", name)
	}

	return project.Spec.Namespace, nil
}
