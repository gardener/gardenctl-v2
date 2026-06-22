/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package target

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"sigs.k8s.io/yaml"
)

// TargetReader can read targets.
type TargetReader interface {
	// Read returns the current target. If no target exists yet, a default
	// (empty) target is returned.
	Read() (Target, error)
}

// TargetWriter can write targets.
type TargetWriter interface {
	// Write takes a target and saves it permanently.
	Write(t Target) error
}

//go:generate mockgen -destination=./mocks/mock_target_trovider.go -package=mocks github.com/gardener/gardenctl-v2/pkg/target TargetProvider

// PersistedTargetReader can read the target exactly as persisted, without
// command-local overlays such as CLI target flags.
type PersistedTargetReader interface {
	// ReadPersisted returns the persisted target before command-local overlays.
	ReadPersisted() (Target, error)
}

// TargetProvider can read effective targets, read persisted targets, and write
// targets.
type TargetProvider interface {
	TargetReader
	PersistedTargetReader
	TargetWriter
}

// fsTargetProvider is a TragetProvider that
// reads and writes from the local filesystem.
type fsTargetProvider struct {
	targetFile string
}

var (
	_ TargetProvider        = &fsTargetProvider{}
	_ PersistedTargetReader = &fsTargetProvider{}
)

func (p *fsTargetProvider) Read() (Target, error) {
	f, err := os.Open(p.targetFile)
	if err != nil {
		if os.IsNotExist(err) {
			return &targetImpl{}, nil
		}

		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to determine filesize: %w", err)
	}

	target := &targetImpl{}

	if stat.Size() > 0 {
		buf, err := os.ReadFile(p.targetFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read target file: %w", err)
		}

		if err = yaml.Unmarshal(buf, target); err != nil {
			return nil, fmt.Errorf("failed to decode as YAML: %w", err)
		}
	}

	if err := target.Validate(); err != nil {
		return nil, fmt.Errorf("target is invalid: %w", err)
	}

	return target, nil
}

func (p *fsTargetProvider) ReadPersisted() (Target, error) {
	return p.Read()
}

// Write takes a target and saves it permanently.
func (p *fsTargetProvider) Write(t Target) error {
	buf, err := yaml.Marshal(t)
	if err != nil {
		return fmt.Errorf("failed to encode as YAML: %w", err)
	}

	if err := os.WriteFile(p.targetFile, buf, 0o600); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// NewTargetProvider returns a new TargetProvider that
// reads and writes the current Target.
func NewTargetProvider(targetFile string, targetFlags TargetFlags) TargetProvider {
	delegate := &fsTargetProvider{
		targetFile: targetFile,
	}

	if targetFlags == nil {
		return delegate
	}

	return &dynamicTargetProvider{
		delegate:    delegate,
		targetFlags: targetFlags,
	}
}

// dynamicTargetProvider is a wrapper that combines the basic
// filesystem based TargetProvider with CLI flags, to allow the user
// to change the target for individual gardenctl commands
// on-the-fly without changing the file on disk every time.
//
// If no CLI flags are given, this functions identical to the regular
// filesystem TargetProvider.
//
// Otherwise, the flags are used to augment the existing target.
type dynamicTargetProvider struct {
	// delegate must be valid a filesystem based TargetProvider (required)
	delegate *fsTargetProvider
	// targetFlags refers to the global target CLI flags (required)
	targetFlags TargetFlags
}

var (
	_ TargetProvider        = &dynamicTargetProvider{}
	_ PersistedTargetReader = &dynamicTargetProvider{}
)

// Read returns the persisted target with CLI target flags applied as an
// overlay. Empty flags leave the persisted target unchanged; provided flags
// update the described scope, and the merged target is validated before it is
// returned.
func (p *dynamicTargetProvider) Read() (Target, error) {
	current, err := p.delegate.Read()
	if err != nil {
		return nil, err
	}

	return merge(current, p.targetFlags)
}

func (p *dynamicTargetProvider) ReadPersisted() (Target, error) {
	return p.delegate.Read()
}

// Write takes a target and saves it permanently.
func (p *dynamicTargetProvider) Write(t Target) error {
	return p.delegate.Write(t)
}

func combine(cliFlags TargetFlags, patternFlags TargetFlags) (TargetFlags, error) {
	result := &targetFlagsImpl{
		gardenName:   combineStringField(cliFlags.GardenName(), patternFlags.GardenName()),
		projectName:  combineStringField(cliFlags.ProjectName(), patternFlags.ProjectName()),
		seedName:     combineStringField(cliFlags.SeedName(), patternFlags.SeedName()),
		shootName:    combineStringField(cliFlags.ShootName(), patternFlags.ShootName()),
		controlPlane: NewBoolFlag(false),
	}

	var conflicts []string

	addStringConflict := func(flagName, cliValue, patternValue string) {
		if cliValue != "" && patternValue != "" && cliValue != patternValue {
			conflicts = append(conflicts, fmt.Sprintf("--%s=%s contradicts pattern (%s=%s)", flagName, cliValue, flagName, patternValue))
		}
	}
	addBoolConflict := func(flagName string, cliFlag, patternFlag BoolFlag) {
		if cliFlag.Provided() && patternFlag.Provided() && cliFlag.Value() != patternFlag.Value() {
			conflicts = append(conflicts, fmt.Sprintf("--%s=%t contradicts pattern (%s=%t)", flagName, cliFlag.Value(), flagName, patternFlag.Value()))
		}
	}

	// Today's pattern keys are garden, project, namespace, shoot (see
	// pkg/config/config.go PatternKey constants); seed and controlPlane can
	// never be set by a pattern, so those checks are defensive and only fire
	// if the pattern key set ever grows.
	addStringConflict("garden", cliFlags.GardenName(), patternFlags.GardenName())
	addStringConflict("project", cliFlags.ProjectName(), patternFlags.ProjectName())
	addStringConflict("seed", cliFlags.SeedName(), patternFlags.SeedName())
	addStringConflict("shoot", cliFlags.ShootName(), patternFlags.ShootName())
	addBoolConflict("control-plane", cliFlags.ControlPlane(), patternFlags.ControlPlane())

	cliCP := cliFlags.ControlPlane()

	patternCP := patternFlags.ControlPlane()
	switch {
	case cliCP.Provided():
		result.controlPlane = newProvidedBoolFlag(cliCP.Value())
	case patternCP.Provided():
		result.controlPlane = newProvidedBoolFlag(patternCP.Value())
	}

	if len(conflicts) > 0 {
		return nil, errors.New(strings.Join(conflicts, "; "))
	}

	return result, nil
}

func combineStringField(cliValue, patternValue string) string {
	if patternValue != "" {
		return patternValue
	}

	return cliValue
}

// merge returns a new target with the specified target flags merged into it.
//
// merge returns a Target value, not a record of flag presence. String selector
// flags are represented by their non-empty values; control-plane is different
// because BoolFlag preserves explicit false separately from omission. Callers
// that need that distinction must inspect tf.ControlPlane().Provided().
//
// Target flags describe a requested selector path:
//
//	garden -> project -> shoot
//	garden -> seed    -> shoot
//
// Persisted target values are used only for context above the first explicitly
// provided selector level. Once a flag starts a selector path, stale deeper
// values and sibling branch values are cleared unless they are explicitly
// provided as flags too. For example, --shoot keeps the current project or seed
// selector, but --garden G --shoot S starts at garden scope and does not inherit
// the persisted project or seed.
func merge(t Target, tf TargetFlags) (Target, error) {
	newTarget := t.DeepCopy()

	if tf.IsEmpty() {
		return newTarget, nil
	}

	pathFlagProvided := targetPathFlagProvided(tf)

	// Setting a garden resets all deeper targeting levels, allowing
	// the user to "move up". For example, when they have targeted a shoot,
	// simply specifying "--garden mygarden" should target the garden, not
	// the same shoot within the garden named mygarden.
	if tf.GardenName() != "" {
		newTarget = newTarget.WithGardenName(tf.GardenName()).WithProjectName("").WithSeedName("").WithShootName("")
	}

	switch {
	case tf.ProjectName() != "" && tf.SeedName() != "":
		// --project and --seed together require --shoot; without it,
		// the target would be ambiguous (seed and project are independent paths).
		if tf.ShootName() == "" {
			return nil, errors.New("cannot specify --project and --seed at the same time")
		}

		// All three specified: the shoot is looked up via project and the
		// seed is validated during manager enrichment.
		newTarget = newTarget.WithProjectName(tf.ProjectName()).WithSeedName(tf.SeedName()).WithShootName(tf.ShootName())

	case tf.ProjectName() != "":
		// Targeting a project resets seed and shoot.
		newTarget = newTarget.WithProjectName(tf.ProjectName()).WithSeedName("").WithShootName("")
		if tf.ShootName() != "" {
			newTarget = newTarget.WithShootName(tf.ShootName())
		}

	case tf.SeedName() != "":
		// Targeting a seed resets project and shoot.
		newTarget = newTarget.WithSeedName(tf.SeedName()).WithProjectName("").WithShootName("")
		if tf.ShootName() != "" {
			newTarget = newTarget.WithShootName(tf.ShootName())
		}

	case tf.ShootName() != "":
		newTarget = newTarget.WithShootName(tf.ShootName())
	}

	// Path selector resets shoot-bound control-plane state.
	if pathFlagProvided {
		newTarget = newTarget.WithControlPlane(false)
	}

	// Explicit --control-plane wins.
	if tf.ControlPlane().Provided() {
		newTarget = newTarget.WithControlPlane(tf.ControlPlane().Value())
	}

	if err := newTarget.Validate(); err != nil {
		return nil, fmt.Errorf("invalid target flags: %w", err)
	}

	return newTarget, nil
}

// targetPathFlagProvided reports whether the user explicitly supplied any
// garden/project/seed/shoot selector field. Used to decide if a stale
// ControlPlane flag from the persisted target should be cleared. Including
// ControlPlane in the comparison would be self-defeating.
func targetPathFlagProvided(tf TargetFlags) bool {
	return tf.GardenName() != "" ||
		tf.ProjectName() != "" ||
		tf.SeedName() != "" ||
		tf.ShootName() != ""
}
