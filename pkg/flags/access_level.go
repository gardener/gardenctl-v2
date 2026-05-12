/*
SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package flags

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/gardener/gardenctl-v2/pkg/config"
)

// AddKubeconfigAccessLevelFlag registers --access-level (full form,
// supports admin/viewer/auto) plus --admin and --viewer as mutually-exclusive
// shorthands on cmd, all bound to value. The shorthands are convenience for
// the common case; the full form is the canonical way to set "auto" via flag
// (and is what completion targets).
//
// This flag set is only meaningful for commands that produce a gardenlogin-
// driven kubeconfig. Registering it per-command rather than as a root
// persistent flag keeps it out of the help output of unrelated commands.
func AddKubeconfigAccessLevelFlag(cmd *cobra.Command, value *config.KubeconfigAccessLevel) {
	flags := cmd.PersistentFlags()
	flags.Var(value, "access-level",
		fmt.Sprintf(`Override default kubeconfig access level for shoots/managed-seeds. One of %q, %q, %q.`,
			config.KubeconfigAccessLevelAdmin, config.KubeconfigAccessLevelViewer, config.KubeconfigAccessLevelAuto))
	addBoolAccessLevelFlag(flags, value, "admin", config.KubeconfigAccessLevelAdmin)
	addBoolAccessLevelFlag(flags, value, "viewer", config.KubeconfigAccessLevelViewer)
	cmd.MarkFlagsMutuallyExclusive("access-level", "admin", "viewer")
	utilruntime.Must(cmd.RegisterFlagCompletionFunc("access-level", kubeconfigAccessLevelCompletionFunc))
}

// addBoolAccessLevelFlag registers a bool-shaped flag whose presence sets
// target to level. Setting NoOptDefVal makes pflag accept `--admin` without
// `=true` - pflag's parser keys on that field, not the IsBoolFlag interface.
func addBoolAccessLevelFlag(flags *pflag.FlagSet, target *config.KubeconfigAccessLevel, name string, level config.KubeconfigAccessLevel) {
	flag := flags.VarPF(newBoolAccessLevel(target, level), name, "",
		fmt.Sprintf("shorthand for --access-level=%s", level))
	flag.NoOptDefVal = "true"
}

// boolAccessLevel is a bool-shaped pflag.Value that, when set to true, writes
// a fixed access level into a shared target pointer. This lets --admin and
// --viewer funnel into the same KubeconfigAccessLevel value as --access-level.
type boolAccessLevel struct {
	target *config.KubeconfigAccessLevel
	level  config.KubeconfigAccessLevel
}

var _ pflag.Value = (*boolAccessLevel)(nil)

func newBoolAccessLevel(target *config.KubeconfigAccessLevel, level config.KubeconfigAccessLevel) *boolAccessLevel {
	return &boolAccessLevel{target: target, level: level}
}

func (b *boolAccessLevel) Set(v string) error {
	parsed, err := strconv.ParseBool(v)
	if err != nil {
		return fmt.Errorf("invalid boolean value %q", v)
	}

	if !parsed {
		return fmt.Errorf("does not accept false; omit the flag instead, or use --access-level=... to pick a different level")
	}

	*b.target = b.level

	return nil
}

// String reports the flag's default for help output. Always "false" since
// these shortcuts default to unset; current state is reflected via *b.target.
func (b *boolAccessLevel) String() string { return "false" }

func (b *boolAccessLevel) Type() string { return "bool" }

// IsBoolFlag tells pflag this flag does not require a value, so users can
// pass `--admin` rather than `--admin=true`.
func (b *boolAccessLevel) IsBoolFlag() bool { return true }

// RegisterKubeconfigAccessLevelCompletion registers the shared admin/viewer/auto
// shell-completion function on flagName.
func RegisterKubeconfigAccessLevelCompletion(cmd *cobra.Command, flagName string) {
	utilruntime.Must(cmd.RegisterFlagCompletionFunc(flagName, kubeconfigAccessLevelCompletionFunc))
}

func kubeconfigAccessLevelCompletionFunc(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return config.AllKubeconfigAccessLevelStrings(), cobra.ShellCompDirectiveNoFileComp
}
