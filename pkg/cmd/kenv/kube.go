/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/
package kenv

import (
	"github.com/spf13/cobra"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
)

// NewCmdKubectlEnv returns a new kubectl-env command.
func NewCmdKubectlEnv(f util.Factory, ioStreams util.IOStreams) *cobra.Command {
	o := &cmdOptions{
		Options: base.Options{
			IOStreams: ioStreams,
		},
	}
	runE := base.WrapRunE(o, f)
	cmd := &cobra.Command{
		Use:   "kubectl-env",
		Short: "generate a script that points the KUBECONFIG environment variable to the targeted cluster for the specified shell",
		Long: `Generate a script that points the KUBECONFIG environment variable to the targeted cluster for the specified shell.
See each sub-command's help for details on how to use the generated script.

The generated script points the KUBECONFIG environment variable to the currently targeted shoot or seed cluster.
`,
		Aliases: []string{"k-env", "cluster-env"},
	}
	o.AddFlags(cmd.PersistentFlags())

	for _, s := range validShells {
		s.AddCommand(cmd, runE)
	}

	return cmd
}
