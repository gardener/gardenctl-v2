/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package target

import (
	"context"
	"crypto/md5" // #nosec G501 -- No cryptographic context.
	"errors"
	"fmt"
	"os"
	"path/filepath"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	internalclient "github.com/gardener/gardenctl-v2/internal/client"
	clientgarden "github.com/gardener/gardenctl-v2/internal/client/garden"
	"github.com/gardener/gardenctl-v2/pkg/ac"
	"github.com/gardener/gardenctl-v2/pkg/config"
)

var (
	ErrNoGardenTargeted              = errors.New("no garden cluster targeted")
	ErrNoProjectTargeted             = errors.New("no project targeted")
	ErrNoSeedTargeted                = errors.New("no seed cluster targeted")
	ErrNoShootTargeted               = errors.New("no shoot targeted")
	ErrNeitherProjectNorSeedTargeted = errors.New("neither project nor seed are targeted")
	ErrNoControlPlaneTargeted        = errors.New("no control plane targeted")
	ErrAborted                       = errors.New("operation aborted")
)

// FlagCompletors provides a set of functions that lists suitable completion values for
// targeting while taking the currently targeted garden account.
type FlagCompletors interface {
	// ShootNames returns all shoots for the current target.
	// The current target must at least point to a garden.
	ShootNames(ctx context.Context) ([]string, error)
	// SeedNames returns all seeds for the current target.
	// The current target must at least point to a garden.
	SeedNames(ctx context.Context) ([]string, error)
	// ProjectNames returns all projects for the currently targeted garden.
	// The current target must at least point to a garden.
	ProjectNames(ctx context.Context) ([]string, error)
	// GardenNames returns all identities and aliases of configured Gardens.
	GardenNames() ([]string, error)
}

//go:generate mockgen -destination=./mocks/mock_manager.go -package=mocks github.com/gardener/gardenctl-v2/pkg/target Manager

// Manager sets and gets the current target configuration.
type Manager interface {
	FlagCompletors

	// CurrentTarget contains the current target configuration
	CurrentTarget() (Target, error)

	// TargetGarden sets the garden target configuration
	// This implicitly unsets project, seed and shoot target configuration
	TargetGarden(ctx context.Context, name string) (Target, error)
	// TargetProject sets the project target configuration
	// This implicitly unsets seed and shoot target configuration
	TargetProject(ctx context.Context, name string) (Target, error)
	// TargetSeed sets the seed target configuration
	// This implicitly unsets project and shoot target configuration
	TargetSeed(ctx context.Context, name string) (Target, error)
	// TargetShoot sets the shoot target configuration
	// This implicitly unsets seed target configuration
	// It will also configure appropriate project and seed values if not already set
	TargetShoot(ctx context.Context, name string) (Target, error)
	// TargetControlPlane sets the control plane target flag
	TargetControlPlane(ctx context.Context) (Target, error)
	// UnsetTargetGarden unsets the garden target configuration
	// This implicitly unsets project, shoot and seed target configuration
	UnsetTargetGarden(ctx context.Context) (string, Target, error)
	// UnsetTargetProject unsets the project target configuration
	// This implicitly unsets shoot target configuration
	UnsetTargetProject(ctx context.Context) (string, Target, error)
	// UnsetTargetSeed unsets the garden seed configuration
	UnsetTargetSeed(ctx context.Context) (string, Target, error)
	// UnsetTargetShoot unsets the garden shoot configuration
	UnsetTargetShoot(ctx context.Context) (string, Target, error)
	// UnsetTargetControlPlane unsets the control plane flag
	UnsetTargetControlPlane(ctx context.Context) (Target, error)
	// TargetMatchPattern replaces the whole target
	// Garden, Project and Shoot values are determined by matching the provided value
	// against patterns defined in gardenctl configuration. Some values may only match a subset
	// of a pattern
	TargetMatchPattern(ctx context.Context, tf TargetFlags, value string) (Target, error)

	// ClientConfig returns the client config for a target.
	// The kubeconfig access level (admin/viewer/auto) is resolved internally from the
	// global --access-level flag and the per-garden config default.
	ClientConfig(ctx context.Context, t Target) (clientcmd.ClientConfig, error)
	// EffectiveAccessLevel returns the kubeconfig access level that gardenctl
	// would use for the given target. The boolean is false when gardenctl did
	// not decide a level (the caller should defer to gardenlogin's default) or
	// the target does not produce a gardenlogin kubeconfig at all.
	//
	// For shoot targets, scope resolution consults the garden cluster to detect
	// whether the shoot backs a managed seed, so the same physical cluster
	// produces the same access level regardless of whether it was reached via
	// `target shoot` or `target seed`.
	EffectiveAccessLevel(ctx context.Context, t Target) (config.KubeconfigAccessLevel, bool, error)
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
	GardenClient(name string) (clientgarden.Client, error)
}

type managerImpl struct {
	config           *config.Config
	targetProvider   TargetProvider
	clientProvider   internalclient.Provider
	sessionDirectory string
	// flagAccessLevel is the value of the global --access-level flag.
	// Empty when unset. Takes precedence over per-garden config defaults.
	flagAccessLevel config.KubeconfigAccessLevel

	currentTarget Target
}

var _ Manager = &managerImpl{}

func newGardenClient(name string, config *config.Config, provider internalclient.Provider) (clientgarden.Client, error) {
	clientConfig, err := config.ClientConfig(name)
	if err != nil {
		return nil, err
	}

	client, err := provider.FromClientConfig(clientConfig)
	if err != nil {
		return nil, err
	}

	garden, err := config.Garden(name)
	if err != nil {
		return nil, err
	}

	return clientgarden.NewClient(clientConfig, client, garden.Name), nil
}

// NewManager returns a new manager. flagAccessLevel is the value of the global
// --access-level flag (empty when unset); the manager combines it with
// per-garden config defaults when resolving the effective level for kubeconfig requests.
func NewManager(config *config.Config, targetProvider TargetProvider, clientProvider internalclient.Provider, sessionDirectory string, flagAccessLevel config.KubeconfigAccessLevel) (Manager, error) {
	return &managerImpl{
		config:           config,
		targetProvider:   targetProvider,
		clientProvider:   clientProvider,
		sessionDirectory: sessionDirectory,
		flagAccessLevel:  flagAccessLevel,
	}, nil
}

// AccessScope identifies which per-scope default applies when resolving the
// kubeconfig access level for a target.
type AccessScope string

const (
	// AccessScopeShoots is the scope for shoot targets.
	AccessScopeShoots AccessScope = "shoots"
	// AccessScopeSeeds is the scope for any seed kubeconfig request: a seed
	// target, a shoot's control plane (which runs on a seed), and a shoot that
	// backs a managed seed (because it physically is the seed cluster).
	AccessScopeSeeds AccessScope = "seeds"
)

// EffectiveAccessLevel returns the access level gardenctl will request for a
// target. The bool is false when gardenctl has no opinion (caller defers to
// gardenlogin's default) or when displaying it would mislead - non-managed
// seeds fall through to a static .login kubeconfig of unverifiable privileges.
func (m *managerImpl) EffectiveAccessLevel(ctx context.Context, t Target) (config.KubeconfigAccessLevel, bool, error) {
	scope, isShootRequest, err := m.scopeForTarget(ctx, t)
	if err != nil || scope == "" {
		return "", false, err
	}

	if !isShootRequest {
		seedName := t.SeedName()
		if seedName == "" && t.ShootName() == "" {
			// A control-plane or seed scope without a known seed name and
			// without a shoot to recover it from has nothing to look up;
			// ClientConfig will surface the underlying problem,
			// EffectiveAccessLevel just stays silent.
			return "", false, nil
		}

		c, err := m.GardenClient(t.GardenName())
		if err != nil {
			return "", false, err
		}

		if seedName == "" {
			// Recover spec.seedName for control-plane targets that do not
			// carry it, matching the ClientConfig lookup path.
			shoot, err := c.FindShoot(ctx, t.WithControlPlane(false).AsListOption())
			if err != nil {
				return "", false, err
			}

			if shoot.Spec.SeedName == nil {
				return "", false, nil
			}

			seedName = *shoot.Spec.SeedName
		}

		isManaged, err := c.IsManagedSeedByName(ctx, seedName)
		if err != nil {
			return "", false, err
		}

		if !isManaged {
			return "", false, nil
		}
	}

	level := m.resolveAccessLevel(t, scope)

	return level, level != "", nil
}

// scopeForTarget returns the access scope plus whether the kubeconfig will be
// served via the shoot path (gardenlogin) rather than GetSeedClientConfig.
// The bool lets EffectiveAccessLevel skip the managed-seed check on the shoot
// path without re-deriving which branch fired.
//
// Order matters: ControlPlane and ShootName must be checked before SeedName,
// since TargetShoot records the seed too (from spec.seedName).
func (m *managerImpl) scopeForTarget(ctx context.Context, t Target) (AccessScope, bool, error) {
	switch {
	case t.ControlPlane():
		return AccessScopeSeeds, false, nil
	case t.ShootName() != "":
		c, err := m.GardenClient(t.GardenName())
		if err != nil {
			return "", false, err
		}

		shootKey, err := resolveShootKey(ctx, c, t)
		if err != nil {
			return "", false, err
		}

		scope, err := scopeForShoot(ctx, c, shootKey.Namespace, shootKey.Name)

		return scope, true, err
	case t.SeedName() != "":
		return AccessScopeSeeds, false, nil
	}

	return "", false, nil
}

// scopeForShoot returns seeds when the shoot also backs a managed seed (same
// physical cluster as `target seed` would reach), shoots otherwise.
func scopeForShoot(ctx context.Context, c clientgarden.Client, namespace, shootName string) (AccessScope, error) {
	isManagedSeed, err := c.IsManagedSeed(ctx, namespace, shootName)
	if err != nil {
		return "", err
	}

	if isManagedSeed {
		return AccessScopeSeeds, nil
	}

	return AccessScopeShoots, nil
}

// resolveShootKey returns the namespace/name of the effective shoot identified
// by t, either from the resolved target's project namespace or by finding the
// shoot cluster-wide when no project is targeted.
func resolveShootKey(ctx context.Context, c clientgarden.Client, t Target) (types.NamespacedName, error) {
	resolvedTarget, err := NewResolver(c).ResolveShootTarget(ctx, t)
	if err != nil {
		return types.NamespacedName{}, err
	}

	if resolvedTarget.ProjectName() != "" {
		namespace, err := getProjectNamespace(ctx, c, resolvedTarget.ProjectName())
		if err != nil {
			return types.NamespacedName{}, err
		}

		return types.NamespacedName{Namespace: *namespace, Name: resolvedTarget.ShootName()}, nil
	}

	shoot, err := c.FindShoot(ctx, resolvedTarget.AsListOption())
	if err != nil {
		return types.NamespacedName{}, err
	}

	return types.NamespacedName{Namespace: shoot.Namespace, Name: shoot.Name}, nil
}

// resolveAccessLevel returns the effective kubeconfig access level for a target+scope.
// Precedence: CLI flag > per-garden default for the requested scope > empty.
//
// Returning empty when neither a flag nor a config default is set lets the
// shoot kubeconfig generator omit the --access-level argument entirely, so
// gardenlogin falls back to its own documented default ("auto") rather than
// gardenctl silently overriding it.
func (m *managerImpl) resolveAccessLevel(t Target, scope AccessScope) config.KubeconfigAccessLevel {
	if m.flagAccessLevel != "" {
		return m.flagAccessLevel
	}

	if t.GardenName() != "" {
		if garden, err := m.config.Garden(t.GardenName()); err == nil && garden.KubeconfigAccessLevelDefaults != nil {
			switch scope {
			case AccessScopeShoots:
				return garden.KubeconfigAccessLevelDefaults.Shoots
			case AccessScopeSeeds:
				return garden.KubeconfigAccessLevelDefaults.Seeds
			}
		}
	}

	return ""
}

func (m *managerImpl) CurrentTarget() (Target, error) {
	if m.currentTarget != nil {
		return m.currentTarget.DeepCopy(), nil
	}

	currentTarget, err := m.targetProvider.Read()
	if err != nil {
		return nil, err
	}

	if currentTarget == nil {
		return nil, errors.New("target provider returned nil target")
	}

	m.currentTarget = currentTarget.DeepCopy()

	return m.currentTarget.DeepCopy(), nil
}

func (m *managerImpl) Configuration() *config.Config {
	return m.config
}

func (m *managerImpl) TargetGarden(ctx context.Context, identity string) (Target, error) {
	if identity == "" {
		return nil, errors.New("garden identity must not be empty")
	}

	// Resolve to the canonical garden name (so an alias becomes its underlying
	// name) before handing off to the overlay path. merge() does not perform
	// this lookup.
	garden, err := m.config.Garden(identity)
	if err != nil {
		return nil, fmt.Errorf("failed to set target garden: %w", err)
	}

	current, err := m.CurrentTarget()
	if err != nil {
		return nil, fmt.Errorf("failed to get current target: %w", err)
	}

	overlay := &targetFlagsImpl{gardenName: garden.Name}

	target, err := m.applyOverlay(ctx, current, overlay)
	if err != nil {
		return nil, err
	}

	return m.updateTarget(ctx, target)
}

func (m *managerImpl) UnsetTargetGarden(ctx context.Context) (string, Target, error) {
	currentTarget, err := m.CurrentTarget()
	if err != nil {
		return "", nil, fmt.Errorf("failed to get current target: %w", err)
	}

	targetedGarden := currentTarget.GardenName()
	if targetedGarden == "" {
		return "", currentTarget, ErrNoGardenTargeted
	}

	target, err := m.patchTarget(ctx, func(t *targetImpl) error {
		t.Garden = ""
		t.Project = ""
		t.Seed = ""
		t.Shoot = ""
		t.ControlPlaneFlag = false

		return nil
	})

	return targetedGarden, target, err
}

func (m *managerImpl) TargetProject(ctx context.Context, projectName string) (Target, error) {
	if projectName == "" {
		return nil, errors.New("project name must not be empty")
	}

	current, err := m.CurrentTarget()
	if err != nil {
		return nil, fmt.Errorf("failed to get current target: %w", err)
	}

	overlay := &targetFlagsImpl{projectName: projectName}

	target, err := m.applyOverlay(ctx, current, overlay)
	if err != nil {
		return nil, err
	}

	return m.updateTarget(ctx, target)
}

func (m *managerImpl) UnsetTargetProject(ctx context.Context) (string, Target, error) {
	currentTarget, err := m.CurrentTarget()
	if err != nil {
		return "", nil, fmt.Errorf("failed to get current target: %w", err)
	}

	targetedName := currentTarget.ProjectName()
	if targetedName == "" {
		return "", currentTarget, ErrNoProjectTargeted
	}

	target, err := m.patchTarget(ctx, func(t *targetImpl) error {
		t.Project = ""
		t.Seed = ""
		t.Shoot = ""
		t.ControlPlaneFlag = false

		return nil
	})

	return targetedName, target, err
}

func (m *managerImpl) TargetSeed(ctx context.Context, seedName string) (Target, error) {
	if seedName == "" {
		return nil, errors.New("seed name must not be empty")
	}

	current, err := m.CurrentTarget()
	if err != nil {
		return nil, fmt.Errorf("failed to get current target: %w", err)
	}

	overlay := &targetFlagsImpl{seedName: seedName}

	target, err := m.applyOverlay(ctx, current, overlay)
	if err != nil {
		return nil, err
	}

	return m.updateTarget(ctx, target)
}

func (m *managerImpl) UnsetTargetSeed(ctx context.Context) (string, Target, error) {
	currentTarget, err := m.CurrentTarget()
	if err != nil {
		return "", nil, fmt.Errorf("failed to get current target: %w", err)
	}

	targetedName := currentTarget.SeedName()
	if targetedName == "" {
		return "", currentTarget, ErrNoSeedTargeted
	}

	target, err := m.patchTarget(ctx, func(t *targetImpl) error {
		t.Seed = ""

		return nil
	})

	return targetedName, target, err
}

func (m *managerImpl) TargetShoot(ctx context.Context, shootName string) (Target, error) {
	if shootName == "" {
		return nil, errors.New("shoot name must not be empty")
	}

	current, err := m.CurrentTarget()
	if err != nil {
		return nil, fmt.Errorf("failed to get current target: %w", err)
	}

	overlay := &targetFlagsImpl{shootName: shootName}

	target, err := m.applyOverlay(ctx, current, overlay)
	if err != nil {
		return nil, err
	}

	return m.updateTarget(ctx, target)
}

func (m *managerImpl) UnsetTargetShoot(ctx context.Context) (string, Target, error) {
	currentTarget, err := m.CurrentTarget()
	if err != nil {
		return "", nil, fmt.Errorf("failed to get current target: %w", err)
	}

	targetedName := currentTarget.ShootName()
	if targetedName == "" {
		return "", currentTarget, ErrNoShootTargeted
	}

	target, err := m.patchTarget(ctx, func(t *targetImpl) error {
		t.Shoot = ""
		t.Seed = ""
		t.ControlPlaneFlag = false

		return nil
	})

	return targetedName, target, err
}

func (m *managerImpl) TargetControlPlane(ctx context.Context) (Target, error) {
	current, err := m.CurrentTarget()
	if err != nil {
		return nil, fmt.Errorf("failed to get current target: %w", err)
	}

	overlay := &targetFlagsImpl{controlPlane: newProvidedBoolFlag(true)}

	target, err := m.applyOverlay(ctx, current, overlay)
	if err != nil {
		return nil, err
	}

	return m.updateTarget(ctx, target)
}

func (m *managerImpl) UnsetTargetControlPlane(ctx context.Context) (Target, error) {
	currentTarget, err := m.CurrentTarget()
	if err != nil {
		return nil, fmt.Errorf("failed to get current target: %w", err)
	}

	// A control plane only exists in the context of a shoot, so unsetting it
	// without a targeted shoot is meaningless. Reject early with a clear error
	// rather than silently patching nothing.
	if currentTarget.ShootName() == "" {
		return currentTarget, ErrNoShootTargeted
	}

	return m.patchTarget(ctx, func(t *targetImpl) error {
		t.ControlPlaneFlag = false

		return nil
	})
}

func (m *managerImpl) TargetMatchPattern(ctx context.Context, tf TargetFlags, value string) (Target, error) {
	persistedTarget, err := m.persistedTarget()
	if err != nil {
		return nil, fmt.Errorf("failed to get current target: %w", err)
	}

	overlay, err := m.resolvePatternOverlay(ctx, tf, persistedTarget, value)
	if err != nil {
		return nil, err
	}

	target, err := m.applyOverlay(ctx, persistedTarget, overlay)
	if err != nil {
		return nil, err
	}

	return m.updateTarget(ctx, target)
}

func (m *managerImpl) persistedTarget() (Target, error) {
	persistedTarget, err := m.targetProvider.ReadPersisted()
	if err != nil {
		return nil, err
	}

	if persistedTarget == nil {
		return nil, errors.New("target provider returned nil target")
	}

	return persistedTarget.DeepCopy(), nil
}

// resolvePatternOverlay matches value against the pattern set for the
// preferred garden, then unions the resulting pattern fields with the CLI
// flags into a single overlay. CLI flags fill fields the pattern leaves
// unset; agreement is fine; disagreement on the same field is rejected so
// the user sees their self-contradiction instead of a silent winner.
func (m *managerImpl) resolvePatternOverlay(ctx context.Context, tf TargetFlags, persistedTarget Target, value string) (TargetFlags, error) {
	gardenName := tf.GardenName()
	if gardenName == "" {
		gardenName = persistedTarget.GardenName()
	}

	tm, err := m.config.MatchPattern(gardenName, value)
	if err != nil {
		return nil, fmt.Errorf("error occurred while trying to match value: %w", err)
	}

	patternFlags, err := m.patternToFlags(ctx, tm)
	if err != nil {
		return nil, err
	}

	return combine(tf, patternFlags)
}

func (m *managerImpl) patternToFlags(ctx context.Context, tm *config.PatternMatch) (TargetFlags, error) {
	if tm.Project != "" && tm.Namespace != "" {
		return nil, fmt.Errorf("project %q and Namespace %q set in target match value. It is forbidden to have both values set", tm.Project, tm.Namespace)
	}

	projectName := tm.Project
	if tm.Namespace != "" {
		var err error

		projectName, err = getProjectNameByNamespace(ctx, tm.Garden, tm.Namespace, m.config, m.clientProvider)
		if err != nil {
			return nil, fmt.Errorf("failed to set target project: %w", err)
		}
	}

	return &targetFlagsImpl{
		gardenName:   tm.Garden,
		projectName:  projectName,
		shootName:    tm.Shoot,
		controlPlane: NewBoolFlag(false),
	}, nil
}

func getProjectNameByNamespace(ctx context.Context, gardenName string, name string, cfg *config.Config, clientProvider internalclient.Provider) (string, error) {
	gardenClient, err := newGardenClient(gardenName, cfg, clientProvider)
	if err != nil {
		return "", err
	}

	namespace, err := gardenClient.GetNamespace(ctx, name)
	if err != nil {
		return "", fmt.Errorf("failed to fetch namespace: %w", err)
	}

	projectName := namespace.Labels["project.gardener.cloud/name"]
	if projectName == "" {
		return "", fmt.Errorf("namespace %q is not related to a gardener project", name)
	}

	return projectName, nil
}

// applyOverlay produces the final persisted Target by overlaying tf onto
// persisted. Single source of truth for set-target behavior: shape rules, API
// existence checks, project/seed enrichment, and access-restriction prompts.
// Pattern path and direct Target* path both route through this. Unset* methods
// use patchTarget instead because they mutate the current target by clearing
// selected levels and preserving the remaining prefix.
//
// Important caller-side invariant: the direct Target* methods pass
// CurrentTarget() (which the dynamic provider has already overlaid with global
// CLI flags), while TargetMatchPattern passes PersistedTarget() (raw) because
// resolvePatternOverlay re-incorporates CLI flags via combine(); using
// CurrentTarget() there would double-apply them. Don't "simplify" this.
func (m *managerImpl) applyOverlay(ctx context.Context, persisted Target, tf TargetFlags) (Target, error) {
	// 1. Pure shape pass: clearing of deeper levels, control-plane reset, and
	// shape validation (DNS-name regex etc. via targetImpl.Validate()).
	merged, err := merge(persisted, tf)
	if err != nil {
		return nil, err
	}

	merged, err = m.validateAndEnrich(ctx, merged)
	if err != nil {
		return nil, err
	}

	if err := m.gateAccessRestrictions(ctx, merged); err != nil {
		return nil, err
	}

	return merged, nil
}

// validateAndEnrich runs API existence checks unconditionally on whichever
// fields are non-empty in t, mirroring the always-validate semantics of the
// builder's Set* actions. When a shoot is set, validateShoot is the only API
// call needed for project+seed scope (completeTargetFromShoot fills and
// canonicalizes values from the shoot object); otherwise project and/or seed
// are checked individually.
func (m *managerImpl) validateAndEnrich(ctx context.Context, t Target) (Target, error) {
	impl, ok := t.(*targetImpl)
	if !ok {
		return nil, errors.New("target must be using targetImpl as its underlying type")
	}

	if impl.Garden == "" {
		// No garden means no API client to talk to. merge()'s Validate already
		// rejects deeper levels without a garden, so by the time we get here a
		// garden-less target is also field-less — nothing to validate.
		return impl, nil
	}

	switch {
	case impl.Shoot != "":
		shoot, err := m.validateShoot(ctx, impl, impl.Shoot)
		if err != nil {
			return nil, err
		}

		if err := m.completeTargetFromShoot(ctx, impl, shoot); err != nil {
			return nil, err
		}
	case impl.Project != "" && impl.Seed != "":
		// Both set without a shoot is rejected by merge(); defensive only.
		if _, err := m.validateProject(ctx, impl.Garden, impl.Project); err != nil {
			return nil, fmt.Errorf("failed to set target project: %w", err)
		}

		if _, err := m.validateSeed(ctx, impl.Garden, impl.Seed); err != nil {
			return nil, fmt.Errorf("failed to set target seed: %w", err)
		}
	case impl.Project != "":
		if _, err := m.validateProject(ctx, impl.Garden, impl.Project); err != nil {
			return nil, fmt.Errorf("failed to set target project: %w", err)
		}
	case impl.Seed != "":
		if _, err := m.validateSeed(ctx, impl.Garden, impl.Seed); err != nil {
			return nil, fmt.Errorf("failed to set target seed: %w", err)
		}
	}

	return impl, nil
}

// validateShoot confirms the named shoot exists in t's project/seed scope and
// returns the API object. No writes to t.
func (m *managerImpl) validateShoot(ctx context.Context, t *targetImpl, name string) (*gardencorev1beta1.Shoot, error) {
	gardenClient, err := m.getGardenClient(t.GardenName())
	if err != nil {
		return nil, err
	}

	shoot, err := gardenClient.FindShoot(ctx, t.WithShootName(name).AsListOption())
	if err != nil {
		return nil, fmt.Errorf("failed to fetch shoot: %w", err)
	}

	if t.Seed != "" && shoot.Spec.SeedName != nil && t.Seed != *shoot.Spec.SeedName {
		return nil, fmt.Errorf("the specified seed %q does not match the actual seed %q of shoot %q", t.Seed, *shoot.Spec.SeedName, name)
	}

	return shoot, nil
}

// completeTargetFromShoot fills missing project, syncs seed, and canonicalizes
// the shoot name from API truth. Assumes validateShoot has already gated scope
// agreement; do not call this without that gate.
func (m *managerImpl) completeTargetFromShoot(ctx context.Context, t *targetImpl, shoot *gardencorev1beta1.Shoot) error {
	if t.Project == "" {
		gardenClient, err := m.getGardenClient(t.GardenName())
		if err != nil {
			return err
		}

		project, err := gardenClient.GetProjectByNamespace(ctx, shoot.Namespace)
		if err != nil {
			return fmt.Errorf("failed to fetch parent project for shoot: %w", err)
		}

		t.Project = project.Name
	}

	if shoot.Spec.SeedName != nil {
		t.Seed = *shoot.Spec.SeedName
	} else {
		t.Seed = ""
	}

	t.Shoot = shoot.Name

	return nil
}

// gateAccessRestrictions runs the access-restriction prompts for a finalized
// target. Two distinct objects can carry restrictions:
//
//   - the selected shoot (when t.ShootName() is set), and
//   - the backing shoot of the managed seed that hosts the control plane
//     (when t.ControlPlane() is set; this is a different shoot from the one
//     above and must be resolved via NewResolver.ResolveShoot).
//
// Both prompts fire when both scopes are present (e.g. `target shoot foo
// --control-plane`); each runs against its own object. The gate refetches
// shoots from the API rather than threading them out of validation — it
// keeps validateAndEnrich's signature clean and the extra LIST is negligible
// for an interactive CLI.
func (m *managerImpl) gateAccessRestrictions(ctx context.Context, t Target) error {
	handler := ac.AccessRestrictionHandlerFromContext(ctx)
	if handler == nil {
		return nil
	}

	if t.GardenName() == "" {
		return nil
	}

	garden, err := m.config.Garden(t.GardenName())
	if err != nil {
		return err
	}

	if len(garden.AccessRestrictions) == 0 {
		// Nothing configured on this garden — no shoot needs to be fetched.
		return nil
	}

	gardenClient, err := m.getGardenClient(t.GardenName())
	if err != nil {
		return err
	}

	resolver := NewResolver(gardenClient)

	if t.ShootName() != "" {
		shoot, err := resolver.FindShoot(ctx, t.WithControlPlane(false))
		if err != nil {
			return err
		}

		if !handler(ac.CheckAccessRestrictions(garden.AccessRestrictions, shoot)) {
			return ErrAborted
		}
	}

	if t.ControlPlane() {
		backingShoot, err := resolver.ResolveShoot(ctx, t)
		if err != nil {
			// Non-managed seeds have no backing shoot to attach access
			// restrictions to
			if apierrors.IsNotFound(err) {
				return nil
			}

			return err
		}

		if !handler(ac.CheckAccessRestrictions(garden.AccessRestrictions, backingShoot)) {
			return ErrAborted
		}
	}

	return nil
}

// validateProject ensures that the project exists and that a corresponding
// namespace is set, otherwise an error is returned.
func (m *managerImpl) validateProject(ctx context.Context, gardenName string, name string) (*gardencorev1beta1.Project, error) {
	gardenClient, err := m.getGardenClient(gardenName)
	if err != nil {
		return nil, err
	}

	project, err := gardenClient.GetProject(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch project: %w", err)
	}

	if project.Spec.Namespace == nil || *project.Spec.Namespace == "" {
		return nil, errors.New("project does not have a corresponding namespace set; most likely it has not yet been fully created")
	}

	return project, nil
}

// validateSeed ensures that the seed exists, otherwise an error is returned.
func (m *managerImpl) validateSeed(ctx context.Context, gardenName string, name string) (*gardencorev1beta1.Seed, error) {
	gardenClient, err := m.getGardenClient(gardenName)
	if err != nil {
		return nil, err
	}

	seed, err := gardenClient.GetSeed(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve seed: %w", err)
	}

	return seed, nil
}

func (m *managerImpl) getGardenClient(gardenName string) (clientgarden.Client, error) {
	return newGardenClient(gardenName, m.config, m.clientProvider)
}

func (m *managerImpl) updateTarget(ctx context.Context, target Target) (Target, error) {
	if target == nil {
		return nil, errors.New("target must not be nil")
	}

	targetToWrite := target.DeepCopy()

	if err := m.targetProvider.Write(targetToWrite); err != nil {
		return nil, err
	}

	m.currentTarget = targetToWrite.DeepCopy()

	if !m.config.SymlinkTargetKubeconfig() {
		return m.currentTarget.DeepCopy(), nil
	}

	if err := m.updateClientConfigSymlink(ctx, targetToWrite); err != nil {
		return nil, err
	}

	return m.currentTarget.DeepCopy(), nil
}

func (m *managerImpl) ClientConfig(ctx context.Context, t Target) (clientcmd.ClientConfig, error) {
	if t.ControlPlane() {
		scope, _, err := m.scopeForTarget(ctx, t)
		if err != nil {
			return nil, err
		}

		// GetSeedClientConfig handles the managed/non-managed split: managed
		// seeds honor accessLevel; non-managed seeds return the static
		// kubeconfig (and reject an explicit "viewer" request).
		accessLevel := m.resolveAccessLevel(t, scope)

		return m.getClientConfig(t, func(client clientgarden.Client) (clientcmd.ClientConfig, error) {
			shoot, err := client.FindShoot(ctx, t.WithControlPlane(false).AsListOption())
			if err != nil {
				return nil, err
			}

			if shoot.Spec.SeedName == nil || *shoot.Spec.SeedName == "" {
				return nil, fmt.Errorf("no seed assigned to shoot %s/%s", shoot.Namespace, shoot.Name)
			}

			if shoot.Status.TechnicalID == "" {
				return nil, fmt.Errorf("no technicalID has been assigned to the shoot %q yet", t.ShootName())
			}

			clientConfig, err := client.GetSeedClientConfig(ctx, *shoot.Spec.SeedName, accessLevel)
			if err != nil {
				return nil, err
			}

			return clientConfigWithNamespace(clientConfig, shoot.Status.TechnicalID)
		})
	}

	if t.ShootName() != "" {
		return m.getClientConfig(t, func(client clientgarden.Client) (clientcmd.ClientConfig, error) {
			shootKey, err := resolveShootKey(ctx, client, t)
			if err != nil {
				return nil, err
			}

			scope, err := scopeForShoot(ctx, client, shootKey.Namespace, shootKey.Name)
			if err != nil {
				return nil, err
			}

			accessLevel := m.resolveAccessLevel(t, scope)

			return client.GetShootClientConfig(ctx, shootKey.Namespace, shootKey.Name, accessLevel)
		})
	}

	if t.SeedName() != "" {
		scope, _, err := m.scopeForTarget(ctx, t)
		if err != nil {
			return nil, err
		}

		accessLevel := m.resolveAccessLevel(t, scope)

		return m.getClientConfig(t, func(client clientgarden.Client) (clientcmd.ClientConfig, error) {
			return client.GetSeedClientConfig(ctx, t.SeedName(), accessLevel)
		})
	}

	if t.ProjectName() != "" {
		return m.getClientConfig(t, func(client clientgarden.Client) (clientcmd.ClientConfig, error) {
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

	filename := filepath.Join(m.sessionDirectory, fmt.Sprintf("kubeconfig.%x.yaml", md5.Sum(data))) // #nosec G401 -- No cryptographic context.

	err = os.WriteFile(filename, data, 0o600)
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

func (m *managerImpl) getClientConfig(t Target, loadClientConfig func(clientgarden.Client) (clientcmd.ClientConfig, error)) (clientcmd.ClientConfig, error) {
	client, err := m.GardenClient(t.GardenName())
	if err != nil {
		return nil, fmt.Errorf("failed to create garden cluster client: %w", err)
	}

	return loadClientConfig(client)
}

func (m *managerImpl) patchTarget(ctx context.Context, patch func(t *targetImpl) error) (Target, error) {
	target, err := m.CurrentTarget()
	if err != nil {
		return nil, err
	}

	targetCopy := target.DeepCopy()

	impl, ok := targetCopy.(*targetImpl)
	if !ok {
		return nil, errors.New("target must be using targetImpl as its underlying type")
	}

	if err := patch(impl); err != nil {
		return nil, err
	}

	return m.updateTarget(ctx, impl)
}

func (m *managerImpl) updateClientConfigSymlink(ctx context.Context, target Target) error {
	symlinkPath := filepath.Join(m.sessionDirectory, "kubeconfig.yaml")

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
		t, err = m.CurrentTarget()
	}

	return t, err
}

func (m *managerImpl) GardenClient(name string) (clientgarden.Client, error) {
	return newGardenClient(name, m.config, m.clientProvider)
}

// ShootNames returns all shoot names for the current target.
func (m *managerImpl) ShootNames(ctx context.Context) ([]string, error) {
	t, err := m.CurrentTarget()
	if err != nil {
		return nil, err
	}

	gardenClient, err := m.GardenClient(t.GardenName())
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client for garden cluster %q: %w", t.GardenName(), err)
	}

	shootList, err := gardenClient.ListShoots(ctx, t.WithShootName("").AsListOption())
	if err != nil {
		return nil, err
	}

	names := sets.NewString()
	for _, shoot := range shootList.Items {
		names.Insert(shoot.Name)
	}

	return names.List(), nil
}

// SeedNames returns all seeds for the current target. The
// target must at least point to a garden.
func (m *managerImpl) SeedNames(ctx context.Context) ([]string, error) {
	t, err := m.CurrentTarget()
	if err != nil {
		return nil, err
	}

	gardenClient, err := m.GardenClient(t.GardenName())
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client for garden cluster %q: %w", t.GardenName(), err)
	}

	seedList, err := gardenClient.ListSeeds(ctx)
	if err != nil {
		return nil, err
	}

	names := sets.NewString()
	for _, seed := range seedList.Items {
		names.Insert(seed.Name)
	}

	return names.List(), nil
}

// ProjectNames returns all projects for the currently targeted garden. The
// target must at least point to a garden.
func (m *managerImpl) ProjectNames(ctx context.Context) ([]string, error) {
	t, err := m.CurrentTarget()
	if err != nil {
		return nil, err
	}

	gardenClient, err := m.GardenClient(t.GardenName())
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client for garden cluster %q: %w", t.GardenName(), err)
	}

	projectList, err := gardenClient.ListProjects(ctx)
	if err != nil {
		return nil, err
	}

	names := sets.NewString()
	for _, project := range projectList.Items {
		names.Insert(project.Name)
	}

	return names.List(), nil
}

// GardenNames returns all identities and aliases of configured Gardens.
func (m *managerImpl) GardenNames() ([]string, error) {
	config := m.Configuration()
	if config == nil {
		return nil, errors.New("could not get configuration")
	}

	names := sets.NewString()
	for _, garden := range config.Gardens {
		names.Insert(garden.Name)

		if garden.Alias != "" {
			names.Insert(garden.Alias)
		}
	}

	return names.List(), nil
}

func writeRawConfig(config clientcmd.ClientConfig) ([]byte, error) {
	rawConfig, err := config.RawConfig()
	if err != nil {
		return nil, err
	}

	return clientcmd.Write(rawConfig)
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
