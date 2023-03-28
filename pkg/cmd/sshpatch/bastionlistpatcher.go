/*
SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package sshpatch

import (
	"context"
	"fmt"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"

	gardenClient "github.com/gardener/gardenctl-v2/internal/gardenclient"
	"github.com/gardener/gardenctl-v2/pkg/target"
)

type bastionLister interface {
	// List lists all bastions for the current target
	List(ctx context.Context) ([]operationsv1alpha1.Bastion, error)
}

type bastionPatcher interface {
	// Patch patches an existing bastion
	Patch(ctx context.Context, oldBastion, newBastion *operationsv1alpha1.Bastion) error
}

type bastionListPatcher interface {
	bastionPatcher
	bastionLister
}

type userBastionListPatcherImpl struct {
	target       target.Target
	gardenClient gardenClient.Client
}

var _ bastionListPatcher = &userBastionListPatcherImpl{}

// newUserBastionListPatcher creates a new bastionListPatcher which only lists bastions
// of the current user.
func newUserBastionListPatcher(manager target.Manager) (bastionListPatcher, error) {
	currentTarget, err := manager.CurrentTarget()
	if err != nil {
		return nil, err
	}

	gardenName := currentTarget.GardenName()

	gardenClient, err := manager.GardenClient(gardenName)
	if err != nil {
		return nil, err
	}

	return &userBastionListPatcherImpl{
		currentTarget,
		gardenClient,
	}, nil
}

func (u *userBastionListPatcherImpl) List(ctx context.Context) ([]operationsv1alpha1.Bastion, error) {
	user, err := u.gardenClient.CurrentUser(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not get current user: %w", err)
	}

	listOption := gardenClient.ProjectFilter{}

	if u.target.ShootName() != "" {
		listOption["spec.shootRef.name"] = u.target.ShootName()
	}

	if u.target.ProjectName() != "" {
		listOption["project"] = u.target.ProjectName()
	} else if u.target.SeedName() != "" {
		listOption[gardencore.ShootSeedName] = u.target.SeedName()
	}

	var bastionsOfUser []operationsv1alpha1.Bastion

	list, err := u.gardenClient.ListBastions(ctx, listOption)
	if err != nil {
		return nil, err
	}

	for _, bastion := range list.Items {
		if createdBy, ok := bastion.Annotations["gardener.cloud/created-by"]; ok {
			if createdBy == user {
				bastionsOfUser = append(bastionsOfUser, bastion)
			}
		}
	}

	return bastionsOfUser, nil
}

func (u *userBastionListPatcherImpl) Patch(ctx context.Context, newBastion, oldBastion *operationsv1alpha1.Bastion) error {
	return u.gardenClient.PatchBastion(ctx, newBastion, oldBastion)
}
