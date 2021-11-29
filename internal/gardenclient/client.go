/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package gardenclient

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/fields"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	openstackv1alpha1 "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/v1alpha1"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	openstackinstall "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/install"
	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

var decoder runtime.Decoder

func init() {
	extensionsScheme := runtime.NewScheme()
	utilruntime.Must(openstackinstall.AddToScheme(extensionsScheme))
	decoder = serializer.NewCodecFactory(extensionsScheme).UniversalDecoder()
}

//go:generate mockgen -destination=./mocks/mock_client.go -package=mocks github.com/gardener/gardenctl-v2/internal/gardenclient Client

// Client returns a new client with functions to get Gardener and Kubernetes resources
type Client interface {
	// GetProject returns a Gardener project resource by name
	GetProject(ctx context.Context, projectName string) (*gardencorev1beta1.Project, error)
	// GetProjectByNamespace returns a Gardener project resource by namespace
	GetProjectByNamespace(ctx context.Context, namespaceName string) (*gardencorev1beta1.Project, error)
	// ListProjects returns all Gardener project resources
	ListProjects(ctx context.Context) ([]gardencorev1beta1.Project, error)

	// GetSeed returns a Gardener seed resource by name
	GetSeed(ctx context.Context, seedName string) (*gardencorev1beta1.Seed, error)
	// ListSeeds returns all Gardener seed resources
	ListSeeds(ctx context.Context) ([]gardencorev1beta1.Seed, error)

	// GetShoot returns a Gardener shoot resource in a namespace by name
	GetShoot(ctx context.Context, namespaceName string, shootName string) (*gardencorev1beta1.Shoot, error)
	// GetShootByProject returns a Gardener shoot resource in a project by name
	GetShootByProject(ctx context.Context, projectName, shootName string) (*gardencorev1beta1.Shoot, error)
	// GetShootBySeed returns a Gardener shoot resource by name
	// An optional seedName can be provided to filter by ShootSeedName field selector
	GetShootBySeed(ctx context.Context, seedName string, shootName string) (*gardencorev1beta1.Shoot, error)
	// FindShoot tries to get a shoot via project (namespace) first, if not provided it tries to find the shot
	// with an (optional) seedname
	FindShoot(ctx context.Context, opts ...client.ListOption) (*gardencorev1beta1.Shoot, error)
	// ListShoots returns all Gardener shoot resources, filtered by a list option
	ListShoots(ctx context.Context, opts ...client.ListOption) ([]gardencorev1beta1.Shoot, error)

	// GetSecretBinding returns a Gardener secretbinding resource
	GetSecretBinding(ctx context.Context, namespace, name string) (*gardencorev1beta1.SecretBinding, error)

	// GetCloudProfile returns a Gardener cloudprofile resource
	GetCloudProfile(ctx context.Context, name string) (*gardencorev1beta1.CloudProfile, error)

	// GetNamespace returns a Kubernetes namespace resource
	GetNamespace(ctx context.Context, namespaceName string) (*corev1.Namespace, error)
	// GetSecret returns a Kubernetes secret resource
	GetSecret(ctx context.Context, namespaceName string, secretName string) (*corev1.Secret, error)

	// RuntimeClient returns the underlying kubernetes runtime client
	// TODO: Remove this when we switched all APIs to the new gardenclient
	RuntimeClient() client.Client
}

type clientImpl struct {
	c client.Client
}

// NewGardenClient returns a new gardenclient
func NewGardenClient(client client.Client) Client {
	return &clientImpl{
		c: client,
	}
}

func (g *clientImpl) GetProject(ctx context.Context, projectName string) (*gardencorev1beta1.Project, error) {
	project := &gardencorev1beta1.Project{}
	key := types.NamespacedName{Name: projectName}

	if err := g.c.Get(ctx, key, project); err != nil {
		return nil, fmt.Errorf("failed to get project %v: %w", key, err)
	}

	return project, nil
}

func (g *clientImpl) GetProjectByNamespace(ctx context.Context, namespaceName string) (*gardencorev1beta1.Project, error) {
	projectList := &gardencorev1beta1.ProjectList{}

	// ctrl-runtime doesn't support FieldSelectors in fake clients
	// ( https://github.com/kubernetes-sigs/controller-runtime/issues/1376 )
	// yet, which affects the unit tests. To ensure proper filtering,
	// the shootList (and projectList later on) are filtered again.
	// In production this does not hurt much, as the FieldSelector is
	// already applied, and in tests very few objects exist anyway.
	if err := g.c.List(ctx, projectList, client.MatchingFields{gardencore.ProjectNamespace: namespaceName}); err != nil {
		return nil, fmt.Errorf("failed to fetch project by namespace: %v", err)
	}

	// if filtering by seed, ignore shoot's whose seed name doesn't match
	// (if ctrl-runntime supported FieldSelectors in tests, this if statement could go away)
	matchingProjects := []*gardencorev1beta1.Project{}

	for i, project := range projectList.Items {
		if project.Spec.Namespace != nil && *project.Spec.Namespace == namespaceName {
			matchingProjects = append(matchingProjects, &projectList.Items[i])
		}
	}

	if len(matchingProjects) == 0 {
		return nil, errors.New("failed to fetch project by namespace")
	}

	return matchingProjects[0], nil
}

func (g *clientImpl) ListProjects(ctx context.Context) ([]gardencorev1beta1.Project, error) {
	projectList := &gardencorev1beta1.ProjectList{}
	if err := g.c.List(ctx, projectList); err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}

	return projectList.Items, nil
}

func (g *clientImpl) GetSeed(ctx context.Context, seedName string) (*gardencorev1beta1.Seed, error) {
	seed := &gardencorev1beta1.Seed{}
	key := types.NamespacedName{Name: seedName}

	if err := g.c.Get(ctx, key, seed); err != nil {
		return nil, fmt.Errorf("failed to get seed %v: %w", key, err)
	}

	return seed, nil
}

func (g *clientImpl) ListSeeds(ctx context.Context) ([]gardencorev1beta1.Seed, error) {
	seedList := &gardencorev1beta1.SeedList{}
	if err := g.c.List(ctx, seedList); err != nil {
		return nil, fmt.Errorf("failed to list seeds: %w", err)
	}

	return seedList.Items, nil
}

func (g *clientImpl) GetShoot(ctx context.Context, namespaceName string, shootName string) (*gardencorev1beta1.Shoot, error) {
	shoot := &gardencorev1beta1.Shoot{}
	key := types.NamespacedName{Name: shootName, Namespace: namespaceName}

	if err := g.c.Get(ctx, key, shoot); err != nil {
		return nil, fmt.Errorf("failed to get shoot %v: %w", key, err)
	}

	return shoot, nil
}

func (g *clientImpl) GetShootBySeed(ctx context.Context, seedName string, shootName string) (*gardencorev1beta1.Shoot, error) {
	// list all shoots, filter by their name and possibly spec.seedName (if seed is set)
	shootList := gardencorev1beta1.ShootList{}
	listOpts := []client.ListOption{}

	if seedName != "" {
		// ctrl-runtime doesn't support FieldSelectors in fake clients
		// ( https://github.com/kubernetes-sigs/controller-runtime/issues/1376 )
		// yet, which affects the unit tests. To ensure proper filtering,
		// the shootList (and projectList later on) are filtered again.
		// In production this does not hurt much, as the FieldSelector is
		// already applied, and in tests very few objects exist anyway.
		listOpts = append(listOpts, client.MatchingFields{gardencore.ShootSeedName: seedName})
	}

	if err := g.c.List(ctx, &shootList, listOpts...); err != nil {
		return nil, fmt.Errorf("failed to list shoot clusters: %w", err)
	}

	// filter found shoots
	matchingShoots := []*gardencorev1beta1.Shoot{}

	for i, s := range shootList.Items {
		if s.Name != shootName {
			continue
		}

		// if filtering by seed, ignore shoot's whose seed name doesn't match
		// (if ctrl-runntime supported FieldSelectors in tests, this if statement could go away)
		if seedName != "" && (s.Spec.SeedName == nil || *s.Spec.SeedName != seedName) {
			continue
		}

		matchingShoots = append(matchingShoots, &shootList.Items[i])
	}

	if len(matchingShoots) == 0 {
		return nil, fmt.Errorf("no shoot named %q exists", shootName)
	}

	if len(matchingShoots) > 1 {
		return nil, fmt.Errorf("there are multiple shoots named %q on this garden, please target a project or seed to make your choice unambiguous", shootName)
	}

	return matchingShoots[0], nil
}

// GetShootByProject returns a Gardener shoot resource in a project by name
func (g *clientImpl) GetShootByProject(ctx context.Context, projectName, shootName string) (*gardencorev1beta1.Shoot, error) {
	project, err := g.GetProject(ctx, projectName)
	if err != nil {
		return nil, err
	}

	return g.GetShoot(ctx, *project.Spec.Namespace, shootName)
}

func (g *clientImpl) FindShoot(ctx context.Context, opts ...client.ListOption) (*gardencorev1beta1.Shoot, error) {
	shoots, err := g.ListShoots(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to list shoot clusters: %w", err)
	}

	if len(shoots) == 0 {
		return nil, fmt.Errorf("no shoot found matching the given list options %q", opts)
	}

	if len(shoots) > 1 {
		return nil, fmt.Errorf("multiple shoots found matching the given list options %q, please target a project or seed to make your choice unambiguous", opts)
	}

	return &shoots[0], nil
}

func (g *clientImpl) resolveListOptions(ctx context.Context, opts ...client.ListOption) error {
	for _, o := range opts {
		if o, ok := o.(InProject); ok {
			if err := o.resolve(ctx, g); err != nil {
				return err
			}
		}
	}

	return nil
}

func (g *clientImpl) ListShoots(ctx context.Context, opts ...client.ListOption) ([]gardencorev1beta1.Shoot, error) {
	shootList := &gardencorev1beta1.ShootList{}

	if err := g.resolveListOptions(ctx, opts...); err != nil {
		return nil, err
	}

	if err := g.c.List(ctx, shootList, opts...); err != nil {
		return nil, fmt.Errorf("failed to list shoots with list options %q: %w", opts, err)
	}

	selectors := []fields.Selector{}

	for _, opt := range opts {
		o := &client.ListOptions{}
		opt.ApplyToList(o)
		selector := o.FieldSelector

		if selector != nil && !selector.Empty() {
			selectors = append(selectors, selector)
		}
	}

	if len(selectors) == 0 {
		return shootList.Items, nil
	}

	// filter found shoots
	items := []gardencorev1beta1.Shoot{}

	for _, shoot := range shootList.Items {
		matches := true
		fields := shootFields(shoot)

		for _, selector := range selectors {
			if !selector.Matches(&fields) {
				matches = false
				break
			}
		}

		if matches {
			items = append(items, shoot)
		}
	}

	return items, nil
}

// GetSecret returns a Kubernetes namespace resource
func (g *clientImpl) GetNamespace(ctx context.Context, namespaceName string) (*corev1.Namespace, error) {
	namespace := &corev1.Namespace{}
	key := types.NamespacedName{Name: namespaceName}

	if err := g.c.Get(ctx, key, namespace); err != nil {
		return nil, fmt.Errorf("failed to get namespace %v: %w", key, err)
	}

	return namespace, nil
}

// GetSecretBinding returns a Gardener secretbinding resource
func (g *clientImpl) GetSecretBinding(ctx context.Context, namespace, name string) (*gardencorev1beta1.SecretBinding, error) {
	secretBinding := gardencorev1beta1.SecretBinding{}
	key := types.NamespacedName{Namespace: namespace, Name: name}

	if err := g.c.Get(ctx, key, &secretBinding); err != nil {
		return nil, fmt.Errorf("failed to get secretbinding %v: %w", key, err)
	}

	return &secretBinding, nil
}

// GetSecret returns a Kubernetes secret resource
func (g *clientImpl) GetSecret(ctx context.Context, namespaceName string, secretName string) (*corev1.Secret, error) {
	secret := corev1.Secret{}
	key := types.NamespacedName{Name: secretName, Namespace: namespaceName}

	if err := g.c.Get(ctx, key, &secret); err != nil {
		return nil, fmt.Errorf("failed to get secret %v: %w", key, err)
	}

	return &secret, nil
}

// GetCloudProfile returns a Gardener cloudprofile resource
func (g *clientImpl) GetCloudProfile(ctx context.Context, name string) (*gardencorev1beta1.CloudProfile, error) {
	cloudProfile := gardencorev1beta1.CloudProfile{}
	key := types.NamespacedName{Name: name}

	if err := g.c.Get(ctx, key, &cloudProfile); err != nil {
		return nil, fmt.Errorf("failed to get cloudprofile %v: %w", key, err)
	}

	return &cloudProfile, nil
}

// RuntimeClient returns the underlying Kubernetes runtime client
func (g *clientImpl) RuntimeClient() client.Client {
	return g.c
}

type CloudProfile gardencorev1beta1.CloudProfile

func (cp CloudProfile) GetOpenstackProviderConfig() (*openstackv1alpha1.CloudProfileConfig, error) {
	const apiVersion = "core.gardener.cloud/v1alpha1.CloudProfile"

	providerConfig := cp.Spec.ProviderConfig
	if providerConfig == nil {
		return nil, fmt.Errorf("providerConfig of %s %s is empty", apiVersion, cp.Name)
	}

	var cloudProfileConfig *openstackv1alpha1.CloudProfileConfig

	switch {
	case providerConfig.Object != nil:
		var ok bool
		if cloudProfileConfig, ok = providerConfig.Object.(*openstackv1alpha1.CloudProfileConfig); !ok {
			return nil, fmt.Errorf("cannot cast providerConfig of %s %s", apiVersion, cp.Name)
		}

	case providerConfig.Raw != nil:
		cloudProfileConfig = &openstackv1alpha1.CloudProfileConfig{
			TypeMeta: metav1.TypeMeta{
				APIVersion: openstackv1alpha1.SchemeGroupVersion.String(),
				Kind:       "CloudProfileConfig",
			},
		}
		if _, _, err := decoder.Decode(providerConfig.Raw, nil, cloudProfileConfig); err != nil {
			return nil, fmt.Errorf("cannot decode providerConfig of %s %s", apiVersion, cp.Name)
		}
	default:
		return nil, fmt.Errorf("providerConfig of %s %s contains neither raw data nor a decoded object", apiVersion, cp.Name)
	}

	return cloudProfileConfig, nil
}

// NewInProject creates a new InProject list option.
func NewInProject(name string) InProject {
	return &inProjectImpl{name: name}
}

// InProject restricts the list operation to the namespace of the given project.
type InProject interface {
	client.ListOption
	resolve(context.Context, Client) error
}

type inProjectImpl struct {
	name string
	o    *client.InNamespace
}

var _ InProject = &inProjectImpl{}

// ApplyToList applies this configuration to the given list options.
func (p *inProjectImpl) ApplyToList(opts *client.ListOptions) {
	if p.o != nil {
		p.o.ApplyToList(opts)
	}
}

func (p *inProjectImpl) resolve(ctx context.Context, g Client) error {
	if p.o == nil {
		project, err := g.GetProject(ctx, p.name)
		if err != nil {
			return fmt.Errorf("failed to resolve project namespace: %w", err)
		}

		if project.Spec.Namespace == nil || *project.Spec.Namespace == "" {
			return fmt.Errorf("project %q has not yet been assigned to a namespace", p.name)
		}

		inNamespace := client.InNamespace(*project.Spec.Namespace)
		p.o = &inNamespace
	}

	return nil
}

type shootFields gardencorev1beta1.Shoot

var _ fields.Fields = &shootFields{}

func (f *shootFields) Has(field string) bool {
	switch field {
	case "metadata.name":
		return true
	case "metadata.namespace":
		return true
	case gardencore.ShootSeedName:
		return f.Spec.SeedName != nil
	}

	return false
}

func (f *shootFields) Get(field string) string {
	switch field {
	case "metadata.name":
		return f.Name
	case "metadata.namespace":
		return f.Namespace
	case gardencore.ShootSeedName:
		if f.Spec.SeedName != nil {
			return *f.Spec.SeedName
		}
	}

	return ""
}
