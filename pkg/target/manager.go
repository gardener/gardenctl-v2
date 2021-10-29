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

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	ErrNoGardenTargeted  = errors.New("no garden cluster targeted")
	ErrNoProjectTargeted = errors.New("no project targeted")
	ErrNoSeedTargeted    = errors.New("no seed cluster targeted")
	ErrNoShootTargeted   = errors.New("no shoot targeted")
)

// Manager sets and gets the current target configuration
type Manager interface {
	// CurrentTarget contains the current target configuration
	CurrentTarget() (Target, error)

	// TargetFlags returns the global target flags
	TargetFlags() TargetFlags

	// TargetGarden sets the garden target configuration
	// This implicitly unsets project, seed and shoot target configuration
	TargetGarden(ctx context.Context, name string) error
	// TargetProject sets the project target configuration
	// This implicitly unsets seed and shoot target configuration
	TargetProject(ctx context.Context, name string) error
	// TargetSeed sets the seed target configuration
	// This implicitly unsets project and shoot target configuration
	TargetSeed(ctx context.Context, name string) error
	// TargetShoot sets the shoot target configuration
	// This implicitly unsets seed target configuration
	// It will also configure appropriate project and seed values if not already set
	TargetShoot(ctx context.Context, name string) error
	// UnsetTargetGarden unsets the garden target configuration
	// This implicitly unsets project, shoot and seed target configuration
	UnsetTargetGarden() (string, error)
	// UnsetTargetProject unsets the project target configuration
	// This implicitly unsets shoot target configuration
	UnsetTargetProject() (string, error)
	// UnsetTargetSeed unsets the garden seed configuration
	UnsetTargetSeed() (string, error)
	// UnsetTargetShoot unsets the garden shoot configuration
	UnsetTargetShoot() (string, error)
	// TargetMatchPattern replaces the whole target
	// Garden, Project and Shoot values are determined by matching the provided value
	// against patterns defined in gardenctl configuration. Some values may only match a subset
	// of a pattern
	TargetMatchPattern(ctx context.Context, value string) error

	// SeedClient controller-runtime client for accessing the configured seed cluster
	SeedClient(ctx context.Context, t Target) (client.Client, error)
	// ShootClusterClient controller-runtime client for accessing the configured shoot cluster
	ShootClusterClient(ctx context.Context, t Target) (client.Client, error)

	// Configuration returns the current gardenctl configuration
	Configuration() *config.Config

	// GardenClient returns a gardenClient for a garden cluster
	GardenClient(name string) (gardenclient.Client, error)
}

type managerImpl struct {
	config          *config.Config
	targetProvider  TargetProvider
	clientProvider  ClientProvider
	kubeconfigCache KubeconfigCache
}

var _ Manager = &managerImpl{}

// GardenClient creates a new Garden client by creating a runtime client via the ClientProvider
// it then wraps the runtime client and returns a Garden client
func GardenClient(name string, config *config.Config, provider ClientProvider) (gardenclient.Client, error) {
	for _, g := range config.Gardens {
		if g.Name == name {
			runtimeClient, err := provider.FromFile(g.Kubeconfig)
			if err != nil {
				return nil, err
			}

			return gardenclient.NewGardenClient(runtimeClient), nil
		}
	}

	return nil, fmt.Errorf("targeted garden cluster %q is not configured", name)
}

// NewManager returns a new manager
func NewManager(config *config.Config, targetProvider TargetProvider, clientProvider ClientProvider, kubeconfigCache KubeconfigCache) (Manager, error) {
	return &managerImpl{
		config:          config,
		targetProvider:  targetProvider,
		clientProvider:  clientProvider,
		kubeconfigCache: kubeconfigCache,
	}, nil
}

func (m *managerImpl) CurrentTarget() (Target, error) {
	return m.targetProvider.Read()
}

func (m *managerImpl) TargetFlags() TargetFlags {
	var tf TargetFlags

	if dtp, ok := m.targetProvider.(*dynamicTargetProvider); ok {
		tf = dtp.targetFlags
	}

	if tf == nil {
		tf = NewTargetFlags("", "", "", "")
	}

	return tf
}

func (m *managerImpl) Configuration() *config.Config {
	return m.config
}

func (m *managerImpl) TargetGarden(ctx context.Context, gardenNameOrAlias string) error {
	tb := NewTargetBuilder(m.config, m.clientProvider)

	currentTarget, err := m.CurrentTarget()
	if err != nil {
		return fmt.Errorf("failed to get current target: %v", err)
	}

	tb.Init(currentTarget)

	target, err := tb.SetGarden(gardenNameOrAlias).Build()
	if err != nil {
		return err
	}

	return m.patchTargetWithTarget(target)
}

func (m *managerImpl) UnsetTargetGarden() (string, error) {
	currentTarget, err := m.CurrentTarget()
	if err != nil {
		return "", fmt.Errorf("failed to get current target: %v", err)
	}

	targetedName := currentTarget.GardenName()
	if targetedName != "" {
		return targetedName, m.patchTarget(func(t *targetImpl) error {
			t.Garden = ""
			t.Project = ""
			t.Seed = ""
			t.Shoot = ""

			return nil
		})
	}

	return "", ErrNoGardenTargeted
}

func (m *managerImpl) TargetProject(ctx context.Context, projectName string) error {
	tb := NewTargetBuilder(m.config, m.clientProvider)

	currentTarget, err := m.CurrentTarget()
	if err != nil {
		return fmt.Errorf("failed to get current target: %v", err)
	}

	tb.Init(currentTarget)

	target, err := tb.SetProject(ctx, projectName).Build()
	if err != nil {
		return err
	}

	return m.patchTargetWithTarget(target)
}

func (m *managerImpl) UnsetTargetProject() (string, error) {
	currentTarget, err := m.CurrentTarget()
	if err != nil {
		return "", fmt.Errorf("failed to get current target: %v", err)
	}

	targetedName := currentTarget.ProjectName()
	if targetedName != "" {
		return targetedName, m.patchTarget(func(t *targetImpl) error {
			t.Project = ""
			t.Shoot = ""

			return nil
		})
	}

	return "", ErrNoProjectTargeted
}

func (m *managerImpl) TargetSeed(ctx context.Context, seedName string) error {
	tb := NewTargetBuilder(m.config, m.clientProvider)

	currentTarget, err := m.CurrentTarget()
	if err != nil {
		return fmt.Errorf("failed to get current target: %v", err)
	}

	tb.Init(currentTarget)

	target, err := tb.SetSeed(ctx, seedName).Build()
	if err != nil {
		return err
	}

	return m.patchTargetWithTarget(target)
}

func (m *managerImpl) UnsetTargetSeed() (string, error) {
	currentTarget, err := m.CurrentTarget()
	if err != nil {
		return "", fmt.Errorf("failed to get current target: %v", err)
	}

	targetedName := currentTarget.SeedName()
	if targetedName != "" {
		return targetedName, m.patchTarget(func(t *targetImpl) error {
			t.Seed = ""

			return nil
		})
	}

	return "", ErrNoSeedTargeted
}

func (m *managerImpl) TargetShoot(ctx context.Context, shootName string) error {
	tb := NewTargetBuilder(m.config, m.clientProvider)

	currentTarget, err := m.CurrentTarget()
	if err != nil {
		return fmt.Errorf("failed to get current target: %v", err)
	}

	tb.Init(currentTarget)

	target, err := tb.SetShoot(ctx, shootName).Build()
	if err != nil {
		return err
	}

	return m.patchTargetWithTarget(target)
}

func (m *managerImpl) UnsetTargetShoot() (string, error) {
	currentTarget, err := m.CurrentTarget()
	if err != nil {
		return "", fmt.Errorf("failed to get current target: %v", err)
	}

	targetedName := currentTarget.ShootName()
	if targetedName != "" {
		return targetedName, m.patchTarget(func(t *targetImpl) error {
			t.Shoot = ""

			return nil
		})
	}

	return "", ErrNoShootTargeted
}

func (m *managerImpl) TargetMatchPattern(ctx context.Context, value string) error {
	tm, err := m.config.MatchPattern(value)
	if err != nil {
		return fmt.Errorf("error occurred while trying to match value: %w", err)
	}

	tb := NewTargetBuilder(m.config, m.clientProvider)

	currentTarget, err := m.CurrentTarget()
	if err != nil {
		return fmt.Errorf("failed to get current target: %v", err)
	}

	tb.Init(currentTarget)

	if err != nil {
		return err
	}

	if tm.Project != "" && tm.Namespace != "" {
		return fmt.Errorf("project %q and Namespace %q set in target match value. It is forbidden to have both values set", tm.Project, tm.Namespace)
	}

	if tm.Garden != "" {
		tb.SetGarden(tm.Garden)
	}

	if tm.Project != "" {
		tb.SetProject(ctx, tm.Project)
	}

	if tm.Namespace != "" {
		tb.SetNamespace(ctx, tm.Namespace)
	}

	if tm.Shoot != "" {
		tb.SetShoot(ctx, tm.Shoot)
	}

	target, err := tb.Build()
	if err != nil {
		return err
	}

	return m.patchTargetWithTarget(target)
}

func (m *managerImpl) patchTargetWithTarget(target Target) error {
	return m.patchTarget(func(t *targetImpl) error {
		t.Garden = target.GardenName()
		t.Project = target.ProjectName()
		t.Seed = target.SeedName()
		t.Shoot = target.ShootName()

		return nil
	})
}

func (m *managerImpl) SeedClient(ctx context.Context, t Target) (client.Client, error) {
	t, err := m.getTarget(t)
	if err != nil {
		return nil, err
	}

	if t.GardenName() == "" {
		return nil, ErrNoGardenTargeted
	}

	if t.SeedName() == "" {
		return nil, ErrNoSeedTargeted
	}

	kubeconfig, err := m.ensureSeedKubeconfig(ctx, t)
	if err != nil {
		return nil, err
	}

	return m.clientProvider.FromBytes(kubeconfig)
}

func (m *managerImpl) ensureSeedKubeconfig(ctx context.Context, t Target) ([]byte, error) {
	if kubeconfig, err := m.kubeconfigCache.Read(t); err == nil {
		return kubeconfig, nil
	}

	gardenClient, err := m.GardenClient(t.GardenName())
	if err != nil {
		return nil, fmt.Errorf("failed to create garden cluster client: %w", err)
	}

	seed, err := gardenClient.GetSeed(ctx, t.SeedName())
	if err != nil {
		return nil, fmt.Errorf("invalid seed cluster: %w", err)
	}

	secret, err := gardenClient.GetSecret(ctx, seed.Spec.SecretRef.Namespace, seed.Spec.SecretRef.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve seed kubeconfig: %w", err)
	}

	if err := m.kubeconfigCache.Write(t, secret.Data["kubeconfig"]); err != nil {
		return nil, fmt.Errorf("failed to update kubeconfig cache: %w", err)
	}

	return secret.Data["kubeconfig"], nil
}

func (m *managerImpl) ShootClusterClient(ctx context.Context, t Target) (client.Client, error) {
	t, err := m.getTarget(t)
	if err != nil {
		return nil, err
	}

	if t.GardenName() == "" {
		return nil, ErrNoGardenTargeted
	}

	// Even if a user targets a shoot directly, without specifying a seed/project,
	// during that operation a seed/project will be selected and saved in the
	// target file; that's why this check can still demand a parent target for
	// the shoot, which is also needed because we need to locate the kubeconfig
	// on disk.
	if t.SeedName() == "" && t.ProjectName() == "" {
		return nil, errors.New("neither project nor seed are targeted")
	}

	if t.ShootName() == "" {
		return nil, ErrNoShootTargeted
	}

	kubeconfig, err := m.ensureShootKubeconfig(ctx, t)
	if err != nil {
		return nil, err
	}

	return m.clientProvider.FromBytes(kubeconfig)
}

func (m *managerImpl) ensureShootKubeconfig(ctx context.Context, t Target) ([]byte, error) {
	if kubeconfig, err := m.kubeconfigCache.Read(t); err == nil {
		return kubeconfig, nil
	}

	gardenClient, err := m.GardenClient(t.GardenName())
	if err != nil {
		return nil, fmt.Errorf("failed to create garden cluster client: %w", err)
	}

	shoot, err := gardenClient.FindShoot(ctx, t.ShootName(), t.ProjectName(), t.SeedName())
	if err != nil {
		return nil, fmt.Errorf("failed to fetch shoot: %w", err)
	}

	secret, err := gardenClient.GetSecret(ctx, shoot.Namespace, fmt.Sprintf("%s.kubeconfig", shoot.Name))
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve seed kubeconfig: %w", err)
	}

	if err := m.kubeconfigCache.Write(t, secret.Data["kubeconfig"]); err != nil {
		return nil, fmt.Errorf("failed to update kubeconfig cache: %w", err)
	}

	return secret.Data["kubeconfig"], nil
}

func (m *managerImpl) patchTarget(patch func(t *targetImpl) error) error {
	target, err := m.targetProvider.Read()
	if err != nil {
		return err
	}

	// this is horrible cheating
	impl, ok := target.(*targetImpl)
	if !ok {
		return errors.New("target must be using targetImpl as its underlying type")
	}

	if err := patch(impl); err != nil {
		return err
	}

	return m.targetProvider.Write(impl)
}

func (m *managerImpl) getTarget(t Target) (Target, error) {
	var err error
	if t == nil {
		t, err = m.targetProvider.Read()
	}

	return t, err
}

func (m *managerImpl) GardenClient(name string) (gardenclient.Client, error) {
	return GardenClient(name, m.config, m.clientProvider)
}
