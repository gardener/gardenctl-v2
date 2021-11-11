/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package cloudenv_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardenclientmocks "github.com/gardener/gardenctl-v2/internal/gardenclient/mocks"
	utilmocks "github.com/gardener/gardenctl-v2/internal/util/mocks"
	"github.com/gardener/gardenctl-v2/pkg/cmd/cloudenv"
	"github.com/gardener/gardenctl-v2/pkg/target"
	targetmocks "github.com/gardener/gardenctl-v2/pkg/target/mocks"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

var _ = Describe("CloudEnv Options", func() {
	Describe("having an instance", func() {
		var (
			ctrl    *gomock.Controller
			factory *utilmocks.MockFactory
			options *cloudenv.TestOptions
			cmd     *cobra.Command
			params,
			args []string
			callAs = func(a ...string) error {
				testCmd := &cobra.Command{Use: "test"}
				testCmd.AddCommand(&cobra.Command{
					Use:     "foo",
					Aliases: []string{"bar"},
					Run: func(c *cobra.Command, a []string) {
						cmd = c
						args = a
					},
				})
				testCmd.SetArgs(a)
				return testCmd.Execute()
			}
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			factory = utilmocks.NewMockFactory(ctrl)
			options = cloudenv.NewTestOptions()
			params = []string{"foo"}
		})

		AfterEach(func() {
			viper.Reset()
			ctrl.Finish()
		})

		JustBeforeEach(func() {
			Expect(callAs(params...)).To(Succeed())
		})

		Describe("completing the command options", func() {
			BeforeEach(func() {
				factory.EXPECT().GardenHomeDir().Return(gardenHomeDir)
			})

			It("should complete options with default shell", func() {
				Expect(options.Complete(factory, cmd, args)).To(Succeed())
				Expect(options.Shell).To(Equal("bash"))
				Expect(options.GardenDir).To(Equal(gardenHomeDir))
				Expect(options.CmdPath).To(Equal([]string{"test", "foo"}))
			})

			Context("when the shell is given", func() {
				BeforeEach(func() {
					params = []string{"bar", "fish"}
				})

				It("should complete options with given shell", func() {
					Expect(options.Complete(factory, cmd, args)).To(Succeed())
					Expect(options.Shell).To(Equal("fish"))
					Expect(options.CmdPath).To(Equal([]string{"test", "bar"}))
				})
			})

			Context("when the shell is not given but configured", func() {
				BeforeEach(func() {
					viper.Set("shell", "powershell")
				})

				It("should complete options with configured shell", func() {
					Expect(options.Complete(factory, cmd, args)).To(Succeed())
					Expect(options.Shell).To(Equal("powershell"))
					Expect(options.CmdPath).To(Equal([]string{"test", "foo"}))
				})
			})
		})

		Describe("validating the command options", func() {
			It("should successfully validate the options", func() {
				options.Shell = "bash"
				Expect(options.Validate()).To(Succeed())
			})

			It("should return an error when the shell is empty", func() {
				options.Shell = ""
				Expect(options.Validate()).To(MatchError("no shell configured or specified"))
			})

			It("should return an error when the shell is invalid", func() {
				options.Shell = "cmd"
				Expect(options.Validate()).To(MatchError(fmt.Sprintf("invalid shell given, must be one of %v", cloudenv.ValidShells)))
			})
		})

		Describe("adding the command flags", func() {
			It("should successfully add the unset flag", func() {
				options.AddFlags(cmd.Flags())
				Expect(cmd.Flag("unset")).NotTo(BeNil())
			})
		})

		Describe("running the command with the given options", func() {
			var (
				ctx               context.Context
				manager           *targetmocks.MockManager
				client            *gardenclientmocks.MockClient
				t                 target.Target
				secretBindingName string
				region            string
				provider          *gardencorev1beta1.Provider
				secretRef         *corev1.SecretReference
				shoot             *gardencorev1beta1.Shoot
				secretBinding     *gardencorev1beta1.SecretBinding
				secret            *corev1.Secret
			)

			BeforeEach(func() {
				ctx = context.Background()
				manager = targetmocks.NewMockManager(ctrl)
				client = gardenclientmocks.NewMockClient(ctrl)
				t = target.NewTarget("test", "project", "seed", "shoot")
				secretBindingName = "secret-binding"
				region = "europe"
				provider = &gardencorev1beta1.Provider{
					Type: "gcp",
				}
				secretRef = &corev1.SecretReference{
					Namespace: "private",
					Name:      "secret",
				}
				options.CmdPath = []string{"cloud-env"}
				options.Shell = "bash"
			})

			JustBeforeEach(func() {
				shoot = &gardencorev1beta1.Shoot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      t.ShootName(),
						Namespace: "garden-" + t.ProjectName(),
					},
					Spec: gardencorev1beta1.ShootSpec{
						Region:            region,
						SecretBindingName: secretBindingName,
						Provider:          *provider.DeepCopy(),
					},
				}
				secretBinding = &gardencorev1beta1.SecretBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      secretBindingName,
						Namespace: shoot.Namespace,
					},
					SecretRef: *secretRef.DeepCopy(),
				}
				secret = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: secretRef.Namespace,
						Name:      secretRef.Name,
					},
					Data: map[string][]byte{
						"serviceaccount.json": []byte(readTestFile(filepath.Join(provider.Type, "serviceaccount.json"))),
					},
				}
			})

			Context("when the command runs successfully", func() {
				BeforeEach(func() {
					factory.EXPECT().Manager().Return(manager, nil)
					manager.EXPECT().GardenClient(t.GardenName()).Return(client, nil)
					factory.EXPECT().Context().Return(ctx)
				})

				JustBeforeEach(func() {
					client.EXPECT().GetSecretBinding(ctx, shoot.Namespace, shoot.Spec.SecretBindingName).Return(secretBinding, nil)
					client.EXPECT().GetSecret(ctx, secretBinding.SecretRef.Namespace, secretBinding.SecretRef.Name).Return(secret, nil)
				})

				Context("and the shoot is targeted via project", func() {
					JustBeforeEach(func() {
						manager.EXPECT().CurrentTarget().Return(t.WithSeedName(""), nil)
						client.EXPECT().GetShootByProject(ctx, t.ProjectName(), t.ShootName()).Return(shoot, nil)
					})

					It("does the work when the shoot is targeted via project", func() {
						Expect(options.Run(factory)).To(Succeed())
						Expect(options.Out()).To(Equal(readTestFile("gcp/export.bash")))
					})

					It("should print how to reset configuration for powershell", func() {
						options.Unset = true
						options.Shell = "powershell"
						Expect(options.Run(factory)).To(Succeed())
						Expect(options.Out()).To(Equal(readTestFile("gcp/unset.pwsh")))
					})
				})

				Context("and the shoot is targeted via seed", func() {
					JustBeforeEach(func() {
						manager.EXPECT().CurrentTarget().Return(t.WithProjectName(""), nil)
						client.EXPECT().GetShootBySeed(ctx, t.SeedName(), t.ShootName()).Return(shoot, nil)
					})

					It("does the work when the shoot is targeted via seed", func() {
						Expect(options.Run(factory)).To(Succeed())
						Expect(options.Out()).To(Equal(readTestFile("gcp/export.bash")))
					})
				})
			})

			Context("when an error occurs before running the command", func() {
				err := errors.New("error")
				t := target.NewTarget("test", "project", "seed", "shoot")

				It("should fail with ManagerError", func() {
					factory.EXPECT().Manager().Return(nil, err)
					Expect(options.Run(factory)).To(BeIdenticalTo(err))
				})

				It("should fail with CurrentTargetError", func() {
					factory.EXPECT().Manager().Return(manager, nil)
					manager.EXPECT().CurrentTarget().Return(nil, err)
					Expect(options.Run(factory)).To(BeIdenticalTo(err))
				})

				It("should fail with ErrNoShootTargeted", func() {
					factory.EXPECT().Manager().Return(manager, nil)
					manager.EXPECT().CurrentTarget().Return(t.WithShootName(""), nil)
					Expect(options.Run(factory)).To(BeIdenticalTo(cloudenv.ErrNoShootTargeted))
				})

				It("should fail with ErrNeitherProjectNorSeedTargeted", func() {
					factory.EXPECT().Manager().Return(manager, nil)
					manager.EXPECT().CurrentTarget().Return(t.WithSeedName("").WithProjectName(""), nil)
					Expect(options.Run(factory)).To(BeIdenticalTo(cloudenv.ErrNeitherProjectNorSeedTargeted))
				})

				It("should fail with ErrProjectAndSeedTargeted", func() {
					factory.EXPECT().Manager().Return(manager, nil)
					manager.EXPECT().CurrentTarget().Return(t, nil)
					Expect(options.Run(factory)).To(BeIdenticalTo(cloudenv.ErrProjectAndSeedTargeted))
				})

				It("should fail with GardenClientError", func() {
					factory.EXPECT().Manager().Return(manager, nil)
					manager.EXPECT().CurrentTarget().Return(t.WithSeedName(""), nil)
					manager.EXPECT().GardenClient(t.GardenName()).Return(nil, err)
					Expect(options.Run(factory)).To(MatchError("failed to create garden cluster client: error"))
				})

				Context("and the error occurs with the GardenClient instance", func() {
					BeforeEach(func() {
						factory.EXPECT().Manager().Return(manager, nil)
						manager.EXPECT().GardenClient(t.GardenName()).Return(client, nil)
						factory.EXPECT().Context().Return(ctx)
					})

					It("should fail with GetShootByProjectError", func() {
						manager.EXPECT().CurrentTarget().Return(t.WithSeedName(""), nil)
						client.EXPECT().GetShootByProject(ctx, t.ProjectName(), t.ShootName()).Return(nil, err)
						Expect(options.Run(factory)).To(BeIdenticalTo(err))
					})

					It("should fail with GetShootBySeedError", func() {
						manager.EXPECT().CurrentTarget().Return(t.WithProjectName(""), nil)
						client.EXPECT().GetShootBySeed(ctx, t.SeedName(), t.ShootName()).Return(nil, err)
						Expect(options.Run(factory)).To(BeIdenticalTo(err))
					})

					It("should fail with GetSecretBindingError", func() {
						manager.EXPECT().CurrentTarget().Return(t.WithSeedName(""), nil)
						client.EXPECT().GetShootByProject(ctx, t.ProjectName(), t.ShootName()).Return(shoot, nil)
						client.EXPECT().GetSecretBinding(ctx, shoot.Namespace, shoot.Spec.SecretBindingName).Return(nil, err)
						Expect(options.Run(factory)).To(BeIdenticalTo(err))
					})

					It("should fail with GetSecretError", func() {
						manager.EXPECT().CurrentTarget().Return(t.WithSeedName(""), nil)
						client.EXPECT().GetShootByProject(ctx, t.ProjectName(), t.ShootName()).Return(shoot, nil)
						client.EXPECT().GetSecretBinding(ctx, shoot.Namespace, shoot.Spec.SecretBindingName).Return(secretBinding, nil)
						client.EXPECT().GetSecret(ctx, secretBinding.SecretRef.Namespace, secretBinding.SecretRef.Name).Return(nil, err)
						Expect(options.Run(factory)).To(BeIdenticalTo(err))
					})
				})
			})
		})

		Describe("rendering the template", func() {
			var (
				shell,
				use,
				namespace,
				shootName,
				secretName,
				region,
				providerType,
				serviceaccountJSON,
				token string
				unset  bool
				shoot  *gardencorev1beta1.Shoot
				secret *corev1.Secret
			)

			BeforeEach(func() {
				shell = "bash"
				unset = false
				use = "cloud-env"
				namespace = "garden-test"
				shootName = "shoot"
				secretName = "secret"
				region = "europe"
				providerType = "gcp"
				serviceaccountJSON = readTestFile("gcp/serviceaccount.json")
				token = "token"
			})

			JustBeforeEach(func() {
				shoot = &gardencorev1beta1.Shoot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      shootName,
						Namespace: namespace,
					},
					Spec: gardencorev1beta1.ShootSpec{
						Region: region,
						Provider: gardencorev1beta1.Provider{
							Type: providerType,
						},
					},
				}
				secret = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      secretName,
						Namespace: namespace,
					},
					Data: map[string][]byte{
						"serviceaccount.json": []byte(serviceaccountJSON),
						"testToken":           []byte(token),
					},
				}
				options.GardenDir = gardenHomeDir
				options.Shell = shell
				options.Unset = unset
				options.CmdPath = []string{use}
			})

			Context("when configuring the shell", func() {
				BeforeEach(func() {
					unset = false
				})

				It("should render the template successfully", func() {
					Expect(options.ExecTmpl(shoot, secret)).To(Succeed())
					Expect(options.Out()).To(Equal(readTestFile("gcp/export.bash")))
				})
			})

			Context("when resetting the shell configuration", func() {
				BeforeEach(func() {
					unset = true
					shell = "powershell"
				})

				It("should render the template successfully", func() {
					Expect(options.ExecTmpl(shoot, secret)).To(Succeed())
					Expect(options.Out()).To(Equal(readTestFile("gcp/unset.pwsh")))
				})
			})

			Context("when JSON input is invalid", func() {
				BeforeEach(func() {
					serviceaccountJSON = "{"
				})

				It("should fail to render the template with JSON parse error", func() {
					Expect(options.ExecTmpl(shoot, secret)).To(MatchError("unexpected end of JSON input"))
				})
			})

			Context("when the shell is invalid", func() {
				BeforeEach(func() {
					shell = "cmd"
				})

				It("should fail to render the template with JSON parse error", func() {
					noTemplateFmt := "template: no template %q associated with template %q"
					Expect(options.ExecTmpl(shoot, secret)).To(MatchError(fmt.Sprintf(noTemplateFmt, shell, "base")))
				})
			})

			Context("when the cloudprovider template is found in garden home dir", func() {
				var filename string

				BeforeEach(func() {
					providerType = "test"
					filename = filepath.Join("templates", providerType+".tmpl")
					writeTempFile(filename, readTestFile(filename))
				})

				AfterEach(func() {
					removeTempFile(filename)
				})

				It("should render the template successfully", func() {
					Expect(options.ExecTmpl(shoot, secret)).To(Succeed())
					Expect(options.Out()).To(Equal(readTestFile("test/export.bash")))
				})
			})

			Context("when the cloudprovider template is not found", func() {
				BeforeEach(func() {
					providerType = "not-found"
				})

				It("should fail to render the template with a not supported error", func() {
					notSupportedFmt := "cloudprovider %q is not supported"
					Expect(options.ExecTmpl(shoot, secret)).To(MatchError(MatchRegexp(fmt.Sprintf(notSupportedFmt, providerType))))
				})
			})

			Context("when the cloudprovider template could not be parsed", func() {
				var filename string

				BeforeEach(func() {
					providerType = "fail"
					filename = filepath.Join("templates", providerType+".tmpl")
					writeTempFile(filename, "{{define \"bash\"}}\nexport TEST_TOKEN={{.testToken | quote }};")
				})

				AfterEach(func() {
					removeTempFile(filename)
				})

				It("should fail to render the template with a not supported error", func() {
					parseErrorFmt := "parsing template for cloudprovider %q failed"
					Expect(options.ExecTmpl(shoot, secret)).To(MatchError(MatchRegexp(fmt.Sprintf(parseErrorFmt, providerType))))
				})
			})
		})
	})

	Describe("detecting the default shell", func() {
		originalShell := os.Getenv("SHELL")

		AfterEach(func() {
			Expect(os.Setenv("SHELL", originalShell)).To(Succeed())
		})

		It("should return the default shell ", func() {
			Expect(os.Unsetenv("SHELL")).To(Succeed())
			By("Running on Darwin")
			Expect(cloudenv.DetectShell("darwin")).To(Equal("bash"))
			By("Running on Windows")
			Expect(cloudenv.DetectShell("windows")).To(Equal("powershell"))
		})

		It("should return the shell defined in the environment", func() {
			Expect(os.Setenv("SHELL", "/bin/fish")).To(Succeed())
			Expect(cloudenv.DetectShell("*")).To(Equal("fish"))
		})
	})

	Describe("validating the shell", func() {
		It("should succeed for all valid shells", func() {
			Expect(cloudenv.ValidateShell("bash")).To(Succeed())
			Expect(cloudenv.ValidateShell("zsh")).To(Succeed())
			Expect(cloudenv.ValidateShell("fish")).To(Succeed())
			Expect(cloudenv.ValidateShell("powershell")).To(Succeed())
		})

		It("should fail for an currently unsupported shell", func() {
			Expect(cloudenv.ValidateShell("cmd")).To(MatchError(fmt.Sprintf("invalid shell given, must be one of %v", cloudenv.ValidShells)))
		})
	})

	Describe("parsing google serviceaccount credentials", func() {
		var (
			serviceaccountJSON string
			values             map[string]interface{}
		)

		BeforeEach(func() {
			serviceaccountJSON = readTestFile("gcp/serviceaccount.json")
		})

		JustBeforeEach(func() {
			values = map[string]interface{}{
				"serviceaccount.json": serviceaccountJSON,
			}
		})

		It("should succeed for all valid shells", func() {
			Expect(cloudenv.ParseCredentials(&values)).To(Succeed())
			Expect(values).To(HaveKeyWithValue("serviceaccount.json", "{\"client_email\":\"test@example.org\",\"project_id\":\"test\"}"))
			Expect(values).To(HaveKey("credentials"))
			Expect(values["credentials"]).To(HaveKeyWithValue("project_id", "test"))
			Expect(values["credentials"]).To(HaveKeyWithValue("client_email", "test@example.org"))
		})

		It("should fail with invalid secret", func() {
			values["serviceaccount.json"] = nil
			Expect(cloudenv.ParseCredentials(&values)).To(MatchError("Invalid serviceaccount in secret"))
		})

		It("should fail with invalid json", func() {
			values["serviceaccount.json"] = "{"
			Expect(cloudenv.ParseCredentials(&values)).To(MatchError("unexpected end of JSON input"))
		})

		It("should fail with invalid json", func() {
			values["serviceaccount.json"] = "{"
			Expect(cloudenv.ParseCredentials(&values)).To(MatchError("unexpected end of JSON input"))
		})
	})

	Describe("generating the usage hint", func() {
		const (
			exportFmt = "# Run this command to configure the %q CLI for your shell:"
			unsetFmt  = "# Run this command to reset the configuration of the %q CLI for your shell:"
		)

		var (
			cloudprovider,
			shell string
			unset bool
			args  []string
			first,
			second string
		)

		BeforeEach(func() {
			cloudprovider = "aws"
			shell = "bash"
			unset = false
			args = []string{"test", "env"}
		})

		JustBeforeEach(func() {
			hint := cloudenv.GenerateUsageHint(cloudprovider, shell, unset, args...)
			lines := strings.Split(hint, "\n")
			Expect(lines).To(HaveLen(2))
			first = lines[0]
			second = lines[1]
		})

		Context("when configuring the shell", func() {
			It("should generate the usage hint", func() {
				Expect(first).To(Equal(fmt.Sprintf(exportFmt, cloudprovider)))
				Expect(second).To(Equal(fmt.Sprintf("# eval $(test env %s)", shell)))
			})
		})

		Context("when resetting the shell configuration", func() {
			BeforeEach(func() {
				unset = true
			})

			It("should generate the usage hint", func() {
				Expect(first).To(Equal(fmt.Sprintf(unsetFmt, cloudprovider)))
				Expect(second).To(Equal(fmt.Sprintf("# eval $(test env -u %s)", shell)))
			})
		})

		Context("when clouprovider is alicloud", func() {
			BeforeEach(func() {
				cloudprovider = "alicloud"
			})

			It("should generate the usage hint", func() {
				Expect(first).To(Equal(fmt.Sprintf(exportFmt, "aliyun")))
			})
		})

		Context("when clouprovider is gcp", func() {
			BeforeEach(func() {
				cloudprovider = "gcp"
			})

			It("should generate the usage hint", func() {
				Expect(first).To(Equal(fmt.Sprintf(exportFmt, "gcloud")))
			})
		})

		Context("when shell is fish", func() {
			BeforeEach(func() {
				shell = "fish"
			})

			It("should generate the usage hint", func() {
				Expect(second).To(Equal(fmt.Sprintf("# eval (test env %s)", shell)))
			})
		})

		Context("when shell is powershell", func() {
			BeforeEach(func() {
				shell = "powershell"
			})

			It("should generate the usage hint", func() {
				Expect(second).To(Equal(fmt.Sprintf("# & test env %s | Invoke-Expression", shell)))
			})
		})
	})
})
