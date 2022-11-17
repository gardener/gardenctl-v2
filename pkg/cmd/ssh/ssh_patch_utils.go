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

	clientauthentication "k8s.io/client-go/pkg/apis/clientauthentication"
	"k8s.io/client-go/tools/clientcmd/api"

	gardenClient "github.com/gardener/gardenctl-v2/internal/gardenclient"
)

// sshPatchUtils provides utility functions to SSHPatchOptions. Beeing a seperate interface it is
// easy to mock below functions for unit testing.
//
//go:generate mockgen -source=./ssh_patch_utils.go -destination=./mocks/mock_ssh_patch_utils.go -package=mocks github.com/gardener/gardenctl-v2/pkg/cmd/ssh sshPatchUtils
type sshPatchUtils interface {
	GetCurrentUser(ctx context.Context, gardenClient gardenClient.Client, authInfo *api.AuthInfo) (string, error)
}

type sshPatchUtilsImpl struct{}

func (u *sshPatchUtilsImpl) GetCurrentUser(ctx context.Context, gardenClient gardenClient.Client, authInfo *api.AuthInfo) (string, error) {
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

var _ sshPatchUtils = &sshPatchUtilsImpl{}
