/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/
package target

import (
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardenctl-v2/internal/gardenclient"
	"github.com/gardener/gardenctl-v2/pkg/config"
)

var (
	ErrNoGardenTargeted              = errors.New("no garden cluster targeted")
	ErrNoProjectTargeted             = errors.New("no project targeted")
	ErrNoSeedTargeted                = errors.New("no seed cluster targeted")
	ErrNoShootTargeted               = errors.New("no shoot targeted")
	ErrNeitherProjectNorSeedTargeted = errors.New("neither project nor seed are targeted")
	ErrNoControlPlaneTargeted        = errors.New("no control plane targeted")
)

//go:generate mockgen -destination=./mocks/mock_manager.go -package=mocks github.com/gardener/gardenctl-v2/pkg/target Manager

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
	// TargetControlPlane sets the control plane target flag
	TargetControlPlane(ctx context.Context) error
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
	// UnsetTargetControlPlane unsets the garden shoot configuration
	UnsetTargetControlPlane() (string, error)
	// TargetMatchPattern replaces the whole target
	// Garden, Project and Shoot values are determined by matching the provided value
	// against patterns defined in gardenctl configuration. Some values may only match a subset
	// of a pattern
	TargetMatchPattern(ctx context.Context, value string) error

	// Kubeconfig returns the kubeconfig for the given target cluster
	Kubeconfig(ctx context.Context, t Target) ([]byte, error)
	// WriteKubeconfig creates a kubeconfig file in the temporary directory of the os
	WriteKubeconfig(data []byte) (string, error)
	// SeedClient controller-runtime client for accessing the configured seed cluster
	SeedClient(ctx context.Context, t Target) (client.Client, error)
	// ShootClient controller-runtime client for accessing the configured shoot cluster
	ShootClient(ctx context.Context, t Target) (client.Client, error)

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
		tf = NewTargetFlags("", "", "", "", false)
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

	return m.updateTarget(target)
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
			t.ControlPlane = false

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

	return m.updateTarget(target)
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
			t.ControlPlane = false

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

	return m.updateTarget(target)
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

	return m.updateTarget(target)
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
			t.ControlPlane = false

			return nil
		})
	}

	return "", ErrNoShootTargeted
}

func (m *managerImpl) TargetControlPlane(ctx context.Context) error {
	tb := NewTargetBuilder(m.config, m.clientProvider)

	currentTarget, err := m.CurrentTarget()
	if err != nil {
		return fmt.Errorf("failed to get current target: %v", err)
	}

	tb.Init(currentTarget)

	target, err := tb.SetControlPlane(ctx).Build()
	if err != nil {
		return err
	}

	return m.updateTarget(target)
}

func (m *managerImpl) UnsetTargetControlPlane() (string, error) {
	currentTarget, err := m.CurrentTarget()
	if err != nil {
		return "", fmt.Errorf("failed to get current target: %v", err)
	}

	targetedName := currentTarget.ShootName()
	if currentTarget.ControlPlaneFlag() {
		return targetedName, m.patchTarget(func(t *targetImpl) error {
			t.ControlPlane = false

			return nil
		})
	}

	return "", ErrNoControlPlaneTargeted
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

	if m.TargetFlags().ControlPlane() {
		tb.SetControlPlane(ctx)
	}

	target, err := tb.Build()
	if err != nil {
		return err
	}

	return m.updateTarget(target)
}

func (m *managerImpl) updateTarget(target Target) error {
	return m.patchTarget(func(t *targetImpl) error {
		t.Garden = target.GardenName()
		t.Project = target.ProjectName()
		t.Seed = target.SeedName()
		t.Shoot = target.ShootName()
		t.ControlPlane = target.ControlPlaneFlag()

		return nil
	})
}

func (m *managerImpl) Kubeconfig(ctx context.Context, t Target) ([]byte, error) {
	if t.ShootName() != "" {
		return m.getKubeconfig(t, func(client gardenclient.Client) ([]byte, error) {
			var namespace string

			if t.ProjectName() != "" {
				project, err := client.GetProject(ctx, t.ProjectName())
				if err != nil {
					return nil, err
				}

				if project.Spec.Namespace == nil || *project.Spec.Namespace == "" {
					return nil, fmt.Errorf("project %q has not yet been assigned to a namespace", t.ProjectName())
				}

				namespace = *project.Spec.Namespace
			} else {
				shoot, err := client.FindShoot(ctx, t.AsListOption())
				if err != nil {
					return nil, err
				}

				namespace = shoot.Namespace
			}

			return client.GetShootKubeconfig(ctx, namespace, t.ShootName())
		})
	}

	if t.SeedName() != "" {
		return m.getKubeconfig(t, func(client gardenclient.Client) ([]byte, error) {
			return client.GetSeedKubeconfig(ctx, t.SeedName())
		})
	}

	if t.GardenName() != "" {
		return m.Configuration().Kubeconfig(t.GardenName())
	}

	return nil, ErrNoGardenTargeted
}

func (m *managerImpl) WriteKubeconfig(data []byte) (string, error) {
	tempDir := filepath.Join(os.TempDir(), "garden", getSessionID())

	err := os.MkdirAll(tempDir, os.ModePerm)
	if err != nil {
		return "", fmt.Errorf("failed to create temporary kubeconfig directory: %w", err)
	}

	filename := filepath.Join(tempDir, fmt.Sprintf("kubeconfig.%x.yaml", md5.Sum(data)))

	err = os.WriteFile(filename, data, 0600)
	if err != nil {
		return "", fmt.Errorf("failed to write temporary kubeconfig file to %s: %w", filename, err)
	}

	return filename, nil
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

	kubeconfig, err := m.Kubeconfig(ctx, t)
	if err != nil {
		return nil, err
	}

	return m.clientProvider.FromBytes(kubeconfig)
}

func (m *managerImpl) ShootClient(ctx context.Context, t Target) (client.Client, error) {
	t, err := m.getTarget(t)
	if err != nil {
		return nil, err
	}

	if t.GardenName() == "" {
		return nil, ErrNoGardenTargeted
	}

	if t.ShootName() == "" {
		return nil, ErrNoShootTargeted
	}

	kubeconfig, err := m.Kubeconfig(ctx, t)
	if err != nil {
		return nil, err
	}

	return m.clientProvider.FromBytes(kubeconfig)
}

func (m *managerImpl) getKubeconfig(t Target, loadKubeconfig func(gardenclient.Client) ([]byte, error)) ([]byte, error) {
	if value, err := m.kubeconfigCache.Read(t); err == nil {
		return value, nil
	}

	client, err := m.GardenClient(t.GardenName())
	if err != nil {
		return nil, fmt.Errorf("failed to create garden cluster client: %w", err)
	}

	value, err := loadKubeconfig(client)
	if err != nil {
		return nil, err
	}

	if err := m.kubeconfigCache.Write(t, value); err != nil {
		return nil, fmt.Errorf("failed to update kubeconfig cache: %w", err)
	}

	return value, nil
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

func getSessionID() string {
	var sid string

	if val, ok := os.LookupEnv("GCTL_SESSION_ID"); ok {
		sid = strings.ToLower(val)
	} else if val, ok = os.LookupEnv("ITERM_SESSION_ID"); ok {
		sid = strings.ToLower(val)
	}

	re := regexp.MustCompile(`([a-f0-9]{8}-[a-f0-9]{4}-4[a-f0-9]{3}-[89ab][a-f0-9]{3}-[a-f0-9]{12})`)

	match := re.FindStringSubmatch(sid)
	if len(match) > 1 {
		return match[1]
	}

	sid = uuid.New().String()
	os.Setenv("GCTL_SESSION_ID", sid)

	return sid
}
