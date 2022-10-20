/*
SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors
SPDX-License-Identifier: Apache-2.0
*/

package target

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
	"github.com/gardener/gardenctl-v2/pkg/target"
)

const (
	historyFile = "history"
)

// NewCmdHistory returns a new target history command
func NewCmdHistory(f util.Factory, o *HistoryOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history",
		Short: "Print the target history",
		Long:  "Print the target history",
		RunE:  base.WrapRunE(o, f),
	}

	return cmd
}

// HistoryOptions is a struct to support target history command
type HistoryOptions struct {
	base.Options
	path string
}

// HistoryWriteOptions is a struct to support target history write command
type HistoryWriteOptions struct {
	base.Options
	cmd  *cobra.Command
	path string
}

// NewHistoryOptions returns initialized HistoryOptions
func NewHistoryOptions(ioStreams util.IOStreams) *HistoryOptions {
	return &HistoryOptions{
		Options: base.Options{
			IOStreams: ioStreams,
		},
	}
}

// NewHistoryWriteOptions returns initialized HistoryWriteOptions
func NewHistoryWriteOptions(ioStreams util.IOStreams) *HistoryWriteOptions {
	return &HistoryWriteOptions{
		Options: base.Options{
			IOStreams: ioStreams,
		},
	}
}

// Run does the actual work of the command.
func (o *HistoryOptions) Run(f util.Factory) error {
	if err := HistoryOutput(o.path, o.Options); err != nil {
		return err
	}

	return nil
}

// HistoryWrite executes history file write
func HistoryWrite(path string, s string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		return fmt.Errorf("history file open error %s", path)
	}
	defer f.Close()

	if _, err := f.WriteString(s); err != nil {
		return fmt.Errorf("history file write error %s", path)
	}

	return nil
}

// HistoryOutput executes history output
func HistoryOutput(path string, o base.Options) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(o.IOStreams.Out, "%s", content)

	return err
}

// HistoryParse executes target history parse
func HistoryParse(currentTarget target.Target) (string, error) {
	var (
		slice        []string
		targetString = "target"
	)

	if currentTarget.GardenName() != "" {
		slice = append(slice, "--garden", currentTarget.GardenName())
	}

	if currentTarget.ProjectName() != "" {
		slice = append(slice, "--project", currentTarget.ProjectName())
	}

	if currentTarget.SeedName() != "" {
		slice = append(slice, "--seed", currentTarget.SeedName())
	}

	if currentTarget.ShootName() != "" {
		slice = append(slice, "--shoot", currentTarget.ShootName())
	}

	if currentTarget.ControlPlane() {
		slice = append(slice, "--control-plane")
	}

	return fmt.Sprintln(os.Args[0], targetString, strings.Join(slice, " ")), nil
}

// Run does the actual work of the command.
func (o *HistoryWriteOptions) Run(f util.Factory) error {
	// keep unset target history
	//nolint
	if o.cmd.CalledAs() == "view" || o.cmd.CalledAs() == "history" {

		return nil
	}

	manager, err := f.Manager()
	if err != nil {
		return err
	}

	currentTarget, err := manager.CurrentTarget()
	if err != nil {
		return fmt.Errorf("failed to get current target: %w", err)
	}

	s, err := HistoryParse(currentTarget)

	if err != nil {
		return err
	}

	return HistoryWrite(o.path, s)
}

// Complete adapts from the command line args to the data required.
func (o *HistoryWriteOptions) Complete(f util.Factory, cmd *cobra.Command, args []string) error {
	o.cmd = cmd
	o.path = filepath.Join(f.GardenHomeDir(), historyFile)

	return nil
}

// Complete adapts from the command line args to the data required.
func (o *HistoryOptions) Complete(f util.Factory, cmd *cobra.Command, args []string) error {
	o.path = filepath.Join(f.GardenHomeDir(), historyFile)
	return nil
}
