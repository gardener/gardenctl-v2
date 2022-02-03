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

// NewTargetKindSeed returns a new target seed command.
func NewTargetKindSeed(f util.Factory, ioStreams util.IOStreams) *cobra.Command {
	o := &TargetOptions{
		Kind: TargetKindSeed,
		Options: base.Options{
			IOStreams: ioStreams,
		},
	}
	cmd := &cobra.Command{
		Use:   "seed",
		Short: "Set seed for next operations",
		Example: `# target seed with name seed_name
gardenctl target seed seed_name`,
		ValidArgsFunction: validTargetFunctionWrapper(f, ioStreams, TargetKindSeed),
		RunE:              runCmdTargetWrapper(f, o),
	}

	o.AddFlags(cmd.Flags())

	return cmd
}
