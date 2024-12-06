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
	"github.com/gardener/gardenctl-v2/pkg/env"
)

// NewCmdKubectlEnv returns a new kubectl-env command.
func NewCmdKubectlEnv(f util.Factory, ioStreams util.IOStreams) *cobra.Command {
	o := &options{
		Options: base.Options{
			IOStreams: ioStreams,
		},
	}
	runE := base.WrapRunE(o, f)
	cmd := &cobra.Command{
		Use:   "kubectl-env",
		Short: "Generate a script that points KUBECONFIG to the targeted cluster for the specified shell",
		Long: `Generate a script that points KUBECONFIG to the currently targeted shoot, seed, or garden cluster for the specified shell.
To apply this setting automatically in every shell session, consider adding the generated script to your shell's startup configuration.

See each sub-command's help for details on how to use the generated script.
`,
		Aliases: []string{"k-env", "cluster-env"},
	}
	o.AddFlags(cmd.PersistentFlags())

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
