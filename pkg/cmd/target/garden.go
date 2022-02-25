/*
SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package target

import (
	"github.com/spf13/cobra"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
)

// NewCmdTargetGarden returns a new target garden command.
func NewCmdTargetGarden(f util.Factory, ioStreams util.IOStreams) *cobra.Command {
	o := &TargetOptions{
		Kind: TargetKindGarden,
		Options: base.Options{
			IOStreams: ioStreams,
		},
	}
	cmd := &cobra.Command{
		Use:   "garden",
		Short: "Target a garden",
		Long:  "Target a garden to set the scope for the next operations",
		Example: `# target garden with name my-garden
gardenctl target garden my-garden`,
		ValidArgsFunction: validTargetFunctionWrapper(f, ioStreams, TargetKindGarden),
		RunE:              base.WrapRunE(o, f),
	}

	o.AddOutputFlag(cmd.Flags())

	return cmd
}
