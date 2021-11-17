/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package cloudenv_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	gardenclientmocks "github.com/gardener/gardenctl-v2/internal/gardenclient/mocks"
	utilmocks "github.com/gardener/gardenctl-v2/internal/util/mocks"
	"github.com/gardener/gardenctl-v2/pkg/cmd/cloudenv"
	"github.com/gardener/gardenctl-v2/pkg/target"
	targetmocks "github.com/gardener/gardenctl-v2/pkg/target/mocks"
	openstackv1alpha1 "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

var _ = Describe("CloudEnv Options", func() {
	Describe("having an instance", func() {
		var (
			ctrl    *gomock.Controller
			factory *utilmocks.MockFactory
			options *cloudenv.TestOptions
			cmdPath,
			shell string
			unset bool
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			factory = utilmocks.NewMockFactory(ctrl)
			options = cloudenv.NewTestOptions()
			cmdPath = "gardenctl cloud-env"
			shell = "default"
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		JustBeforeEach(func() {
			options.Shell = shell
			options.CmdPath = cmdPath
			options.Unset = unset
		})

		Describe("completing the command options", func() {
			var (
				root,
				parent,
				child *cobra.Command
			)

			BeforeEach(func() {
				root = &cobra.Command{Use: "root"}
				parent = &cobra.Command{Use: "parent"}
				child = &cobra.Command{Use: "child"}
				parent.AddCommand(child)
				root.AddCommand(parent)
				factory.EXPECT().GardenHomeDir().Return(gardenHomeDir)
			})

			It("should complete options with default shell", func() {
				Expect(options.Complete(factory, child, nil)).To(Succeed())
				Expect(options.Shell).To(Equal(child.Name()))
				Expect(options.GardenDir).To(Equal(gardenHomeDir))
				Expect(options.CmdPath).To(Equal(root.Name() + " " + parent.Name()))
			})
		})

		Describe("validating the command options", func() {
			It("should successfully validate the options", func() {
				options.Shell = "bash"
				Expect(options.Validate()).To(Succeed())
			})

			It("should return an error when the shell is empty", func() {
				options.Shell = ""
				Expect(options.Validate()).To(MatchError(pflag.ErrHelp))
			})

			It("should return an error when the shell is invalid", func() {
				options.Shell = "cmd"
				Expect(options.Validate()).To(MatchError(fmt.Sprintf("invalid shell given, must be one of %v", cloudenv.ValidShells)))
			})
		})

		Describe("adding the command flags", func() {
			It("should successfully add the unset flag", func() {
				cmd := &cobra.Command{}
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
				cloudProfileName  string
				region            string
				provider          *gardencorev1beta1.Provider
				secretRef         *corev1.SecretReference
				shoot             *gardencorev1beta1.Shoot
				secretBinding     *gardencorev1beta1.SecretBinding
				cloudProfile      *gardencorev1beta1.CloudProfile
				providerConfig    *openstackv1alpha1.CloudProfileConfig
				secret            *corev1.Secret
			)

			BeforeEach(func() {
				ctx = context.Background()
				manager = targetmocks.NewMockManager(ctrl)
				client = gardenclientmocks.NewMockClient(ctrl)
				t = target.NewTarget("test", "project", "seed", "shoot")
				secretBindingName = "secret-binding"
				cloudProfileName = "cloud-profile"
				region = "europe"
				provider = &gardencorev1beta1.Provider{
					Type: "gcp",
				}
				providerConfig = nil
				secretRef = &corev1.SecretReference{
					Namespace: "private",
					Name:      "secret",
				}
				shell = "bash"
			})

			JustBeforeEach(func() {
				shoot = &gardencorev1beta1.Shoot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      t.ShootName(),
						Namespace: "garden-" + t.ProjectName(),
					},
					Spec: gardencorev1beta1.ShootSpec{
						CloudProfileName:  cloudProfileName,
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
				cloudProfile = &gardencorev1beta1.CloudProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name: t.ShootName(),
					},
					Spec: gardencorev1beta1.CloudProfileSpec{
						Type: provider.Type,
						ProviderConfig: &runtime.RawExtension{
							Object: providerConfig,
							Raw:    nil,
						},
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
					client.EXPECT().GetCloudProfile(ctx, shoot.Spec.CloudProfileName).Return(cloudProfile, nil)
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
				namespace,
				shootName,
				secretName,
				cloudProfileName,
				region,
				providerType,
				serviceaccountJSON,
				token string
				shoot          *gardencorev1beta1.Shoot
				secret         *corev1.Secret
				cloudProfile   *gardencorev1beta1.CloudProfile
				providerConfig *openstackv1alpha1.CloudProfileConfig
			)

			BeforeEach(func() {
				shell = "bash"
				unset = false
				namespace = "garden-test"
				shootName = "shoot"
				secretName = "secret"
				cloudProfileName = "cloud-profile"
				region = "europe"
				providerType = "gcp"
				providerConfig = nil
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
						CloudProfileName: cloudProfileName,
						Region:           region,
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
				cloudProfile = &gardencorev1beta1.CloudProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name: cloudProfileName,
					},
					Spec: gardencorev1beta1.CloudProfileSpec{
						Type: providerType,
						ProviderConfig: &runtime.RawExtension{
							Object: providerConfig,
							Raw:    nil,
						},
					},
				}
				options.GardenDir = gardenHomeDir
			})

			Context("when configuring the shell", func() {
				BeforeEach(func() {
					unset = false
				})

				It("should render the template successfully", func() {
					Expect(options.ExecTmpl(shoot, secret, cloudProfile)).To(Succeed())
					Expect(options.Out()).To(Equal(readTestFile("gcp/export.bash")))
				})
			})

			Context("when resetting the shell configuration", func() {
				BeforeEach(func() {
					unset = true
					shell = "powershell"
				})

				It("should render the template successfully", func() {
					Expect(options.ExecTmpl(shoot, secret, cloudProfile)).To(Succeed())
					Expect(options.Out()).To(Equal(readTestFile("gcp/unset.pwsh")))
				})
			})

			Context("when JSON input is invalid", func() {
				JustBeforeEach(func() {
					secret.Data["serviceaccount.json"] = []byte("{")
				})

				It("should fail to render the template with JSON parse error", func() {
					Expect(options.ExecTmpl(shoot, secret, cloudProfile)).To(MatchError("unexpected end of JSON input"))
				})
			})

			Context("when the shell is invalid", func() {
				BeforeEach(func() {
					shell = "cmd"
				})

				It("should fail to render the template with JSON parse error", func() {
					noTemplateFmt := "template: no template %q associated with template %q"
					Expect(options.ExecTmpl(shoot, secret, cloudProfile)).To(MatchError(fmt.Sprintf(noTemplateFmt, shell, "base")))
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
					Expect(options.ExecTmpl(shoot, secret, cloudProfile)).To(Succeed())
					Expect(options.Out()).To(Equal(readTestFile("test/export.bash")))
				})
			})

			Context("when the cloudprovider template is not found", func() {
				BeforeEach(func() {
					providerType = "not-found"
				})

				It("should fail to render the template with a not supported error", func() {
					notSupportedFmt := "cloud provider %q is not supported"
					Expect(options.ExecTmpl(shoot, secret, cloudProfile)).To(MatchError(MatchRegexp(fmt.Sprintf(notSupportedFmt, providerType))))
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
					parseErrorFmt := "parsing template for cloud provider %q failed"
					Expect(options.ExecTmpl(shoot, secret, cloudProfile)).To(MatchError(MatchRegexp(fmt.Sprintf(parseErrorFmt, providerType))))
				})
			})

			Context("when the cloudprovider is openstack", func() {
				const (
					username   = "user"
					password   = "secret"
					tenantName = "tenant"
					domainName = "domain"
				)

				BeforeEach(func() {
					providerType = "openstack"
					providerConfig = &openstackv1alpha1.CloudProfileConfig{KeyStoneURL: "keyStoneURL"}
				})

				JustBeforeEach(func() {
					secret.Data["username"] = []byte(username)
					secret.Data["password"] = []byte(password)
					secret.Data["tenantName"] = []byte(tenantName)
					secret.Data["domainName"] = []byte(domainName)
				})

				It("should render the template successfully", func() {
					Expect(options.ExecTmpl(shoot, secret, cloudProfile)).To(Succeed())
					Expect(options.Out()).To(Equal(readTestFile("openstack/export.bash")))
				})
			})
		})

		Describe("generating the usage hint", func() {
			const (
				exportFmt = "# Run this command to configure the %q CLI for your shell:"
				unsetFmt  = "# Run this command to reset the configuration of the %q CLI for your shell:"
			)

			var (
				providerType string
				hint         []string
			)

			BeforeEach(func() {
				providerType = "aws"
				shell = "bash"
				unset = false
			})

			JustBeforeEach(func() {
				hint = strings.Split(options.GenerateUsageHint(providerType), "\n")
				Expect(hint).To(HaveLen(2))
			})

			Context("when configuring the shell", func() {
				It("should generate the usage hint", func() {
					Expect(hint[0]).To(Equal(fmt.Sprintf(exportFmt, providerType)))
					Expect(hint[1]).To(Equal(fmt.Sprintf("# eval $(%s %s)", cmdPath, shell)))
				})
			})

			Context("when resetting the shell configuration", func() {
				BeforeEach(func() {
					unset = true
				})

				It("should generate the usage hint", func() {
					Expect(hint[0]).To(Equal(fmt.Sprintf(unsetFmt, providerType)))
					Expect(hint[1]).To(Equal(fmt.Sprintf("# eval $(%s -u %s)", cmdPath, shell)))
				})
			})

			Context("when clouprovider is alicloud", func() {
				BeforeEach(func() {
					providerType = "alicloud"
				})

				It("should generate the usage hint", func() {
					Expect(hint[0]).To(Equal(fmt.Sprintf(exportFmt, "aliyun")))
				})
			})

			Context("when clouprovider is gcp", func() {
				BeforeEach(func() {
					providerType = "gcp"
				})

				It("should generate the usage hint", func() {
					Expect(hint[0]).To(Equal(fmt.Sprintf(exportFmt, "gcloud")))
				})
			})

			Context("when shell is fish", func() {
				BeforeEach(func() {
					shell = "fish"
				})

				It("should generate the usage hint", func() {
					Expect(hint[1]).To(Equal(fmt.Sprintf("# eval (%s %s)", cmdPath, shell)))
				})
			})

			Context("when shell is powershell", func() {
				BeforeEach(func() {
					shell = "powershell"
				})

				It("should generate the usage hint", func() {
					Expect(hint[1]).To(Equal(fmt.Sprintf("# & %s %s | Invoke-Expression", cmdPath, shell)))
				})
			})
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

	Describe("modifying the values before template execution", func() {
		var (
			c      cloudenv.CloudProvider
			values map[string]interface{}
		)

		BeforeEach(func() {
			values = map[string]interface{}{}
		})

		Context("when the cloud provider is not gcp", func() {
			BeforeEach(func() {
				c = cloudenv.CloudProvider("foo")
			})

			It("should do nothing", func() {
				Expect(cloudenv.BeforeExecuteTemplate(c, &values)).To(Succeed())
			})
		})

		Context("when the cloud provider is gcp", func() {
			var serviceaccountJSON = readTestFile("gcp/serviceaccount.json")

			BeforeEach(func() {
				c = cloudenv.CloudProvider("gcp")
				values["serviceaccount.json"] = serviceaccountJSON
			})

			It("should succeed for all valid shells", func() {
				Expect(cloudenv.BeforeExecuteTemplate(c, &values)).To(Succeed())
				Expect(values).To(HaveKeyWithValue("serviceaccount.json", "{\"client_email\":\"test@example.org\",\"project_id\":\"test\"}"))
				Expect(values).To(HaveKey("credentials"))
				Expect(values["credentials"]).To(HaveKeyWithValue("project_id", "test"))
				Expect(values["credentials"]).To(HaveKeyWithValue("client_email", "test@example.org"))
			})

			It("should fail with invalid secret", func() {
				values["serviceaccount.json"] = nil
				Expect(cloudenv.BeforeExecuteTemplate(c, &values)).To(MatchError("Invalid serviceaccount in Secret"))
			})

			It("should fail with invalid json", func() {
				values["serviceaccount.json"] = "{"
				Expect(cloudenv.BeforeExecuteTemplate(c, &values)).To(MatchError("unexpected end of JSON input"))
			})

			It("should fail with invalid json", func() {
				values["serviceaccount.json"] = "{"
				Expect(cloudenv.BeforeExecuteTemplate(c, &values)).To(MatchError("unexpected end of JSON input"))
			})
		})
	})

})
