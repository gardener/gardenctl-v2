/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package env

import (
	"fmt"
	"os"
	"regexp"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
)

// NewCmdRC returns a new rc command.
func NewCmdRC(f util.Factory, ioStreams util.IOStreams) *cobra.Command {
	o := &rcOptions{
		Options: base.Options{
			IOStreams: ioStreams,
		},
	}
	cmd := &cobra.Command{
		Use:   "rc",
		Short: "Generate a gardenctl startup script for the specified shell",
		Long: `Generate a gardenctl startup script for the specified shell that contains various tweaks,
such as setting environment variables, loading completions and adding some helpful aliases or functions.

See each sub-command's help for details on how to use the generated shell startup script.
`,
		Aliases: []string{"profile"},
	}

	var (
		subCmd   *cobra.Command
		selfPath = "gardenctl"
	)

	if p, err := os.Executable(); err != nil {
		selfPath = p
	}

	cmdPath := selfPath + " rc"
	runE := base.WrapRunE(o, f)

	subCmd = createBashCommand(cmdPath)
	subCmd.RunE = runE
	o.AddFlags(subCmd.Flags())
	cmd.AddCommand(subCmd)

	subCmd = createZshCommand(cmdPath)
	subCmd.RunE = runE
	o.AddFlags(subCmd.Flags())
	cmd.AddCommand(subCmd)

	subCmd = createFishCommand(cmdPath)
	subCmd.RunE = runE
	o.AddFlags(subCmd.Flags())
	cmd.AddCommand(subCmd)

	subCmd = createPowershellCommand(cmdPath)
	subCmd.RunE = runE
	o.AddFlags(subCmd.Flags())
	cmd.AddCommand(subCmd)

	return cmd
}

func createBashCommand(cmdPath string) *cobra.Command {
	shell := Shell("bash")

	return &cobra.Command{
		Use:   string(shell),
		Short: fmt.Sprintf("Generate a gardenctl startup script for %s", shell),
		Long: fmt.Sprintf(`Generate a gardenctl startup script for %[1]s that contains various tweaks,
such as setting environment variables, loading completions and adding some helpful aliases or functions.

To load gardenctl startup script for each %[1]s session, execute once:

    echo 'source <(%[2]s %[1]s)' >> ~/.bashrc
`,
			shell, cmdPath,
		),
	}
}

func createZshCommand(cmdPath string) *cobra.Command {
	shell := Shell("zsh")

	return &cobra.Command{
		Use:   string(shell),
		Short: fmt.Sprintf("Generate a gardenctl startup script for %s", shell),
		Long: fmt.Sprintf(`Generate a gardenctl startup script for %[1]s that contains various tweaks,
such as setting environment variables, loading completions and adding some helpful aliases or functions.

If shell completion is not already enabled in your environment you will need
to enable it. You can execute the following once:

    echo "autoload -U compinit; compinit" >> ~/.zshrc

To load gardenctl startup script for each %[1]s session, execute once:

    echo 'source <(%[2]s %[1]s)' >> ~/.zshrc
`,
			shell, cmdPath,
		),
	}
}

func createFishCommand(cmdPath string) *cobra.Command {
	shell := Shell("fish")

	return &cobra.Command{
		Use:   string(shell),
		Short: fmt.Sprintf("Generate a gardenctl startup script for %s", shell),
		Long: fmt.Sprintf(`Generate a gardenctl startup script for %[1]s that contains various tweaks,
such as setting environment variables, loading completions and adding some helpful aliases or functions.

To load gardenctl startup script for each %[1]s session, execute once:

    echo '%[2]s %[1]s | source' >> ~/.config/fish/config.fish
`,
			shell, cmdPath,
		),
	}
}

func createPowershellCommand(cmdPath string) *cobra.Command {
	shell := Shell("powershell")

	return &cobra.Command{
		Use:   string(shell),
		Short: fmt.Sprintf("Generate a gardenctl startup script for %s", shell),
		Long: fmt.Sprintf(`Generate a gardenctl startup script for %[1]s, that contains various tweaks,
such as setting environment variables, loadings completions and adding some helpful aliases or functions.

To load gardenctl startup script for each %[1]s session, execute once:

    echo '%[2]s %[1]s | Out-String | Invoke-Expression' >> $profile
`,
			shell, cmdPath,
		),
	}
}

var prefixRegexp = regexp.MustCompile(`^[[:alpha:]][\w-]*$`)

type rcOptions struct {
	base.Options
	// Shell to configure.
	Shell string
	// CmdPath is the path of the called command.
	CmdPath string
	// Prefix is prefix for shell aliases and functions
	Prefix string
	// Template is the script template
	Template Template
}

// Complete adapts from the command line args to the data required.
func (o *rcOptions) Complete(f util.Factory, cmd *cobra.Command, args []string) error {
	o.Shell = cmd.Name()
	o.CmdPath = cmd.Parent().CommandPath()
	o.Template = newTemplate("rc")

	return nil
}

// Validate validates the provided command options.
func (o *rcOptions) Validate() error {
	if o.Shell == "" {
		return pflag.ErrHelp
	}

	s := Shell(o.Shell)
	if err := s.Validate(); err != nil {
		return err
	}

	if !prefixRegexp.MatchString(o.Prefix) {
		return fmt.Errorf("prefix must start with an alphabetic character may be followed by alphanumeric characters, underscore or dash")
	}

	return nil
}

// Run does the actual work of the command.
func (o *rcOptions) Run(f util.Factory) error {
	data := map[string]interface{}{
		"shell":  o.Shell,
		"prefix": o.Prefix,
	}

	return o.Template.ExecuteTemplate(o.IOStreams.Out, o.Shell, data)
}

// AddFlags binds the command options to a given flagset.
func (o *rcOptions) AddFlags(flags *pflag.FlagSet) {
	flags.StringVarP(&o.Prefix, "prefix", "p", "g", "The prefix used for aliases and functions")
}
