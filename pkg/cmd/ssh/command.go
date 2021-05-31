/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package ssh

import (
	"encoding/json"
	"fmt"

	"github.com/gardener/gardenctl-v2/internal/util"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"k8s.io/component-base/version"
)

// NewCommand returns a new version command.
func NewCommand(f util.Factory, o *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the gardenctl version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Complete(f, cmd, args); err != nil {
				return fmt.Errorf("failed to complete command options: %w", err)
			}
			if err := o.Validate(); err != nil {
				return err
			}

			return runCommand(o)
		},
	}

	cmd.Flags().BoolVar(&o.Short, "short", o.Short, "If true, print just the version number.")
	cmd.Flags().StringVarP(&o.Output, "output", "o", o.Output, "One of 'yaml' or 'json'.")

	return cmd
}

/*
// FetchShootFromTarget fetches shoot object from given target
func FetchShootFromTarget(target TargetInterface) (*gardencorev1beta1.Shoot, error) {
	gardenClientset, err := target.GardenerClient()
	if err != nil {
		return nil, err
	}

	var shoot *gardencorev1beta1.Shoot
	if target.Stack()[1].Kind == TargetKindProject {
		project, err := gardenClientset.CoreV1beta1().Projects().Get(target.Stack()[1].Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}

		shoot, err = gardenClientset.CoreV1beta1().Shoots(*project.Spec.Namespace).Get(target.Stack()[2].Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	} else {
		shootList, err := gardenClientset.CoreV1beta1().Shoots(metav1.NamespaceAll).List(metav1.ListOptions{})
		if err != nil {
			return nil, err
		}

		for index, s := range shootList.Items {
			if s.Name == target.Stack()[2].Name && *s.Spec.SeedName == target.Stack()[1].Name {
				shoot = &shootList.Items[index]
				break
			}
		}
	}

	return shoot, nil
}
*/

func runCommand(opt *Options) error {
	/*
		shoot, err := util.FetchShootFromTarget(target)

		// no node name specified, so just print a list of available nodes
		if len(args) == 0 && flagproviderid == "" {
			return printNodeNames(target, shoot.Name)
		}

		fmt.Println("Check Public IP")
		myPublicIP := util.GetPublicIP()

		// create bastion resource in garden cluster
		// wait for gardenlet, GCM and provider-extensions to do their magic
		// wait until status.ingress is available on the bastion
		// ssh into the node
	*/
	versionInfo := version.Get()

	switch opt.Output {
	case "":
		if opt.Short {
			fmt.Fprintf(opt.IOStreams.Out, "Version: %s\n", versionInfo.GitVersion)
		} else {
			fmt.Fprintf(opt.IOStreams.Out, "Version: %s\n", fmt.Sprintf("%#v", versionInfo))
		}

	case "yaml":
		marshalled, err := yaml.Marshal(&versionInfo)
		if err != nil {
			return err
		}

		fmt.Fprintln(opt.IOStreams.Out, string(marshalled))

	case "json":
		marshalled, err := json.MarshalIndent(&versionInfo, "", "  ")
		if err != nil {
			return err
		}

		fmt.Fprintln(opt.IOStreams.Out, string(marshalled))

	default:
		// There is a bug in the program if we hit this case.
		// However, we follow a policy of never panicking.
		return fmt.Errorf("options were not validated: --output=%q should have been rejected", opt.Output)
	}

	return nil
}
