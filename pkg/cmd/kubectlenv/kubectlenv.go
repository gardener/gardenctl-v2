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
		Long: `Generate a script that points KUBECONFIG to the targeted cluster for the specified shell.
See each sub-command's help for details on how to use the generated script.

The generated script points the KUBECONFIG environment variable to the currently targeted shoot, seed or garden cluster.
To point the KUBECONFIG to the targeted cluster in all shell sessions consider adding the script to your shell's startup configuration.
`,
		Aliases: []string{"k-env", "cluster-env"},
	}
	o.AddFlags(cmd.PersistentFlags())

	for _, s := range env.ValidShells() {
		prompt := s.Prompt(runtime.GOOS)
		evalCommand := s.EvalCommand(fmt.Sprintf("gardenctl %s %s", cmd.Name(), s))
		cmd.AddCommand(&cobra.Command{
			Use:   string(s),
			Short: fmt.Sprintf("Generate a script that points KUBECONFIG to the targeted cluster for %s", s),
			Long: fmt.Sprintf(`Generate a script that points KUBECONFIG to the targeted cluster for %s.

To load the kubectl configuration script in your current shell session:
%s

To load the kubectl configuration for each shell session add the following line at the end of the %s file:

    %s

You will need to start a new shell for this setup to take effect.
`,
				s, prompt+evalCommand, s.Config(), evalCommand,
			),
			RunE: runE,
		})
	}

	return cmd
}
