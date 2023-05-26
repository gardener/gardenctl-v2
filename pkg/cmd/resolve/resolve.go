/*
SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package resolve

import (
	"github.com/spf13/cobra"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
	"github.com/gardener/gardenctl-v2/pkg/flags"
)

// NewCmdResolve returns a new resolve command.
func NewCmdResolve(f util.Factory, ioStreams util.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resolve",
		Short: "Resolve the current target",
		Long:  "Resolve garden, seed, project or shoot for the current target",
	}

	cmd.AddCommand(newCmdResolveGarden(f, ioStreams))
	cmd.AddCommand(newCmdResolveProject(f, ioStreams))
	cmd.AddCommand(newCmdResolveSeed(f, ioStreams))
	cmd.AddCommand(newCmdResolveShoot(f, ioStreams))

	return cmd
}

// newCmdResolveGarden returns a new resolve garden command.
func newCmdResolveGarden(f util.Factory, ioStreams util.IOStreams) *cobra.Command {
	o := newOptions(ioStreams, KindGarden)
	cmd := &cobra.Command{
		Use:   "garden",
		Short: "Resolve garden for the current target",
		RunE:  base.WrapRunE(o, f),
	}

	o.AddFlags(cmd.Flags())
	o.RegisterCompletionsForOutputFlag(cmd)

	f.TargetFlags().AddGardenFlag(cmd.Flags())
	flags.RegisterCompletionFuncsForTargetFlags(cmd, f, ioStreams, cmd.Flags())

	return cmd
}

// newCmdResolveProject returns a new resolve seed command.
func newCmdResolveProject(f util.Factory, ioStreams util.IOStreams) *cobra.Command {
	o := newOptions(ioStreams, KindProject)
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Resolve project for the current target",
		RunE:  base.WrapRunE(o, f),
	}

	o.AddFlags(cmd.Flags())
	o.RegisterCompletionsForOutputFlag(cmd)

	f.TargetFlags().AddGardenFlag(cmd.Flags())
	f.TargetFlags().AddProjectFlag(cmd.Flags())
	f.TargetFlags().AddSeedFlag(cmd.Flags())
	f.TargetFlags().AddShootFlag(cmd.Flags())
	flags.RegisterCompletionFuncsForTargetFlags(cmd, f, ioStreams, cmd.Flags())

	return cmd
}

// newCmdResolveSeed returns a new resolve seed command.
func newCmdResolveSeed(f util.Factory, ioStreams util.IOStreams) *cobra.Command {
	o := newOptions(ioStreams, KindSeed)
	cmd := &cobra.Command{
		Use:   "seed",
		Short: "Resolve seed for the current target",
		RunE:  base.WrapRunE(o, f),
	}

	o.AddFlags(cmd.Flags())
	o.RegisterCompletionsForOutputFlag(cmd)

	f.TargetFlags().AddGardenFlag(cmd.Flags())
	f.TargetFlags().AddProjectFlag(cmd.Flags())
	f.TargetFlags().AddSeedFlag(cmd.Flags())
	f.TargetFlags().AddShootFlag(cmd.Flags())
	flags.RegisterCompletionFuncsForTargetFlags(cmd, f, ioStreams, cmd.Flags())

	return cmd
}

// newCmdResolveShoot returns a new resolve shoot command.
func newCmdResolveShoot(f util.Factory, ioStreams util.IOStreams) *cobra.Command {
	o := newOptions(ioStreams, KindShoot)
	cmd := &cobra.Command{
		Use:   "shoot",
		Short: "Resolve shoot for the current target",
		Long: `Resolve shoot for the current target.
This command is particularly useful when you need to understand which shoot the current target translates to, regardless of whether a seed or a shoot is targeted.
It fetches and displays information about its associated garden, project, seed, and shoot, including any access restrictions in place.
A garden and either a seed or shoot must be specified, either from a previously saved target or directly via target flags. Target flags temporarily override the saved target for the current command run.`,
		Example: `# Resolve shoot for managed seed
gardenctl resolve shoot --garden mygarden --seed myseed

# Resolve shoot. Output in json format
gardenctl resolve shoot --garden mygarden --shoot myseed -ojson

# Resolve shoot cluster details for a shoot that might have the same name as others across different projects
# Use fully qualified target flags to specify the correct garden, project, and shoot
gardenctl resolve shoot --garden mygarden --project myproject --shoot myshoot`,
		RunE: base.WrapRunE(o, f),
	}

	o.AddFlags(cmd.Flags())
	o.RegisterCompletionsForOutputFlag(cmd)

	f.TargetFlags().AddGardenFlag(cmd.Flags())
	f.TargetFlags().AddProjectFlag(cmd.Flags())
	f.TargetFlags().AddSeedFlag(cmd.Flags())
	f.TargetFlags().AddShootFlag(cmd.Flags())
	f.TargetFlags().AddControlPlaneFlag(cmd.Flags())
	flags.RegisterCompletionFuncsForTargetFlags(cmd, f, ioStreams, cmd.Flags())

	return cmd
}
