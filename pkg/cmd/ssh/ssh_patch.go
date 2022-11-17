package ssh

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
)

func NewCmdSSHPatch(f util.Factory, ioStreams util.IOStreams) *cobra.Command {
	o := newSSHPatchOptions(ioStreams)
	cmd := &cobra.Command{
		Use:   "ssh-patch [BASTION_NAME]",
		Short: "Update a bastion host previously created through the ssh command",
		Example: `# Update CIDRs on one of your bastion hosts. You can specify multiple CIDRs.
		gardenctl ssh-patch cli-xxxxxxxx --cidr 8.8.8.8/20 --cidr dead:beaf::/64
		
		# You can also omit the CIDR-flag and your system's public IPs (v4 and v6) will be auto-detected.
		gardenctl ssh-patch cli-xxxxxxxx`,
		Args: cobra.RangeArgs(0, 1),
		RunE: base.WrapRunE(o, f),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) != 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}

			bastionNames, err := o.GetBastionNameCompletions(f, cmd, toComplete)
			if err != nil {
				fmt.Fprintln(o.IOStreams.ErrOut, err.Error())
				return nil, cobra.ShellCompDirectiveNoFileComp
			}

			return bastionNames, cobra.ShellCompDirectiveNoFileComp
		},
	}

	o.AddFlags(cmd.PersistentFlags())

	return cmd
}
