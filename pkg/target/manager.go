/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/
package target

import (
	"context"
	"errors"
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	ErrNoGardenTargeted  = errors.New("no garden cluster targeted")
	ErrNoProjectTargeted = errors.New("no project targeted")
	ErrNoSeedTargeted    = errors.New("no seed cluster targeted")
	ErrNoShootTargeted   = errors.New("no shoot targeted")
)

type Manager interface {
	CurrentTarget() (Target, error)

	TargetGarden(name string) error
	TargetProject(ctx context.Context, name string) error
	TargetSeed(ctx context.Context, name string) error
	TargetShoot(ctx context.Context, name string) error

	GardenClient(t Target) (client.Client, error)
	ProjectClient(t Target) (client.Client, error)
	SeedClient(t Target) (client.Client, error)
	ShootNamespaceClient(t Target) (client.Client, error)
	ShootClusterClient(t Target) (client.Client, error)
}

type managerImpl struct {
	config          *Config
	targetProvider  TargetProvider
	clientProvider  ClientProvider
	kubeconfigCache KubeconfigCache
}

var _ Manager = &managerImpl{}

func NewManager(config *Config, targetProvider TargetProvider, clientProvider ClientProvider, kubeconfigCache KubeconfigCache) (Manager, error) {
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

func (m *managerImpl) TargetGarden(gardenName string) error {
	for _, g := range m.config.Gardens {
		if g.Name == gardenName {
			return m.patchTarget(func(t *targetImpl) error {
				t.Garden = gardenName
				t.Project = ""
				t.Seed = ""
				t.Shoot = ""

				return nil
			})
		}
	}

	return fmt.Errorf("garden %q is not defined in gardenctl configuration", gardenName)
}

func (m *managerImpl) TargetProject(ctx context.Context, projectName string) error {
	return m.patchTarget(func(t *targetImpl) error {
		if t.Garden == "" {
			return ErrNoGardenTargeted
		}

		gardenClient, err := m.clientForGarden(t.Garden)
		if err != nil {
			return fmt.Errorf("could not create Kubernetes client for garden cluster: %w", err)
		}

		// validate that the project exists
		if _, err := m.resolveProjectName(ctx, gardenClient, projectName); err != nil {
			return fmt.Errorf("failed to validate project: %w", err)
		}

		t.Seed = ""
		t.Project = projectName
		t.Shoot = ""

		return nil
	})
}

func (m *managerImpl) resolveProjectName(ctx context.Context, gardenClient client.Client, projectName string) (*gardencorev1beta1.Project, error) {
	// validate that the project exists
	project := &gardencorev1beta1.Project{}
	key := types.NamespacedName{Name: projectName}
	err := gardenClient.Get(ctx, key, project)

	return project, err
}

func (m *managerImpl) TargetSeed(ctx context.Context, seedName string) error {
	return m.patchTarget(func(t *targetImpl) error {
		if t.Garden == "" {
			return ErrNoGardenTargeted
		}

		gardenClient, err := m.clientForGarden(t.Garden)
		if err != nil {
			return fmt.Errorf("could not create Kubernetes client for garden cluster: %w", err)
		}

		// validate that the seed exists
		seed, err := m.resolveSeedName(ctx, gardenClient, seedName)
		if err != nil {
			return fmt.Errorf("failed to validate seed: %w", err)
		}

		t.Seed = seedName
		t.Project = ""
		t.Shoot = ""

		if err := m.updateSeedKubeconfig(ctx, gardenClient, t, seed); err != nil {
			return fmt.Errorf("failed to fetch kubeconfig for seed cluster: %w", err)
		}

		return nil
	})
}

func (m *managerImpl) updateSeedKubeconfig(ctx context.Context, gardenClient client.Client, t Target, seed *gardencorev1beta1.Seed) error {
	// fetch kubeconfig secret
	secret := corev1.Secret{}
	key := types.NamespacedName{Name: seed.Spec.SecretRef.Name, Namespace: seed.Spec.SecretRef.Namespace}
	if err := gardenClient.Get(ctx, key, &secret); err != nil {
		return fmt.Errorf("failed to retrieve seed kubeconfig: %w", err)
	}

	return m.kubeconfigCache.Write(t, secret.Data["kubeconfig"])
}

func (m *managerImpl) resolveSeedName(ctx context.Context, gardenClient client.Client, seedName string) (*gardencorev1beta1.Seed, error) {
	seed := &gardencorev1beta1.Seed{}
	key := types.NamespacedName{Name: seedName}
	if err := gardenClient.Get(ctx, key, seed); err != nil {
		return nil, err
	}

	if seed.Spec.SecretRef == nil {
		return nil, errors.New("spec.SecretRef is missing in this seed, seed not reachable")
	}

	return seed, nil
}

func (m *managerImpl) TargetShoot(ctx context.Context, shootName string) error {
	return m.patchTarget(func(t *targetImpl) error {
		if t.Garden == "" {
			return ErrNoGardenTargeted
		}

		if t.Project == "" && t.Seed == "" {
			return errors.New("must target project or seed first")
		}

		gardenClient, err := m.clientForGarden(t.Garden)
		if err != nil {
			return fmt.Errorf("could not create Kubernetes client for garden cluster: %w", err)
		}

		var (
			project *gardencorev1beta1.Project
			seed    *gardencorev1beta1.Seed
		)

		if t.Project != "" {
			project, err = m.resolveProjectName(ctx, gardenClient, t.Project)
			if err != nil {
				return fmt.Errorf("failed to validate project: %w", err)
			}
		} else {
			seed, err = m.resolveSeedName(ctx, gardenClient, t.Seed)
			if err != nil {
				return fmt.Errorf("failed to fetch kubeconfig for seed cluster: %w", err)
			}
		}

		t.Shoot = shootName

		if err := m.updateShootKubeconfig(ctx, gardenClient, project, seed, t); err != nil {
			return fmt.Errorf("failed to fetch kubeconfig for shoot cluster: %w", err)
		}

		return nil
	})
}

// updateShootKubeconfig must be called with *either* project or seed, but never both
// and never neither.
func (m *managerImpl) updateShootKubeconfig(
	ctx context.Context,
	gardenClient client.Client,
	project *gardencorev1beta1.Project,
	seed *gardencorev1beta1.Seed,
	t Target,
) error {
	shoot := &gardencorev1beta1.Shoot{}

	// If a shoot is targeted via a project, we fetch the project and find
	// the shoot by listing all shoots in the project's spec.namespace.
	// If the target uses a seed, _all_ shoots in the garden are filtered
	// for shoots with matching seed and name. It's an error if no or multiple
	// matching shoots are found.

	if project != nil {
		// fetch shoot from project namespace
		// TODO: a project's spec.namespace can be nil, what to do about those cases?
		key := types.NamespacedName{Name: t.ShootName(), Namespace: *project.Spec.Namespace}

		if err := gardenClient.Get(ctx, key, shoot); err != nil {
			return fmt.Errorf("invalid shoot %q: %w", key.Name, err)
		}
	} else if seed != nil {
		// list all shoots, filter by spec.seedName
		shootList := gardencorev1beta1.ShootList{}
		if err := gardenClient.List(ctx, &shootList, &client.ListOptions{}); err != nil {
			return fmt.Errorf("failed to list shoot clusters: %w", err)
		}

		// filter found shoots; if multiple shoots have the same name, but are in different
		// projects, we have to warn the user about their ambiguous targeting
		matchingShoots := []*gardencorev1beta1.Shoot{}
		for i, s := range shootList.Items {
			if s.Name == t.ShootName() && s.Spec.SeedName != nil && *s.Spec.SeedName == seed.Name {
				matchingShoots = append(matchingShoots, &shootList.Items[i])
			}
		}

		if len(matchingShoots) == 0 {
			return fmt.Errorf("invalid shoot %q: not found on seed %q", t.ShootName(), seed.Name)
		}

		if len(matchingShoots) > 1 {
			return fmt.Errorf("there are multiple shoots named %q on this garden, please target via project to make your choice unambiguous", t.ShootName())
		}

		shoot = matchingShoots[0]
	} else {
		return errors.New("neither project nor seed were provided to filter the shoot by")
	}

	// fetch kubeconfig secret
	secret := corev1.Secret{}
	key := types.NamespacedName{
		Name:      fmt.Sprintf("%s.kubeconfig", shoot.Name),
		Namespace: shoot.Namespace,
	}
	if err := gardenClient.Get(ctx, key, &secret); err != nil {
		return fmt.Errorf("failed to retrieve shoot kubeconfig: %w", err)
	}

	return m.kubeconfigCache.Write(t, secret.Data["kubeconfig"])
}

func (m *managerImpl) GardenClient(t Target) (client.Client, error) {
	t, err := m.getTarget(t)
	if err != nil {
		return nil, err
	}

	if t.GardenName() == "" {
		return nil, ErrNoGardenTargeted
	}

	return m.clientForGarden(t.GardenName())
}

func (m *managerImpl) clientForGarden(name string) (client.Client, error) {
	for _, g := range m.config.Gardens {
		if g.Name == name {
			return m.clientProvider.FromFile(g.Kubeconfig)
		}
	}

	return nil, fmt.Errorf("targeted garden cluster %q is not configured", name)
}

func (m *managerImpl) ProjectClient(t Target) (client.Client, error) {
	t, err := m.getTarget(t)
	if err != nil {
		return nil, err
	}

	if t.GardenName() == "" {
		return nil, ErrNoGardenTargeted
	}
	if t.ProjectName() == "" {
		return nil, ErrNoProjectTargeted
	}

	// project clients point to the garden cluster, but should be pinned to
	// the project namespace; pinning is still TODO
	return m.GardenClient(t)
}

func (m *managerImpl) SeedClient(t Target) (client.Client, error) {
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

	kubeconfig, err := m.kubeconfigCache.Read(t)
	if err != nil {
		return nil, err
	}

	return m.clientProvider.FromBytes(kubeconfig)
}

func (m *managerImpl) ShootNamespaceClient(t Target) (client.Client, error) {
	return m.SeedClient(t) // TODO: pre-configure the client to use a certain namespace?
}

func (m *managerImpl) ShootClusterClient(t Target) (client.Client, error) {
	t, err := m.getTarget(t)
	if err != nil {
		return nil, err
	}

	if t.GardenName() == "" {
		return nil, ErrNoGardenTargeted
	}
	if t.SeedName() == "" && t.ProjectName() == "" {
		return nil, errors.New("neither project nor seed are targeted")
	}
	if t.ShootName() == "" {
		return nil, ErrNoShootTargeted
	}

	kubeconfig, err := m.kubeconfigCache.Read(t)
	if err != nil {
		return nil, err
	}

	return m.clientProvider.FromBytes(kubeconfig)
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
