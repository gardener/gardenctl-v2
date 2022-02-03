/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Projecter contributors

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
		Short: "Set project for next operations",
		Example: `# target project with name project_name
gardenctl target project project_name`,
		ValidArgsFunction: validTargetFunctionWrapper(f, ioStreams, TargetKindProject),
		RunE:              runCmdTargetWrapper(f, o),
	}

	o.AddFlags(cmd.Flags())

	return cmd
}
