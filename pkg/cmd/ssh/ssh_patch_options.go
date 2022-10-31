package ssh

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"path"
	"strings"
	"time"

	gardenoperationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	"github.com/spf13/cobra"
	networkingv1 "k8s.io/api/networking/v1"
	clientauthentication "k8s.io/client-go/pkg/apis/clientauthentication"
	"k8s.io/client-go/tools/clientcmd/api"

	gardenClient "github.com/gardener/gardenctl-v2/internal/gardenclient"
	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
	"github.com/gardener/gardenctl-v2/pkg/target"
)

// wrappers used for unit tests only
var (
	timeNow        = time.Now
	getCurrentUser = func(ctx context.Context, gardenClient gardenClient.Client, authInfo *api.AuthInfo) (string, error) {
		baseDir, err := api.MakeAbs(path.Dir(authInfo.LocationOfOrigin), "")
		if err != nil {
			return "", fmt.Errorf("Could not parse location of kubeconfig origin")
		}

		if len(authInfo.ClientCertificateData) == 0 && len(authInfo.ClientCertificate) > 0 {
			err := api.FlattenContent(&authInfo.ClientCertificate, &authInfo.ClientCertificateData, baseDir)
			if err != nil {
				return "", err
			}
		} else if len(authInfo.Token) == 0 && len(authInfo.TokenFile) > 0 {
			var tmpValue = []byte{}
			err := api.FlattenContent(&authInfo.TokenFile, &tmpValue, baseDir)
			if err != nil {
				return "", err
			}
			authInfo.Token = string(tmpValue)
		} else if authInfo.Exec != nil && len(authInfo.Exec.Command) > 0 {
			// The command originates from the users kubeconfig and is also executed when using kubectl.
			// So it should be safe to execute it here as well.
			execCmd := exec.Command(authInfo.Exec.Command, authInfo.Exec.Args...)
			var out bytes.Buffer
			execCmd.Stdout = &out

			err := execCmd.Run()
			if err != nil {
				return "", err
			}

			var execCredential clientauthentication.ExecCredential
			err = json.Unmarshal(out.Bytes(), &execCredential)
			if err != nil {
				return "", err
			}

			if token := execCredential.Status.Token; len(token) > 0 {
				authInfo.Token = token
			} else if cert := execCredential.Status.ClientCertificateData; len(cert) > 0 {
				authInfo.ClientCertificateData = []byte(cert)
			}
		}

		if len(authInfo.ClientCertificateData) > 0 {
			block, _ := pem.Decode(authInfo.ClientCertificateData) // does not return an error, just nil
			if block == nil {
				return "", fmt.Errorf("Could not decode PEM certificate")
			}

			cert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				return "", err
			}

			user := cert.Subject.CommonName
			if len(user) > 0 {
				return user, nil
			}
		}

		if len(authInfo.Token) > 0 {
			tokenReview, err := gardenClient.CreateTokenReview(ctx, authInfo.Token)
			if err != nil {
				return "", err
			}
			if user := tokenReview.Status.User.Username; user != "" {
				return user, nil
			}
		}

		return "", fmt.Errorf("Could not detect current user")
	}
)

//nolint:revive
type SSHPatchOptions struct {
	sshBaseOptions

	// BastionName is the patch targets name
	BastionName string
	// Factory gives access to the gardenclient and target manager
	Factory util.Factory
	// Bastion is the Bastion corresponding to the provided BastionName
	Bastion *gardenoperationsv1alpha1.Bastion
}

func NewSSHPatchOptions(ioStreams util.IOStreams) *SSHPatchOptions {
	return &SSHPatchOptions{
		sshBaseOptions: sshBaseOptions{
			Options: base.Options{
				IOStreams: ioStreams,
			},
		},
	}
}

func getBastion(ctx context.Context, o *SSHPatchOptions, gardenClient gardenClient.Client, currentTarget target.Target) (*gardenoperationsv1alpha1.Bastion, error) {
	listOptions := currentTarget.AsListOption()

	shoot, err := gardenClient.FindShoot(ctx, listOptions)
	if err != nil {
		return nil, err
	}

	bastion, err := gardenClient.GetBastion(ctx, shoot.Namespace, o.BastionName)
	if err != nil {
		return nil, err
	}

	if bastion.ObjectMeta.UID == "" {
		return nil, fmt.Errorf("Bastion '%s' in namespace '%s' not found", o.BastionName, shoot.Namespace)
	}

	return bastion, nil
}

func getAuthInfo(ctx context.Context, manager target.Manager) (*api.AuthInfo, error) {
	currentTarget, err := manager.CurrentTarget()
	if err != nil {
		return nil, err
	}

	gardenTarget := target.NewTarget(currentTarget.GardenName(), "", "", "")

	clientConfig, err := manager.ClientConfig(ctx, gardenTarget)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve client config for target garden %s: %w", currentTarget.GardenName(), err)
	}

	rawConfig, err := clientConfig.RawConfig()
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

func getBastionsOfUser(ctx context.Context, gardenClient gardenClient.Client, authInfo *api.AuthInfo) ([]*gardenoperationsv1alpha1.Bastion, error) {
	var bastionOfUser []*gardenoperationsv1alpha1.Bastion

	currentUser, err := getCurrentUser(ctx, gardenClient, authInfo)
	if err != nil {
		return nil, err
	}

	list, err := gardenClient.ListBastions(ctx)
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

func patchBastionIngress(ctx context.Context, o *SSHPatchOptions, gardenClient gardenClient.Client, currentTarget target.Target) error {
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

	return gardenClient.PatchBastion(ctx, o.Bastion, oldBastion)
}

func getManagerTargetAndGardenClient(f util.Factory) (target.Manager, target.Target, gardenClient.Client, error) {
	manager, err := f.Manager()
	if err != nil {
		return nil, nil, nil, err
	}

	currentTarget, err := manager.CurrentTarget()
	if err != nil {
		return nil, nil, nil, err
	}

	gardenClient, err := manager.GardenClient(currentTarget.GardenName())
	if err != nil {
		return nil, nil, nil, err
	}

	return manager, currentTarget, gardenClient, nil
}

func (o *SSHPatchOptions) Run(_ util.Factory) error {
	_, currentTarget, gardenClient, err := getManagerTargetAndGardenClient(o.Factory)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(o.Factory.Context(), 30*time.Second)
	defer cancel()

	return patchBastionIngress(ctx, o, gardenClient, currentTarget)
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

func (o *SSHPatchOptions) Complete(f util.Factory, cmd *cobra.Command, args []string) error {
	o.Factory = f
	if len(args) > 0 {
		o.BastionName = args[0]
	}

	ctx, cancel := context.WithTimeout(o.Factory.Context(), 30*time.Second)
	defer cancel()

	manager, currentTarget, gardenClient, err := getManagerTargetAndGardenClient(f)
	if err != nil {
		return err
	}

	if err := o.sshBaseOptions.Complete(o.Factory, cmd, args); err != nil {
		return err
	}

	authInfo, err := getAuthInfo(ctx, manager)
	if err != nil {
		return fmt.Errorf("Could not get current authInfo: %w", err)
	}

	if o.BastionName == "" {
		bastions, err := getBastionsOfUser(ctx, gardenClient, authInfo)
		if err != nil {
			return err
		}

		if len(bastions) == 1 {
			b := bastions[0]
			name := b.Name
			age := formatDuration(timeNow().Sub(b.CreationTimestamp.Time))
			fmt.Fprintf(o.IOStreams.Out, "Auto-selected bastion %q created %s ago targeting shoot \"%s/%s\"\n", name, age, b.Namespace, b.Spec.ShootRef.Name)
			o.BastionName = b.Name
			o.Bastion = b
		} else if len(bastions) == 0 {
			fmt.Fprint(o.IOStreams.Out, "No bastions were found\n")
		} else {
			fmt.Fprint(o.IOStreams.Out, "Multiple bastions were found and the target bastion needs to be explictly defined\n")
		}
	} else {
		bastion, err := getBastion(ctx, o, gardenClient, currentTarget)
		if err != nil {
			return err
		}
		o.Bastion = bastion
	}

	return nil
}

func (o *SSHPatchOptions) getBastionNameCompletions(f util.Factory, cmd *cobra.Command, toComplete string) ([]string, error) {
	var completions []string

	manager, _, gardenClient, err := getManagerTargetAndGardenClient(f)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(f.Context(), 30*time.Second)
	defer cancel()

	authInfo, err := getAuthInfo(ctx, manager)
	if err != nil {
		return nil, fmt.Errorf("Could not get current authInfo: %w", err)
	}

	bastions, err := getBastionsOfUser(ctx, gardenClient, authInfo)
	if err != nil {
		return nil, err
	}

	for _, b := range bastions {
		if strings.HasPrefix(b.Name, toComplete) {
			age := formatDuration(timeNow().Sub(b.CreationTimestamp.Time))

			completion := fmt.Sprintf("%s\t created %s ago targeting shoot \"%s/%s\"", b.Name, age, b.Namespace, b.Spec.ShootRef.Name)
			completions = append(completions, completion)
		}
	}

	return completions, nil
}

func (o *SSHPatchOptions) Validate() error {
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
