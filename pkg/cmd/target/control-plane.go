/*
SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Projecter contributors

SPDX-License-Identifier: Apache-2.0
*/

package target

import (
	"github.com/spf13/cobra"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
	"github.com/gardener/gardenctl-v2/pkg/flags"
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
gardenctl target control-plane --garden my-garden --project my-project --shoot my-shoot`,
		ValidArgsFunction: validTargetFunctionWrapper(f, ioStreams, TargetKindControlPlane),
		RunE:              base.WrapRunE(o, f),
	}

	o.AddFlags(cmd.Flags())

	f.TargetFlags().AddGardenFlag(cmd.Flags())
	f.TargetFlags().AddProjectFlag(cmd.Flags())
	f.TargetFlags().AddShootFlag(cmd.Flags())
	f.TargetFlags().AddSeedFlag(cmd.Flags())
	flags.RegisterCompletionFuncsForTargetFlags(cmd, f, ioStreams, cmd.Flags())

	return cmd
}
