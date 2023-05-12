/*
SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package rc

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
	"github.com/gardener/gardenctl-v2/pkg/env"
)

// NewCmdRC returns a new rc command.
func NewCmdRC(f util.Factory, ioStreams util.IOStreams) *cobra.Command {
	o := &options{
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
	shell := env.Shell("bash")

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
	shell := env.Shell("zsh")

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
	shell := env.Shell("fish")

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
	shell := env.Shell("powershell")

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
