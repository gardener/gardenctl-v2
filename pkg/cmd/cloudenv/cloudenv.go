/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package cloudenv

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig"
	gardencore "github.com/gardener/gardener/pkg/apis/core"

	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
	"github.com/spf13/viper"

	v1 "k8s.io/api/core/v1"

	"github.com/gardener/gardenctl-v2/internal/util"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/types"
	controllerruntime "sigs.k8s.io/controller-runtime/pkg/client"
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
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Complete(f, cmd, args); err != nil {
				return fmt.Errorf("failed to complete command options: %w", err)
			}
			if err := o.Validate(); err != nil {
				return err
			}

			return o.runCmd(f)
		},
	}

	cmd.Flags().BoolVarP(&o.Unset, "unset", "u", o.Unset, "Unset environment variables instead of setting them")

	return cmd
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

// NewCloudEnvOptions returns initialized command options
func NewCloudEnvOptions(ioStreams util.IOStreams) *CloudEnvOptions {
	return &CloudEnvOptions{
		Options: base.Options{
			IOStreams: ioStreams,
		},
		Unset: false,
	}
}

// Complete adapts from the command line args to the data required.
func (o *CloudEnvOptions) Complete(f util.Factory, cmd *cobra.Command, args []string) error {
	o.Shell = viper.GetString("shell")

	if len(args) > 0 {
		o.Shell = strings.TrimSpace(args[0])
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

// ValidateShell validates that the provided shell is supported
func ValidateShell(shell string) error {
	shells := []string{"bash", "fish", "powershell", "zsh"}
	for _, s := range shells {
		if s == shell {
			return nil
		}
	}

	return fmt.Errorf("invalid shell given, must be one of %v", shells)
}

func (o *CloudEnvOptions) runCmd(f util.Factory) error {
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
	garden := NewGardenClient(client, gardenName)
	ctx := f.Context()

	var shoot *gardencorev1beta1.Shoot

	if projectName != "" {
		shoot, err = garden.GetShootByProjectAndName(ctx, projectName, shootName)
		if err != nil {
			return err
		}
	} else if seedName != "" {
		shoot, err = garden.FindShootBySeedAndName(ctx, seedName, shootName)
		if err != nil {
			return err
		}
	}

	secret, err := garden.GetSecretBySecretBinding(ctx, shoot.Namespace, shoot.Spec.SecretBindingName)
	if err != nil {
		return err
	}

	return o.execTmpl(shoot, secret)
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

type tmplValues map[string]interface{}

// parseCredentials attempts to parse GcpCredentials from a JSON string.
func parseCredentials(tv *tmplValues) error {
	m := *tv
	data := []byte(m["serviceaccount.json"].(string))
	sa := map[string]interface{}{}

	if err := json.Unmarshal(data, &sa); err != nil {
		return err
	}

	m["credentials"] = sa
	if data, err := json.Marshal(sa); err == nil {
		m["serviceaccount.json"] = string(data)
	}

	return nil
}

func cliByCloudprovider(cp string) string {
	switch cp {
	case "alicloud":
		return "aliyun"
	case "gcp":
		return "gcloud"
	default:
		return cp
	}
}

func (o *CloudEnvOptions) execTmpl(shoot *gardencorev1beta1.Shoot, secret *v1.Secret) error {
	cloudprovider := shoot.Spec.Provider.Type

	t, err := o.parseTmpl(cloudprovider)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("parsing template for cloudprovider %q failed: %w", cloudprovider, err)
		}

		return fmt.Errorf("cloudprovider %q is not supported: %w", cloudprovider, err)
	}

	m := make(tmplValues)
	m["shell"] = o.Shell
	m["unset"] = o.Unset
	m["usageHint"] = o.generateUsageHint(cliByCloudprovider(cloudprovider), o.CmdPath...)
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

func (o *CloudEnvOptions) generateUsageHint(cli string, a ...string) string {
	var (
		cmd     string
		desc    string
		comment = "#"
		cmdLine string
	)

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
		cmd = fmt.Sprintf("& %s | Invoke-Expression", cmdLine)
	default:
		cmd = fmt.Sprintf("eval $(%s)", cmdLine)
	}

	return strings.Join([]string{
		comment + " " + desc,
		comment + " " + cmd,
	}, "\n")
}

type GardenClient interface {
	GetProject(ctx context.Context, name string) (*gardencorev1beta1.Project, error)
	GetShoot(ctx context.Context, namespace, name string) (*gardencorev1beta1.Shoot, error)
	GetShootByProjectAndName(ctx context.Context, projectName, name string) (*gardencorev1beta1.Shoot, error)
	FindShootBySeedAndName(ctx context.Context, seedName, name string) (*gardencorev1beta1.Shoot, error)
	GetSecretBinding(ctx context.Context, namespace, name string) (*gardencorev1beta1.SecretBinding, error)
	GetSecret(ctx context.Context, namespace, name string) (*v1.Secret, error)
	GetSecretBySecretBinding(ctx context.Context, namespace, name string) (*v1.Secret, error)
}

type gardenClientImpl struct {
	name   string
	client controllerruntime.Client
}

var _ GardenClient = &gardenClientImpl{}

func NewGardenClient(client controllerruntime.Client, gardenName string) GardenClient {
	return &gardenClientImpl{
		name:   gardenName,
		client: client,
	}
}

func (g *gardenClientImpl) GetProject(ctx context.Context, name string) (*gardencorev1beta1.Project, error) {
	project := &gardencorev1beta1.Project{}

	if err := g.client.Get(ctx, types.NamespacedName{Name: name}, project); err != nil {
		return nil, fmt.Errorf("failed to get project '%s': %w", name, err)
	}

	return project, nil
}

func (g *gardenClientImpl) GetShoot(ctx context.Context, namespace, name string) (*gardencorev1beta1.Shoot, error) {
	shoot := &gardencorev1beta1.Shoot{}

	if err := g.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, shoot); err != nil {
		return nil, fmt.Errorf("failed to get shoot '%s/%s': %w", namespace, name, err)
	}

	return shoot, nil
}

func (g *gardenClientImpl) GetShootByProjectAndName(ctx context.Context, projectName, name string) (*gardencorev1beta1.Shoot, error) {
	project, err := g.GetProject(ctx, projectName)
	if err != nil {
		return nil, err
	}

	shootNamespace := *project.Spec.Namespace
	shoot, err := g.GetShoot(ctx, shootNamespace, name)

	if err != nil {
		return nil, err
	}

	return shoot, nil
}

func (g *gardenClientImpl) FindShootBySeedAndName(ctx context.Context, seedName, name string) (*gardencorev1beta1.Shoot, error) {
	shootList := &gardencorev1beta1.ShootList{}
	labels := controllerruntime.MatchingLabels{"metadata.name": name}
	fields := controllerruntime.MatchingFields{gardencore.ShootSeedName: seedName}

	if err := g.client.List(ctx, shootList, labels, fields); err != nil {
		return nil, fmt.Errorf("failed to list shoots with name '%s' and seed '%s': %w", name, seedName, err)
	}

	if len(shootList.Items) > 1 {
		return nil, fmt.Errorf("found more than one shoot with name '%s' and seed '%s'", name, seedName)
	} else if len(shootList.Items) < 1 {
		return nil, fmt.Errorf("found no shoot with name '%s' and seed '%s'", name, seedName)
	}

	return &shootList.Items[0], nil
}

func (g *gardenClientImpl) GetSecretBinding(ctx context.Context, namespace, name string) (*gardencorev1beta1.SecretBinding, error) {
	secretBinding := &gardencorev1beta1.SecretBinding{}

	if err := g.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, secretBinding); err != nil {
		return nil, fmt.Errorf("failed to get secretBinding '%s/%s': %w", namespace, name, err)
	}

	return secretBinding, nil
}

func (g *gardenClientImpl) GetSecret(ctx context.Context, namespace, name string) (*v1.Secret, error) {
	secret := &v1.Secret{}

	if err := g.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, secret); err != nil {
		return nil, fmt.Errorf("failed to get secret '%s/%s': %w", namespace, name, err)
	}

	return secret, nil
}

func (g *gardenClientImpl) GetSecretBySecretBinding(ctx context.Context, namespace, name string) (*v1.Secret, error) {
	secretBinding, err := g.GetSecretBinding(ctx, namespace, name)
	if err != nil {
		return nil, err
	}

	secretRef := secretBinding.SecretRef

	secret, err := g.GetSecret(ctx, secretRef.Namespace, secretRef.Name)
	if err != nil {
		return nil, err
	}

	return secret, nil
}
