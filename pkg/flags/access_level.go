/*
SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package flags

import (
	"fmt"

	"github.com/spf13/cobra"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/gardener/gardenctl-v2/pkg/config"
)

// AddKubeconfigAccessLevelFlag registers the --kubeconfig-access-level flag as a
// persistent flag on cmd, binding it to value. Shell completion for the flag
// is registered alongside.
//
// This flag is only meaningful for commands that produce a gardenlogin-driven
// kubeconfig (target, kubeconfig, kubectl-env, ssh, sshpatch). Registering it
// per-command rather than as a root persistent flag keeps it out of the help
// output of unrelated commands like `gardenctl config` or `gardenctl version`.
func AddKubeconfigAccessLevelFlag(cmd *cobra.Command, value *config.KubeconfigAccessLevel) {
	cmd.PersistentFlags().Var(value, "kubeconfig-access-level",
		fmt.Sprintf(`Override default kubeconfig access level for shoots/managed-seeds. One of %q, %q, %q.`,
			config.KubeconfigAccessLevelAdmin, config.KubeconfigAccessLevelViewer, config.KubeconfigAccessLevelAuto))
	utilruntime.Must(cmd.RegisterFlagCompletionFunc("kubeconfig-access-level", kubeconfigAccessLevelCompletionFunc))
}

// RegisterKubeconfigAccessLevelCompletion registers the shared admin/viewer/auto
// shell-completion function on flagName.
func RegisterKubeconfigAccessLevelCompletion(cmd *cobra.Command, flagName string) {
	utilruntime.Must(cmd.RegisterFlagCompletionFunc(flagName, kubeconfigAccessLevelCompletionFunc))
}

func kubeconfigAccessLevelCompletionFunc(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return config.AllKubeconfigAccessLevelStrings(), cobra.ShellCompDirectiveNoFileComp
}
