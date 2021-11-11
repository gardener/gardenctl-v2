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

	sprigv3 "github.com/Masterminds/sprig/v3"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
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
	CmdPath []string
}

var _ base.CommandOptions = &cmdOptions{}

// NewCmdOptions returns a new CommandOptions instance for the cloudenv command
func NewCmdOptions(ioStreams util.IOStreams) base.CommandOptions {
	return &cmdOptions{
		Options: base.Options{
			IOStreams: ioStreams,
		},
		Unset: false,
	}
}

// Complete adapts from the command line args to the data required.
func (o *cmdOptions) Complete(f util.Factory, cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		o.Shell = strings.TrimSpace(args[0])
	} else if viper.IsSet("shell") {
		o.Shell = viper.GetString("shell")
	} else {
		o.Shell = detectShell(runtime.GOOS)
	}

	o.GardenDir = f.GardenHomeDir()

	o.CmdPath = strings.Fields(cmd.CommandPath())

	calledAs := cmd.CalledAs()
	if calledAs != "" {
		o.CmdPath[len(o.CmdPath)-1] = calledAs
	}

	return nil
}

// Validate validates the provided command options.
func (o *cmdOptions) Validate() error {
	if o.Shell == "" {
		return errors.New("no shell configured or specified")
	}

	if err := validateShell(o.Shell); err != nil {
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

	return o.execTmpl(shoot, secret)
}

// AddFlags binds the command options to a given flagset.
func (o *cmdOptions) AddFlags(flags *pflag.FlagSet) {
	flags.BoolVarP(&o.Unset, "unset", "u", o.Unset, "Reset environment variables and configuration of the cloudprovider CLI for your shell")
}

func (o *cmdOptions) parseTmpl(name string) (*template.Template, error) {
	var tmpl *template.Template

	baseTmpl := template.New("base").Funcs(sprigv3.TxtFuncMap())
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

func (o *cmdOptions) execTmpl(shoot *gardencorev1beta1.Shoot, secret *corev1.Secret) error {
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
	m["usageHint"] = generateUsageHint(cloudprovider, o.Shell, o.Unset, o.CmdPath...)
	m["region"] = shoot.Spec.Region

	for key, value := range secret.Data {
		m[key] = string(value)
	}

	switch cloudprovider {
	case "gcp":
		if err := parseCredentials(&m); err != nil {
			return err
		}
	}

	return t.ExecuteTemplate(o.IOStreams.Out, o.Shell, m)
}

func generateUsageHint(cloudprovider, shell string, unset bool, a ...string) string {
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

	if unset {
		desc = fmt.Sprintf("Run this command to reset the configuration of the %q CLI for your shell:", cli)
		cmdLine = strings.Join(append(a, "-u", shell), " ")
	} else {
		desc = fmt.Sprintf("Run this command to configure the %q CLI for your shell:", cli)
		cmdLine = strings.Join(append(a, shell), " ")
	}

	switch shell {
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

func parseCredentials(values *map[string]interface{}) error {
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

func validateShell(shell string) error {
	for _, s := range validShells {
		if s == shell {
			return nil
		}
	}

	return fmt.Errorf("invalid shell given, must be one of %v", validShells)
}

func detectShell(goos string) string {
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
