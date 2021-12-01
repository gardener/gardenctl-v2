/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package kenv

import (
	"embed"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"text/template"

	sprigv3 "github.com/Masterminds/sprig/v3"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
)

//go:embed templates
var fsys embed.FS

var (
	ErrNoGardenTargeted              = errors.New("no garden targeted")
	ErrNeitherProjectNorSeedTargeted = errors.New("neither project nor seed is targeted")
	ErrProjectAndSeedTargeted        = errors.New("project and seed must not be targeted at the same time")
)

type cmdOptions struct {
	base.Options

	// Unset resets environment variables and configuration of the cloudprovider CLI for your shell.
	Unset bool
	// Shell to configure.
	Shell string
	// GardenDir is the configuration directory of gardenctl.
	GardenDir string
	// CmdPath is the path of the called command.
	CmdPath string
}

var _ base.CommandOptions = &cmdOptions{}

// Complete adapts from the command line args to the data required.
func (o *cmdOptions) Complete(f util.Factory, cmd *cobra.Command, _ []string) error {
	o.GardenDir = f.GardenHomeDir()
	o.Shell = cmd.Name()
	o.CmdPath = cmd.Parent().CommandPath()

	return nil
}

// Validate validates the provided command options.
func (o *cmdOptions) Validate() error {
	if o.Shell == "" {
		return pflag.ErrHelp
	}

	s := Shell(o.Shell)
	if err := s.Validate(); err != nil {
		return err
	}

	return nil
}

// Run does the actual work of the command.
func (o *cmdOptions) Run(f util.Factory) error {
	// target manager
	m, err := f.Manager()
	if err != nil {
		return err
	}

	// current target
	t, err := m.CurrentTarget()
	if err != nil {
		return err
	}

	ctx := f.Context()

	// kubeconfig for current target
	kubeconfig, err := m.Kubeconfig(ctx, t)
	if err != nil {
		return err
	}

	return o.execTmpl(kubeconfig)
}

// AddFlags binds the command options to a given flagset.
func (o *cmdOptions) AddFlags(flags *pflag.FlagSet) {
	usage := fmt.Sprintf("Generate the script to unset the KUBECONFIG environment variable for %s", o.Shell)
	flags.BoolVarP(&o.Unset, "unset", "u", o.Unset, usage)
}

func (o *cmdOptions) execTmpl(kubeconfig []byte) error {
	m := make(map[string]interface{})
	m["__meta"] = o.generateMetadata()

	if !o.Unset {
		filename, err := writeKubeconfig(o.GardenDir, "kubeconfig.*.yaml", kubeconfig)
		if err != nil {
			return err
		}

		m["filename"] = filename
	}

	t := template.New("base").Funcs(sprigv3.TxtFuncMap()).Funcs(template.FuncMap{"shellEscape": util.ShellEscape})
	t = template.Must(t.ParseFS(fsys,
		filepath.Join("templates", "kubectl.tmpl"),
		filepath.Join("templates", "usage-hint.tmpl"),
	))

	return t.ExecuteTemplate(o.IOStreams.Out, o.Shell, m)
}

func writeKubeconfig(dir, pattern string, data []byte) (string, error) {
	tmpDir := filepath.Join(dir, "tmp")
	if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
		err := os.Mkdir(tmpDir, os.ModePerm)
		if err != nil {
			return "", fmt.Errorf("failed to create gardenctl tmpdir %s: %w", tmpDir, err)
		}
	}

	tmpFile, err := ioutil.TempFile(tmpDir, pattern)
	if err != nil {
		return "", err
	}

	filename := tmpFile.Name()
	if err = ioutil.WriteFile(tmpFile.Name(), data, 0600); err != nil {
		return "", fmt.Errorf("failed to write kubeconfig file to %s: %w", filename, err)
	}

	return filename, nil
}

// generateMetadata generate shell specific usage hint
func (o *cmdOptions) generateMetadata() map[string]interface{} {
	return map[string]interface{}{
		"unset":       o.Unset,
		"shell":       o.Shell,
		"cli":         "kubectl",
		"prompt":      Shell(o.Shell).Prompt(runtime.GOOS),
		"commandPath": o.CmdPath,
	}
}

// Shell represents the type of shell
type Shell string

const (
	bash       Shell = "bash"
	zsh        Shell = "zsh"
	fish       Shell = "fish"
	powershell Shell = "powershell"
)

var validShells = []Shell{bash, zsh, fish, powershell}

// EvalCommand returns the script that evaluates the given command
func (s Shell) EvalCommand(cmd string) string {
	var format string

	switch s {
	case fish:
		format = "eval (%s)"
	case powershell:
		// Invoke-Expression cannot execute multi-line functions!!!
		format = "& %s | Invoke-Expression"
	default:
		format = "eval $(%s)"
	}

	return fmt.Sprintf(format, cmd)
}

// Prompt returns the typical prompt for a given os
func (s Shell) Prompt(goos string) string {
	switch s {
	case powershell:
		if goos == "windows" {
			return "PS C:\\> "
		}

		return "PS /> "
	default:
		return "$ "
	}
}

// AddCommand adds a shell sub command to a parent
func (s Shell) AddCommand(parent *cobra.Command, runE func(cmd *cobra.Command, args []string) error) {
	shortFormat := "generate the cloud provider CLI configuration script for %s"
	longFormat := `Generate the cloud provider CLI configuration script for %s.

To load the cloud provider CLI configuration script in your current shell session:
%s
`
	cmdWithPrompt := s.Prompt(runtime.GOOS) + s.EvalCommand(fmt.Sprintf("%s %s", parent.CommandPath(), s))
	shell := string(s)
	cmd := &cobra.Command{
		Use:   shell,
		Short: fmt.Sprintf(shortFormat, shell),
		Long:  fmt.Sprintf(longFormat, shell, cmdWithPrompt),
		RunE:  runE,
	}
	parent.AddCommand(cmd)
}

// Validate checks if the shell is valid
func (s Shell) Validate() error {
	for _, shell := range validShells {
		if s == shell {
			return nil
		}
	}

	return fmt.Errorf("invalid shell given, must be one of %v", validShells)
}
