/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package base

import (
	"encoding/json"
	"fmt"
	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// Options contains all settings that are used across all commands in gardenctl.
type Options struct {
	// IOStreams provides the standard names for iostreams
	IOStreams util.IOStreams

	// Output defines the output format of the version information. Either 'yaml' or 'json'
	Output string
}

// NewOptions returns initialized Options
func NewOptions(ioStreams util.IOStreams) *Options {
	return &Options{
		IOStreams: ioStreams,
	}
}

func (o *Options) AddOutputFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&o.Output, "output", "o", o.Output, "One of 'yaml' or 'json'.")
}

func (o *Options) PrintObject(obj interface{}) error {
	switch o.Output {
	case "":
		fmt.Fprintf(o.IOStreams.Out, "%v", obj)

	case "yaml":
		marshalled, err := yaml.Marshal(&obj)
		if err != nil {
			return err
		}

		fmt.Fprintln(o.IOStreams.Out, string(marshalled))

	case "json":
		marshalled, err := json.MarshalIndent(&obj, "", "  ")
		if err != nil {
			return err
		}

		fmt.Fprintln(o.IOStreams.Out, string(marshalled))

	default:
		// There is a bug in the program if we hit this case.
		// However, we follow a policy of never panicking.
		return fmt.Errorf("options were not validated: --output=%q should have been rejected", o.Output)
	}

	return nil
}

// Validate validates the provided options
func (o *Options) Validate() error {
	if o.Output != "" && o.Output != "yaml" && o.Output != "json" {
		return fmt.Errorf(`--output must be either 'yaml' or 'json'`)
	}

	return nil
}
