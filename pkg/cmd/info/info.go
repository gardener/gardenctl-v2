/*
SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package info

import (
	"github.com/spf13/cobra"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
)

// NewCmdInfo returns a new info command.
func NewCmdInfo(f util.Factory, o *options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info",
		Short: "Get landscape informations and shows the number of shoots per seed, e.g. \"gardenctl info\"",
		Args:  cobra.NoArgs,
		RunE:  base.WrapRunE(o, f),
	}

	return cmd
}
