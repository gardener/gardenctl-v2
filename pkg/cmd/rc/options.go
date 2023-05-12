/*
SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package rc

import (
	"fmt"
	"regexp"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
	"github.com/gardener/gardenctl-v2/pkg/env"
)

var prefixRegexp = regexp.MustCompile(`^[[:alpha:]][\w-]*$`)

type options struct {
	base.Options
	// Shell to configure.
	Shell string
	// CmdPath is the path of the called command.
	CmdPath string
	// Prefix is prefix for shell aliases and functions
	Prefix string
	// NoCompletion if the value is true tab completion is not part of the startup script
	NoCompletion bool
	// NoKubeconfig if the value is true the KUBECONFIG environment variable is not modified in the startup script
	NoKubeconfig bool
	// Template is the script template
	Template env.Template
}

// Complete adapts from the command line args to the data required.
func (o *options) Complete(_ util.Factory, cmd *cobra.Command, _ []string) error {
	o.Shell = cmd.Name()
	o.CmdPath = cmd.Parent().CommandPath()
	o.Template = env.NewTemplate("rc")

	return nil
}

// Validate validates the provided command options.
func (o *options) Validate() error {
	if o.Shell == "" {
		return pflag.ErrHelp
	}

	s := env.Shell(o.Shell)
	if err := s.Validate(); err != nil {
		return err
	}

	if !prefixRegexp.MatchString(o.Prefix) {
		return fmt.Errorf("prefix must start with an alphabetic character may be followed by alphanumeric characters, underscore or dash")
	}

	return nil
}

// Run does the actual work of the command.
func (o *options) Run(_ util.Factory) error {
	data := map[string]interface{}{
		"shell":        o.Shell,
		"prefix":       o.Prefix,
		"noCompletion": o.NoCompletion,
		"noKubeconfig": o.NoKubeconfig,
	}

	return o.Template.ExecuteTemplate(o.IOStreams.Out, o.Shell, data)
}

// AddFlags binds the command options to a given flagset.
func (o *options) AddFlags(flags *pflag.FlagSet) {
	flags.StringVarP(&o.Prefix, "prefix", "p", "g", "The prefix used for aliases and functions")
	flags.BoolVar(&o.NoCompletion, "no-completion", false, "The startup script should not setup completion")
	flags.BoolVar(&o.NoKubeconfig, "no-kubeconfig", false, "The startup script should not modify the KUBECONFIG environment variable")
}
