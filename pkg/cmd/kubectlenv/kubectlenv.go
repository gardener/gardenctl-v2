/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package kubectlenv

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
	"github.com/gardener/gardenctl-v2/pkg/config"
	"github.com/gardener/gardenctl-v2/pkg/env"
	"github.com/gardener/gardenctl-v2/pkg/flags"
)

// NewCmdKubectlEnv returns a new kubectl-env command. accessLevel is bound to
// the --access-level flag. It is only consulted when linkKubeconfig is false;
// in symlink mode kubectl-env merely points KUBECONFIG at the existing session
// kubeconfig and Run warns if the flag was set anyway.
func NewCmdKubectlEnv(f util.Factory, ioStreams util.IOStreams, accessLevel *config.KubeconfigAccessLevel) *cobra.Command {
	o := &options{
		Options: base.Options{
			IOStreams: ioStreams,
		},
		AccessLevel: accessLevel,
	}
	runE := base.WrapRunE(o, f)
	cmd := &cobra.Command{
		Use:   "kubectl-env",
		Short: "Generate a script that points KUBECONFIG to the targeted cluster for the specified shell",
		Long: `Generate a script that points KUBECONFIG to the currently targeted shoot, seed, or garden cluster for the specified shell.

Each sub-command produces a shell-specific script.
For details on how to use the printed shell script, such as applying it temporarily to your current session or permanently through your shell's startup file, refer to the corresponding sub-command's help.

Note: --access-level only has an effect when linkKubeconfig is false. In symlink mode (the default) this command just points KUBECONFIG at the existing session kubeconfig, so the access level is whatever the most recent ` + "`gardenctl target`" + ` chose.
`,
		Aliases: []string{"k-env", "cluster-env"},
	}
	o.AddFlags(cmd.PersistentFlags())
	flags.AddKubeconfigAccessLevelFlag(cmd, accessLevel)

	for _, s := range env.ValidShells() {
		cmd.AddCommand(&cobra.Command{
			Use:   string(s),
			Short: fmt.Sprintf("Generate a script that points KUBECONFIG to the targeted cluster for %s", s),
			Long: fmt.Sprintf(`Generate a script that points KUBECONFIG to the targeted cluster for %s.

To load the kubectl configuration script in your current shell session:
%s

To apply this setting automatically in every shell session, consider adding the command at the end of your %s file.
`,
				s, s.Prompt(runtime.GOOS)+s.EvalCommand(fmt.Sprintf("gardenctl %s %s", cmd.Name(), s)), s.Config(),
			),
			RunE: runE,
		})
	}

	return cmd
}
