/*
SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Projecter contributors

SPDX-License-Identifier: Apache-2.0
*/

package target

import (
	"github.com/spf13/cobra"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
	"github.com/gardener/gardenctl-v2/pkg/flags"
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
gardenctl target shoot my-shoot --garden my-garden --project my-project`,
		ValidArgsFunction: validTargetFunctionWrapper(f, ioStreams, TargetKindShoot),
		RunE:              base.WrapRunE(o, f),
	}

	o.AddFlags(cmd.Flags())

	manager, err := f.Manager()
	utilruntime.Must(err)
	manager.TargetFlags().AddGardenFlag(cmd.Flags())
	manager.TargetFlags().AddProjectFlag(cmd.Flags())
	manager.TargetFlags().AddSeedFlag(cmd.Flags())
	flags.RegisterCompletionFuncsForTargetFlags(cmd, f, ioStreams, cmd.Flags())

	return cmd
}
