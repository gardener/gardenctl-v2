/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
SPDX-License-Identifier: Apache-2.0
*/

package target

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
)

const (
	historyFile string = "history"
)

// NewCmdHistory returns a new target history command.
func NewCmdHistory(f util.Factory, o base.Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history",
		Short: "Print the target history",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHistoryCommand(f, o)
		},
	}

	return cmd
}

func runHistoryCommand(f util.Factory, o base.Options) error {
	if err := checkInstalled("fzf"); err != nil {
		return errors.New("fzf not installed. Please install from https://github.com/junegunn/fzf")
	}

	if err := HistoryOutput(historyPath(f), o); err != nil {
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
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	return o.PrintObject(string(content))
}

func checkInstalled(name string) error {
	if _, err := exec.LookPath(name); err != nil {
		return fmt.Errorf(name + " is not installed")
	}

	return nil
}

func HistoryParse(f util.Factory, c *cobra.Command, name string) (string, error) {
	var (
		callAs string
		slice  []string
	)

	if name == "" {
		callAs = c.CalledAs()
	} else {
		callAs = name
	}

	m, err := f.Manager()
	if err != nil {
		return "", err
	}

	currentTarget, err := m.CurrentTarget()
	if err != nil {
		return "", fmt.Errorf("failed to get current target: %v", err)
	}

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

	return fmt.Sprintln(c.Root().Name(), callAs, strings.Join(slice, " ")), nil
}

func historyPath(f util.Factory) string {
	return f.GardenHomeDir() + "/" + historyFile
}
