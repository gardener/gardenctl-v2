/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package providerenv

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"

	clientgarden "github.com/gardener/gardenctl-v2/internal/client/garden"
	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/ac"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
	"github.com/gardener/gardenctl-v2/pkg/config"
	"github.com/gardener/gardenctl-v2/pkg/env"
	"github.com/gardener/gardenctl-v2/pkg/provider/common/allowpattern"
	"github.com/gardener/gardenctl-v2/pkg/provider/credvalidate"
	"github.com/gardener/gardenctl-v2/pkg/target"
)

type options struct {
	base.Options

	// Unset resets environment variables and configuration of the cloudprovider CLI for your shell.
	Unset bool
	// Shell to configure.
	Shell string
	// GardenDir is the configuration directory of gardenctl.
	GardenDir string
	// SessionDir is the session directory of gardenctl.
	SessionDir string
	// CmdPath is the path of the called command.
	CmdPath string
	// Target is the target used when executing the command
	Target target.Target
	// TargetFlags are the target override flags
	TargetFlags target.TargetFlags
	// Template is the script template
	Template env.Template
	// Force generates the script even if there are access restrictions to be confirmed
	// Deprecated: Use ConfirmAccessRestriction instead
	Force bool
	// ConfirmAccessRestriction, when set to true, implies the user's understanding of the access restrictions for the targeted shoot.
	// When set to false and access restrictions are present, the command will terminate with an error.
	ConfirmAccessRestriction bool
	// GCPAllowedPatterns is a list of JSON-formatted allowed patterns for GCP credential config fields
	GCPAllowedPatterns []string
	// GCPAllowedURIPatterns is a list of simple field=uri patterns for GCP credential config fields
	GCPAllowedURIPatterns []string
	// OpenStackAllowedPatterns is a list of JSON-formatted allowed patterns for OpenStack credential config fields
	OpenStackAllowedPatterns []string
	// OpenStackAllowedURIPatterns is a list of simple field=uri patterns for OpenStack credential config fields
	OpenStackAllowedURIPatterns []string
	// MergedAllowedPatterns contains the merged allowed patterns for all providers from defaults, config, and flags
	MergedAllowedPatterns *MergedProviderPatterns
}

// MergedProviderPatterns contains merged allowed patterns for cloud providers that support pattern-based field validation.
// Currently, only GCP and OpenStack providers support allowed patterns.
type MergedProviderPatterns struct {
	// GCP contains the merged allowed patterns for GCP credential fields.
	// Supported fields include: universe_domain, token_uri, auth_uri, auth_provider_x509_cert_url,
	// client_x509_cert_url, token_url, service_account_impersonation_url, private_key_id,
	// client_id, client_email, audience, and others.
	GCP []allowpattern.Pattern
	// OpenStack contains the merged allowed patterns for OpenStack credential fields.
	// Currently, only the 'authURL' field is supported for pattern validation.
	OpenStack []allowpattern.Pattern
}

// Complete adapts from the command line args to the data required.
func (o *options) Complete(f util.Factory, cmd *cobra.Command, _ []string) error {
	ctx := f.Context()

	logger := klog.FromContext(ctx)

	if cmd.Name() != "provider-env" {
		o.Shell = cmd.Name()
	}

	o.CmdPath = cmd.Parent().CommandPath()
	o.GardenDir = f.GardenHomeDir()
	o.Template = env.NewTemplate("helpers")

	manager, err := f.Manager()
	if err != nil {
		return err
	}

	o.SessionDir = manager.SessionDir()
	o.TargetFlags = f.TargetFlags()

	if o.Force {
		o.ConfirmAccessRestriction = true

		logger.Info("The --force flag is deprecated and will be removed in a future gardenctl version. Please use the --confirm-access-restriction flag instead.")
	}

	cfg := manager.Configuration()

	gcpPatterns, err := o.processGCPPatterns(cfg, logger)
	if err != nil {
		return err
	}

	openstackPatterns, err := o.processOpenStackPatterns(cfg, logger)
	if err != nil {
		return err
	}

	o.MergedAllowedPatterns = &MergedProviderPatterns{
		GCP:       gcpPatterns,
		OpenStack: openstackPatterns,
	}

	return nil
}

// processProviderPatterns is a generic function to process and merge allowed patterns for any provider.
func processProviderPatterns(
	logger klog.Logger,
	providerName string,
	defaultPatterns []allowpattern.Pattern,
	configPatterns []allowpattern.Pattern,
	validationContext *allowpattern.ValidationContext,
	jsonPatterns []string,
	uriPatterns []string,
) ([]allowpattern.Pattern, error) {
	flagPatterns, err := allowpattern.ParseAllowedPatterns(validationContext, jsonPatterns, uriPatterns)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s flag allowed patterns: %w", providerName, err)
	}

	if len(configPatterns) > 0 {
		logger.Info(fmt.Sprintf("Using custom %s allowed patterns from configuration", providerName))
		logger.V(6).Info("allowed patterns from configuration", "provider", providerName, "patterns", configPatterns)
	}

	if len(flagPatterns) > 0 {
		logger.Info(fmt.Sprintf("Using custom %s allowed patterns from flags", providerName))
		logger.V(6).Info("allowed patterns from flags", "provider", providerName, "patterns", flagPatterns)
	}

	var mergedPatterns []allowpattern.Pattern

	mergedPatterns = append(mergedPatterns, defaultPatterns...)
	mergedPatterns = append(mergedPatterns, configPatterns...)
	mergedPatterns = append(mergedPatterns, flagPatterns...)

	var normalizedPatterns []allowpattern.Pattern

	for _, pattern := range mergedPatterns {
		if err := pattern.ValidateWithContext(validationContext); err != nil {
			return nil, fmt.Errorf("invalid %s allowed pattern: %w", providerName, err)
		}

		normalized, err := pattern.ToNormalizedPattern()
		if err != nil {
			return nil, fmt.Errorf("failed to normalize %s pattern: %w", providerName, err)
		}

		normalizedPatterns = append(normalizedPatterns, *normalized)
	}

	if len(normalizedPatterns) > 0 {
		logger.V(6).Info("final normalized allowed patterns", "provider", providerName, "count", len(normalizedPatterns), "patterns", normalizedPatterns)
	}

	return normalizedPatterns, nil
}

// processGCPPatterns processes and merges GCP allowed patterns from defaults, config, and flags.
func (o *options) processGCPPatterns(cfg *config.Config, logger klog.Logger) ([]allowpattern.Pattern, error) {
	var configPatterns []allowpattern.Pattern
	if cfg != nil && cfg.Provider != nil && cfg.Provider.GCP != nil {
		configPatterns = cfg.Provider.GCP.AllowedPatterns
	}

	return processProviderPatterns(
		logger,
		"GCP",
		credvalidate.DefaultGCPAllowedPatterns(),
		configPatterns,
		credvalidate.GetGCPValidationContext(),
		o.GCPAllowedPatterns,
		o.GCPAllowedURIPatterns,
	)
}

// processOpenStackPatterns processes and merges OpenStack allowed patterns from defaults, config, and flags.
// Note: Only the 'authURL' field is supported for OpenStack pattern validation.
func (o *options) processOpenStackPatterns(cfg *config.Config, logger klog.Logger) ([]allowpattern.Pattern, error) {
	var configPatterns []allowpattern.Pattern
	if cfg != nil && cfg.Provider != nil && cfg.Provider.OpenStack != nil {
		configPatterns = cfg.Provider.OpenStack.AllowedPatterns
	}

	return processProviderPatterns(
		logger,
		"OpenStack",
		credvalidate.DefaultOpenStackAllowedPatterns(),
		configPatterns,
		credvalidate.GetOpenStackValidationContext(),
		o.OpenStackAllowedPatterns,
		o.OpenStackAllowedURIPatterns,
	)
}

// Validate validates the provided command options.
func (o *options) Validate() error {
	if o.Shell == "" && o.Output == "" {
		return pflag.ErrHelp
	}

	// Usually, we would check and return an error if both shell and output are set (not empty). However, this is not required because the output flag is not set for the shell subcommands.

	if o.Shell != "" {
		s := env.Shell(o.Shell)

		return s.Validate()
	}

	return o.Options.Validate()
}

// AddFlags binds the command options to a given flagset.
func (o *options) AddFlags(flags *pflag.FlagSet) {
	flags.BoolVarP(&o.Force, "force", "f", false, "Deprecated. Use --confirm-access-restriction instead. Generate the script even if there are access restrictions to be confirmed.")
	flags.BoolVarP(&o.ConfirmAccessRestriction, "confirm-access-restriction", "y", o.ConfirmAccessRestriction, "Confirm any access restrictions. Set this flag only if you are completely aware of the access restrictions.")
	flags.BoolVarP(&o.Unset, "unset", "u", o.Unset, fmt.Sprintf("Generate the script to unset the cloud provider CLI environment variables and logout for %s", o.Shell))
	flags.StringSliceVar(&o.GCPAllowedPatterns, "gcp-allowed-patterns", nil,
		`Additional allowed patterns for GCP credential fields in JSON format.
Supported fields include: universe_domain, token_uri, auth_uri, auth_provider_x509_cert_url, client_x509_cert_url,
token_url, service_account_impersonation_url, private_key_id, client_id, client_email, audience.
Each pattern should be a JSON object with fields like:
{"field": "universe_domain", "host": "example.com"}
{"field": "token_uri", "host": "example.com", "path": "/token"}
{"field": "service_account_impersonation_url", "host": "iamcredentials.googleapis.com", "regexPath": "^/v1/projects/-/serviceAccounts/[^/:]+:generateAccessToken$"}
{"field": "client_id", "regexValue": "^[0-9]{15,25}$"}
These are merged with defaults and configuration.`)

	flags.StringSliceVar(&o.GCPAllowedURIPatterns, "gcp-allowed-uri-patterns", nil,
		`Simplified URI patterns for GCP credential fields in the format 'field=uri'.
For example:
"token_uri=https://example.com/token"
"client_x509_cert_url=https://example.com/{client_email}"
The URI is parsed and host and path are set accordingly. These are merged with defaults and configuration.`)

	flags.StringSliceVar(&o.OpenStackAllowedPatterns, "openstack-allowed-patterns", nil,
		`Additional allowed patterns for OpenStack credential fields in JSON format.
Note: Only the 'authURL' field is supported for OpenStack pattern validation.
Each pattern should be a JSON object with fields like:
{"field": "authURL", "host": "keystone.example.com"}
{"field": "authURL", "host": "keystone.example.com", "path": "/v3"}
{"field": "authURL", "regexValue": "^https://[a-z0-9.-]+\\.example\\.com(:[0-9]+)?/.*$"}
These are merged with defaults and configuration.`)

	flags.StringSliceVar(&o.OpenStackAllowedURIPatterns, "openstack-allowed-uri-patterns", nil,
		`Simplified URI patterns for OpenStack credential fields in the format 'field=uri'.
Note: Only the 'authURL' field is supported for OpenStack pattern validation.
For example:
"authURL=https://keystone.example.com:5000/v3"
"authURL=https://keystone.example.com/identity/v3"
The URI is parsed and host and path are set accordingly. These are merged with defaults and configuration.`)
}

// Run does the actual work of the command.
func (o *options) Run(f util.Factory) error {
	ctx := f.Context()

	logger := klog.FromContext(ctx)

	manager, err := f.Manager()
	if err != nil {
		return err
	}

	o.Target, err = manager.CurrentTarget()
	if err != nil {
		return err
	}

	if o.Target.GardenName() == "" {
		return target.ErrNoGardenTargeted
	}

	client, err := manager.GardenClient(o.Target.GardenName())
	if err != nil {
		return fmt.Errorf("failed to create garden cluster client: %w", err)
	}

	if o.Target.ShootName() == "" && o.Target.SeedName() != "" {
		shoot, err := client.GetShootOfManagedSeed(ctx, o.Target.SeedName())
		if err != nil {
			if apierrors.IsNotFound(err) {
				return fmt.Errorf("cannot generate cloud provider CLI configuration script for non-managed seeds: %w", err)
			}

			return err
		}

		logger.V(1).Info("using referred shoot of managed seed",
			"shoot", klog.ObjectRef{
				Namespace: "garden",
				Name:      shoot.Name,
			},
			"seed", o.Target.SeedName())

		o.Target = o.Target.WithProjectName("garden").WithShootName(shoot.Name)
	}

	if o.Target.ShootName() == "" {
		return target.ErrNoShootTargeted
	}

	shoot, err := client.FindShoot(ctx, o.Target.AsListOption())
	if err != nil {
		return err
	}

	if (shoot.Spec.SecretBindingName == nil || *shoot.Spec.SecretBindingName == "") &&
		(shoot.Spec.CredentialsBindingName == nil || *shoot.Spec.CredentialsBindingName == "") {
		return fmt.Errorf("shoot %q is not bound to a cloud provider credential", o.Target.ShootName())
	}

	var credentialsRef corev1.ObjectReference

	if shoot.Spec.SecretBindingName != nil && *shoot.Spec.SecretBindingName != "" {
		secretBinding, err := client.GetSecretBinding(ctx, shoot.Namespace, *shoot.Spec.SecretBindingName)
		if err != nil {
			return err
		}

		credentialsRef = corev1.ObjectReference{
			APIVersion: "v1",
			Kind:       "Secret",
			Namespace:  secretBinding.SecretRef.Namespace,
			Name:       secretBinding.SecretRef.Name,
		}
	} else {
		credentialsBinding, err := client.GetCredentialsBinding(ctx, shoot.Namespace, *shoot.Spec.CredentialsBindingName)
		if err != nil {
			return err
		}

		credentialsRef = credentialsBinding.CredentialsRef
	}

	if shoot.Spec.CloudProfile == nil {
		return fmt.Errorf("shoot %q does not reference a cloud profile", o.Target.ShootName())
	}

	cloudProfile, err := client.GetCloudProfile(ctx, *shoot.Spec.CloudProfile)
	if err != nil {
		return err
	}

	// check access restrictions
	messages, err := o.checkAccessRestrictions(manager.Configuration(), o.Target.GardenName(), shoot)
	if err != nil {
		return err
	}

	aborted, err := maybeAbortDueToAccessRestrictions(o, messages)
	if err != nil {
		return err
	}

	if aborted {
		return nil
	}

	return printProviderEnv(o, ctx, client, shoot, credentialsRef, cloudProfile, messages)
}

func printProviderEnv(
	o *options,
	ctx context.Context,
	c clientgarden.Client,
	shoot *gardencorev1beta1.Shoot,
	credentialsRef corev1.ObjectReference,
	cloudProfile *clientgarden.CloudProfileUnion,
	messages ac.AccessRestrictionMessages,
) error {
	providerType := shoot.Spec.Provider.Type

	if err := ValidateProviderType(providerType); err != nil {
		return fmt.Errorf("invalid provider type: %w", err)
	}

	cli := getProviderCLI(providerType)

	if err := ValidateCLIName(cli); err != nil {
		return fmt.Errorf("invalid cli name: %w", err)
	}

	metadata := generateMetadata(o, cli, credentialsRef.Kind)

	if len(messages) > 0 {
		metadata["notification"] = messages.String()
	}

	data, err := generateData(o, ctx, c, shoot, credentialsRef, cloudProfile, providerType, cli, metadata)
	if err != nil {
		return err
	}

	if o.Output != "" {
		return o.PrintObject(data)
	}

	var filename string

	if providerType == "stackit" {
		// TODO(maboehm): add full support once the provider extension is open sourced.
		filename = filepath.Join(o.GardenDir, "templates", "openstack.tmpl")
	} else {
		filename = filepath.Join(o.GardenDir, "templates", providerType+".tmpl")
	}

	if err := o.Template.ParseFiles(filename); err != nil {
		return fmt.Errorf("failed to generate the cloud provider CLI configuration script: %w", err)
	}

	return o.Template.ExecuteTemplate(o.IOStreams.Out, o.Shell, data)
}

func generateData(
	o *options,
	ctx context.Context,
	c clientgarden.Client,
	shoot *gardencorev1beta1.Shoot,
	credentialsRef corev1.ObjectReference,
	cloudProfile *clientgarden.CloudProfileUnion,
	providerType string,
	cli string,
	metadata map[string]interface{},
) (map[string]interface{}, error) {
	configDir := filepath.Join(o.SessionDir, ".config", cli)
	if !o.Unset {
		if err := os.MkdirAll(configDir, 0o700); err != nil {
			return nil, fmt.Errorf("failed to create %s configuration directory: %w", cli, err)
		}
	}

	p := providerFor(providerType, ctx, o.MergedAllowedPatterns)

	data := make(map[string]interface{})

	switch credentialsRef.GroupVersionKind() {
	case corev1.SchemeGroupVersion.WithKind("Secret"):
		secret, err := c.GetSecret(ctx, credentialsRef.Namespace, credentialsRef.Name)
		if err != nil {
			return nil, err
		}

		for key, value := range secret.Data {
			data[key] = string(value)
		}

		if p != nil {
			data, err = p.FromSecret(o, shoot, secret, cloudProfile, configDir)
			if err != nil {
				return nil, err
			}
		}

	default:
		return nil, fmt.Errorf("unsupported credentials kind %q", credentialsRef.Kind)
	}

	// baseline reserved fields that any template can use
	data["__meta"] = metadata
	data["region"] = shoot.Spec.Region
	data["configDir"] = configDir

	return data, nil
}

func generateMetadata(o *options, cli string, credentialKind string) map[string]interface{} {
	metadata := make(map[string]interface{})
	metadata["unset"] = o.Unset
	metadata["commandPath"] = o.CmdPath
	metadata["cli"] = cli
	metadata["targetFlags"] = getTargetFlags(o.Target)
	metadata["credentialKind"] = credentialKind

	if o.Shell != "" {
		metadata["shell"] = o.Shell
		metadata["prompt"] = env.Shell(o.Shell).Prompt(runtime.GOOS)
	}

	return metadata
}

func getProviderCLI(providerType string) string {
	switch providerType {
	case "alicloud":
		return "aliyun"
	case "gcp":
		return "gcloud"
	case "azure":
		return "az"
	default:
		return providerType
	}
}

func getTargetFlags(t target.Target) string {
	if t.ProjectName() != "" {
		return fmt.Sprintf("--garden %s --project %s --shoot %s", t.GardenName(), t.ProjectName(), t.ShootName())
	}

	return fmt.Sprintf("--garden %s --seed %s --shoot %s", t.GardenName(), t.SeedName(), t.ShootName())
}

// maybeAbortDueToAccessRestrictions processes access restriction messages and updates metadata or outputs
// a confirmation prompt. It returns (aborted=true) when it has already written output and the caller
// should stop further processing. When no messages or when only metadata is updated, it returns aborted=false.
func maybeAbortDueToAccessRestrictions(o *options, messages ac.AccessRestrictionMessages) (bool, error) {
	if len(messages) == 0 {
		return false, nil
	}

	if o.TargetFlags.ShootName() == "" || o.ConfirmAccessRestriction {
		return false, nil
	}

	if o.Output != "" {
		return false, errors.New(
			"the cloud provider CLI configuration script can only be generated if you confirm the access despite the existing restrictions. Use the --confirm-access-restriction flag to confirm the access",
		)
	}

	s := env.Shell(o.Shell)

	if err := o.Template.ExecuteTemplate(o.IOStreams.Out, "printf", map[string]interface{}{
		"format": messages.String() + "\n%s %s\n%s\n",
		"arguments": []string{
			"The cloud provider CLI configuration script can only be generated if you confirm the access despite the existing restrictions.",
			"Use the --confirm-access-restriction flag to confirm the access.",
			s.Prompt(runtime.GOOS) + s.EvalCommand(fmt.Sprintf("%s --confirm-access-restriction %s", o.CmdPath, o.Shell)),
		},
	}); err != nil {
		return false, err
	}

	return true, nil
}

func getKeyStoneURL(cloudProfile *clientgarden.CloudProfileUnion, region string) (string, error) {
	config, err := cloudProfile.GetOpenstackProviderConfig()
	if err != nil {
		return "", fmt.Errorf("failed to get openstack provider config: %w", err)
	}

	for _, keyStoneURL := range config.KeyStoneURLs {
		if keyStoneURL.Region == region {
			return keyStoneURL.URL, nil
		}
	}

	if config.KeyStoneURL != "" {
		return config.KeyStoneURL, nil
	}

	return "", fmt.Errorf("cannot find keystone URL for region %q in cloudprofile %q", region, cloudProfile.GetObjectMeta().Name)
}

func (o *options) checkAccessRestrictions(cfg *config.Config, gardenName string, shoot *gardencorev1beta1.Shoot) (ac.AccessRestrictionMessages, error) {
	if cfg == nil {
		return nil, errors.New("garden configuration is required")
	}

	garden, err := cfg.Garden(gardenName)
	if err != nil {
		return nil, err
	}

	messages := ac.CheckAccessRestrictions(garden.AccessRestrictions, shoot)

	return messages, nil
}
