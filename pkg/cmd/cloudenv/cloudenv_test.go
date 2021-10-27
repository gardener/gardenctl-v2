/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package cloudenv_test

import (
	"embed"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	internalfake "github.com/gardener/gardenctl-v2/internal/fake"
	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/cloudenv"
	"github.com/gardener/gardenctl-v2/pkg/config"
	"github.com/gardener/gardenctl-v2/pkg/target"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

//go:embed testdata
var testdata embed.FS

func init() {
	utilruntime.Must(gardencorev1beta1.AddToScheme(scheme.Scheme))
}

func readFile(name string) string {
	data, err := testdata.ReadFile(filepath.Join("testdata", name))
	utilruntime.Must(err)

	return string(data)
}

var _ = Describe("CloudEnv Command", func() {
	var (
		gardenName,
		projectName,
		seedName,
		shootName,
		secretBindingName,
		secretName,
		kubeconfig string
		serviceaccountJSON = readFile("gcp/serviceaccount.json")
		factory            util.Factory
		streams            util.IOStreams
		buf                *util.SafeBytesBuffer
		args               []string
	)

	BeforeEach(func() {
		gardenName = "test"
		projectName = "foo"
		seedName = ""
		shootName = "bar"
		secretBindingName = "secret-binding"
		secretName = "secret"
		streams, _, buf, _ = util.NewTestIOStreams()
		args = make([]string, 0, 5)
	})

	JustBeforeEach(func() {
		// create config
		cfg := &config.Config{
			Gardens: []config.Garden{{
				Name:       gardenName,
				Kubeconfig: kubeconfig,
			}},
		}
		// create garden client
		fakeGardenClient := fake.NewClientBuilder()
		if projectName != "" {
			namespace := "garden-" + projectName
			fakeGardenClient.WithObjects(&gardencorev1beta1.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: projectName,
				},
				Spec: gardencorev1beta1.ProjectSpec{
					Namespace: &namespace,
				},
			})
			if shootName != "" {
				fakeGardenClient.WithObjects(&gardencorev1beta1.Shoot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      shootName,
						Namespace: namespace,
					},
					Spec: gardencorev1beta1.ShootSpec{
						Region:            "europe",
						SecretBindingName: secretBindingName,
						Provider: gardencorev1beta1.Provider{
							Type: "gcp",
						},
					},
				})
			}
			if secretBindingName != "" && secretName != "" {
				fakeGardenClient.WithObjects(&gardencorev1beta1.SecretBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      secretBindingName,
						Namespace: namespace,
					},
					SecretRef: corev1.SecretReference{
						Namespace: namespace,
						Name:      secretName,
					},
				})
				fakeGardenClient.WithObjects(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      secretName,
						Namespace: namespace,
					},
					Data: map[string][]byte{
						"serviceaccount.json": []byte(serviceaccountJSON),
					},
				})
			}
		}
		// create client provider
		clientProvider := internalfake.NewFakeClientProvider().WithClient(kubeconfig, fakeGardenClient.Build())
		// create target provider
		target := target.NewTarget(gardenName, projectName, seedName, shootName)
		targetProvider := internalfake.NewFakeTargetProvider(target)
		// create factory
		factory = internalfake.NewFakeFactory(cfg, nil, clientProvider, nil, targetProvider)
	})

	Describe("running the cloud-env command", func() {
		It("should create the command and add flags", func() {
			cmd := cloudenv.NewCmdCloudEnv(factory, streams)
			Expect(cmd.Use).To(Equal("cloud-env [bash | fish | powershell | zsh]"))
			Expect(cmd.Aliases).To(Equal(cloudenv.Aliases))
			Expect(cmd.ValidArgs).To(Equal(cloudenv.ValidShells))
			flag := cmd.Flag("unset")
			Expect(flag).NotTo(BeNil())
			Expect(flag.Shorthand).To(Equal("u"))
			Expect(cmd.Flag("output")).To(BeNil())
		})

		It("should run the command for bash shell without any flags", func() {
			cmd := cloudenv.NewCmdCloudEnv(factory, streams)
			cmd.SetArgs(append(args, "bash"))
			Expect(cmd.Execute()).To(Succeed())
			Expect(cmd.Flag("unset").Value.String()).To(Equal("false"))
			Expect(buf.String()).To(Equal(readFile("gcp/export.bash")))
		})

		It("should run the command for powershell with flags --unset", func() {
			cmd := cloudenv.NewCmdCloudEnv(factory, streams)
			cmd.SetArgs(append(args, "--unset", "powershell"))
			Expect(cmd.Execute()).To(Succeed())
			Expect(cmd.Flag("unset").Value.String()).To(Equal("true"))
			Expect(buf.String()).To(Equal(readFile("gcp/unset.pwsh")))
		})
	})

	Describe("detecting the default shell", func() {
		originalShell := os.Getenv("SHELL")

		AfterEach(func() {
			os.Setenv("SHELL", originalShell)
		})

		It("should return the default shell ", func() {
			os.Unsetenv("SHELL")
			By("Running on Darwin")
			Expect(cloudenv.DetectShell("darwin")).To(Equal("bash"))
			By("Running on Windows")
			Expect(cloudenv.DetectShell("windows")).To(Equal("powershell"))
		})

		It("should return the shell defined in the environment", func() {
			os.Setenv("SHELL", "/bin/fish")
			Expect(cloudenv.DetectShell("*")).To(Equal("fish"))
		})

	})
})
