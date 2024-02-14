/*
SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package resolve

import (
	"context"
	"errors"
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/spf13/cobra"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clientgarden "github.com/gardener/gardenctl-v2/internal/client/garden"
	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/ac"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
	"github.com/gardener/gardenctl-v2/pkg/config"
	"github.com/gardener/gardenctl-v2/pkg/target"
)

// options is a struct to support resolve command.
type options struct {
	base.Options

	// Kind is the kind to resolve, for example "garden", "project", "seed" or "shoot"
	Kind Kind

	// CurrentTarget holds the current target configuration
	CurrentTarget target.Target

	// Garden is the garden config, depending on the current target
	Garden *config.Garden

	// GardenClient is the client for the garden cluster
	GardenClient clientgarden.Client
}

// Kind is representing the type of things that can be resolved.
type Kind string

const (
	KindGarden  Kind = "garden"
	KindProject Kind = "project"
	KindSeed    Kind = "seed"
	KindShoot   Kind = "shoot"
)

// Garden represents a garden cluster.
type Garden struct {
	// Name is a unique identifier of this Garden that can be used to target this Garden
	Name string `json:"name"`

	// Alias is a unique identifier of this Garden that can be used as an alternate name to target this Garden
	Alias string `json:"alias,omitempty"`
}

// Project represents a gardener project.
type Project struct {
	// Name is the name of the project.
	Name string `json:"name"`

	// Namespace is the namespace within which the project exists.
	Namespace *string `json:"namespace,omitempty"`
}

// Shoot represents a shoot cluster.
type Shoot struct {
	// Name is the name of the shoot cluster.
	Name string `json:"name"`

	// Namespace is the namespace within which the shoot exists.
	Namespace string `json:"namespace"`

	// AccessRestriction holds the rendered access restriction messages
	AccessRestriction *string `json:"accessRestriction,omitempty"`
}

// Seed represents a seed cluster.
type Seed struct {
	// Name is the name of the seed cluster.
	Name string `json:"name"`
}

// ResolvedTarget represents the resolved target.
// It contains the details of the Garden, Project, Shoot, and Seed.
type ResolvedTarget struct {
	// Garden is the garden where the clusters are hosted.
	Garden Garden `json:"garden"`

	// Project is the project related to the resolved target. It is optional, hence the omitempty tag.
	Project *Project `json:"project,omitempty"`

	// Shoot is the shoot cluster related to the resolved target. It is optional, hence the omitempty tag.
	Shoot *Shoot `json:"shoot,omitempty"`

	// Seed is the seed cluster related to the resolved target. It is optional, hence the omitempty tag.
	Seed *Seed `json:"seed,omitempty"`
}

// newOptions returns initialized options.
func newOptions(ioStreams util.IOStreams, kind Kind) *options {
	return &options{
		Options: base.Options{
			IOStreams: ioStreams,
			Output:    "yaml",
		},
		Kind: kind,
	}
}

// Complete adapts from the command line args to the data required.
func (o *options) Complete(f util.Factory, _ *cobra.Command, _ []string) error {
	manager, err := f.Manager()
	if err != nil {
		return err
	}

	currentTarget, err := manager.CurrentTarget()
	if err != nil {
		return err
	}

	if currentTarget.GardenName() == "" {
		return target.ErrNoGardenTargeted
	}

	o.CurrentTarget = currentTarget

	garden, err := manager.Configuration().Garden(currentTarget.GardenName())
	if err != nil {
		return err
	}

	o.Garden = garden

	gardenClient, err := manager.GardenClient(currentTarget.GardenName())
	if err != nil {
		return err
	}

	o.GardenClient = gardenClient

	return nil
}

// Validate validates the provided command options.
func (o *options) Validate() error {
	if o.Options.Output == "" {
		return errors.New("output must be 'yaml' or 'json'")
	}

	return nil
}

// Run does the actual work of the command.
func (o *options) Run(f util.Factory) error {
	ctx := f.Context()

	resolvedTarget := ResolvedTarget{
		Garden: Garden{
			Name:  o.Garden.Name,
			Alias: o.Garden.Alias,
		},
	}

	if o.Kind == KindGarden {
		return o.PrintObject(resolvedTarget)
	}

	if o.CurrentTarget.ProjectName() != "" && o.Kind == KindProject {
		project, err := o.GardenClient.GetProject(ctx, o.CurrentTarget.ProjectName())
		if err != nil {
			return err
		}

		resolvedTarget.Project = &Project{
			Name:      project.Name,
			Namespace: project.Spec.Namespace,
		}

		return o.PrintObject(resolvedTarget)
	}

	if o.CurrentTarget.SeedName() != "" && o.Kind == KindSeed {
		// We already have the seed name, however we get the seed in order to verify that it exists
		seed, err := o.GardenClient.GetSeed(ctx, o.CurrentTarget.SeedName())
		if err != nil {
			return err
		}

		resolvedTarget.Seed = &Seed{
			Name: seed.Name,
		}

		return o.PrintObject(resolvedTarget)
	}

	shoot, err := findShoot(ctx, o.GardenClient, o.CurrentTarget)
	if err != nil {
		if errors.Is(err, target.ErrNoShootTargeted) {
			switch o.Kind {
			case KindProject:
				return target.ErrNoProjectTargeted
			case KindSeed:
				return target.ErrNoSeedTargeted
			}
		}

		return err
	}

	if o.Kind == KindSeed {
		resolvedTarget.Seed = &Seed{
			Name: *shoot.Spec.SeedName,
		}

		return o.PrintObject(resolvedTarget)
	}

	if o.CurrentTarget.ControlPlane() {
		shoot, err = findShoot(ctx, o.GardenClient, o.CurrentTarget.WithSeedName("").WithProjectName("garden").WithShootName(*shoot.Spec.SeedName).WithControlPlane(false))
		if err != nil {
			return err
		}
	}

	resolvedTarget.Seed = &Seed{
		Name: *shoot.Spec.SeedName,
	}

	if resolvedTarget.Project == nil {
		project, err := o.GardenClient.GetProjectByNamespace(ctx, shoot.Namespace)
		if err != nil {
			return err
		}

		resolvedTarget.Project = &Project{
			Name:      project.Name,
			Namespace: project.Spec.Namespace,
		}
	}

	if o.Kind == KindProject {
		resolvedTarget.Seed = nil
		return o.PrintObject(resolvedTarget)
	}

	resolvedTarget.Shoot = &Shoot{
		Name:      shoot.Name,
		Namespace: shoot.Namespace,
	}

	messages := ac.CheckAccessRestrictions(o.Garden.AccessRestrictions, shoot)
	if len(messages) != 0 {
		resolvedTarget.Shoot.AccessRestriction = ptr.To(messages.String())
	}

	return o.PrintObject(resolvedTarget)
}

func findShoot(ctx context.Context, gardenclient clientgarden.Client, t target.Target) (*gardencorev1beta1.Shoot, error) {
	opt, err := shootListOption(ctx, gardenclient, t)
	if err != nil {
		return nil, err
	}

	shoot, err := gardenclient.FindShoot(ctx, opt)
	if err != nil {
		return nil, err
	}

	if shoot.Spec.SeedName == nil {
		return nil, fmt.Errorf("no seed assigned to shoot %s/%s", shoot.Namespace, shoot.Name)
	}

	return shoot, nil
}

// shootListOption returns the list options for the shoot.
// If no shoot or seed (that is a managed seed) was targeted, target.ErrNoShootTargeted is returned.
func shootListOption(ctx context.Context, gardenClient clientgarden.Client, t target.Target) (client.ListOption, error) {
	if t.ShootName() != "" {
		return t.AsListOption(), nil
	}

	if t.SeedName() != "" {
		shootOfManagedSeed, err := gardenClient.GetShootOfManagedSeed(ctx, t.SeedName())
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil, fmt.Errorf("%s is not a managed seed: %w", t.SeedName(), err)
			}

			return nil, err
		}

		return clientgarden.ProjectFilter{
			"metadata.name": shootOfManagedSeed.Name,
			"project":       "garden",
		}, nil
	}

	return nil, target.ErrNoShootTargeted
}
