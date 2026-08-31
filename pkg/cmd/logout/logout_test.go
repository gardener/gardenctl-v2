/*
SPDX-FileCopyrightText: Contributors to the Gardener project

SPDX-License-Identifier: Apache-2.0
*/

package logout_test

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/gardener/gardenctl-v2/internal/util"
	utilmocks "github.com/gardener/gardenctl-v2/internal/util/mocks"
	cmdlogout "github.com/gardener/gardenctl-v2/pkg/cmd/logout"
	"github.com/gardener/gardenctl-v2/pkg/config"
	targetmocks "github.com/gardener/gardenctl-v2/pkg/target/mocks"
)

const (
	testIssuer    = "https://oidc.example.com"
	otherIssuer   = "https://other.example.com"
	testGarden    = "my-garden"
	testClientID  = "my-client"
)

// makeJWT builds a minimal JWT whose payload contains the given iss claim.
func makeJWT(iss string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	payload, _ := json.Marshal(map[string]string{"iss": iss, "sub": "user"})
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	return header + "." + encodedPayload + ".fakesig"
}

// writeCacheFile writes a kubelogin-format token cache file to dir and returns the path.
func writeCacheFile(dir, name, iss string) string {
	data, _ := json.Marshal(map[string]string{
		"id_token":      makeJWT(iss),
		"refresh_token": "sometoken",
	})
	path := filepath.Join(dir, name)
	Expect(os.WriteFile(path, data, 0o600)).To(Succeed())
	return path
}

// writeGardenKubeconfig writes a minimal kubeconfig with an OIDC exec plugin to dir
// and returns the path. The exec plugin includes --oidc-issuer-url and optionally
// --token-cache-dir when cacheDir is non-empty.
func writeGardenKubeconfig(dir, issuerURL, cacheDir string) string {
	cfg := clientcmdapi.NewConfig()
	cfg.CurrentContext = "ctx"

	cluster := clientcmdapi.NewCluster()
	cluster.Server = "https://api.example.com"
	cfg.Clusters["ctx"] = cluster

	authInfo := clientcmdapi.NewAuthInfo()
	args := []string{
		"oidc-login",
		"get-token",
		fmt.Sprintf("--oidc-issuer-url=%s", issuerURL),
		fmt.Sprintf("--oidc-client-id=%s", testClientID),
	}
	if cacheDir != "" {
		args = append(args, fmt.Sprintf("--token-cache-dir=%s", cacheDir))
	}
	authInfo.Exec = &clientcmdapi.ExecConfig{
		Command:    "kubectl-oidc-login",
		Args:       args,
		APIVersion: "client.authentication.k8s.io/v1beta1",
	}
	cfg.AuthInfos["user"] = authInfo

	ctx := clientcmdapi.NewContext()
	ctx.Cluster = "ctx"
	ctx.AuthInfo = "user"
	cfg.Contexts["ctx"] = ctx

	path := filepath.Join(dir, "kubeconfig.yaml")
	Expect(clientcmd.WriteToFile(*cfg, path)).To(Succeed())
	return path
}

var _ = Describe("ParseExecArgs", func() {
	It("extracts --oidc-issuer-url and uses default cache dir when --token-cache-dir is absent", func() {
		cacheDir, issuerURL := cmdlogout.ParseExecArgs([]string{
			"oidc-login",
			"get-token",
			"--oidc-issuer-url=https://issuer.example.com",
			"--oidc-client-id=my-client",
		})
		Expect(issuerURL).To(Equal("https://issuer.example.com"))
		// default expands to ~/.kube/cache/oidc-login
		Expect(cacheDir).To(HaveSuffix(filepath.Join(".kube", "cache", "oidc-login")))
	})

	It("extracts --token-cache-dir when present as =value", func() {
		cacheDir, _ := cmdlogout.ParseExecArgs([]string{
			"--token-cache-dir=/custom/cache",
			"--oidc-issuer-url=https://issuer.example.com",
		})
		Expect(cacheDir).To(Equal("/custom/cache"))
	})

	It("extracts --token-cache-dir when present as separate value", func() {
		cacheDir, _ := cmdlogout.ParseExecArgs([]string{
			"--token-cache-dir", "/custom/cache2",
			"--oidc-issuer-url=https://issuer.example.com",
		})
		Expect(cacheDir).To(Equal("/custom/cache2"))
	})

	It("extracts --oidc-issuer-url when present as separate value", func() {
		_, issuerURL := cmdlogout.ParseExecArgs([]string{
			"--oidc-issuer-url", "https://separate.example.com",
		})
		Expect(issuerURL).To(Equal("https://separate.example.com"))
	})
})

var _ = Describe("IssuerFromJWT", func() {
	It("extracts the iss claim from a valid JWT", func() {
		jwt := makeJWT(testIssuer)
		iss, err := cmdlogout.IssuerFromJWT(jwt)
		Expect(err).NotTo(HaveOccurred())
		Expect(iss).To(Equal(testIssuer))
	})

	It("returns an error for an invalid JWT", func() {
		_, err := cmdlogout.IssuerFromJWT("notajwt")
		Expect(err).To(HaveOccurred())
	})

	It("returns an error for a JWT with no iss claim", func() {
		header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256"}`))
		payload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"user"}`))
		jwt := header + "." + payload + ".sig"
		_, err := cmdlogout.IssuerFromJWT(jwt)
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("RemoveCachedTokens", func() {
	var cacheDir string

	BeforeEach(func() {
		var err error
		cacheDir, err = os.MkdirTemp("", "oidc-cache-*")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(cacheDir)).To(Succeed())
	})

	It("removes only files matching the issuer URL", func() {
		writeCacheFile(cacheDir, "token-matching", testIssuer)
		writeCacheFile(cacheDir, "token-other", otherIssuer)

		removed, err := cmdlogout.RemoveCachedTokens(cacheDir, testIssuer)
		Expect(err).NotTo(HaveOccurred())
		Expect(removed).To(Equal(1))

		entries, _ := os.ReadDir(cacheDir)
		Expect(entries).To(HaveLen(1))
		Expect(entries[0].Name()).To(Equal("token-other"))
	})

	It("returns 0 when no files match", func() {
		writeCacheFile(cacheDir, "token-other", otherIssuer)

		removed, err := cmdlogout.RemoveCachedTokens(cacheDir, testIssuer)
		Expect(err).NotTo(HaveOccurred())
		Expect(removed).To(Equal(0))
	})

	It("returns 0 when the cache directory does not exist", func() {
		removed, err := cmdlogout.RemoveCachedTokens("/nonexistent/path", testIssuer)
		Expect(err).NotTo(HaveOccurred())
		Expect(removed).To(Equal(0))
	})

	It("skips .lock files", func() {
		writeCacheFile(cacheDir, "token-matching", testIssuer)
		// write a matching .lock file — should not be removed
		lockData, _ := json.Marshal(map[string]string{"id_token": makeJWT(testIssuer)})
		Expect(os.WriteFile(filepath.Join(cacheDir, "token-matching.lock"), lockData, 0o600)).To(Succeed())

		removed, err := cmdlogout.RemoveCachedTokens(cacheDir, testIssuer)
		Expect(err).NotTo(HaveOccurred())
		Expect(removed).To(Equal(1))

		entries, _ := os.ReadDir(cacheDir)
		Expect(entries).To(HaveLen(1))
		Expect(entries[0].Name()).To(Equal("token-matching.lock"))
	})

	It("skips files that are not token cache files", func() {
		Expect(os.WriteFile(filepath.Join(cacheDir, "not-a-token"), []byte("garbage"), 0o600)).To(Succeed())

		removed, err := cmdlogout.RemoveCachedTokens(cacheDir, testIssuer)
		Expect(err).NotTo(HaveOccurred())
		Expect(removed).To(Equal(0))
	})

	It("removes multiple matching files", func() {
		for i := range 3 {
			writeCacheFile(cacheDir, fmt.Sprintf("token-%d", i), testIssuer)
		}
		writeCacheFile(cacheDir, "token-other", otherIssuer)

		removed, err := cmdlogout.RemoveCachedTokens(cacheDir, testIssuer)
		Expect(err).NotTo(HaveOccurred())
		Expect(removed).To(Equal(3))

		entries, _ := os.ReadDir(cacheDir)
		Expect(entries).To(HaveLen(1))
	})
})

var _ = Describe("Logout Command", func() {
	var (
		ctrl     *gomock.Controller
		factory  *utilmocks.MockFactory
		manager  *targetmocks.MockManager
		streams  util.IOStreams
		out      *util.SafeBytesBuffer
		errOut   *util.SafeBytesBuffer
		tmpDir   string
		cacheDir string
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		factory = utilmocks.NewMockFactory(ctrl)
		manager = targetmocks.NewMockManager(ctrl)

		streams, _, out, errOut = util.NewTestIOStreams()

		var err error
		tmpDir, err = os.MkdirTemp("", "gardenctl-logout-*")
		Expect(err).NotTo(HaveOccurred())

		cacheDir = filepath.Join(tmpDir, "oidc-cache")
		Expect(os.MkdirAll(cacheDir, 0o700)).To(Succeed())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(tmpDir)).To(Succeed())
		ctrl.Finish()
	})

	It("removes cached tokens for a garden with OIDC exec plugin", func() {
		kubeconfigPath := writeGardenKubeconfig(tmpDir, testIssuer, cacheDir)
		writeCacheFile(cacheDir, "token-1", testIssuer)
		writeCacheFile(cacheDir, "token-other", otherIssuer)

		cfg := &config.Config{
			Gardens: []config.Garden{
				{Name: testGarden, Kubeconfig: kubeconfigPath},
			},
		}
		factory.EXPECT().Manager().Return(manager, nil)
		manager.EXPECT().Configuration().Return(cfg)

		cmd := cmdlogout.NewCmdLogout(factory, streams)
		Expect(cmd.Execute()).To(Succeed())

		Expect(out.String()).To(ContainSubstring("Removed 1 cached token file(s) for garden %q", testGarden))
		Expect(out.String()).To(ContainSubstring(testIssuer))

		// token-other should still exist
		entries, _ := os.ReadDir(cacheDir)
		Expect(entries).To(HaveLen(1))
		Expect(entries[0].Name()).To(Equal("token-other"))
	})

	It("reports no tokens found when cache is empty", func() {
		kubeconfigPath := writeGardenKubeconfig(tmpDir, testIssuer, cacheDir)

		cfg := &config.Config{
			Gardens: []config.Garden{
				{Name: testGarden, Kubeconfig: kubeconfigPath},
			},
		}
		factory.EXPECT().Manager().Return(manager, nil)
		manager.EXPECT().Configuration().Return(cfg)

		cmd := cmdlogout.NewCmdLogout(factory, streams)
		Expect(cmd.Execute()).To(Succeed())

		Expect(out.String()).To(ContainSubstring("No cached tokens found for garden %q", testGarden))
	})

	It("warns and skips a garden whose kubeconfig has no OIDC exec plugin", func() {
		// Write a kubeconfig with no exec plugin.
		kubeconfigPath := filepath.Join(tmpDir, "plain-kubeconfig.yaml")
		plainCfg := clientcmdapi.NewConfig()
		plainCfg.CurrentContext = "ctx"
		cluster := clientcmdapi.NewCluster()
		cluster.Server = "https://api.example.com"
		plainCfg.Clusters["ctx"] = cluster
		authInfo := clientcmdapi.NewAuthInfo()
		authInfo.Token = "static-token"
		plainCfg.AuthInfos["user"] = authInfo
		ctx := clientcmdapi.NewContext()
		ctx.Cluster = "ctx"
		ctx.AuthInfo = "user"
		plainCfg.Contexts["ctx"] = ctx
		Expect(clientcmd.WriteToFile(*plainCfg, kubeconfigPath)).To(Succeed())

		cfg := &config.Config{
			Gardens: []config.Garden{
				{Name: testGarden, Kubeconfig: kubeconfigPath},
			},
		}
		factory.EXPECT().Manager().Return(manager, nil)
		manager.EXPECT().Configuration().Return(cfg)

		cmd := cmdlogout.NewCmdLogout(factory, streams)
		Expect(cmd.Execute()).To(Succeed())

		Expect(errOut.String()).To(ContainSubstring("no OIDC exec plugin found"))
		Expect(errOut.String()).To(ContainSubstring(testGarden))
	})

	It("returns an error when --garden names an unknown garden", func() {
		cfg := &config.Config{
			Gardens: []config.Garden{
				{Name: testGarden, Kubeconfig: filepath.Join(tmpDir, "kc.yaml")},
			},
		}
		factory.EXPECT().Manager().Return(manager, nil)
		manager.EXPECT().Configuration().Return(cfg)

		cmd := cmdlogout.NewCmdLogout(factory, streams)
		cmd.SetArgs([]string{"--garden", "nonexistent"})
		Expect(cmd.Execute()).To(HaveOccurred())
	})

	It("restricts to the specified --garden only", func() {
		kubeconfigPath := writeGardenKubeconfig(tmpDir, testIssuer, cacheDir)
		writeCacheFile(cacheDir, "token-1", testIssuer)

		cfg := &config.Config{
			Gardens: []config.Garden{
				{Name: testGarden, Kubeconfig: kubeconfigPath},
				{Name: "other-garden", Kubeconfig: kubeconfigPath},
			},
		}
		factory.EXPECT().Manager().Return(manager, nil)
		manager.EXPECT().Configuration().Return(cfg)

		cmd := cmdlogout.NewCmdLogout(factory, streams)
		cmd.SetArgs([]string{"--garden", testGarden})
		Expect(cmd.Execute()).To(Succeed())

		Expect(out.String()).To(ContainSubstring(testGarden))
		Expect(out.String()).NotTo(ContainSubstring("other-garden"))
	})
})
