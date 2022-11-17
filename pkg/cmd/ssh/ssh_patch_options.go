package ssh

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	gardenoperationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"

	gardenClient "github.com/gardener/gardenctl-v2/internal/gardenclient"
	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
	"github.com/gardener/gardenctl-v2/pkg/target"
)

type sshPatchOptions struct {
	sshBaseOptions

	// BastionName is the patch targets name
	BastionName string
	// Bastion is the Bastion corresponding to the provided BastionName
	Bastion *gardenoperationsv1alpha1.Bastion

	clientConfig  clientcmd.ClientConfig
	gardenClient  gardenClient.Client
	currentTarget target.Target
	clock         util.Clock
	authInfo      *api.AuthInfo

	// Utils contains helper methods that can be replaced by a mock impl for tests
	Utils sshPatchUtils
}

func newSSHPatchOptions(ioStreams util.IOStreams) *sshPatchOptions {
	o := &sshPatchOptions{
		sshBaseOptions: sshBaseOptions{
			Options: base.Options{
				IOStreams: ioStreams,
			},
		},
		Utils: &sshPatchUtilsImpl{},
	}

	return o
}

func (o *sshPatchOptions) getBastion(ctx context.Context) (*gardenoperationsv1alpha1.Bastion, error) {
	listOptions := o.currentTarget.AsListOption()

	shoot, err := o.gardenClient.FindShoot(ctx, listOptions)
	if err != nil {
		return nil, err
	}

	bastion, err := o.gardenClient.GetBastion(ctx, shoot.Namespace, o.BastionName)
	if err != nil {
		return nil, err
	}

	if bastion.ObjectMeta.UID == "" {
		return nil, fmt.Errorf("Bastion '%s' in namespace '%s' not found", o.BastionName, shoot.Namespace)
	}

	return bastion, nil
}

func (o *sshPatchOptions) getAuthInfo(ctx context.Context) (*api.AuthInfo, error) {
	rawConfig, err := o.clientConfig.RawConfig()
	if err != nil {
		return nil, fmt.Errorf("could not retrieve raw config: %w", err)
	}

	context, ok := rawConfig.Contexts[rawConfig.CurrentContext]
	if !ok {
		return nil, fmt.Errorf("no context found for current context %s", rawConfig.CurrentContext)
	}

	authInfo, ok := rawConfig.AuthInfos[context.AuthInfo]
	if !ok {
		return nil, fmt.Errorf("no auth info found with name %s", context.AuthInfo)
	}

	return authInfo, nil
}

func (o *sshPatchOptions) getBastionsOfUser(ctx context.Context) ([]*gardenoperationsv1alpha1.Bastion, error) {
	var bastionOfUser []*gardenoperationsv1alpha1.Bastion

	currentUser, err := o.Utils.GetCurrentUser(ctx, o.gardenClient, o.authInfo)
	if err != nil {
		return nil, err
	}

	list, err := o.gardenClient.ListBastions(ctx)
	if err != nil || len(list.Items) == 0 {
		return nil, err
	}

	for i := range list.Items {
		bastion := list.Items[i]
		if createdBy, ok := bastion.Annotations["gardener.cloud/created-by"]; ok {
			if createdBy == currentUser {
				bastionOfUser = append(bastionOfUser, &bastion)
			}
		}
	}

	return bastionOfUser, nil
}

func (o *sshPatchOptions) patchBastionIngress(ctx context.Context) error {
	var policies []gardenoperationsv1alpha1.BastionIngressPolicy

	oldBastion := o.Bastion.DeepCopy()

	for _, cidr := range o.CIDRs {
		if *o.Bastion.Spec.ProviderType == "gcp" {
			ip, _, err := net.ParseCIDR(cidr)
			if err != nil {
				return err
			}

			if ip.To4() == nil {
				if !o.AutoDetected {
					return fmt.Errorf("GCP only supports IPv4: %s", cidr)
				}

				fmt.Fprintf(o.IOStreams.Out, "GCP only supports IPv4, skipped CIDR: %s\n", cidr)

				continue // skip
			}
		}

		policies = append(policies, gardenoperationsv1alpha1.BastionIngressPolicy{
			IPBlock: networkingv1.IPBlock{
				CIDR: cidr,
			},
		})
	}

	if len(policies) == 0 {
		return errors.New("no ingress policies could be created")
	}

	o.Bastion.Spec.Ingress = policies

	return o.gardenClient.PatchBastion(ctx, o.Bastion, oldBastion)
}

func (o *sshPatchOptions) Run(f util.Factory) error {
	ctx, cancel := context.WithTimeout(f.Context(), 30*time.Second)
	defer cancel()

	return o.patchBastionIngress(ctx)
}

func formatDuration(d time.Duration) string {
	const (
		hFactor int = 3600
		mFactor int = 60
	)

	s := int(d.Seconds())
	h := s / hFactor
	s -= h * hFactor
	m := s / mFactor
	s -= m * mFactor

	if h > 0 {
		return fmt.Sprintf("%dh%dm%ds", h, m, s)
	}

	if m > 0 {
		return fmt.Sprintf("%dm%ds", m, s)
	}

	return fmt.Sprintf("%ds", s)
}

func (o *sshPatchOptions) Complete(f util.Factory, cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(f.Context(), 30*time.Second)
	defer cancel()

	manager, err := f.Manager()
	if err != nil {
		return err
	}

	currentTarget, err := manager.CurrentTarget()
	if err != nil {
		return err
	}

	o.currentTarget = currentTarget

	gardenClient, err := manager.GardenClient(currentTarget.GardenName())
	if err != nil {
		return err
	}

	o.gardenClient = gardenClient
	gardenName := currentTarget.GardenName()

	clientConfig, err := manager.ClientConfig(ctx, target.NewTarget(gardenName, "", "", ""))
	if err != nil {
		return fmt.Errorf("could not retrieve client config for target garden %s: %w", gardenName, err)
	}

	o.clientConfig = clientConfig

	authInfo, err := o.getAuthInfo(ctx)
	if err != nil {
		return fmt.Errorf("Could not get current authInfo: %w", err)
	}

	o.authInfo = authInfo

	o.clock = f.Clock()

	if len(args) > 0 {
		o.BastionName = args[0]
	}

	if err != nil {
		return err
	}

	if err := o.sshBaseOptions.Complete(f, cmd, args); err != nil {
		return err
	}

	if o.BastionName == "" {
		bastions, err := o.getBastionsOfUser(ctx)
		if err != nil {
			return err
		}

		if len(bastions) == 1 {
			b := bastions[0]
			name := b.Name
			age := formatDuration(o.clock.Now().Sub(b.CreationTimestamp.Time))
			fmt.Fprintf(o.IOStreams.Out, "Auto-selected bastion %q created %s ago targeting shoot \"%s/%s\"\n", name, age, b.Namespace, b.Spec.ShootRef.Name)
			o.BastionName = b.Name
			o.Bastion = b
		} else if len(bastions) == 0 {
			fmt.Fprint(o.IOStreams.Out, "No bastions were found\n")
		} else {
			fmt.Fprint(o.IOStreams.Out, "Multiple bastions were found and the target bastion needs to be explictly defined\n")
		}
	} else {
		bastion, err := o.getBastion(ctx)
		if err != nil {
			return err
		}
		o.Bastion = bastion
	}

	return nil
}

func (o *sshPatchOptions) GetBastionNameCompletions(f util.Factory, cmd *cobra.Command, toComplete string) ([]string, error) {
	ctx, cancel := context.WithTimeout(f.Context(), 30*time.Second)
	defer cancel()

	manager, err := f.Manager()
	if err != nil {
		return nil, err
	}

	currentTarget, err := manager.CurrentTarget()
	if err != nil {
		return nil, err
	}

	o.currentTarget = currentTarget

	gardenClient, err := manager.GardenClient(currentTarget.GardenName())
	if err != nil {
		return nil, err
	}

	o.gardenClient = gardenClient
	gardenName := currentTarget.GardenName()

	clientConfig, err := manager.ClientConfig(ctx, target.NewTarget(gardenName, "", "", ""))
	if err != nil {
		return nil, fmt.Errorf("could not retrieve client config for target garden %s: %w", gardenName, err)
	}

	o.clientConfig = clientConfig

	authInfo, err := o.getAuthInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("Could not get current authInfo: %w", err)
	}

	o.authInfo = authInfo

	o.clock = f.Clock()

	var completions []string

	bastions, err := o.getBastionsOfUser(ctx)
	if err != nil {
		return nil, err
	}

	for _, b := range bastions {
		if strings.HasPrefix(b.Name, toComplete) {
			age := formatDuration(o.clock.Now().Sub(b.CreationTimestamp.Time))

			completion := fmt.Sprintf("%s\t created %s ago targeting shoot \"%s/%s\"", b.Name, age, b.Namespace, b.Spec.ShootRef.Name)
			completions = append(completions, completion)
		}
	}

	return completions, nil
}

func (o *sshPatchOptions) Validate() error {
	if err := o.sshBaseOptions.Validate(); err != nil {
		return err
	}

	if o.BastionName == "" {
		return fmt.Errorf("BastionName is required")
	}

	if o.Bastion == nil {
		return fmt.Errorf("Bastion is required")
	}

	if o.Bastion.Name != o.BastionName {
		return fmt.Errorf("BastionName does not match Bastion.Name in SSHPatchOptions")
	}

	return nil
}

func (o *sshPatchOptions) AddFlags(flags *pflag.FlagSet) {
	flags.StringArrayVar(&o.CIDRs, "cidr", o.CIDRs, "CIDRs to allow access to the bastion host; if not given, your system's public IPs (v4 and v6) are auto-detected.")
}
