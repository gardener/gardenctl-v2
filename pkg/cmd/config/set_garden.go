/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package config

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/component-base/cli/flag"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
	"github.com/gardener/gardenctl-v2/pkg/config"
	flagsutil "github.com/gardener/gardenctl-v2/pkg/flags"
)

const (
	FlagDefaultShootAccessLevel = "default-shoot-access-level"
	FlagDefaultSeedAccessLevel  = "default-seed-access-level"
)

// NewCmdConfigSetGarden returns a new (config) set-garden command.
func NewCmdConfigSetGarden(f util.Factory, ioStreams util.IOStreams) *cobra.Command {
	o := &setGardenOptions{
		Options: base.Options{
			IOStreams: ioStreams,
		},
	}
	cmd := &cobra.Command{
		Use:   "set-garden",
		Short: "Modify or add a garden to the gardenctl configuration",
		Long: `Modify or add a garden to the gardenctl configuration.
A valid garden configuration consists of a name (required), kubeconfig path (required), a context as well as any number of patterns.
In order to share the configuration with gardenlogin, you need to set the name to the cluster identity.`,
		Example: `# add new garden my-garden with no additional values
gardenctl config set-garden my-garden

# add new garden with name set to cluster identity and path to kubeconfig file configured
export KUBECONFIG=~/path/to/garden-cluster/kubeconfig.yaml
CLUSTER_IDENTITY=$(kubectl -n kube-system get configmap cluster-identity -ojsonpath={.data.cluster-identity})
gardenctl config set-garden $CLUSTER_IDENTITY --kubeconfig $KUBECONFIG

# configure my-garden with a context and patterns
gardenctl config set-garden my-garden --context garden-context --pattern "^(?:landscape-dev/)?shoot--(?P<project>.+)--(?P<shoot>.+)$" --pattern "https://dashboard\.gardener\.cloud/namespace/(?P<namespace>[^/]+)/shoots/(?P<shoot>[^/]+)"

# configure prd-garden so shoot kubeconfigs default to read-only viewer access (seed access stays at admin)
gardenctl config set-garden prd-garden --default-shoot-access-level viewer`,
		ValidArgsFunction: validGardenArgsFunctionWrapper(f, ioStreams),
		RunE:              base.WrapRunE(o, f),
	}

	o.AddFlags(cmd.Flags())

	flagsutil.RegisterKubeconfigAccessLevelCompletion(cmd, FlagDefaultShootAccessLevel)
	flagsutil.RegisterKubeconfigAccessLevelCompletion(cmd, FlagDefaultSeedAccessLevel)

	return cmd
}

type setGardenOptions struct {
	base.Options
	// Configuration is the gardenctl configuration
	Configuration *config.Config
	// Name is a unique name of this Garden that can be used to target this Garden
	Name string
	// Alias is a unique alias of this Garden that can be used to target this Garden
	// +optional
	Alias flag.StringFlag
	// KubeconfigFlag is the path to the kubeconfig file of the Garden cluster
	KubeconfigFlag flag.StringFlag
	// ContextFlag Overrides the current-context of the garden cluster kubeconfig
	// +optional
	ContextFlag flag.StringFlag
	// Patterns is a list of regex patterns that can be defined to use custom input formats for targeting
	// Use named capturing groups to match target values.
	// Supported capturing groups: project, namespace, shoot
	// +optional
	Patterns []string
	// DefaultShootAccessLevelFlag sets kubeconfigAccessLevelDefaults.shoots in the
	// stored Garden config (admin | viewer | auto | "" to delegate to gardenlogin's default).
	// +optional
	DefaultShootAccessLevelFlag accessLevelFlag
	// DefaultSeedAccessLevelFlag sets kubeconfigAccessLevelDefaults.seeds in the
	// stored Garden config (admin | viewer | auto | "" to delegate to gardenlogin's default).
	// +optional
	DefaultSeedAccessLevelFlag accessLevelFlag
}

// accessLevelFlag wraps flag.StringFlag to validate config.KubeconfigAccessLevel
// values at parse time while inheriting the standard provided-tracking semantics
// used by the other flags in this command.
type accessLevelFlag struct {
	flag.StringFlag
}

var _ pflag.Value = (*accessLevelFlag)(nil)

func (f *accessLevelFlag) Set(value string) error {
	if err := config.KubeconfigAccessLevel(value).Validate(); err != nil {
		return err
	}

	return f.StringFlag.Set(value)
}

func (f *accessLevelFlag) AccessLevel() config.KubeconfigAccessLevel {
	return config.KubeconfigAccessLevel(f.Value())
}

// Complete adapts from the command line args to the data required.
func (o *setGardenOptions) Complete(f util.Factory, _ *cobra.Command, args []string) error {
	manager, err := f.Manager()
	if err != nil {
		return fmt.Errorf("failed to get target manager: %w", err)
	}

	cfg := manager.Configuration()
	if cfg == nil {
		return errors.New("failed to get configuration")
	}

	o.Configuration = cfg

	if len(args) > 0 {
		o.Name = strings.TrimSpace(args[0])
	}

	return nil
}

// Validate validates the provided options.
func (o *setGardenOptions) Validate() error {
	if o.Name == "" {
		return errors.New("garden identity is required")
	}

	if err := config.ValidateGardenName(o.Name); err != nil {
		return fmt.Errorf("invalid garden name %q: %w", o.Name, err)
	}

	if o.Alias.Provided() {
		if err := config.ValidateGardenName(o.Alias.Value()); err != nil {
			return fmt.Errorf("invalid garden alias %q: %w", o.Alias.Value(), err)
		}
	}

	return validatePatterns(o.Patterns)
}

// AddFlags adds flags to adjust the output to a cobra command.
func (o *setGardenOptions) AddFlags(flags *pflag.FlagSet) {
	flags.Var(&o.KubeconfigFlag, "kubeconfig", "path to kubeconfig file for this garden cluster")
	flags.Var(&o.ContextFlag, "context", "override the current-context of the garden cluster kubeconfig")
	flags.Var(&o.Alias, "alias", "unique alias of this garden that can be used instead of the name to target this garden")
	flags.StringArrayVar(&o.Patterns, "pattern", nil, `define regex match patterns for this garden for custom input formats for targeting.
Use named capturing groups to match target values.
Supported capturing groups: project, namespace, shoot.
Note that if you set this flag it will overwrite the pattern list in the config file.
You may specify any number of extra patterns.`)
	flags.Var(&o.DefaultShootAccessLevelFlag, FlagDefaultShootAccessLevel,
		fmt.Sprintf(`default kubeconfig access level when targeting shoots in this garden. One of %q, %q, %q. Pass an empty value to unset and delegate to gardenlogin's default.`,
			config.KubeconfigAccessLevelAdmin, config.KubeconfigAccessLevelViewer, config.KubeconfigAccessLevelAuto))
	flags.Var(&o.DefaultSeedAccessLevelFlag, FlagDefaultSeedAccessLevel,
		fmt.Sprintf(`default kubeconfig access level when targeting seeds in this garden (and shoots that back a managed seed, since they physically are the seed cluster). One of %q, %q, %q. Pass an empty value to unset and delegate to gardenlogin's default.`,
			config.KubeconfigAccessLevelAdmin, config.KubeconfigAccessLevelViewer, config.KubeconfigAccessLevelAuto))
}

// Run executes the command.
func (o *setGardenOptions) Run(_ util.Factory) error {
	garden, err := o.Configuration.Garden(o.Name)
	if err == nil {
		if o.KubeconfigFlag.Provided() {
			garden.Kubeconfig = o.KubeconfigFlag.Value()
		}

		if o.ContextFlag.Provided() {
			garden.Context = o.ContextFlag.Value()
		}

		if o.Alias.Provided() {
			garden.Alias = o.Alias.Value()
		}

		if o.Patterns != nil {
			firstPattern := o.Patterns[0]
			if len(firstPattern) > 0 {
				garden.Patterns = o.Patterns
			} else {
				garden.Patterns = nil
			}
		}

		o.applyAccessLevelFlags(garden)
	} else {
		newGarden := config.Garden{
			Name:       o.Name,
			Kubeconfig: o.KubeconfigFlag.Value(),
			Context:    o.ContextFlag.Value(),
			Alias:      o.Alias.Value(),
			Patterns:   o.Patterns,
		}

		o.applyAccessLevelFlags(&newGarden)

		o.Configuration.Gardens = append(o.Configuration.Gardens, newGarden)
	}

	err = o.Configuration.Save()
	if err != nil {
		return fmt.Errorf("failed to configure garden: %w", err)
	}

	fmt.Fprintf(o.IOStreams.Out, "Successfully configured garden %q\n", o.Name)

	if o.DefaultShootAccessLevelFlag.Provided() || o.DefaultSeedAccessLevelFlag.Provided() {
		fmt.Fprintf(o.IOStreams.ErrOut,
			"\nNote: existing session kubeconfigs are not regenerated. Run `gardenctl target` again for the new access level to take effect.\n")
	}

	return nil
}

// applyAccessLevelFlags writes the per-scope access level flags into the Garden's
// KubeconfigAccessLevelDefaults, allocating the struct lazily and clearing it when
// both fields end up empty so we don't litter the config file with empty objects.
func (o *setGardenOptions) applyAccessLevelFlags(garden *config.Garden) {
	if !o.DefaultShootAccessLevelFlag.Provided() && !o.DefaultSeedAccessLevelFlag.Provided() {
		return
	}

	if garden.KubeconfigAccessLevelDefaults == nil {
		garden.KubeconfigAccessLevelDefaults = &config.KubeconfigAccessLevels{}
	}

	if o.DefaultShootAccessLevelFlag.Provided() {
		garden.KubeconfigAccessLevelDefaults.Shoots = o.DefaultShootAccessLevelFlag.AccessLevel()
	}

	if o.DefaultSeedAccessLevelFlag.Provided() {
		garden.KubeconfigAccessLevelDefaults.Seeds = o.DefaultSeedAccessLevelFlag.AccessLevel()
	}

	if garden.KubeconfigAccessLevelDefaults.Shoots == "" && garden.KubeconfigAccessLevelDefaults.Seeds == "" {
		garden.KubeconfigAccessLevelDefaults = nil
	}
}

func validatePatterns(patterns []string) error {
	if patterns == nil || patterns[0] == "" && len(patterns) == 1 {
		return nil
	}

	for i, p := range patterns {
		if p == "" {
			return fmt.Errorf("pattern[%d] must not be empty", i)
		}

		re, err := regexp.Compile(p)
		if err != nil {
			return fmt.Errorf("pattern[%d] is not a valid regular expression: %w", i, err)
		}

		names := re.SubexpNames()
		for _, name := range names[1:] {
			if name != "project" && name != "namespace" && name != "shoot" {
				return fmt.Errorf("pattern[%d] contains an invalid subexpression %q", i, name)
			}
		}
	}

	return nil
}
