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

	"github.com/Masterminds/sprig"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
	"github.com/gardener/gardenctl-v2/pkg/cmd/cloudenv/garden"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/spf13/viper"
	v1 "k8s.io/api/core/v1"

	"github.com/spf13/pflag"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/spf13/cobra"
)

//go:embed templates
var fsys embed.FS

var (
	ErrNoShootTargeted               = errors.New("no shoot cluster targeted")
	ErrNeitherProjectNorSeedTargeted = errors.New("neither project nor seed is targeted")
	ErrProjectAndSeedTargeted        = errors.New("project and seed must not be targeted at the same time")
)

// NewCmdCloudEnv returns a new cloudenv command
func NewCmdCloudEnv(f util.Factory, ioStreams util.IOStreams) *cobra.Command {
	o := NewCloudEnvOptions(ioStreams)

	cmd := &cobra.Command{
		Use:     "configure-cloudprovider [bash | fish | powershell | zsh]",
		Short:   "Show the commands to configure cloudprovider CLI of the target cluster",
		Aliases: []string{"configure-cloud", "cloudprovider-env", "cloud-env"},
		RunE:    WrapRunE(f, o),
	}

	o.AddFlags(cmd.Flags())

	return cmd
}

// WrapRunE wraps the RunE function of a cobra.Command
func WrapRunE(f util.Factory, o CommandOptions) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if err := o.Complete(f, cmd, args); err != nil {
			return fmt.Errorf("failed to complete command options: %w", err)
		}

		if err := o.Validate(); err != nil {
			return err
		}

		return o.Run(f)
	}
}

// CommandOptions is the basic interface for command options
type CommandOptions interface {
	Complete(util.Factory, *cobra.Command, []string) error
	Validate() error
	Run(util.Factory) error
	AddFlags(*pflag.FlagSet)
}

// NewCloudEnvOptions returns a new options instance
func NewCloudEnvOptions(ioStreams util.IOStreams) *CloudEnvOptions {
	return &CloudEnvOptions{
		Options: base.Options{
			IOStreams: ioStreams,
		},
		Unset: false,
	}
}

// CloudEnvOptions is a struct to support the target cloudenv command
// nolint
type CloudEnvOptions struct {
	base.Options

	// Unset environment variables instead of setting them
	Unset bool
	// Shell to configure
	Shell string
	// GardenDir is the configuration directory of gardenctl
	GardenDir string
	// CmdPath is the path of the called command
	CmdPath []string
}

var _ CommandOptions = &CloudEnvOptions{}

// Complete adapts from the command line args to the data required.
func (o *CloudEnvOptions) Complete(f util.Factory, cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		o.Shell = strings.TrimSpace(args[0])
	} else if viper.IsSet("shell") {
		o.Shell = viper.GetString("shell")
	} else {
		o.Shell = DefaultShell(runtime.GOOS)
	}

	o.GardenDir = f.GardenHomeDir()

	o.CmdPath = strings.Fields(cmd.CommandPath())
	o.CmdPath[len(o.CmdPath)-1] = cmd.CalledAs()

	return nil
}

// Validate validates the provided command options
func (o *CloudEnvOptions) Validate() error {
	if o.Shell == "" {
		return errors.New("no shell configured or specified")
	}

	if err := ValidateShell(o.Shell); err != nil {
		return err
	}

	return nil
}

// Run does the actual work of the command.
func (o *CloudEnvOptions) Run(f util.Factory) error {
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

	// garden controller runtime client
	client, err := m.GardenClient(target)
	if err != nil {
		return fmt.Errorf("failed to create garden cluster client: %w", err)
	}

	// garden client
	gardenClient := garden.NewClient(client, gardenName)
	ctx := f.Context()

	var shoot *gardencorev1beta1.Shoot

	if projectName != "" {
		shoot, err = gardenClient.GetShootByProjectAndName(ctx, projectName, shootName)
		if err != nil {
			return err
		}
	} else if seedName != "" {
		shoot, err = gardenClient.FindShootBySeedAndName(ctx, seedName, shootName)
		if err != nil {
			return err
		}
	}

	secret, err := gardenClient.GetSecretBySecretBinding(ctx, shoot.Namespace, shoot.Spec.SecretBindingName)
	if err != nil {
		return err
	}

	return o.ExecuteTemplate(shoot, secret)
}

// AddFlags binds the command options to a given flagset
func (o *CloudEnvOptions) AddFlags(flags *pflag.FlagSet) {
	flags.BoolVarP(&o.Unset, "unset", "u", o.Unset, "Unset environment variables instead of setting them")
}

func (o *CloudEnvOptions) parseTmpl(name string) (*template.Template, error) {
	var tmpl *template.Template

	baseTmpl := template.New("base").Funcs(sprig.TxtFuncMap())
	filename := filepath.Join("templates", name+".tmpl")
	defaultTmpl, err := baseTmpl.ParseFS(fsys, filename)

	if err != nil {
		tmpl, err = baseTmpl.ParseFiles(filepath.Join(o.GardenDir, filename))
	} else {
		tmpl, err = defaultTmpl.ParseFiles(filepath.Join(o.GardenDir, filename))
		if err != nil {
			// use the embedded default template if it does not exist in the garden home dir
			if errors.Is(err, os.ErrNotExist) {
				tmpl, err = defaultTmpl, nil
			}
		}
	}

	return tmpl, err
}

func (o *CloudEnvOptions) ExecuteTemplate(shoot *gardencorev1beta1.Shoot, secret *v1.Secret) error {
	cloudprovider := shoot.Spec.Provider.Type

	t, err := o.parseTmpl(cloudprovider)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("parsing template for cloudprovider %q failed: %w", cloudprovider, err)
		}

		return fmt.Errorf("cloudprovider %q is not supported: %w", cloudprovider, err)
	}

	m := make(map[string]interface{})
	m["shell"] = o.Shell
	m["unset"] = o.Unset
	m["usageHint"] = o.GenerateUsageHint(cloudprovider, o.CmdPath...)
	m["region"] = shoot.Spec.Region

	for key, value := range secret.Data {
		m[key] = string(value)
	}

	switch cloudprovider {
	case "gcp":
		if err := ParseCredentials(&m); err != nil {
			return err
		}
	}

	return t.ExecuteTemplate(o.IOStreams.Out, o.Shell, m)
}

func (o *CloudEnvOptions) GenerateUsageHint(cloudprovider string, a ...string) string {
	var (
		cmd     string
		desc    string
		cmdLine string
		comment = "#"
		cli     = cloudprovider
	)

	switch cloudprovider {
	case "alicloud":
		cli = "aliyun"
	case "gcp":
		cli = "gcloud"
	}

	if o.Unset {
		desc = fmt.Sprintf("Run this command to reset the configuration of the %q CLI for your shell:", cli)
		cmdLine = strings.Join(append(a, "-u", o.Shell), " ")
	} else {
		desc = fmt.Sprintf("Run this command to configure the %q CLI for your shell:", cli)
		cmdLine = strings.Join(append(a, o.Shell), " ")
	}

	switch o.Shell {
	case "fish":
		cmd = fmt.Sprintf("eval (%s)", cmdLine)
	case "powershell":
		// Invoke-Expression cannot execute multi-line functions!!!
		cmd = fmt.Sprintf("& %s | Invoke-Expression", cmdLine)
	default:
		cmd = fmt.Sprintf("eval $(%s)", cmdLine)
	}

	return strings.Join([]string{
		comment + " " + desc,
		comment + " " + cmd,
	}, "\n")
}

// ParseCredentials attempts to parse GcpCredentials from a JSON string.
func ParseCredentials(values *map[string]interface{}) error {
	m := *values

	privateKey, ok := m["serviceaccount.json"].(string)
	if !ok {
		return errors.New("Invalid serviceaccount in secret")
	}

	credentials := map[string]interface{}{}
	if err := json.Unmarshal([]byte(privateKey), &credentials); err != nil {
		return err
	}

	m["credentials"] = credentials
	if data, err := json.Marshal(credentials); err == nil {
		m["serviceaccount.json"] = string(data)
	}

	return nil
}

var validShells = [...]string{"bash", "fish", "powershell", "zsh"}

// ValidateShell validates that the provided shell is supported
func ValidateShell(shell string) error {
	for _, s := range validShells {
		if s == shell {
			return nil
		}
	}

	return fmt.Errorf("invalid shell given, must be one of %v", validShells)
}

// DefaultShell detects user's current shell.
func DefaultShell(goos string) string {
	if shell, ok := os.LookupEnv("SHELL"); ok && shell != "" {
		return filepath.Base(shell)
	}

	switch goos {
	case "windows":
		return "powershell"
	default:
		return "bash"
	}
}
