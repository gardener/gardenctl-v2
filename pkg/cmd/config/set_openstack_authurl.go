/*
SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package config

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
	"github.com/gardener/gardenctl-v2/pkg/config"
	"github.com/gardener/gardenctl-v2/pkg/provider/common/allowpattern"
	"github.com/gardener/gardenctl-v2/pkg/provider/credvalidate"
)

// NewCmdConfigSetOpenStackAuthURL returns a new (config) set-openstack-authurl command.
func NewCmdConfigSetOpenStackAuthURL(f util.Factory, ioStreams util.IOStreams) *cobra.Command {
	o := &setOpenStackAuthURLOptions{
		Options: base.Options{
			IOStreams: ioStreams,
		},
	}
	cmd := &cobra.Command{
		Use:   "set-openstack-authurl",
		Short: "Configure allowed OpenStack auth URLs",
		Long: `Configure allowed OpenStack auth URLs for provider environment validation.

This command allows you to set one or more OpenStack auth URLs that will be allowed
when using the provider-env command. By default, setting new URIs will replace any
existing authURL patterns in the configuration.`,
		Example: `# Set single authURL (replaces existing)
gardenctl config set-openstack-authurl --uri-pattern https://keystone.example.com:5000/v3

# Set multiple authURLs (replaces existing)
gardenctl config set-openstack-authurl \
  --uri-pattern https://keystone.example.com:5000/v3 \
  --uri-pattern https://keystone.another.com/v3

# Clear all authURL patterns
gardenctl config set-openstack-authurl --clear`,
		RunE: base.WrapRunE(o, f),
	}

	o.AddFlags(cmd.Flags())

	return cmd
}

type setOpenStackAuthURLOptions struct {
	base.Options
	// Configuration is the gardenctl configuration
	Configuration *config.Config
	// URIPatterns is a list of OpenStack auth URLs to allow
	URIPatterns []string
	// Clear removes all existing authURL patterns
	Clear bool
}

// Complete adapts from the command line args to the data required.
func (o *setOpenStackAuthURLOptions) Complete(f util.Factory, _ *cobra.Command, _ []string) error {
	config, err := getConfiguration(f)
	if err != nil {
		return err
	}

	o.Configuration = config

	return nil
}

// Validate validates the provided options.
func (o *setOpenStackAuthURLOptions) Validate() error {
	if o.Clear && len(o.URIPatterns) > 0 {
		return errors.New("cannot specify --uri-pattern when --clear flag is set")
	}

	if !o.Clear && len(o.URIPatterns) == 0 {
		return errors.New("at least one --uri-pattern must be specified, or use --clear to remove all patterns")
	}

	// Validate each URI pattern
	ctx := credvalidate.GetOpenStackValidationContext()

	for i, uri := range o.URIPatterns {
		p := allowpattern.Pattern{
			Field: "authURL",
			URI:   uri,
		}
		if err := p.ValidateWithContext(ctx); err != nil {
			return fmt.Errorf("validation failed for URI at index %d: %w", i, err)
		}
	}

	return nil
}

// AddFlags adds flags to adjust the output to a cobra command.
func (o *setOpenStackAuthURLOptions) AddFlags(flags *pflag.FlagSet) {
	flags.StringArrayVar(&o.URIPatterns, "uri-pattern", nil,
		"OpenStack auth URL to allow. May be specified multiple times. "+
			"Setting URIs will replace any existing authURL patterns.")
	flags.BoolVar(&o.Clear, "clear", false,
		"Clear all OpenStack authURL patterns from the configuration")
}

// Run executes the command.
func (o *setOpenStackAuthURLOptions) Run(_ util.Factory) error {
	// Initialize provider config if needed
	if o.Configuration.Provider == nil {
		o.Configuration.Provider = &config.ProviderConfig{}
	}

	if o.Configuration.Provider.OpenStack == nil {
		o.Configuration.Provider.OpenStack = &config.OpenStackConfig{}
	}

	if o.Clear {
		o.Configuration.Provider.OpenStack.AllowedPatterns = nil
		fmt.Fprintln(o.IOStreams.Out, "Successfully cleared all OpenStack authURL patterns")
	} else {
		// Replace with new patterns
		patterns := make([]allowpattern.Pattern, len(o.URIPatterns))
		for i, uri := range o.URIPatterns {
			patterns[i] = allowpattern.Pattern{
				Field:          "authURL",
				URI:            uri,
				IsUserProvided: true,
			}
		}

		o.Configuration.Provider.OpenStack.AllowedPatterns = patterns
		fmt.Fprintf(o.IOStreams.Out, "Successfully configured %d OpenStack authURL pattern(s)\n", len(patterns))
	}

	if err := o.Configuration.Save(); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	return nil
}
