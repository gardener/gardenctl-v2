/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

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
		Short: "Set garden for next operations",
		Example: `# target garden with name garden_name
gardenctl target garden garden_name`,
		ValidArgsFunction: validTargetFunctionWrapper(f, ioStreams, TargetKindGarden),
		RunE:              runCmdTargetWrapper(f, o),
	}

	o.AddFlags(cmd.Flags())

	return cmd
}
