/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/
package target

import (
	"context"
	"errors"
	"fmt"

	"github.com/gardener/gardenctl-v2/internal/gardenclient"

	"github.com/gardener/gardenctl-v2/pkg/config"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// TargetBuilder builds, completes and validates target values to create valid targets
//nolint
type TargetBuilder interface {
	// Init updates the TargetBuilder with the provided target
	// Use this function to overwrite target baseline data before updating with new values
	Init(Target) TargetBuilder
	// SetGarden updates TargetBuilder with a Garden name
	// gardenName can be Garden Name or Alias
	SetGarden(string) TargetBuilder
	// SetProject updates TargetBuilder with a Project name
	SetProject(context.Context, string) TargetBuilder
	// SetNamespace updates TargetBuilder with a Namespace name
	// The Namespace will be used to resolve the associated project during build
	SetNamespace(context.Context, string) TargetBuilder
	// SetSeed updates TargetBuilder with a Seed name
	SetSeed(context.Context, string) TargetBuilder
	// SetShoot updates TargetBuilder with a  Shoot name
	SetShoot(context.Context, string) TargetBuilder
	// Build uses the values set for TargetBuilder to create and return a new target
	// This function validates the target values and tries to complete missing values
	// If the provided values do not represent a valid and unique target, an error is returned
	Build() (Target, error)
}

type handler func(t *targetImpl) error

type targetBuilderImpl struct {
	config         *config.Config
	clientProvider ClientProvider
	target         Target
	actions        []handler
}

var _ TargetBuilder = &targetBuilderImpl{}

// NewTargetBuilder returns a new target builder
func NewTargetBuilder(config *config.Config, clientProvider ClientProvider) TargetBuilder {
	return &targetBuilderImpl{
		config:         config,
		clientProvider: clientProvider,
	}
}

func (b *targetBuilderImpl) Init(t Target) TargetBuilder {
	b.target = t
	return b
}

func (b *targetBuilderImpl) SetGarden(name string) TargetBuilder {
	b.actions = append(b.actions, func(t *targetImpl) error {
		gardenName, err := b.config.GardenName(name)
		if err != nil {
			return fmt.Errorf("failed to set target garden: %w", err)
		}

		t.Garden = gardenName
		t.Project = ""
		t.Seed = ""
		t.Shoot = ""

		return nil
	})

	return b
}

func (b *targetBuilderImpl) SetProject(ctx context.Context, name string) TargetBuilder {
	b.actions = append(b.actions, func(t *targetImpl) error {
		if t.GardenName() == "" {
			return ErrNoGardenTargeted
		}

		// validate that the project exists
		project, err := b.validateProject(ctx, t.GardenName(), name)
		if err != nil {
			return fmt.Errorf("failed to set target project: %w", err)
		}

		t.Project = project.Name
		t.Seed = ""
		t.Shoot = ""

		return nil
	})

	return b
}

func (b *targetBuilderImpl) SetNamespace(ctx context.Context, name string) TargetBuilder {
	b.actions = append(b.actions, func(t *targetImpl) error {
		if t.GardenName() == "" {
			return ErrNoGardenTargeted
		}

		// validate that the namespace exists and is related to a project
		projectName, err := b.validateNamespace(ctx, t.GardenName(), name)
		if err != nil {
			return fmt.Errorf("failed to set target project: %w", err)
		}

		// validate that the project exists
		project, err := b.validateProject(ctx, t.GardenName(), projectName)
		if err != nil {
			return fmt.Errorf("failed to set target project: %w", err)
		}

		t.Project = project.Name
		t.Seed = ""
		t.Shoot = ""

		return nil
	})

	return b
}

func (b *targetBuilderImpl) SetSeed(ctx context.Context, name string) TargetBuilder {
	b.actions = append(b.actions, func(t *targetImpl) error {
		if t.GardenName() == "" {
			return ErrNoGardenTargeted
		}

		// validate that the seed exists
		seed, err := b.validateSeed(ctx, t.GardenName(), name)
		if err != nil {
			return fmt.Errorf("failed to set target seed: %w", err)
		}

		t.Project = ""
		t.Seed = seed.Name
		t.Shoot = ""

		return nil
	})

	return b
}

func (b *targetBuilderImpl) SetShoot(ctx context.Context, name string) TargetBuilder {
	b.actions = append(b.actions, func(t *targetImpl) error {
		if t.GardenName() == "" {
			return ErrNoGardenTargeted
		}

		gardenClient, err := b.getGardenClient(t.GardenName())
		if err != nil {
			return err
		}

		shoot, err := gardenClient.FindShoot(ctx, name, t.ProjectName(), t.SeedName())
		if err != nil {
			return fmt.Errorf("failed to set target shoot: %w", err)
		}

		if t.ProjectName() == "" {
			// we need to resolve the project name as it is not already set
			// This is important to ensure that the target stays unambiguous and the shoot can be found faster in subsequent operations
			project, err := gardenClient.GetProjectByNamespace(ctx, shoot.Namespace)
			if err != nil {
				return fmt.Errorf("failed to fetch parent project for shoot: %w", err)
			}

			t.Project = project.Name
		}

		t.Seed = ""
		t.Shoot = shoot.Name

		return nil
	})

	return b
}

func (b *targetBuilderImpl) Build() (Target, error) {
	target := b.target
	if target == nil {
		target = NewTarget("", "", "", "")
	}

	t := &targetImpl{
		target.GardenName(),
		target.ProjectName(),
		target.SeedName(),
		target.ShootName(),
	}

	for _, a := range b.actions {
		if err := a(t); err != nil {
			return nil, err
		}
	}

	return t, nil
}

// returns gardener project
func (b *targetBuilderImpl) validateProject(ctx context.Context, gardenName string, name string) (*gardencorev1beta1.Project, error) {
	// validate that the project exists
	gardenClient, err := b.getGardenClient(gardenName)
	if err != nil {
		return nil, err
	}

	project, err := gardenClient.GetProject(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch project: %w", err)
	}

	// validate the project
	if project.Spec.Namespace == nil || *project.Spec.Namespace == "" {
		return nil, errors.New("project does not have a corresponding namespace set; most likely it has not yet been fully created")
	}

	return project, nil
}

// returns gardener project name for namespace
func (b *targetBuilderImpl) validateNamespace(ctx context.Context, gardenName string, name string) (string, error) {
	gardenClient, err := b.getGardenClient(gardenName)
	if err != nil {
		return "", err
	}

	namespace, err := gardenClient.GetNamespace(ctx, name)

	if err != nil {
		return "", fmt.Errorf("failed to fetch namespace: %w", err)
	}

	if namespace == nil {
		return "", fmt.Errorf("invalid namespace: %s", name)
	}

	projectName := namespace.Labels["project.gardener.cloud/name"]
	if projectName == "" {
		return "", fmt.Errorf("namespace %q is not related to a gardener project", projectName)
	}

	return projectName, nil
}

// returns gardener seed
func (b *targetBuilderImpl) validateSeed(ctx context.Context, gardenName string, name string) (*gardencorev1beta1.Seed, error) {
	// validate that the seed exists
	gardenClient, err := b.getGardenClient(gardenName)
	if err != nil {
		return nil, err
	}

	seed, err := gardenClient.GetSeed(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve seed: %w", err)
	}

	// validate the seed
	if seed.Spec.SecretRef == nil {
		return nil, errors.New("spec.SecretRef is missing in this seed, seed not reachable")
	}

	return seed, nil
}

func (b *targetBuilderImpl) getGardenClient(gardenName string) (gardenclient.Client, error) {
	return GardenClient(gardenName, b.config, b.clientProvider)
}
