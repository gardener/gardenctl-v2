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
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// TargetBuilder builds, completes and validates target values to create valid targets
//nolint
type TargetBuilder interface {
	// UpdateWithCurrentTarget updates all values of the TargetBuilder with the provided target
	// Use this function to set target baseline data before updating with new values
	// The function does NOT ensure that the Target is valid
	UpdateWithCurrentTarget() error
	// GetTarget uses the values set for TargetBuilder to create and return a new target
	GetTarget() Target

	// SetAndValidateGardenName updates TargetBuilder with a validated Garden name
	// gardenName can be Garden Name or Alias
	// The function ensures that the Garden is valid
	// This implicitly unsets project, seed and shoot values
	SetAndValidateGardenName(gardenName string) error
	// SetAndValidateProjectName updates TargetBuilder with a validated Project name
	// The function ensures that the Project is valid
	// This implicitly unsets seed and shoot values
	SetAndValidateProjectName(ctx context.Context, projectName string) error
	// SetAndValidateProjectNameWithNamespace updates TargetBuilder with validated Project name by resolving the project via associated namespace
	// The function ensures that the Project is valid
	// This implicitly unsets seed and shoot values
	SetAndValidateProjectNameWithNamespace(ctx context.Context, namespaceName string) error
	// SetAndValidateSeedName updates TargetBuilder with a validated Seed name
	// The function ensures that the Seed is valid
	// This implicitly unsets project and shoot values
	SetAndValidateSeedName(ctx context.Context, seedName string) error
	// SetAndValidateShootName updates TargetBuilder with a validated Shoot name
	// The function ensures that the Shoot is valid
	// This implicitly unsets seed value
	SetAndValidateShootName(ctx context.Context, shootName string) error
}

type targetBuilderImpl struct {
	m           Manager
	gardeName   string
	projectName string
	seedName    string
	shootName   string
}

var _ TargetBuilder = &targetBuilderImpl{}

// NewTargetBuilder returns a new target builder
func NewTargetBuilder(manager Manager) TargetBuilder {
	return &targetBuilderImpl{
		m: manager,
	}
}

func (tb *targetBuilderImpl) UpdateWithCurrentTarget() error {
	currentTarget, err := tb.m.CurrentTarget()
	if err != nil {
		return fmt.Errorf("failed to get current target: %v", err)
	}

	tb.gardeName = currentTarget.GardenName()
	tb.projectName = currentTarget.ProjectName()
	tb.seedName = currentTarget.SeedName()
	tb.shootName = currentTarget.ShootName()

	return nil
}

func (tb *targetBuilderImpl) SetAndValidateGardenName(gardenName string) error {
	gardenName, err := tb.m.Configuration().FindGarden(gardenName)
	if err != nil {
		return err
	}

	tb.gardeName = gardenName
	tb.projectName = ""
	tb.seedName = ""
	tb.shootName = ""

	return nil
}

func (tb *targetBuilderImpl) SetAndValidateProjectName(ctx context.Context, projectName string) error {
	if tb.gardeName == "" {
		return ErrNoGardenTargeted
	}
	// validate that the project exists
	gardenClient, err := tb.getGardenClient()
	if err != nil {
		return err
	}

	project, err := gardenClient.GetProjectByName(ctx, projectName)
	if err != nil {
		return fmt.Errorf("failed to fetch project: %w", err)
	}

	// validate the project
	if err := tb.validateProject(project); err != nil {
		return fmt.Errorf("invalid project: %w", err)
	}

	tb.projectName = projectName
	tb.seedName = ""
	tb.shootName = ""

	return nil
}

func (tb *targetBuilderImpl) SetAndValidateProjectNameWithNamespace(ctx context.Context, namespaceName string) error {
	if tb.gardeName == "" {
		return ErrNoGardenTargeted
	}

	gardenClient, err := tb.getGardenClient()
	if err != nil {
		return err
	}

	namespace, err := gardenClient.GetNamespaceByName(ctx, namespaceName)

	if err != nil {
		return fmt.Errorf("failed to fetch namespace: %w", err)
	}

	if namespace == nil {
		return fmt.Errorf("invalid namespace: %s", namespaceName)
	}

	projectName := namespace.Labels["project.gardener.cloud/name"]
	if projectName == "" {
		return fmt.Errorf("namespace %q is not related to a gardener project", projectName)
	}

	return tb.SetAndValidateProjectName(ctx, projectName)
}

func (tb *targetBuilderImpl) SetAndValidateSeedName(ctx context.Context, seedName string) error {
	if tb.gardeName == "" {
		return ErrNoGardenTargeted
	}

	// validate that the seed exists
	gardenClient, err := tb.getGardenClient()
	if err != nil {
		return err
	}

	seed, err := gardenClient.GetSeedByName(ctx, seedName)
	if err != nil {
		return fmt.Errorf("failed to resolve seed: %w", err)
	}

	// validate the seed
	if err := tb.validateSeed(seed); err != nil {
		return fmt.Errorf("invalid seed: %w", err)
	}

	tb.projectName = ""
	tb.seedName = seedName
	tb.shootName = ""

	return nil
}

func (tb *targetBuilderImpl) SetAndValidateShootName(ctx context.Context, shootName string) error {
	if tb.gardeName == "" {
		return ErrNoGardenTargeted
	}

	gardenClient, err := tb.getGardenClient()
	if err != nil {
		return err
	}

	var shoot *gardencorev1beta1.Shoot

	if tb.projectName != "" {
		// project name set, get shoot within project namespace
		project, err := gardenClient.GetProjectByName(ctx, tb.projectName)
		if err != nil {
			return fmt.Errorf("failed to fetch project: %w", err)
		}

		shoot, err = gardenClient.GetShootByNamespaceAndName(ctx, *project.Spec.Namespace, shootName)
		if err != nil {
			return fmt.Errorf("failed to fetch shoot %q inside namespace %q: %w", shootName, *project.Spec.Namespace, err)
		}
	} else {
		shoot, err = gardenClient.GetShootBySeedAndName(ctx, tb.seedName, shootName)
		if err != nil {
			return fmt.Errorf("failed to fetch shoot %q using ShootSeedName field selector %q: %w", shootName, tb.seedName, err)
		}

		// we need to resolve the project name as it is not already set
		// This is important to ensure that the target stays unambiguous and the shoot can be found faster in subsequent operations
		project, err := gardenClient.GetProjectByNamespace(ctx, shoot.Namespace)
		if err != nil {
			return fmt.Errorf("failed to fetch parent project for shoot: %w", err)
		}

		tb.projectName = project.Name
	}

	// validate the shoot
	if err := tb.validateShoot(shoot); err != nil {
		return fmt.Errorf("invalid shoot: %w", err)
	}

	tb.seedName = ""
	tb.shootName = shootName

	return nil
}

func (tb *targetBuilderImpl) GetTarget() Target {
	return NewTarget(tb.gardeName, tb.projectName, tb.seedName, tb.shootName)
}

func (tb *targetBuilderImpl) getGardenClient() (gardenclient.Client, error) {
	t := NewTarget(tb.gardeName, "", "", "")

	client, err := tb.m.GardenClient(t)
	if err != nil {
		return nil, fmt.Errorf("could not create Kubernetes client for garden cluster: %w", err)
	}

	return client, nil
}

func (tb *targetBuilderImpl) validateProject(project *gardencorev1beta1.Project) error {
	if project.Spec.Namespace == nil || *project.Spec.Namespace == "" {
		return errors.New("project does not have a corresponding namespace set; most likely it has not yet been fully created")
	}

	return nil
}

func (tb *targetBuilderImpl) validateSeed(seed *gardencorev1beta1.Seed) error {
	if seed.Spec.SecretRef == nil {
		return errors.New("spec.SecretRef is missing in this seed, seed not reachable")
	}

	return nil
}

func (tb *targetBuilderImpl) validateShoot(shoot *gardencorev1beta1.Shoot) error {
	if shoot == nil {
		return errors.New("failed to validate shoot")
	}

	return nil
}
