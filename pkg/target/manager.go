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
	"path"
	"path/filepath"

	"k8s.io/client-go/tools/clientcmd"
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
	UnsetTargetGarden(ctx context.Context) (string, error)
	// UnsetTargetProject unsets the project target configuration
	// This implicitly unsets shoot target configuration
	UnsetTargetProject(ctx context.Context) (string, error)
	// UnsetTargetSeed unsets the garden seed configuration
	UnsetTargetSeed(ctx context.Context) (string, error)
	// UnsetTargetShoot unsets the garden shoot configuration
	UnsetTargetShoot(ctx context.Context) (string, error)
	// UnsetTargetControlPlane unsets the control plane flag
	UnsetTargetControlPlane(ctx context.Context) error
	// TargetMatchPattern replaces the whole target
	// Garden, Project and Shoot values are determined by matching the provided value
	// against patterns defined in gardenctl configuration. Some values may only match a subset
	// of a pattern
	TargetMatchPattern(ctx context.Context, value string) error

	//ClientConfig returns the client config for a target
	ClientConfig(ctx context.Context, t Target) (clientcmd.ClientConfig, error)
	// WriteClientConfig creates a kubeconfig file in the session directory of the operating system
	WriteClientConfig(config clientcmd.ClientConfig) (string, error)
	// SeedClient controller-runtime client for accessing the configured seed cluster
	SeedClient(ctx context.Context, t Target) (client.Client, error)
	// ShootClient controller-runtime client for accessing the configured shoot cluster
	ShootClient(ctx context.Context, t Target) (client.Client, error)

	// SessionDir is the path of the session directory. All state related to
	// the current gardenctl shell session will be stored under this directory.
	SessionDir() string
	// Configuration returns the current gardenctl configuration
	Configuration() *config.Config

	// GardenClient returns a gardenClient for a garden cluster
	GardenClient(name string) (gardenclient.Client, error)
}

type managerImpl struct {
	config           *config.Config
	targetProvider   TargetProvider
	clientProvider   ClientProvider
	sessionDirectory string
}

var _ Manager = &managerImpl{}

func newGardenClient(name string, config *config.Config, provider ClientProvider) (gardenclient.Client, error) {
	clientConfig, err := config.ClientConfig(name)
	if err != nil {
		return nil, err
	}

	client, err := provider.FromClientConfig(clientConfig)
	if err != nil {
		return nil, err
	}

	return gardenclient.NewGardenClient(client, name), nil
}

// NewManager returns a new manager
func NewManager(config *config.Config, targetProvider TargetProvider, clientProvider ClientProvider, sessionDirectory string) (Manager, error) {
	return &managerImpl{
		config:           config,
		targetProvider:   targetProvider,
		clientProvider:   clientProvider,
		sessionDirectory: sessionDirectory,
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

func (m *managerImpl) TargetGarden(ctx context.Context, identity string) error {
	tb, err := NewTargetBuilder(m.config, m.clientProvider)
	if err != nil {
		return fmt.Errorf("failed to create new target builder: %w", err)
	}

	currentTarget, err := m.CurrentTarget()
	if err != nil {
		return fmt.Errorf("failed to get current target: %w", err)
	}

	tb.Init(currentTarget)

	target, err := tb.SetGarden(identity).Build()
	if err != nil {
		return fmt.Errorf("failed to build target: %w", err)
	}

	return m.updateTarget(ctx, target)
}

func (m *managerImpl) UnsetTargetGarden(ctx context.Context) (string, error) {
	currentTarget, err := m.CurrentTarget()
	if err != nil {
		return "", fmt.Errorf("failed to get current target: %w", err)
	}

	targetedGarden := currentTarget.GardenName()
	if targetedGarden == "" {
		return "", ErrNoGardenTargeted
	}

	return targetedGarden, m.patchTarget(ctx, func(t *targetImpl) error {
		t.Garden = ""
		t.Project = ""
		t.Seed = ""
		t.Shoot = ""
		t.ControlPlaneFlag = false

		return nil
	})
}

func (m *managerImpl) TargetProject(ctx context.Context, projectName string) error {
	tb, err := NewTargetBuilder(m.config, m.clientProvider)
	if err != nil {
		return fmt.Errorf("failed to create new target builder: %w", err)
	}

	currentTarget, err := m.CurrentTarget()
	if err != nil {
		return fmt.Errorf("failed to get current target: %w", err)
	}

	tb.Init(currentTarget)

	target, err := tb.SetProject(ctx, projectName).Build()
	if err != nil {
		return err
	}

	return m.updateTarget(ctx, target)
}

func (m *managerImpl) UnsetTargetProject(ctx context.Context) (string, error) {
	currentTarget, err := m.CurrentTarget()
	if err != nil {
		return "", fmt.Errorf("failed to get current target: %w", err)
	}

	targetedName := currentTarget.ProjectName()
	if targetedName == "" {
		return "", ErrNoProjectTargeted
	}

	return targetedName, m.patchTarget(ctx, func(t *targetImpl) error {
		t.Project = ""
		t.Shoot = ""
		t.ControlPlaneFlag = false

		return nil
	})
}

func (m *managerImpl) TargetSeed(ctx context.Context, seedName string) error {
	tb, err := NewTargetBuilder(m.config, m.clientProvider)
	if err != nil {
		return fmt.Errorf("failed to create new target builder: %w", err)
	}

	currentTarget, err := m.CurrentTarget()
	if err != nil {
		return fmt.Errorf("failed to get current target: %w", err)
	}

	tb.Init(currentTarget)

	target, err := tb.SetSeed(ctx, seedName).Build()
	if err != nil {
		return err
	}

	return m.updateTarget(ctx, target)
}

func (m *managerImpl) UnsetTargetSeed(ctx context.Context) (string, error) {
	currentTarget, err := m.CurrentTarget()
	if err != nil {
		return "", fmt.Errorf("failed to get current target: %w", err)
	}

	targetedName := currentTarget.SeedName()
	if targetedName == "" {
		return "", ErrNoSeedTargeted
	}

	return targetedName, m.patchTarget(ctx, func(t *targetImpl) error {
		t.Seed = ""

		return nil
	})
}

func (m *managerImpl) TargetShoot(ctx context.Context, shootName string) error {
	tb, err := NewTargetBuilder(m.config, m.clientProvider)
	if err != nil {
		return fmt.Errorf("failed to create new target builder: %w", err)
	}

	currentTarget, err := m.CurrentTarget()
	if err != nil {
		return fmt.Errorf("failed to get current target: %w", err)
	}

	tb.Init(currentTarget)

	target, err := tb.SetShoot(ctx, shootName).Build()
	if err != nil {
		return err
	}

	return m.updateTarget(ctx, target)
}

func (m *managerImpl) UnsetTargetShoot(ctx context.Context) (string, error) {
	currentTarget, err := m.CurrentTarget()
	if err != nil {
		return "", fmt.Errorf("failed to get current target: %w", err)
	}

	targetedName := currentTarget.ShootName()
	if targetedName == "" {
		return "", ErrNoShootTargeted
	}

	return targetedName, m.patchTarget(ctx, func(t *targetImpl) error {
		t.Shoot = ""
		t.ControlPlaneFlag = false

		return nil
	})
}

func (m *managerImpl) TargetControlPlane(ctx context.Context) error {
	tb, err := NewTargetBuilder(m.config, m.clientProvider)
	if err != nil {
		return fmt.Errorf("failed to create new target builder: %w", err)
	}

	currentTarget, err := m.CurrentTarget()
	if err != nil {
		return fmt.Errorf("failed to get current target: %w", err)
	}

	tb.Init(currentTarget)

	target, err := tb.SetControlPlane(ctx).Build()
	if err != nil {
		return err
	}

	return m.updateTarget(ctx, target)
}

func (m *managerImpl) UnsetTargetControlPlane(ctx context.Context) error {
	currentTarget, err := m.CurrentTarget()
	if err != nil {
		return fmt.Errorf("failed to get current target: %w", err)
	}

	if !currentTarget.ControlPlane() {
		return ErrNoControlPlaneTargeted
	}

	return m.patchTarget(ctx, func(t *targetImpl) error {
		t.ControlPlaneFlag = false

		return nil
	})
}

func (m *managerImpl) TargetMatchPattern(ctx context.Context, value string) error {
	currentTarget, err := m.CurrentTarget()
	if err != nil {
		return fmt.Errorf("failed to get current target: %w", err)
	}

	gardenName := currentTarget.GardenName()

	if m.config == nil {
		return errors.New("config must not be nil")
	}

	tm, err := m.config.MatchPattern(gardenName, value)
	if err != nil {
		return fmt.Errorf("error occurred while trying to match value: %w", err)
	}

	tb, err := NewTargetBuilder(m.config, m.clientProvider)
	if err != nil {
		return fmt.Errorf("failed to create new target builder: %w", err)
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

	return m.updateTarget(ctx, target)
}

func (m *managerImpl) updateTarget(ctx context.Context, target Target) error {
	return m.patchTarget(ctx, func(t *targetImpl) error {
		t.Garden = target.GardenName()
		t.Project = target.ProjectName()
		t.Seed = target.SeedName()
		t.Shoot = target.ShootName()
		t.ControlPlaneFlag = target.ControlPlane()

		return nil
	})
}

func (m *managerImpl) ClientConfig(ctx context.Context, t Target) (clientcmd.ClientConfig, error) {
	if t.ControlPlane() {
		return m.getClientConfig(t, func(client gardenclient.Client) (clientcmd.ClientConfig, error) {
			shoot, err := client.FindShoot(ctx, t.WithControlPlane(false).AsListOption())
			if err != nil {
				return nil, err
			}

			if shoot.Spec.SeedName == nil || *shoot.Spec.SeedName == "" {
				return nil, fmt.Errorf("shoot %q has not yet been assigned to a seed", t.ShootName())
			}

			if shoot.Status.TechnicalID == "" {
				return nil, fmt.Errorf("no technicalID has been assigned to the shoot %q yet", t.ShootName())
			}

			clientConfig, err := client.GetSeedClientConfig(ctx, *shoot.Spec.SeedName)
			if err != nil {
				return nil, err
			}

			return clientConfigWithNamespace(clientConfig, shoot.Status.TechnicalID)
		})
	}

	if t.ShootName() != "" {
		return m.getClientConfig(t, func(client gardenclient.Client) (clientcmd.ClientConfig, error) {
			var namespace string

			if t.ProjectName() != "" {
				projectNamespace, err := getProjectNamespace(ctx, client, t.ProjectName())
				if err != nil {
					return nil, err
				}

				namespace = *projectNamespace
			} else {
				shoot, err := client.FindShoot(ctx, t.AsListOption())
				if err != nil {
					return nil, err
				}

				namespace = shoot.Namespace
			}

			return client.GetShootClientConfig(ctx, namespace, t.ShootName())
		})
	}

	if t.SeedName() != "" {
		return m.getClientConfig(t, func(client gardenclient.Client) (clientcmd.ClientConfig, error) {
			return client.GetSeedClientConfig(ctx, t.SeedName())
		})
	}

	if t.ProjectName() != "" {
		return m.getClientConfig(t, func(client gardenclient.Client) (clientcmd.ClientConfig, error) {
			clientConfig, err := m.Configuration().DirectClientConfig(t.GardenName())
			if err != nil {
				return nil, err
			}

			projectNamespace, err := getProjectNamespace(ctx, client, t.ProjectName())
			if err != nil {
				return nil, err
			}

			return clientConfigWithNamespace(clientConfig, *projectNamespace)
		})
	}

	if t.GardenName() != "" {
		return m.Configuration().DirectClientConfig(t.GardenName())
	}

	return nil, ErrNoGardenTargeted
}

func (m *managerImpl) WriteClientConfig(config clientcmd.ClientConfig) (string, error) {
	data, err := writeRawConfig(config)
	if err != nil {
		return "", fmt.Errorf("failed to serialize temporary kubeconfig file: %w", err)
	}

	filename := filepath.Join(m.sessionDirectory, fmt.Sprintf("kubeconfig.%x.yaml", md5.Sum(data)))

	err = os.WriteFile(filename, data, 0600)
	if err != nil {
		return "", fmt.Errorf("failed to write temporary kubeconfig file to %s: %w", filename, err)
	}

	return filename, nil
}

func (m *managerImpl) SessionDir() string {
	return m.sessionDirectory
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

	config, err := m.ClientConfig(ctx, t)
	if err != nil {
		return nil, err
	}

	return m.clientProvider.FromClientConfig(config)
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

	config, err := m.ClientConfig(ctx, t)
	if err != nil {
		return nil, err
	}

	return m.clientProvider.FromClientConfig(config)
}

func (m *managerImpl) getClientConfig(t Target, loadClientConfig func(gardenclient.Client) (clientcmd.ClientConfig, error)) (clientcmd.ClientConfig, error) {
	client, err := m.GardenClient(t.GardenName())
	if err != nil {
		return nil, fmt.Errorf("failed to create garden cluster client: %w", err)
	}

	return loadClientConfig(client)
}

func (m *managerImpl) patchTarget(ctx context.Context, patch func(t *targetImpl) error) error {
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

	err = m.targetProvider.Write(impl)
	if err != nil {
		return err
	}

	if !m.config.SymlinkTargetKubeconfig() {
		return nil
	}

	return m.updateClientConfigSymlink(ctx, target)
}

func (m *managerImpl) updateClientConfigSymlink(ctx context.Context, target Target) error {
	symlinkPath := path.Join(m.sessionDirectory, "kubeconfig.yaml")

	_, err := os.Lstat(symlinkPath)
	if err == nil {
		err = os.Remove(symlinkPath)
		if err != nil {
			return err
		}
	}

	config, err := m.ClientConfig(ctx, target)
	if err != nil {
		if errors.Is(err, ErrNoGardenTargeted) {
			return nil
		}

		return err
	}

	filename, err := m.WriteClientConfig(config)
	if err != nil {
		return err
	}

	return os.Symlink(filename, symlinkPath)
}

func (m *managerImpl) getTarget(t Target) (Target, error) {
	var err error
	if t == nil {
		t, err = m.targetProvider.Read()
	}

	return t, err
}

func (m *managerImpl) GardenClient(name string) (gardenclient.Client, error) {
	return newGardenClient(name, m.config, m.clientProvider)
}

func writeRawConfig(config clientcmd.ClientConfig) ([]byte, error) {
	rawConfig, err := config.RawConfig()
	if err != nil {
		return nil, err
	}

	return clientcmd.Write(rawConfig)
}

func getProjectNamespace(ctx context.Context, client gardenclient.Client, name string) (*string, error) {
	project, err := client.GetProject(ctx, name)
	if err != nil {
		return nil, err
	}

	if project.Spec.Namespace == nil || *project.Spec.Namespace == "" {
		return nil, fmt.Errorf("project %q has not yet been assigned to a namespace", name)
	}

	return project.Spec.Namespace, nil
}

func clientConfigWithNamespace(clientConfig clientcmd.ClientConfig, namespace string) (clientcmd.ClientConfig, error) {
	rawConfig, err := clientConfig.RawConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get raw client configuration: %w", err)
	}

	err = clientcmd.Validate(rawConfig)
	if err != nil {
		return nil, fmt.Errorf("validation of client configuration failed: %w", err)
	}

	for _, context := range rawConfig.Contexts {
		context.Namespace = namespace
	}

	overrides := &clientcmd.ConfigOverrides{}

	return clientcmd.NewDefaultClientConfig(rawConfig, overrides), nil
}
