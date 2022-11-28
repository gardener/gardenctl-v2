package ssh

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os/exec"
	"path"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardenoperationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	clientauthentication "k8s.io/client-go/pkg/apis/clientauthentication"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardenClient "github.com/gardener/gardenctl-v2/internal/gardenclient"
	"github.com/gardener/gardenctl-v2/pkg/target"
)

// sshPatchUtils provides shared utility functions to sshPatchOptions and sshPatchCompletions.
//
//go:generate mockgen -source=./ssh_patch_utils.go -destination=./mocks/mock_ssh_patch_utils.go -package=mocks github.com/gardener/gardenctl-v2/pkg/cmd/ssh sshPatchUtils
type sshPatchUtils interface {
	GetCurrentUser(ctx context.Context, gardenClient gardenClient.Client, authInfo *clientcmdapi.AuthInfo) (string, error)
	GetAuthInfo(clientConfig clientcmd.ClientConfig) (*clientcmdapi.AuthInfo, error)
	GetBastionsOfUser(ctx context.Context, user string, gardenClient gardenClient.Client, bastionListOption client.ListOption) ([]gardenoperationsv1alpha1.Bastion, error)
	TargetAsListOption(target target.Target) client.ListOption
}

type sshPatchUtilsImpl struct{}

var _ sshPatchUtils = &sshPatchUtilsImpl{}

func (u *sshPatchUtilsImpl) GetCurrentUser(ctx context.Context, gardenClient gardenClient.Client, authInfo *clientcmdapi.AuthInfo) (string, error) {
	baseDir, err := clientcmdapi.MakeAbs(path.Dir(authInfo.LocationOfOrigin), "")
	if err != nil {
		return "", fmt.Errorf("Could not parse location of kubeconfig origin")
	}

	if len(authInfo.ClientCertificateData) == 0 && len(authInfo.ClientCertificate) > 0 {
		err := clientcmdapi.FlattenContent(&authInfo.ClientCertificate, &authInfo.ClientCertificateData, baseDir)
		if err != nil {
			return "", err
		}
	} else if len(authInfo.Token) == 0 && len(authInfo.TokenFile) > 0 {
		var tmpValue = []byte{}
		err := clientcmdapi.FlattenContent(&authInfo.TokenFile, &tmpValue, baseDir)
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

func (u *sshPatchUtilsImpl) GetAuthInfo(clientConfig clientcmd.ClientConfig) (*clientcmdapi.AuthInfo, error) {
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

func (u *sshPatchUtilsImpl) GetBastionsOfUser(ctx context.Context, user string, gardenClient gardenClient.Client, bastionListOption client.ListOption) ([]gardenoperationsv1alpha1.Bastion, error) {
	var bastionsOfUser []gardenoperationsv1alpha1.Bastion

	list, err := gardenClient.ListBastions(ctx, bastionListOption)
	if err != nil {
		return nil, err
	}

	for _, bastion := range list.Items {
		// bastion := list.Items[i]
		if createdBy, ok := bastion.Annotations["gardener.cloud/created-by"]; ok {
			if createdBy == user {
				bastionsOfUser = append(bastionsOfUser, bastion)
			}
		}
	}

	return bastionsOfUser, nil
}

// TargetAsListOption is similiar to target.Target.AsListOption but for bastions.
func (u *sshPatchUtilsImpl) TargetAsListOption(target target.Target) client.ListOption {
	opt := gardenClient.ProjectFilter{}

	if target.ShootName() != "" {
		opt["spec.shootRef.name"] = target.ShootName()
	}

	if target.ProjectName() != "" {
		opt["project"] = target.ProjectName()
	} else if target.SeedName() != "" {
		opt[gardencore.ShootSeedName] = target.SeedName()
	}

	return opt
}
