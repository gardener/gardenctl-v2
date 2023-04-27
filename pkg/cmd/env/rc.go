/*
SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package env

import (
	"fmt"
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

	runE := base.WrapRunE(o, f)
	cmdPath := "gardenctl rc"
	subCmds := []*cobra.Command{
		createBashCommand(cmdPath),
		createZshCommand(cmdPath),
		createFishCommand(cmdPath),
		createPowershellCommand(cmdPath),
	}

	for _, subCmd := range subCmds {
		subCmd.RunE = runE
		o.AddFlags(subCmd.Flags())
		cmd.AddCommand(subCmd)
	}

	return cmd
}

func createBashCommand(cmdPath string) *cobra.Command {
	shell := Shell("bash")

	return &cobra.Command{
		Use:   string(shell),
		Short: fmt.Sprintf("Generate a gardenctl startup script for %s", shell),
		Long: fmt.Sprintf(`Generate a gardenctl startup script for %[1]s that contains various tweaks,
such as setting environment variables, loading completions and adding some helpful aliases or functions.

To load gardenctl startup script for each %[1]s session add the following line at the end of the ~/.bashrc file:

    source <(%[2]s %[1]s)

You will need to start a new shell for this setup to take effect.
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

If shell completion is not already enabled in your environment you need to add at least this command to your zsh configuration file:

    autoload -Uz compinit && compinit

To load gardenctl startup script for each %[1]s session add the following line at the end of the ~/.zshrc file:

    source <(%[2]s %[1]s)

You will need to start a new shell for this setup to take effect.

### Zshell frameworks or plugin manager

If you use a framework for managing your zsh configuration you may need to load this code as a custom plugin in your framework.

#### oh-my-zsh:
Create a file `+"`"+`~/.oh-my-zsh/custom/plugins/gardenctl/gardenctl.plugin.zsh`+"`"+` with the following content:

    if (( $+commands[gardenctl] )); then
      source <(gardenctl rc zsh)
    fi

To use it, add gardenctl to the plugins array in your ~/.zshrc file:

    plugins=(... gardenctl)

For more information about oh-my-zsh custom plugins please refer to https://github.com/ohmyzsh/ohmyzsh#custom-plugins-and-themes.

#### zgen:
Create an oh-my-zsh plugin for gardenctl like described above and load it in the .zshrc file:

    zgen load /path/to/custom/plugins/gardenctl

For more information about loading plugins with zgen please refer to https://github.com/tarjoilija/zgen#load-plugins-and-completions

#### zinit:
Create an oh-my-zsh plugin for gardenctl like described above and load it in the .zshrc file:

    zinit snippet /path/to/custom/plugins/gardenctl/gardenctl.plugin.zsh

For more information about loading plugins and snippets with zinit please refer to https://github.com/zdharma-continuum/zinit#plugins-and-snippets.
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

To load gardenctl startup script for each %[1]s session add the following line at the end of the ~/.config/fish/config.fish file:

    %[2]s %[1]s | source

You will need to start a new shell for this setup to take effect.
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

To load gardenctl startup script for each %[1]s session add the following line at the end of the $profile file:

    %[2]s %[1]s | Out-String | Invoke-Expression

You will need to start a new shell for this setup to take effect.
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
	// NoCompletion if the value is true tab completion is not part of the startup script
	NoCompletion bool
	// NoKubeconfig if the value is true the KUBECONFIG environment variable is not modified in the startup script
	NoKubeconfig bool
	// Template is the script template
	Template Template
}

// Complete adapts from the command line args to the data required.
func (o *rcOptions) Complete(_ util.Factory, cmd *cobra.Command, _ []string) error {
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
func (o *rcOptions) Run(_ util.Factory) error {
	data := map[string]interface{}{
		"shell":        o.Shell,
		"prefix":       o.Prefix,
		"noCompletion": o.NoCompletion,
		"noKubeconfig": o.NoKubeconfig,
	}

	return o.Template.ExecuteTemplate(o.IOStreams.Out, o.Shell, data)
}

// AddFlags binds the command options to a given flagset.
func (o *rcOptions) AddFlags(flags *pflag.FlagSet) {
	flags.StringVarP(&o.Prefix, "prefix", "p", "g", "The prefix used for aliases and functions")
	flags.BoolVar(&o.NoCompletion, "no-completion", false, "The startup script should not setup completion")
	flags.BoolVar(&o.NoKubeconfig, "no-kubeconfig", false, "The startup script should not modify the KUBECONFIG environment variable")
}
