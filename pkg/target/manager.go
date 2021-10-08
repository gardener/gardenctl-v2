/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/
package target

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/gardener/gardenctl-v2/pkg/config"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
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

// Manager sets and gets the current target configuration
type Manager interface {
	// CurrentTarget contains the current target configuration
	CurrentTarget() (Target, error)

	// TargetFlags returns the global target flags
	TargetFlags() TargetFlags

	// TargetGarden sets the garden target configuration
	// This implicitly drops project, seed and shoot target configuration
	TargetGarden(name string) error
	// TargetProject sets the project target configuration
	// This implicitly unsets seed and shoot target configuration
	TargetProject(ctx context.Context, name string) error
	// TargetSeed sets the seed target configuration
	// This implicitly unsets project and shoot target configuration
	TargetSeed(ctx context.Context, name string) error
	// TargetShoot sets the shoot target configuration
	// It will also configure appropriate project and seed values if not already set
	TargetShoot(ctx context.Context, name string) error
	// DropTargetGarden unsets the garden target configuration
	// This implicitly unsets project, shoot and seed target configuration
	DropTargetGarden() (string, error)
	// DropTargetProject unsets the project target configuration
	// This implicitly unsets seed and shoot target configuration
	DropTargetProject() (string, error)
	// DropTargetSeed unsets the garden seed configuration
	// This implicitly unsets project and shoot target configuration
	DropTargetSeed() (string, error)
	// DropTargetShoot unsets the garden shoot configuration
	DropTargetShoot() (string, error)

	// GardenClient controller-runtime client for accessing the configured garden cluster
	GardenClient(t Target) (client.Client, error)
	// SeedClient controller-runtime client for accessing the configured seed cluster
	SeedClient(ctx context.Context, t Target) (client.Client, error)
	// ShootClusterClient controller-runtime client for accessing the configured shoot cluster
	ShootClusterClient(ctx context.Context, t Target) (client.Client, error)

	// Configuration returns the current gardenctl configuration
	Configuration() *config.Config
}

type managerImpl struct {
	config          *config.Config
	targetProvider  TargetProvider
	clientProvider  ClientProvider
	kubeconfigCache KubeconfigCache
}

var _ Manager = &managerImpl{}

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

func (m *managerImpl) DropTargetGarden() (string, error) {
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

	return "", fmt.Errorf("no garden targeted")
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
		project, err := m.resolveProjectName(ctx, gardenClient, projectName)
		if err != nil {
			return fmt.Errorf("failed to resolve project: %w", err)
		}

		// validate the project
		if err := m.validateProject(ctx, project); err != nil {
			return fmt.Errorf("invalid project: %w", err)
		}

		t.Seed = ""
		t.Project = projectName
		t.Shoot = ""

		return nil
	})
}

func (m *managerImpl) DropTargetProject() (string, error) {
	currentTarget, err := m.CurrentTarget()
	if err != nil {
		return "", fmt.Errorf("failed to get current target: %v", err)
	}

	targetedName := currentTarget.ProjectName()
	if targetedName != "" {
		return targetedName, m.patchTarget(func(t *targetImpl) error {
			t.Project = ""
			t.Seed = ""
			t.Shoot = ""

			return nil
		})
	}

	return "", fmt.Errorf("no project targeted")
}

func (m *managerImpl) resolveProjectName(ctx context.Context, gardenClient client.Client, projectName string) (*gardencorev1beta1.Project, error) {
	project := &gardencorev1beta1.Project{}
	key := types.NamespacedName{Name: projectName}
	err := gardenClient.Get(ctx, key, project)

	return project, err
}

func (m *managerImpl) resolveProjectNamespace(ctx context.Context, gardenClient client.Client, projectNamespace string) (*corev1.Namespace, error) {
	namespace := &corev1.Namespace{}
	key := types.NamespacedName{Name: projectNamespace}
	err := gardenClient.Get(ctx, key, namespace)

	return namespace, err
}

func (m *managerImpl) validateProject(ctx context.Context, project *gardencorev1beta1.Project) error {
	if project.Spec.Namespace == nil || *project.Spec.Namespace == "" {
		return errors.New("project does not have a corresponding namespace set; most likely it has not yet been fully created")
	}

	return nil
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
			return fmt.Errorf("failed to resolve seed: %w", err)
		}

		// validate the seed
		if err := m.validateSeed(ctx, seed); err != nil {
			return fmt.Errorf("invalid seed: %w", err)
		}

		t.Seed = seedName
		t.Project = ""
		t.Shoot = ""

		return nil
	})
}

func (m *managerImpl) DropTargetSeed() (string, error) {
	currentTarget, err := m.CurrentTarget()
	if err != nil {
		return "", fmt.Errorf("failed to get current target: %v", err)
	}

	targetedName := currentTarget.SeedName()
	if targetedName != "" {
		return targetedName, m.patchTarget(func(t *targetImpl) error {
			t.Seed = ""
			t.Project = ""
			t.Shoot = ""

			return nil
		})
	}

	return "", fmt.Errorf("no seed targeted")
}

func (m *managerImpl) resolveSeedName(ctx context.Context, gardenClient client.Client, seedName string) (*gardencorev1beta1.Seed, error) {
	seed := &gardencorev1beta1.Seed{}
	key := types.NamespacedName{Name: seedName}

	if err := gardenClient.Get(ctx, key, seed); err != nil {
		return nil, err
	}

	return seed, nil
}

func (m *managerImpl) validateSeed(ctx context.Context, seed *gardencorev1beta1.Seed) error {
	if seed.Spec.SecretRef == nil {
		return errors.New("spec.SecretRef is missing in this seed, seed not reachable")
	}

	return nil
}

func domainsFromConfiguration(m *managerImpl) (map[string]string, map[string]string, error) {
	domainRegexp, err := regexp.Compile(`(http://|https://)?(.+\..+)`)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to compile regular expression: %v", err)
	}

	dashboardDomains := make(map[string]string)
	shootDomains := make(map[string]string)

	for _, g := range m.Configuration().Gardens {
		match := domainRegexp.FindStringSubmatch(g.DashboardDomain)
		if match != nil {
			dashboardDomains[g.Name] = match[2]
		}

		match = domainRegexp.FindStringSubmatch(g.ShootDomain)
		if match != nil {
			shootDomains[g.Name] = match[2]
		}
	}

	return dashboardDomains, shootDomains, nil
}

func matchGardenDomain(domains map[string]string, domain string) (string, string) {
	for garden, gDomain := range domains {
		if strings.Contains(domain, gDomain) {
			return garden, domain
		}
	}

	return "", ""
}

func targetFlagsFromDomain(ctx context.Context, m *managerImpl, t *targetImpl, name string) (TargetFlags, error) {
	dashboardDomains, shootDomains, err := domainsFromConfiguration(m)
	if err != nil {
		return nil, fmt.Errorf("invalid domain format in configured gardens: %w", err)
	}

	if len(dashboardDomains) == 0 && len(shootDomains) == 0 {
		return nil, fmt.Errorf("no domain found in configured gardens. Shoot targeting via domain requires shoot and / or dashboard domain configuration for each garden that you want to enable domain targeting for")
	}

	garden, domain := matchGardenDomain(dashboardDomains, name)
	if domain != "" {
		domainRegexp, err := regexp.Compile(`.+/namespace/([^/]+)/shoots/([^/]+)/?`)
		if err != nil {
			return nil, fmt.Errorf("could not extract project and shoot from url: %w", err)
		}

		match := domainRegexp.FindStringSubmatch(domain)
		if match != nil {
			gardenClient, err := m.clientForGarden(t.Garden)
			if err != nil {
				return nil, fmt.Errorf("could not create Kubernetes client for garden cluster: %w", err)
			}

			namespace, err := m.resolveProjectNamespace(ctx, gardenClient, match[1])
			if err != nil {
				return nil, fmt.Errorf("failed to validate project: %w", err)
			}

			if namespace == nil {
				return nil, fmt.Errorf("invalid namespace in shoot url")
			}

			projectName := namespace.Labels["project.gardener.cloud/name"]
			if projectName == "" {
				return nil, fmt.Errorf("namespace in url is not related to a gardener project")
			}

			return NewTargetFlags(garden, projectName, "", match[2]), nil
		}
	}

	garden, domain = matchGardenDomain(shootDomains, name)
	if domain != "" {
		domainRegexp, err := regexp.Compile(`([^.]+).([^.]+)\..+`)
		if err != nil {
			return nil, fmt.Errorf("could not extract project and shoot from url: %w", err)
		}

		match := domainRegexp.FindStringSubmatch(domain)
		if match != nil {
			return NewTargetFlags(garden, match[2], "", match[1]), nil
		}
	}

	return nil, nil
}

func isDomainFormat(name string) bool {
	// as shoots must not contain dots we can assume that the user wants to target via domain
	match, err := regexp.MatchString(`.+\..+`, name)
	return err == nil && match
}

func (m *managerImpl) TargetShoot(ctx context.Context, shootName string) error {
	return m.patchTarget(func(t *targetImpl) error {
		if isDomainFormat(shootName) {
			targetFlags, err := targetFlagsFromDomain(ctx, m, t, shootName)
			if err != nil {
				return err
			}

			if targetFlags != nil {
				t.Garden = targetFlags.GardenName()
				t.Project = targetFlags.ProjectName()
				t.Shoot = targetFlags.ShootName()
				return nil
			}

			return fmt.Errorf("failed to target shoot with url: %s", shootName)
		}

		if t.Garden == "" {
			return ErrNoGardenTargeted
		}

		gardenClient, err := m.clientForGarden(t.Garden)
		if err != nil {
			return fmt.Errorf("could not create Kubernetes client for garden cluster: %w", err)
		}

		var (
			project *gardencorev1beta1.Project
			seed    *gardencorev1beta1.Seed
		)

		// *if* project or seed are set, resolve them to aid in finding the shoot later on
		if t.Project != "" {
			project, err = m.resolveProjectName(ctx, gardenClient, t.Project)
			if err != nil {
				return fmt.Errorf("failed to validate project: %w", err)
			}
		} else if t.Seed != "" {
			seed, err = m.resolveSeedName(ctx, gardenClient, t.Seed)
			if err != nil {
				return fmt.Errorf("failed to fetch kubeconfig for seed cluster: %w", err)
			}
		}

		project, seed, shoot, err := m.resolveShootName(ctx, gardenClient, project, seed, shootName)
		if err != nil {
			return fmt.Errorf("failed to resolve shoot: %w", err)
		}

		// validate the shoot
		if err := m.validateShoot(ctx, shoot); err != nil {
			return fmt.Errorf("invalid shoot: %w", err)
		}

		t.Shoot = shootName

		// update the target path to the shoot; this is primarily important
		// when so far neither project nor seed were set. By updating the
		// target, we persist the result of the resolving step earlier and make
		// it easier to other gardenctl commands to ingest the target without
		// having to re-resolve the shoot name again.
		// resolveShootName will only ever return either a project or a seed,
		// never both. The decision what to prefer happens there as well.
		if project != nil {
			t.Project = project.Name
		}

		if seed != nil {
			t.Seed = seed.Name
		}

		return nil
	})
}

func (m *managerImpl) DropTargetShoot() (string, error) {
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

	return "", fmt.Errorf("no shoot targeted")
}

// resolveShootName takes a shoot name and tries to find the matching shoot
// on the given garden. Either project or seed can be supplied to help in
// finding the Shoot. If no or multiple Shoots match the given criteria, an
// error is returned.
// If a project or a seed are given, they are returned directly (unless an
// error is returned). If neither are given, the function will decide how
// to best find the Shoot later by returning either a project or a seed,
// never both.
func (m *managerImpl) resolveShootName(
	ctx context.Context,
	gardenClient client.Client,
	project *gardencorev1beta1.Project,
	seed *gardencorev1beta1.Seed,
	shootName string,
) (*gardencorev1beta1.Project, *gardencorev1beta1.Seed, *gardencorev1beta1.Shoot, error) {
	shoot := &gardencorev1beta1.Shoot{}

	// If a shoot is targeted via a project, we fetch it based on the project's namespace.
	// If the target uses a seed, _all_ shoots in the garden are filtered
	// for shoots with matching seed and name.
	// If neither project nor seed are given, _all_ shoots in the garden are filtered by
	// their name.
	// It's an error if no or multiple matching shoots are found.

	if project != nil {
		// fetch shoot from project namespace
		key := types.NamespacedName{Name: shootName, Namespace: *project.Spec.Namespace}

		if err := gardenClient.Get(ctx, key, shoot); err != nil {
			return nil, nil, nil, fmt.Errorf("failed to get shoot %v: %w", key, err)
		}

		return project, nil, shoot, nil
	}

	// list all shoots, filter by their name and possibly spec.seedName (if seed is set)
	shootList := gardencorev1beta1.ShootList{}
	listOpts := []client.ListOption{}

	if seed != nil {
		// ctrl-runtime doesn't support FieldSelectors in fake clients
		// ( https://github.com/kubernetes-sigs/controller-runtime/issues/1376 )
		// yet, which affects the unit tests. To ensure proper filtering,
		// the shootList (and projectList later on) are filtered again.
		// In production this does not hurt much, as the FieldSelector is
		// already applied, and in tests very few objects exist anyway.
		listOpts = append(listOpts, client.MatchingFields{gardencore.ShootSeedName: seed.Name})
	}

	if err := gardenClient.List(ctx, &shootList, listOpts...); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to list shoot clusters: %w", err)
	}

	// filter found shoots
	matchingShoots := []*gardencorev1beta1.Shoot{}

	for i, s := range shootList.Items {
		if s.Name != shootName {
			continue
		}

		// if filtering by seed, ignore shoot's whose seed name doesn't match
		// (if ctrl-runntime supported FieldSelectors in tests, this if statement could go away)
		if seed != nil && (s.Status.SeedName == nil || *s.Status.SeedName != seed.Name) {
			continue
		}

		matchingShoots = append(matchingShoots, &shootList.Items[i])
	}

	if len(matchingShoots) == 0 {
		return nil, nil, nil, fmt.Errorf("no shoot named %q exists", shootName)
	}

	if len(matchingShoots) > 1 {
		return nil, nil, nil, fmt.Errorf("there are multiple shoots named %q on this garden, please target a project or seed to make your choice unambiguous", shootName)
	}

	shoot = matchingShoots[0]

	// if the user specifically targeted via a seed, keep their choice
	if seed != nil {
		return nil, seed, shoot, nil
	}

	// given how fast we can resolve shoots by project and that shoots
	// always have a project, but not always a seed (yet), we prefer
	// for users later to use the project path in their target
	projectList := &gardencorev1beta1.ProjectList{}
	if err := gardenClient.List(ctx, projectList, client.MatchingFields{gardencore.ProjectNamespace: shoot.Namespace}); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to fetch parent project for shoot: %v", err)
	}

	// see note above on why we have to filter again because ctrl-runtime doesn't support FieldSelectors in tests
	projectList.Items = filterProjectsByNamespace(projectList.Items, shoot.Namespace)

	if len(projectList.Items) == 0 {
		// this should never happen, but to aid in inspecting broken
		// installations, try to find the seed instead as a fallback
		if shoot.Status.SeedName != nil && *shoot.Status.SeedName != "" {
			var err error

			seed, err = m.resolveSeedName(ctx, gardenClient, *shoot.Status.SeedName)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("failed to fetch project or seed for shoot: %v", err)
			}
		}
	} else {
		project = &projectList.Items[0]
	}

	// only project or seed will be non-nil at this point

	return project, seed, shoot, nil
}

func filterProjectsByNamespace(items []gardencorev1beta1.Project, namespace string) []gardencorev1beta1.Project {
	result := []gardencorev1beta1.Project{}

	for i, project := range items {
		if project.Spec.Namespace != nil && *project.Spec.Namespace == namespace {
			result = append(result, items[i])
		}
	}

	return result
}

func (m *managerImpl) validateShoot(ctx context.Context, seed *gardencorev1beta1.Shoot) error {
	return nil
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

	gardenClient, err := m.GardenClient(t)
	if err != nil {
		return nil, fmt.Errorf("failed to create garden cluster client: %w", err)
	}

	seed, err := m.resolveSeedName(ctx, gardenClient, t.SeedName())
	if err != nil {
		return nil, fmt.Errorf("invalid seed cluster: %w", err)
	}

	secret := corev1.Secret{}
	key := types.NamespacedName{Name: seed.Spec.SecretRef.Name, Namespace: seed.Spec.SecretRef.Namespace}

	if err := gardenClient.Get(ctx, key, &secret); err != nil {
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

	gardenClient, err := m.GardenClient(t)
	if err != nil {
		return nil, fmt.Errorf("failed to create garden cluster client: %w", err)
	}

	var (
		project *gardencorev1beta1.Project
		seed    *gardencorev1beta1.Seed
	)

	// *if* project or seed are set, resolve them to aid in finding the shoot later on
	if t.ProjectName() != "" {
		project, err = m.resolveProjectName(ctx, gardenClient, t.ProjectName())
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve project: %w", err)
		}
	} else if t.SeedName() != "" {
		seed, err = m.resolveSeedName(ctx, gardenClient, t.SeedName())
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve seed: %w", err)
		}
	}

	_, _, shoot, err := m.resolveShootName(ctx, gardenClient, project, seed, t.ShootName())
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve shoot: %w", err)
	}

	secret := corev1.Secret{}
	key := types.NamespacedName{
		Name:      fmt.Sprintf("%s.kubeconfig", shoot.Name),
		Namespace: shoot.Namespace,
	}

	if err := gardenClient.Get(ctx, key, &secret); err != nil {
		return nil, fmt.Errorf("failed to retrieve shoot kubeconfig: %w", err)
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
