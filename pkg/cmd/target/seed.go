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

// NewCmdTargetSeed returns a new target seed command.
func NewCmdTargetSeed(f util.Factory, ioStreams util.IOStreams) *cobra.Command {
	o := &TargetOptions{
		Kind: TargetKindSeed,
		Options: base.Options{
			IOStreams: ioStreams,
		},
	}
	cmd := &cobra.Command{
		Use:   "seed",
		Short: "Target a seed",
		Long:  "Target a seed to set the scope for the next operations",
		Example: `# target seed with name my-seed of currently selected garden
gardenctl target seed my-seed

# target seed with name my-seed of garden my-garden
gardenctl target seed my-seed --garden my-garden`,
		ValidArgsFunction: validTargetFunctionWrapper(f, ioStreams, TargetKindSeed),
		RunE:              base.WrapRunE(o, f),
	}

	o.AddFlags(cmd.Flags())

	manager, err := f.Manager()
	utilruntime.Must(err)
	manager.TargetFlags().AddGardenFlag(cmd.Flags())
	flags.RegisterCompletionFuncsForTargetFlags(cmd, f, ioStreams, cmd.Flags())

	return cmd
}
