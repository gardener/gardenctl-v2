/*
SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Projecter contributors

SPDX-License-Identifier: Apache-2.0
*/

package target

import (
	"github.com/spf13/cobra"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
)

// NewCmdTargetProject returns a new target project command.
func NewCmdTargetProject(f util.Factory, ioStreams util.IOStreams) *cobra.Command {
	o := &TargetOptions{
		Kind: TargetKindProject,
		Options: base.Options{
			IOStreams: ioStreams,
		},
	}
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Target a project",
		Long:  "Target a project to set the scope for the next operations",
		Example: `# target project with name my-project of currently selected garden
gardenctl target project my-project

# target project with name my-project of garden my-garden
gardenctl target project my-project --garden my-garden`,
		ValidArgsFunction: validTargetFunctionWrapper(f, ioStreams, TargetKindProject),
		RunE:              base.WrapRunE(o, f),
	}

	o.AddOutputFlag(cmd.Flags())

	f.TF().AddTargetGardenFlag(cmd.Flags())

	o.RegisterTargetFlagCompletions(f, cmd, ioStreams)

	return cmd
}
