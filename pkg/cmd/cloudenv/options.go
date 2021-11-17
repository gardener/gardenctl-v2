/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package cloudenv

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"

	openstackv1alpha1 "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/v1alpha1"

	sprigv3 "github.com/Masterminds/sprig/v3"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

//go:embed templates
var fsys embed.FS

var (
	ErrNoShootTargeted               = errors.New("no shoot cluster targeted")
	ErrNeitherProjectNorSeedTargeted = errors.New("neither project nor seed is targeted")
	ErrProjectAndSeedTargeted        = errors.New("project and seed must not be targeted at the same time")
)

type cmdOptions struct {
	base.Options

	// Unset resets environment variables and configuration of the cloudprovider CLI for your shell.
	Unset bool
	// Shell to configure.
	Shell string
	// GardenDir is the configuration directory of gardenctl.
	GardenDir string
	// CmdPath is the path of the called command.
	CmdPath string
}

var _ base.CommandOptions = &cmdOptions{}

// Complete adapts from the command line args to the data required.
func (o *cmdOptions) Complete(f util.Factory, cmd *cobra.Command, _ []string) error {
	o.GardenDir = f.GardenHomeDir()
	o.Shell = cmd.Name()
	o.CmdPath = cmd.Parent().CommandPath()

	return nil
}

// Validate validates the provided command options.
func (o *cmdOptions) Validate() error {
	if o.Shell == "" {
		return pflag.ErrHelp
	}

	s := Shell(o.Shell)
	if err := s.Validate(); err != nil {
		return err
	}

	return nil
}

// Run does the actual work of the command.
func (o *cmdOptions) Run(f util.Factory) error {
	// target manager
	m, err := f.Manager()
	if err != nil {
		return err
	}

	// current target
	target, err := m.CurrentTarget()
	if err != nil {
		return err
	}

	gardenName := target.GardenName()
	projectName := target.ProjectName()
	seedName := target.SeedName()
	shootName := target.ShootName()

	// validate current target
	if shootName == "" {
		return ErrNoShootTargeted
	} else if projectName == "" && seedName == "" {
		return ErrNeitherProjectNorSeedTargeted
	} else if projectName != "" && seedName != "" {
		return ErrProjectAndSeedTargeted
	}

	// garden client
	gardenClient, err := m.GardenClient(gardenName)
	if err != nil {
		return fmt.Errorf("failed to create garden cluster client: %w", err)
	}

	ctx := f.Context()

	var shoot *gardencorev1beta1.Shoot

	if projectName != "" {
		shoot, err = gardenClient.GetShootByProject(ctx, projectName, shootName)
		if err != nil {
			return err
		}
	} else {
		shoot, err = gardenClient.GetShootBySeed(ctx, seedName, shootName)
		if err != nil {
			return err
		}
	}

	secretBinding, err := gardenClient.GetSecretBinding(ctx, shoot.Namespace, shoot.Spec.SecretBindingName)
	if err != nil {
		return err
	}

	secret, err := gardenClient.GetSecret(ctx, secretBinding.SecretRef.Namespace, secretBinding.SecretRef.Name)
	if err != nil {
		return err
	}

	cloudProfile, err := gardenClient.GetCloudProfile(ctx, shoot.Spec.CloudProfileName)
	if err != nil {
		return err
	}

	return o.execTmpl(shoot, secret, cloudProfile)
}

// AddFlags binds the command options to a given flagset.
func (o *cmdOptions) AddFlags(flags *pflag.FlagSet) {
	usage := fmt.Sprintf("Generate the script to unset environment variables and logout the account of the cloud provider CLI for %s", o.Shell)
	flags.BoolVarP(&o.Unset, "unset", "u", o.Unset, usage)
}

func (o *cmdOptions) execTmpl(shoot *gardencorev1beta1.Shoot, secret *corev1.Secret, cloudProfile *gardencorev1beta1.CloudProfile) error {
	c := CloudProvider(shoot.Spec.Provider.Type)

	t, err := parseTemplate(c, o.GardenDir)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("parsing template for cloud provider %q failed: %w", c, err)
		}

		return fmt.Errorf("cloud provider %q is not supported: %w", c, err)
	}

	m := make(map[string]interface{})
	m["shell"] = o.Shell
	m["unset"] = o.Unset
	m["usageHint"] = o.generateUsageHint(c)
	m["region"] = shoot.Spec.Region
	m["cloudProfileName"] = shoot.Spec.CloudProfileName
	m["cloudProfileConfig"] = cloudProfile.Spec.ProviderConfig.Object

	for key, value := range secret.Data {
		m[key] = string(value)
	}

	if err := beforeExecuteTemplate(c, &m); err != nil {
		return err
	}

	return t.ExecuteTemplate(o.IOStreams.Out, o.Shell, m)
}

// generateUsageHint generate a cloud and shell specific usage hint
func (o *cmdOptions) generateUsageHint(c CloudProvider) string {
	cmd := o.CmdPath
	action := "configure"

	if o.Unset {
		cmd += " -u"
		action = "reset the configuration of"
	}

	cmd += " " + o.Shell
	s := Shell(o.Shell)
	prefix := s.Comment() + " "

	return strings.Join([]string{
		prefix + fmt.Sprintf("Run this command to %s the %q CLI for your shell:", action, c.CLI()),
		prefix + s.EvalCommand(cmd),
	}, "\n")
}

// parseTemplate returns the parsed template found whether in the embedded filesystem or in the given directory
func parseTemplate(c CloudProvider, dir string) (*template.Template, error) {
	var tmpl *template.Template

	baseTmpl := template.New("base").Funcs(sprigv3.TxtFuncMap())
	filename := filepath.Join("templates", string(c)+".tmpl")
	defaultTmpl, err := baseTmpl.ParseFS(fsys, filename)

	if err != nil {
		tmpl, err = baseTmpl.ParseFiles(filepath.Join(dir, filename))
	} else {
		tmpl, err = defaultTmpl.ParseFiles(filepath.Join(dir, filename))
		if err != nil {
			// use the embedded default template if it does not exist in the garden home dir
			if errors.Is(err, os.ErrNotExist) {
				tmpl, err = defaultTmpl, nil
			}
		}
	}

	return tmpl, err
}

// beforeExecuteTemplate allows modifying or enhancing the data before the template is executed
func beforeExecuteTemplate(c CloudProvider, mPtr *map[string]interface{}) error {
	m := *mPtr

	switch c {
	case gcp:
		privateKey, ok := m["serviceaccount.json"].(string)
		if !ok {
			return errors.New("Invalid serviceaccount in Secret")
		}

		credentials := map[string]interface{}{}
		if err := json.Unmarshal([]byte(privateKey), &credentials); err != nil {
			return err
		}

		m["credentials"] = credentials
		if data, err := json.Marshal(credentials); err == nil {
			m["serviceaccount.json"] = string(data)
		}
	case openstack:
		cloudProfileConfig, ok := m["cloudProfileConfig"].(*openstackv1alpha1.CloudProfileConfig)
		if !ok {
			return errors.New("Invalid providerConfig in CloudProfile")
		}

		region := m["region"].(string)

		authURL, err := getKeyStoneURL(cloudProfileConfig, region)
		if err != nil {
			return err
		}

		m["authURL"] = authURL
	}

	return nil
}

// Shell represents the type of shell
type Shell string

const (
	bash       Shell = "bash"
	zsh        Shell = "zsh"
	fish       Shell = "fish"
	powershell Shell = "powershell"
)

var validShells = []Shell{bash, zsh, fish, powershell}

// Comment returns the character use for comments
func (s Shell) Comment() string {
	return "#"
}

// EvalCommand returns the script that evaluates the given command
func (s Shell) EvalCommand(cmd string) string {
	var format string

	switch s {
	case fish:
		format = "eval (%s)"
	case powershell:
		// Invoke-Expression cannot execute multi-line functions!!!
		format = "& %s | Invoke-Expression"
	default:
		format = "eval $(%s)"
	}

	return fmt.Sprintf(format, cmd)
}

// Prompt returns the typical prompt for a given os
func (s Shell) Prompt(goos string) string {
	switch s {
	case powershell:
		if goos == "windows" {
			return "PS C:\\> "
		}

		return "PS /> "
	default:
		return "$ "
	}
}

// AddCommand adds a shell sub command to a parent
func (s Shell) AddCommand(parent *cobra.Command, runE func(cmd *cobra.Command, args []string) error) {
	shortFormat := "generate the cloud provider CLI configuration script for %s"
	longFormat := `Generate the cloud provider CLI configuration script for %s.

To load the cloud provider CLI configuration script in your current shell session:
%s
`
	cmdWithPrompt := s.EvalCommand(fmt.Sprintf("%s%s %s", s.Prompt(runtime.GOOS), parent.CommandPath(), s))
	shell := string(s)
	cmd := &cobra.Command{
		Use:   shell,
		Short: fmt.Sprintf(shortFormat, shell),
		Long:  fmt.Sprintf(longFormat, shell, cmdWithPrompt),
		RunE:  runE,
	}
	parent.AddCommand(cmd)
}

// Validate checks if the shell is valid
func (s Shell) Validate() error {
	for _, shell := range validShells {
		if s == shell {
			return nil
		}
	}

	return fmt.Errorf("invalid shell given, must be one of %v", validShells)
}

// CloudProvider represent the type of cloud provider
type CloudProvider string

const (
	alicloud  CloudProvider = "alicloud"
	gcp       CloudProvider = "gcp"
	openstack CloudProvider = "openstack"
)

// CLI returns the CLI for the cloud provider
func (c CloudProvider) CLI() string {
	switch c {
	case alicloud:
		return "aliyun"
	case gcp:
		return "gcloud"
	default:
		return string(c)
	}
}

func getKeyStoneURL(config *openstackv1alpha1.CloudProfileConfig, region string) (string, error) {
	if config.KeyStoneURL != "" {
		return config.KeyStoneURL, nil
	}

	for _, keyStoneURL := range config.KeyStoneURLs {
		if keyStoneURL.Region == region {
			return keyStoneURL.URL, nil
		}
	}

	return "", fmt.Errorf("cannot find KeyStone URL for region %q in CloudProfileConfig", region)
}
