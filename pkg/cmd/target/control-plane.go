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

// NewCmdTargetControlPlane returns a new target control plane command.
func NewCmdTargetControlPlane(f util.Factory, ioStreams util.IOStreams) *cobra.Command {
	o := &TargetOptions{
		Kind: TargetKindControlPlane,
		Options: base.Options{
			IOStreams: ioStreams,
		},
	}
	cmd := &cobra.Command{
		Use:   "control-plane",
		Short: "Target the control plane of the shoot",
		Long:  "Target the control plane of the shoot cluster to set the scope for the next operations",
		Example: `# target control-plane of currently selected shoot cluster
gardenctl target control-plane

# target control-plane of shoot my-shoot
gardenctl target control-plane --shoot my-shoot --project my-project --garden my-garden`,
		ValidArgsFunction: validTargetFunctionWrapper(f, ioStreams, TargetKindControlPlane),
		RunE:              runCmdTargetWrapper(f, o),
	}

	o.AddFlags(cmd.Flags())

	return cmd
}
