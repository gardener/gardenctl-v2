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
	historyFile string = "history"
)

// NewCmdHistory returns a new target history command.
func NewCmdHistory(f util.Factory, o *HistoryOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history",
		Short: "Print the target history",
		Long: `Print the target history
The fuzzy finder must be installed.
Please refer to the installation instructions of the 3rd party tools:
* fuzzy finder -https://github.com/junegunn/fzf`,
		RunE: base.WrapRunE(o, f),
	}

	return cmd
}

type HistoryOptions struct {
	base.Options
}

// NewHistoryOptions returns initialized HistoryOptions
func NewHistoryOptions(ioStreams util.IOStreams) *HistoryOptions {
	return &HistoryOptions{
		Options: base.Options{
			IOStreams: ioStreams,
		},
	}
}

// Run does the actual work of the command
func (o *HistoryOptions) Run(f util.Factory) error {
	if err := HistoryOutput(filepath.Join(f.GardenHomeDir(), historyFile), o.Options); err != nil {
		return err
	}

	return nil
}

// HistoryWrite history write
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

// HistoryOutput history output
func HistoryOutput(path string, o base.Options) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(o.IOStreams.Out, "%s", content)

	return err
}

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
