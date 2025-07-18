/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package providerenv

import (
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
	// GCPAllowedPatterns is a list of allowed patterns for GCP service account fields.
	// Each entry is a key-value pair where the key matches a credential config field
	// (e.g., "universe_domain=googleapis.com", "token_uri=https://oauth2.googleapis.com/token").
	GCPAllowedPatterns []string
	// MergedGCPAllowedPatterns is the merged allowed patterns for GCP from defaults, config, and flags.
	MergedGCPAllowedPatterns map[string][]string
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

	defaultMap := defaultAllowedPatterns()

	var configPatterns []string
	if cfg := manager.Configuration(); cfg != nil && cfg.Provider != nil && cfg.Provider.GCP != nil {
		configPatterns = cfg.Provider.GCP.AllowedPatterns
	}

	configMap, err := parseAllowedPatterns(configPatterns)
	if err != nil {
		return fmt.Errorf("failed to parse config allowed patterns: %w", err)
	}

	flagMap, err := parseAllowedPatterns(o.GCPAllowedPatterns)
	if err != nil {
		return fmt.Errorf("failed to parse flag allowed patterns: %w", err)
	}

	mergedMap := make(map[string][]string)
	for field, values := range defaultMap {
		mergedMap[field] = append(mergedMap[field], values...)
	}

	for field, values := range configMap {
		mergedMap[field] = append(mergedMap[field], values...)
	}

	for field, values := range flagMap {
		mergedMap[field] = append(mergedMap[field], values...)
	}

	o.MergedGCPAllowedPatterns = mergedMap

	return nil
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
	flags.StringSliceVar(&o.GCPAllowedPatterns, "gcp-allowed-patterns", nil, "Additional allowed patterns for GCP service account fields, in the format 'field=value', e.g., 'universe_domain=googleapis.com'. These are merged with defaults and configuration.")
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

	var (
		secretName      string
		secretNamespace string
	)

	if shoot.Spec.SecretBindingName != nil && *shoot.Spec.SecretBindingName != "" {
		secretBinding, err := client.GetSecretBinding(ctx, shoot.Namespace, *shoot.Spec.SecretBindingName)
		if err != nil {
			return err
		}

		secretName = secretBinding.SecretRef.Name
		secretNamespace = secretBinding.SecretRef.Namespace
	} else {
		// TODO: This code should eventually support credentials of type workload identity
		credentialsBinding, err := client.GetCredentialsBinding(ctx, shoot.Namespace, *shoot.Spec.CredentialsBindingName)
		if err != nil {
			return err
		}

		secretName = credentialsBinding.CredentialsRef.Name
		secretNamespace = credentialsBinding.CredentialsRef.Namespace
	}

	secret, err := client.GetSecret(ctx, secretNamespace, secretName)
	if err != nil {
		return err
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

	return printProviderEnv(o, shoot, secret, cloudProfile, messages)
}

func printProviderEnv(o *options, shoot *gardencorev1beta1.Shoot, secret *corev1.Secret, cloudProfile *clientgarden.CloudProfileUnion, messages ac.AccessRestrictionMessages) error {
	providerType := shoot.Spec.Provider.Type
	cli := getProviderCLI(providerType)

	metadata := generateMetadata(o, cli)

	if len(messages) > 0 {
		if o.TargetFlags.ShootName() == "" || o.ConfirmAccessRestriction {
			metadata["notification"] = messages.String()
		} else {
			if o.Output != "" {
				return errors.New(
					"the cloud provider CLI configuration script can only be generated if you confirm the access despite the existing restrictions. Use the --confirm-access-restriction flag to confirm the access",
				)
			}

			s := env.Shell(o.Shell)

			return o.Template.ExecuteTemplate(o.IOStreams.Out, "printf", map[string]interface{}{
				"format": messages.String() + "\n%s %s\n%s\n",
				"arguments": []string{
					"The cloud provider CLI configuration script can only be generated if you confirm the access despite the existing restrictions.",
					"Use the --confirm-access-restriction flag to confirm the access.",
					s.Prompt(runtime.GOOS) + s.EvalCommand(fmt.Sprintf("%s --confirm-access-restriction %s", o.CmdPath, o.Shell)),
				},
			})
		}
	}

	data, err := generateData(o, shoot, secret, cloudProfile, providerType, metadata)
	if err != nil {
		return err
	}

	if o.Output != "" {
		return o.PrintObject(data)
	}

	return o.Template.ExecuteTemplate(o.IOStreams.Out, o.Shell, data)
}

func generateData(o *options, shoot *gardencorev1beta1.Shoot, secret *corev1.Secret, cloudProfile *clientgarden.CloudProfileUnion, providerType string, metadata map[string]interface{}) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"__meta": metadata,
		"region": shoot.Spec.Region,
	}

	for key, value := range secret.Data {
		data[key] = string(value)
	}

	switch providerType {
	case "azure":
		if !o.Unset {
			configDir, err := createProviderConfigDir(o.SessionDir, providerType)
			if err != nil {
				return nil, err
			}

			data["configDir"] = configDir
		}
	case "gcp":
		credentials := make(map[string]interface{})

		serviceaccountJSON, err := validateAndParseGCPServiceAccount(secret, &credentials, o.MergedGCPAllowedPatterns)
		if err != nil {
			return nil, err
		}

		if !o.Unset {
			configDir, err := createProviderConfigDir(o.SessionDir, providerType)
			if err != nil {
				return nil, err
			}

			data["configDir"] = configDir
		}

		data["credentials"] = credentials
		data["serviceaccount.json"] = string(serviceaccountJSON)
		data["allowedPatterns"] = o.MergedGCPAllowedPatterns
	case "openstack":
		authURL, err := getKeyStoneURL(cloudProfile, shoot.Spec.Region)
		if err != nil {
			return nil, err
		}

		data["authURL"] = authURL

		_, ok := data["applicationCredentialSecret"]
		if ok {
			data["authType"] = "v3applicationcredential"
			data["authStrategy"] = ""
			data["tenantName"] = ""
			data["username"] = ""
			data["password"] = ""
		} else {
			data["authStrategy"] = "keystone"
			data["authType"] = ""
			data["applicationCredentialID"] = ""
			data["applicationCredentialName"] = ""
			data["applicationCredentialSecret"] = ""
		}
	}

	filename := filepath.Join(o.GardenDir, "templates", providerType+".tmpl")
	if err := o.Template.ParseFiles(filename); err != nil {
		return nil, fmt.Errorf("failed to generate the cloud provider CLI configuration script: %w", err)
	}

	return data, nil
}

func generateMetadata(o *options, cli string) map[string]interface{} {
	metadata := make(map[string]interface{})
	metadata["unset"] = o.Unset
	metadata["commandPath"] = o.CmdPath
	metadata["cli"] = cli
	metadata["targetFlags"] = getTargetFlags(o.Target)

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

func createProviderConfigDir(sessionDir string, providerType string) (string, error) {
	cli := getProviderCLI(providerType)
	configDir := filepath.Join(sessionDir, ".config", cli)

	err := os.MkdirAll(configDir, 0o700)
	if err != nil {
		return "", fmt.Errorf("failed to create %s configuration directory: %w", cli, err)
	}

	return configDir, nil
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
