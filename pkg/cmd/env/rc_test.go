/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package env_test

import (
	"fmt"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/gardener/gardenctl-v2/internal/util"
	utilmocks "github.com/gardener/gardenctl-v2/internal/util/mocks"
	"github.com/gardener/gardenctl-v2/pkg/cmd/env"
)

var _ = Describe("Env Commands", func() {
	Describe("Having a RC command instance", func() {
		var (
			ctrl    *gomock.Controller
			factory *utilmocks.MockFactory
			streams util.IOStreams
			out     *util.SafeBytesBuffer
			cmd     *cobra.Command
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			factory = utilmocks.NewMockFactory(ctrl)
			streams, _, out, _ = util.NewTestIOStreams()
			cmd = env.NewCmdRC(factory, streams)
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		It("should have Use, Flags and SubCommands", func() {
			Expect(cmd.Use).To(Equal("rc"))
			Expect(cmd.Aliases).To(HaveLen(1))
			Expect(cmd.Aliases).To(Equal([]string{"profile"}))
			Expect(cmd.Flag("output")).To(BeNil())
			subCmds := cmd.Commands()
			Expect(len(subCmds)).To(Equal(4))
			for _, c := range subCmds {
				s := env.Shell(c.Name())
				Expect(s).To(BeElementOf(env.ValidShells))
				flag := c.Flag("prefix")
				Expect(flag).NotTo(BeNil())
				Expect(flag.Shorthand).To(Equal("p"))
				Expect(flag.DefValue).To(Equal("g"))
			}
		})

		It("should execute the bash subcommand", func() {
			cmd.SetArgs([]string{"bash"})
			Expect(cmd.Execute()).To(Succeed())
			Expect(out.String()).To(Equal(`if [ -z "$GCTL_SESSION_ID" ] && [ -z "$TERM_SESSION_ID" ]; then
  export GCTL_SESSION_ID=$(uuidgen)
fi
source <(gardenctl completion bash)
alias g=gardenctl
complete -o default -F __start_gardenctl g
alias gtv='gardenctl target view -o yaml'
alias gtc='gardenctl target control-plane'
alias gtc-='gardenctl target unset control-plane'
alias gk='eval $(gardenctl kubectl-env bash)'
alias gp='eval $(gardenctl provider-env bash)'
`))
		})

		It("should execute the zsh subcommand", func() {
			cmd.SetArgs([]string{"zsh"})
			Expect(cmd.Execute()).To(Succeed())
			Expect(out.String()).To(Equal(`if [ -z "$GCTL_SESSION_ID" ] && [ -z "$TERM_SESSION_ID" ]; then
  export GCTL_SESSION_ID=$(uuidgen)
fi
if (( $+commands[gardenctl] )); then
    gctl_completion_file="${fpath[1]}/_gardenctl"
    if [[ ! -f "$gctl_completion_file" ]]; then
        gardenctl completion zsh >| "$gctl_completion_file"
        source "$gctl_completion_file"
    else
        source "$gctl_completion_file"
        gardenctl completion zsh >| "$gctl_completion_file" &|
    fi
    unset gctl_completion_file
fi
alias g=gardenctl
alias gtv='gardenctl target view -o yaml'
alias gtc='gardenctl target control-plane'
alias gtc-='gardenctl target unset control-plane'
alias gk='eval $(gardenctl kubectl-env bash)'
alias gp='eval $(gardenctl provider-env bash)'
`))
		})

		It("should execute the fish subcommand", func() {
			cmd.SetArgs([]string{"fish"})
			Expect(cmd.Execute()).To(Succeed())
			Expect(out.String()).To(Equal(`if [ -z "$GCTL_SESSION_ID" ] && [ -z "$TERM_SESSION_ID" ];
  set -gx GCTL_SESSION_ID (uuidgen)
end
gardenctl completion fish | source
alias g=gardenctl
complete -c g -w gardenctl
alias gtv='gardenctl target view -o yaml'
alias gtc='gardenctl target control-plane'
alias gtc-='gardenctl target unset control-plane'
alias gk='eval (gardenctl kubectl-env bash)'
alias gp='eval (gardenctl provider-env bash)'
`))
		})

		It("should execute the powershell subcommand", func() {
			cmd.SetArgs([]string{"powershell"})
			Expect(cmd.Execute()).To(Succeed())
			Expect(out.String()).To(Equal(`if ( !(Test-Path Env:GCTL_SESSION_ID) -and !(Test-Path Env:TERM_SESSION_ID) ) {
  $Env:GCTL_SESSION_ID = [guid]::NewGuid().ToString()
}
Set-Alias -Name g -Value (get-command gardenctl).Path -Option AllScope -Force
function Gardenctl-Completion-Powershell {
  $s = (gardenctl completion powershell)
  @(
    ($s -replace "(?ms)^Register-ArgumentCompleter -CommandName 'gardenctl' -ScriptBlock", "` + "`" + `$scriptBlock =")
    "Register-ArgumentCompleter -CommandName 'gardenctl' -ScriptBlock ` + "`" + `$scriptBlock"
    "Register-ArgumentCompleter -CommandName 'g' -ScriptBlock ` + "`" + `$scriptBlock"
  )
}
Gardenctl-Completion-Powershell | Out-String | Invoke-Expression
function Gardenctl-Target-View {
  gardenctl target view -o yaml
}
Set-Alias -Name gtv -Value Gardenctl-Target-View -Option AllScope -Force
function Gardenctl-Target-ControlPlane {
  gardenctl target control-plane
}
Set-Alias -Name gtc -Value Gardenctl-Target-ControlPlane -Option AllScope -Force
function Gardenctl-Target-Unset-ControlPlane {
  gardenctl target unset control-plane
}
Set-Alias -Name gtc- -Value Gardenctl-Target-Unset-ControlPlane -Option AllScope -Force
function Gardenctl-KubectlEnv {
  gardenctl kubectl-env powershell | Out-String | Invoke-Expression
}
Set-Alias -Name gk -Value Gardenctl-KubectlEnv -Option AllScope -Force
function Gardenctl-ProviderEnv {
  gardenctl provider-env powershell | Out-String | Invoke-Expression
}
Set-Alias -Name gp -Value Gardenctl-ProviderEnv -Option AllScope -Force
`))
		})

		It("should execute the bash subcommand with prefix flag", func() {
			cmd.SetArgs([]string{"--prefix=gctl", "bash"})
			Expect(cmd.Execute()).To(Succeed())
			Expect(out.String()).To(MatchRegexp(`(?m)^alias gctl=gardenctl$`))
		})
	})

	Describe("Validating the RC command options", func() {
		var options *env.RCOptions

		BeforeEach(func() {
			options = &env.RCOptions{}
			options.Shell = "bash"
			options.Prefix = "g"
		})

		It("should successfully validate the options", func() {
			Expect(options.Validate()).To(Succeed())
		})

		It("should return an error when the shell is empty", func() {
			options.Shell = ""
			Expect(options.Validate()).To(MatchError(pflag.ErrHelp))
		})

		It("should return an error when the shell is invalid", func() {
			options.Shell = "cmd"
			Expect(options.Validate()).To(MatchError(fmt.Sprintf("invalid shell given, must be one of %v", env.ValidShells)))
		})

		It("should return an error when the prefix is invalid", func() {
			options.Prefix = "!"
			Expect(options.Validate()).To(MatchError("prefix must start with an alphabetic character may be followed by alphanumeric characters, underscore or dash"))
		})
	})
})
