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
	"regexp"

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
	Describe("having an Options instance", func() {
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
				t = target.NewTarget("test", "project", "seed", "shoot", false)
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
						currentTarget := t.WithSeedName("")
						manager.EXPECT().CurrentTarget().Return(currentTarget, nil)
						client.EXPECT().FindShoot(ctx, currentTarget.AsListOption()).Return(shoot, nil)
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
						currentTarget := t.WithProjectName("")
						manager.EXPECT().CurrentTarget().Return(currentTarget, nil)
						client.EXPECT().FindShoot(ctx, currentTarget.AsListOption()).Return(shoot, nil)
					})

					It("does the work when the shoot is targeted via seed", func() {
						Expect(options.Run(factory)).To(Succeed())
						Expect(options.Out()).To(Equal(readTestFile("gcp/export.seed.bash")))
					})
				})
			})

			Context("when an error occurs before running the command", func() {
				err := errors.New("error")

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
						currentTarget := t.WithSeedName("")
						manager.EXPECT().CurrentTarget().Return(currentTarget, nil)
						client.EXPECT().FindShoot(ctx, currentTarget.AsListOption()).Return(nil, err)
						Expect(options.Run(factory)).To(BeIdenticalTo(err))
					})

					It("should fail with GetShootBySeedError", func() {
						currentTarget := t.WithProjectName("")
						manager.EXPECT().CurrentTarget().Return(currentTarget, nil)
						client.EXPECT().FindShoot(ctx, currentTarget.AsListOption()).Return(nil, err)
						Expect(options.Run(factory)).To(BeIdenticalTo(err))
					})

					It("should fail with GetSecretBindingError", func() {
						currentTarget := t.WithSeedName("")
						manager.EXPECT().CurrentTarget().Return(currentTarget, nil)
						client.EXPECT().FindShoot(ctx, currentTarget.AsListOption()).Return(shoot, nil)
						client.EXPECT().GetSecretBinding(ctx, shoot.Namespace, shoot.Spec.SecretBindingName).Return(nil, err)
						Expect(options.Run(factory)).To(BeIdenticalTo(err))
					})

					It("should fail with GetSecretError", func() {
						currentTarget := t.WithSeedName("")
						manager.EXPECT().CurrentTarget().Return(currentTarget, nil)
						client.EXPECT().FindShoot(ctx, currentTarget.AsListOption()).Return(shoot, nil)
						client.EXPECT().GetSecretBinding(ctx, shoot.Namespace, shoot.Spec.SecretBindingName).Return(secretBinding, nil)
						client.EXPECT().GetSecret(ctx, secretBinding.SecretRef.Namespace, secretBinding.SecretRef.Name).Return(nil, err)
						Expect(options.Run(factory)).To(BeIdenticalTo(err))
					})

					It("should fail with GetCloudProfileError", func() {
						currentTarget := t.WithSeedName("")
						manager.EXPECT().CurrentTarget().Return(currentTarget, nil)
						client.EXPECT().FindShoot(ctx, currentTarget.AsListOption()).Return(shoot, nil)
						client.EXPECT().GetSecretBinding(ctx, shoot.Namespace, shoot.Spec.SecretBindingName).Return(secretBinding, nil)
						client.EXPECT().GetSecret(ctx, secretBinding.SecretRef.Namespace, secretBinding.SecretRef.Name).Return(secret, nil)
						client.EXPECT().GetCloudProfile(ctx, cloudProfileName).Return(nil, err)
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
				options.CurrentTarget = target.NewTarget("test", "project", "", shootName, false)
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
					writeTempFile(filename, "{{define \"bash\"}}\nexport TEST_TOKEN={{.testToken | quote}};")
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

				It("should fail with invalid provider config", func() {
					cloudProfile.Spec.ProviderConfig = nil
					Expect(options.ExecTmpl(shoot, secret, cloudProfile)).To(MatchError(MatchRegexp("^failed to get openstack provider config:")))
				})
			})
		})

		Describe("rendering the usage hint", func() {
			var (
				providerType,
				targetFlags,
				cli string
				meta map[string]interface{}
				t    = target.NewTarget("test", "project", "seed", "shoot", false)
			)

			BeforeEach(func() {
				providerType = "alicloud"
				cli = cloudenv.CloudProvider(providerType).CLI()
				shell = "bash"
				unset = false
				options.CurrentTarget = t.WithSeedName("")
			})

			JustBeforeEach(func() {
				meta = options.GenerateMetadata(providerType)
				targetFlags = fmt.Sprintf("--garden %s --project %s --shoot %s", t.GardenName(), t.ProjectName(), t.ShootName())
				Expect(cloudenv.BaseTemplate().ExecuteTemplate(options.IOStreams.Out, "usage-hint", meta)).To(Succeed())
			})

			Context("when configuring the shell", func() {
				It("should generate the metadata and render the export hint", func() {
					Expect(meta["unset"]).To(BeFalse())
					Expect(meta["shell"]).To(Equal(shell))
					Expect(meta["cli"]).To(Equal(cli))
					Expect(meta["commandPath"]).To(Equal(options.CmdPath))
					Expect(meta["targetFlags"]).To(Equal(targetFlags))
					regex := regexp.MustCompile(`(?m)\Aprintf '(.*)\\n\\n(.*)\\n(.*)\\n';\n\n(.*)\n(.*)\n\z`)
					match := regex.FindStringSubmatch(options.Out())
					Expect(match).NotTo(BeNil())
					Expect(len(match)).To(Equal(6))
					Expect(match[1]).To(Equal(fmt.Sprintf("Successfully configured the %q CLI for your current shell session.", cli)))
					Expect(match[2]).To(Equal("# Run the following command to reset this configuration:"))
					Expect(match[3]).To(Equal(fmt.Sprintf("# eval $(%s %s -u %s)", options.CmdPath, targetFlags, shell)))
					Expect(match[4]).To(Equal(fmt.Sprintf("# Run this command to configure the %q CLI for your shell:", cli)))
					Expect(match[5]).To(Equal(fmt.Sprintf("# eval $(%s %s)", options.CmdPath, shell)))
				})
			})

			Context("when resetting the shell configuration", func() {
				BeforeEach(func() {
					unset = true
				})

				It("should generate the metadata and render the unset", func() {
					Expect(meta["unset"]).To(BeTrue())
					regex := regexp.MustCompile(`(?m)\A\n(.*)\n(.*)\n\z`)
					match := regex.FindStringSubmatch(options.Out())
					Expect(match).NotTo(BeNil())
					Expect(len(match)).To(Equal(3))
					Expect(match[1]).To(Equal(fmt.Sprintf("# Run this command to reset the configuration of the %q CLI for your shell:", cli)))
					Expect(match[2]).To(Equal(fmt.Sprintf("# eval $(%s -u %s)", options.CmdPath, shell)))
				})
			})
		})
	})

	Describe("Shell", func() {
		Describe("validation", func() {
			It("should succeed for all valid shells", func() {
				Expect(cloudenv.Shell("bash").Validate()).To(Succeed())
				Expect(cloudenv.Shell("zsh").Validate()).To(Succeed())
				Expect(cloudenv.Shell("fish").Validate()).To(Succeed())
				Expect(cloudenv.Shell("powershell").Validate()).To(Succeed())
			})

			It("should fail for a currently unsupported shell", func() {
				Expect(cloudenv.Shell("cmd").Validate()).To(MatchError(fmt.Sprintf("invalid shell given, must be one of %v", cloudenv.ValidShells)))
			})
		})

		Describe("getting the prompt", func() {
			It("should return the typical prompt for the given shell and goos", func() {
				Expect(cloudenv.Shell("bash").Prompt("linux")).To(Equal("$ "))
				Expect(cloudenv.Shell("powershell").Prompt("darwin")).To(Equal("PS /> "))
				Expect(cloudenv.Shell("powershell").Prompt("windows")).To(Equal("PS C:\\> "))
			})
		})
	})

	Describe("parsing gcp credentials", func() {
		var (
			serviceaccountJSON = readTestFile("gcp/serviceaccount.json")
			secretName         = "gcp"
			secret             *corev1.Secret
			credentials        map[string]interface{}
		)

		BeforeEach(func() {
			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: secretName,
				},
				Data: map[string][]byte{
					"serviceaccount.json": []byte(serviceaccountJSON),
				},
			}
			credentials = make(map[string]interface{})
		})

		It("should succeed for all valid shells", func() {
			data, err := cloudenv.ParseGCPCredentials(secret, &credentials)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(Equal("{\"client_email\":\"test@example.org\",\"project_id\":\"test\"}"))
			Expect(credentials).To(HaveKeyWithValue("project_id", "test"))
			Expect(credentials).To(HaveKeyWithValue("client_email", "test@example.org"))
		})

		It("should fail with invalid secret", func() {
			secret.Data["serviceaccount.json"] = nil
			_, err := cloudenv.ParseGCPCredentials(secret, &credentials)
			Expect(err).To(MatchError(fmt.Sprintf("no \"serviceaccount.json\" data in Secret %q", secretName)))
		})

		It("should fail with invalid json", func() {
			secret.Data["serviceaccount.json"] = []byte("{")
			_, err := cloudenv.ParseGCPCredentials(secret, &credentials)
			Expect(err).To(MatchError("unexpected end of JSON input"))
		})
	})

	Describe("getting the keyStoneURL", func() {
		var (
			cloudProfileName   = "cloud-profile-name"
			region             = "europe"
			cloudProfile       *gardencorev1beta1.CloudProfile
			cloudProfileConfig *openstackv1alpha1.CloudProfileConfig
		)

		BeforeEach(func() {
			cloudProfileConfig = &openstackv1alpha1.CloudProfileConfig{
				KeyStoneURLs: []openstackv1alpha1.KeyStoneURL{
					{URL: "bar", Region: region},
				},
			}
			cloudProfile = &gardencorev1beta1.CloudProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: cloudProfileName,
				},
				Spec: gardencorev1beta1.CloudProfileSpec{
					ProviderConfig: &runtime.RawExtension{
						Object: cloudProfileConfig,
						Raw:    nil,
					},
				},
			}
		})

		It("should return a global url", func() {
			cloudProfileConfig.KeyStoneURL = "foo"
			url, err := cloudenv.GetKeyStoneURL(cloudProfile, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(url).To(Equal(cloudProfileConfig.KeyStoneURL))
		})

		It("should return region specific url", func() {
			url, err := cloudenv.GetKeyStoneURL(cloudProfile, region)
			Expect(err).NotTo(HaveOccurred())
			Expect(url).To(Equal("bar"))
		})

		It("should fail with not found", func() {
			cloudProfile.Spec.ProviderConfig = nil
			_, err := cloudenv.GetKeyStoneURL(cloudProfile, region)
			Expect(err).To(MatchError(MatchRegexp("^failed to get openstack provider config:")))
		})

		It("should fail with not found", func() {
			region = "asia"
			_, err := cloudenv.GetKeyStoneURL(cloudProfile, region)
			Expect(err).To(MatchError(fmt.Sprintf("cannot find keystone URL for region %q in cloudprofile %q", region, cloudProfileName)))
		})
	})
})
