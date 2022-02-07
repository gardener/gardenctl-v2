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

// NewCmdTargetShoot returns a new target shoot command.
func NewCmdTargetShoot(f util.Factory, ioStreams util.IOStreams) *cobra.Command {
	o := &TargetOptions{
		Kind: TargetKindShoot,
		Options: base.Options{
			IOStreams: ioStreams,
		},
	}
	cmd := &cobra.Command{
		Use:   "shoot",
		Short: "Target a shoot",
		Long:  "Target a shoot to set the scope for the next operations",
		Example: `# target shoot with name my-shoot of currently selected project
gardenctl target shoot my-shoot

# target shoot with name my-shoot of project my-project
gardenctl target shoot my-shoot --project my-project --garden my-garden`,
		ValidArgsFunction: validTargetFunctionWrapper(f, ioStreams, TargetKindShoot),
		RunE:              runCmdTargetWrapper(f, o),
	}

	o.AddFlags(cmd.Flags())

	return cmd
}
