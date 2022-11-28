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
	clientapi "k8s.io/client-go/tools/clientcmd/api"

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

	gardenClient  gardenClient.Client
	currentTarget target.Target
	clock         util.Clock
	authInfo      *clientapi.AuthInfo

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

func (o *sshPatchOptions) getBastionsOfUser(ctx context.Context) ([]gardenoperationsv1alpha1.Bastion, error) {
	currentUser, err := o.Utils.GetCurrentUser(ctx, o.gardenClient, o.authInfo)
	if err != nil {
		return nil, err
	}

	listOption := o.Utils.TargetAsListOption(o.currentTarget)

	return o.Utils.GetBastionsOfUser(ctx, currentUser, o.gardenClient, listOption)
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

	if err := o.patchBastionIngress(ctx); err != nil {
		return err
	}

	fmt.Fprintf(o.IOStreams.Out, "Successfully patched bastion %q\n", o.BastionName)

	return nil
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

	authInfo, err := o.Utils.GetAuthInfo(clientConfig)
	if err != nil {
		return fmt.Errorf("could not get current authInfo: %w", err)
	}

	o.authInfo = authInfo

	o.clock = f.Clock()

	if len(args) > 0 {
		o.BastionName = args[0]
	}

	if err := o.sshBaseOptions.Complete(f, cmd, args); err != nil {
		return err
	}

	bastions, err := o.getBastionsOfUser(ctx)
	if err != nil {
		return err
	}

	if len(bastions) == 0 {
		return errors.New("no bastions found for current user")
	}

	if o.BastionName == "" {
		if len(bastions) > 1 {
			return fmt.Errorf("multiple bastions were found and the target bastion needs to be explicitly defined")
		}

		o.Bastion = &bastions[0]
		o.BastionName = o.Bastion.Name

		age := o.clock.Now().Sub(o.Bastion.CreationTimestamp.Time).Round(time.Second).String()
		fmt.Fprintf(o.IOStreams.Out, "Auto-selected bastion %q created %s ago targeting shoot \"%s/%s\"\n", o.BastionName, age, o.Bastion.Namespace, o.Bastion.Spec.ShootRef.Name)
	} else {
		for _, b := range bastions {
			if b.Name == o.BastionName {
				o.Bastion = &b
				break
			}
		}

		if o.Bastion == nil {
			return fmt.Errorf("Bastion %q for current user not found", o.BastionName)
		}
	}

	return nil
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

type sshPatchCompletions struct {
	// Utils contains helper methods that can be replaced by a mock impl for tests
	Utils sshPatchUtils
}

func newSSHPatchCompletions() *sshPatchCompletions {
	return &sshPatchCompletions{
		Utils: &sshPatchUtilsImpl{},
	}
}

func (c *sshPatchCompletions) GetBastionNameCompletions(f util.Factory, cmd *cobra.Command, prefix string) ([]string, error) {
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

	gardenName := currentTarget.GardenName()

	gardenClient, err := manager.GardenClient(gardenName)
	if err != nil {
		return nil, err
	}

	clientConfig, err := manager.ClientConfig(ctx, target.NewTarget(gardenName, "", "", ""))
	if err != nil {
		return nil, fmt.Errorf("could not retrieve client config for target garden %s: %w", gardenName, err)
	}

	authInfo, err := c.Utils.GetAuthInfo(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("could not get current authInfo: %w", err)
	}

	currentUser, err := c.Utils.GetCurrentUser(ctx, gardenClient, authInfo)
	if err != nil {
		return nil, fmt.Errorf("could not get current user: %w", err)
	}

	listOptions := c.Utils.TargetAsListOption(currentTarget)

	bastions, err := c.Utils.GetBastionsOfUser(ctx, currentUser, gardenClient, listOptions)
	if err != nil {
		return nil, err
	}

	var completions []string

	clock := f.Clock()

	for _, b := range bastions {
		if strings.HasPrefix(b.Name, prefix) {
			age := clock.Now().Sub(b.CreationTimestamp.Time).Round(time.Second).String()

			completion := fmt.Sprintf("%s\t created %s ago targeting shoot \"%s/%s\"", b.Name, age, b.Namespace, b.Spec.ShootRef.Name)
			completions = append(completions, completion)
		}
	}

	return completions, nil
}
