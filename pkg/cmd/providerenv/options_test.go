/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package providerenv_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	openstackv1alpha1 "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	corev1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardensecurityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	clientgarden "github.com/gardener/gardenctl-v2/internal/client/garden"
	gardenclientmocks "github.com/gardener/gardenctl-v2/internal/client/garden/mocks"
	utilmocks "github.com/gardener/gardenctl-v2/internal/util/mocks"
	"github.com/gardener/gardenctl-v2/pkg/cmd/providerenv"
	"github.com/gardener/gardenctl-v2/pkg/config"
	"github.com/gardener/gardenctl-v2/pkg/env"
	"github.com/gardener/gardenctl-v2/pkg/target"
	targetmocks "github.com/gardener/gardenctl-v2/pkg/target/mocks"
)

var _ = Describe("Env Commands - Options", func() {
	Describe("having an Options instance", func() {
		var (
			ctrl    *gomock.Controller
			factory *utilmocks.MockFactory
			manager *targetmocks.MockManager
			options *providerenv.TestOptions
			cmdPath,
			shell string
			output       string
			providerType string
			unset        bool
			baseTemplate env.Template
			cfg          *config.Config
			tf           target.TargetFlags
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			factory = utilmocks.NewMockFactory(ctrl)
			manager = targetmocks.NewMockManager(ctrl)
			options = providerenv.NewOptions()
			cmdPath = "gardenctl provider-env"
			baseTemplate = env.NewTemplate("helpers")
			shell = "default"
			output = ""
			providerType = "aws"
			cfg = &config.Config{
				LinkKubeconfig: ptr.To(false),
				Gardens:        []config.Garden{{Name: "test"}},
			}
			tf = target.NewTargetFlags("", "", "", "", false)
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		JustBeforeEach(func() {
			options.Shell = shell
			options.Output = output
			options.CmdPath = cmdPath
			options.Unset = unset
			options.Template = baseTemplate
		})

		Describe("completing the command options", func() {
			var (
				root,
				parent,
				child *cobra.Command
				ctx context.Context
			)

			BeforeEach(func() {
				ctx = context.Background()
				root = &cobra.Command{Use: "root"}
				parent = &cobra.Command{Use: "parent", Aliases: []string{"alias"}}
				child = &cobra.Command{Use: "child"}
				parent.AddCommand(child)
				root.AddCommand(parent)
				factory.EXPECT().GardenHomeDir().Return(gardenHomeDir)
				factory.EXPECT().Context().Return(ctx)
				root.SetArgs([]string{"alias", "child"})
				Expect(root.Execute()).To(Succeed())
				baseTemplate = nil
				providerType = ""
			})

			Context("when the providerType is empty", func() {
				It("should complete options with default shell", func() {
					factory.EXPECT().Manager().Return(manager, nil)
					factory.EXPECT().TargetFlags().Return(tf)
					manager.EXPECT().SessionDir().Return(sessionDir)
					manager.EXPECT().Configuration().Return(cfg)
					Expect(options.Template).To(BeNil())
					Expect(options.Complete(factory, child, nil)).To(Succeed())
					Expect(options.Shell).To(Equal(child.Name()))
					Expect(options.GardenDir).To(Equal(gardenHomeDir))
					Expect(options.SessionDir).To(Equal(sessionDir))
					Expect(options.CmdPath).To(Equal(root.Name() + " " + parent.Name()))
					Expect(options.Template).NotTo(BeNil())
					t, ok := options.Template.(providerenv.TestTemplate)
					Expect(ok).To(BeTrue())
					Expect(t.Delegate().Lookup("usage-hint")).NotTo(BeNil())
					Expect(t.Delegate().Lookup("bash")).To(BeNil())
				})

				It("should fail to complete options", func() {
					err := errors.New("error")
					factory.EXPECT().Manager().Return(nil, err)
					Expect(options.Complete(factory, child, nil)).To(MatchError(err))
				})
			})

			Context("when the providerType is azure", func() {
				BeforeEach(func() {
					providerType = "azure"
				})

				It("should complete options", func() {
					factory.EXPECT().Manager().Return(manager, nil)
					factory.EXPECT().TargetFlags().Return(tf)
					manager.EXPECT().SessionDir().Return(sessionDir)
					manager.EXPECT().Configuration().Return(cfg)
					Expect(options.Template).To(BeNil())
					Expect(options.Complete(factory, child, nil)).To(Succeed())
					Expect(options.Template).NotTo(BeNil())
					t, ok := options.Template.(providerenv.TestTemplate)
					Expect(ok).To(BeTrue())
					Expect(t.Delegate().Lookup("usage-hint")).NotTo(BeNil())
				})
			})
		})

		Describe("validating the command options", func() {
			Context("when output is set", func() {
				BeforeEach(func() {
					shell = ""
				})

				It("should successfully validate the options", func() {
					options.Output = "json"
					Expect(options.Validate()).To(Succeed())
				})

				It("should return an error when output is invalid", func() {
					options.Output = "invalid"
					Expect(options.Validate()).To(MatchError("--output must be either 'yaml' or 'json'"))
				})
			})

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
				Expect(options.Validate()).To(MatchError(fmt.Sprintf("invalid shell given, must be one of %v", env.ValidShells())))
			})
		})

		Describe("adding the command flags", func() {
			It("should successfully add the unset flag", func() {
				cmd := &cobra.Command{}
				options.AddFlags(cmd.Flags())
				Expect(cmd.Flag("unset")).NotTo(BeNil())
			})
		})

		Describe("running the provider-env command with the given options", func() {
			var (
				ctx                    context.Context
				manager                *targetmocks.MockManager
				client                 *gardenclientmocks.MockClient
				t                      target.Target
				secretBindingName      string
				credentialsBindingName string
				cloudProfileName       string
				region                 string
				provider               *gardencorev1beta1.Provider
				secretRef              *corev1.SecretReference
				// secretRef              *corev1.SecretReference
				cloudProfileRef    *gardencorev1beta1.CloudProfileReference
				shoot              *gardencorev1beta1.Shoot
				secretBinding      *gardencorev1beta1.SecretBinding
				credentialsBinding *gardensecurityv1alpha1.CredentialsBinding
				cloudProfile       *clientgarden.CloudProfileUnion
				providerConfig     *openstackv1alpha1.CloudProfileConfig
				secret             *corev1.Secret
			)

			BeforeEach(func() {
				manager = targetmocks.NewMockManager(ctrl)
				client = gardenclientmocks.NewMockClient(ctrl)
				t = target.NewTarget("test", "project", "seed", "shoot")
				secretBindingName = "secret-binding"
				credentialsBindingName = "credentials-binding"
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
				cloudProfileRef = &gardencorev1beta1.CloudProfileReference{
					Kind: corev1beta1constants.CloudProfileReferenceKindCloudProfile,
					Name: cloudProfileName,
				}
				shell = "bash"
				options.SessionDir = sessionDir
				ctx = context.Background()

				factory.EXPECT().Context().Return(ctx)
			})

			JustBeforeEach(func() {
				shoot = &gardencorev1beta1.Shoot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      t.ShootName(),
						Namespace: "garden-" + t.ProjectName(),
					},
					Spec: gardencorev1beta1.ShootSpec{
						CloudProfile:      cloudProfileRef,
						Region:            region,
						SecretBindingName: &secretBindingName,
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
				credentialsBinding = &gardensecurityv1alpha1.CredentialsBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      credentialsBindingName,
						Namespace: shoot.Namespace,
					},
					CredentialsRef: corev1.ObjectReference{
						Kind:       "Secret",
						APIVersion: corev1.SchemeGroupVersion.String(),
						Namespace:  secretRef.Namespace,
						Name:       secretRef.Name,
					},
				}
				secret = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: secretRef.Namespace,
						Name:      secretRef.Name,
					},
					Data: map[string][]byte{
						"serviceaccount.json": []byte(readTestFile(provider.Type + "/serviceaccount.json")),
					},
				}
				cloudProfile = &clientgarden.CloudProfileUnion{
					CloudProfile: &gardencorev1beta1.CloudProfile{
						ObjectMeta: metav1.ObjectMeta{
							Name: cloudProfileName,
						},
						Spec: gardencorev1beta1.CloudProfileSpec{
							Type: provider.Type,
							ProviderConfig: &runtime.RawExtension{
								Object: providerConfig,
								Raw:    nil,
							},
						},
					},
				}
			})

			Context("when the command runs successfully", func() {
				BeforeEach(func() {
					factory.EXPECT().Manager().Return(manager, nil)
					manager.EXPECT().GardenClient(t.GardenName()).Return(client, nil)
				})

				JustBeforeEach(func() {
					client.EXPECT().GetSecret(ctx, secretBinding.SecretRef.Namespace, secretBinding.SecretRef.Name).Return(secret, nil)
					client.EXPECT().GetCloudProfile(ctx, *shoot.Spec.CloudProfile).Return(cloudProfile, nil)
				})

				Context("and the shoot is targeted via project", func() {
					JustBeforeEach(func() {
						client.EXPECT().GetSecretBinding(ctx, shoot.Namespace, *shoot.Spec.SecretBindingName).Return(secretBinding, nil)
						currentTarget := t.WithSeedName("")
						manager.EXPECT().CurrentTarget().Return(currentTarget, nil)
						client.EXPECT().FindShoot(ctx, currentTarget.AsListOption()).Return(shoot, nil)
						manager.EXPECT().Configuration().Return(cfg)
					})

					It("does the work when the shoot is targeted via project", func() {
						Expect(options.Run(factory)).To(Succeed())
						expected := strings.NewReplacer("PLACEHOLDER_CONFIG_DIR", filepath.Join(sessionDir, ".config", "gcloud")).Replace(readTestFile("gcp/export.bash"))
						Expect(options.String()).To(Equal(expected))
					})

					It("should print how to reset configuration for powershell", func() {
						options.Unset = true
						options.Shell = "powershell"
						Expect(options.Run(factory)).To(Succeed())
						Expect(options.String()).To(Equal(readTestFile("gcp/unset.pwsh")))
					})
				})

				Context("and the shoot is targeted via seed", func() {
					JustBeforeEach(func() {
						currentTarget := t.WithProjectName("")
						manager.EXPECT().CurrentTarget().Return(currentTarget, nil)
						client.EXPECT().FindShoot(ctx, currentTarget.AsListOption()).Return(shoot, nil)
						manager.EXPECT().Configuration().Return(cfg)
					})

					Context("and the shoot uses secret binding", func() {
						JustBeforeEach(func() {
							client.EXPECT().GetSecretBinding(ctx, shoot.Namespace, *shoot.Spec.SecretBindingName).Return(secretBinding, nil)
						})

						It("does the work when the shoot is targeted via seed", func() {
							Expect(options.Run(factory)).To(Succeed())
							expected := strings.NewReplacer("PLACEHOLDER_CONFIG_DIR", filepath.Join(sessionDir, ".config", "gcloud")).Replace(readTestFile("gcp/export.seed.bash"))
							Expect(options.String()).To(Equal(expected))
						})
					})

					Context("and the shoot uses credentials binding", func() {
						JustBeforeEach(func() {
							shoot.Spec.SecretBindingName = nil
							shoot.Spec.CredentialsBindingName = &credentialsBindingName
							client.EXPECT().GetCredentialsBinding(ctx, shoot.Namespace, *shoot.Spec.CredentialsBindingName).Return(credentialsBinding, nil)
						})

						It("does the work when the shoot is targeted via seed", func() {
							Expect(options.Run(factory)).To(Succeed())
							expected := strings.NewReplacer("PLACEHOLDER_CONFIG_DIR", filepath.Join(sessionDir, ".config", "gcloud")).Replace(readTestFile("gcp/export.seed.bash"))
							Expect(options.String()).To(Equal(expected))
						})
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
					manager.EXPECT().CurrentTarget().Return(t.WithShootName("").WithSeedName(""), nil)
					manager.EXPECT().GardenClient(t.GardenName()).Return(client, nil)
					Expect(options.Run(factory)).To(BeIdenticalTo(target.ErrNoShootTargeted))
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
						client.EXPECT().GetSecretBinding(ctx, shoot.Namespace, *shoot.Spec.SecretBindingName).Return(nil, err)
						Expect(options.Run(factory)).To(BeIdenticalTo(err))
					})

					It("should fail with GetSecretError", func() {
						currentTarget := t.WithSeedName("")
						manager.EXPECT().CurrentTarget().Return(currentTarget, nil)
						client.EXPECT().FindShoot(ctx, currentTarget.AsListOption()).Return(shoot, nil)
						client.EXPECT().GetSecretBinding(ctx, shoot.Namespace, *shoot.Spec.SecretBindingName).Return(secretBinding, nil)
						client.EXPECT().GetSecret(ctx, secretBinding.SecretRef.Namespace, secretBinding.SecretRef.Name).Return(nil, err)
						Expect(options.Run(factory)).To(BeIdenticalTo(err))
					})

					It("should fail with GetCloudProfileError", func() {
						currentTarget := t.WithSeedName("")
						manager.EXPECT().CurrentTarget().Return(currentTarget, nil)
						client.EXPECT().FindShoot(ctx, currentTarget.AsListOption()).Return(shoot, nil)
						client.EXPECT().GetSecretBinding(ctx, shoot.Namespace, *shoot.Spec.SecretBindingName).Return(secretBinding, nil)
						client.EXPECT().GetSecret(ctx, secretBinding.SecretRef.Namespace, secretBinding.SecretRef.Name).Return(secret, nil)
						client.EXPECT().GetCloudProfile(ctx, *shoot.Spec.CloudProfile).Return(nil, err)
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
				serviceaccountJSON,
				token string
				shoot           *gardencorev1beta1.Shoot
				secret          *corev1.Secret
				cloudProfile    *clientgarden.CloudProfileUnion
				cloudProfileRef *gardencorev1beta1.CloudProfileReference
				providerConfig  *openstackv1alpha1.CloudProfileConfig
			)

			BeforeEach(func() {
				shell = "bash"
				unset = false
				namespace = "garden-test"
				shootName = "shoot"
				secretName = "secret"
				cloudProfileName = "cloud-profile"
				cloudProfileRef = &gardencorev1beta1.CloudProfileReference{
					Kind: corev1beta1constants.CloudProfileReferenceKindCloudProfile,
					Name: cloudProfileName,
				}
				region = "europe"
				providerType = "gcp"
				providerConfig = nil
				serviceaccountJSON = readTestFile("gcp/serviceaccount.json")
				token = "token"
				options.Target = target.NewTarget("test", "project", "", shootName)
			})

			JustBeforeEach(func() {
				shoot = &gardencorev1beta1.Shoot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      shootName,
						Namespace: namespace,
					},
					Spec: gardencorev1beta1.ShootSpec{
						CloudProfile: cloudProfileRef,
						Region:       region,
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

				cloudProfile = &clientgarden.CloudProfileUnion{
					CloudProfile: &gardencorev1beta1.CloudProfile{
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
					},
				}
				options.GardenDir = gardenHomeDir
				options.SessionDir = sessionDir
			})

			Context("when configuring the shell", func() {
				BeforeEach(func() {
					unset = false
				})

				It("should render the template successfully", func() {
					Expect(options.PrintProviderEnv(shoot, secret, cloudProfile)).To(Succeed())
					expected := strings.NewReplacer("PLACEHOLDER_CONFIG_DIR", filepath.Join(sessionDir, ".config", "gcloud")).Replace(readTestFile("gcp/export.bash"))
					Expect(options.String()).To(Equal(expected))
				})
			})

			Context("when resetting the shell configuration", func() {
				BeforeEach(func() {
					unset = true
					shell = "powershell"
				})

				It("should render the template successfully", func() {
					Expect(options.PrintProviderEnv(shoot, secret, cloudProfile)).To(Succeed())
					Expect(options.String()).To(Equal(readTestFile("gcp/unset.pwsh")))
				})
			})

			Context("when JSON input is invalid", func() {
				JustBeforeEach(func() {
					secret.Data["serviceaccount.json"] = []byte("{")
				})

				It("should fail to render the template with JSON parse error", func() {
					Expect(options.PrintProviderEnv(shoot, secret, cloudProfile)).To(MatchError(ContainSubstring("unexpected end of JSON input")))
				})
			})

			Context("when the shell is invalid", func() {
				BeforeEach(func() {
					shell = "cmd"
				})

				It("should fail to render the template with JSON parse error", func() {
					noTemplateFmt := "template: no template %q associated with template %q"
					Expect(options.PrintProviderEnv(shoot, secret, cloudProfile)).To(MatchError(fmt.Sprintf(noTemplateFmt, shell, "base")))
				})
			})

			Context("when the cloudprovider template is found in garden home dir", func() {
				var filename string

				BeforeEach(func() {
					providerType = "test"
					filename = filepath.Join("templates", providerType+".tmpl")
					writeTempFile(filename, readTestFile("templates/"+providerType+".tmpl"))
				})

				AfterEach(func() {
					removeTempFile(filename)
				})

				It("should render the template successfully", func() {
					Expect(options.PrintProviderEnv(shoot, secret, cloudProfile)).To(Succeed())
					Expect(options.String()).To(Equal(readTestFile("test/export.bash")))
				})
			})

			Context("when the cloudprovider template is not found", func() {
				BeforeEach(func() {
					providerType = "not-found"
				})

				It("should fail to render the template with a not supported error", func() {
					message := "failed to generate the cloud provider CLI configuration script"
					Expect(options.PrintProviderEnv(shoot, secret, cloudProfile)).To(MatchError(MatchRegexp(message)))
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
					message := "failed to generate the cloud provider CLI configuration script"
					Expect(options.PrintProviderEnv(shoot, secret, cloudProfile)).To(MatchError(MatchRegexp(message)))
				})
			})

			Context("when the configuration directory could not be created", func() {
				It("should fail a mkdir error", func() {
					options.SessionDir = string([]byte{0})
					Expect(options.PrintProviderEnv(shoot, secret, cloudProfile)).To(MatchError(MatchRegexp("^failed to create gcloud configuration directory:")))
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
					Expect(options.PrintProviderEnv(shoot, secret, cloudProfile)).To(Succeed())
					Expect(options.String()).To(Equal(readTestFile("openstack/export.bash")))
				})

				It("should fail with invalid provider config", func() {
					cloudProfile.GetCloudProfileSpec().ProviderConfig = nil
					Expect(options.PrintProviderEnv(shoot, secret, cloudProfile)).To(MatchError(MatchRegexp("^failed to get openstack provider config:")))
				})

				Context("when applicationCredentialSecret is empty", func() {
					JustBeforeEach(func() {
						secret.Data["applicationCredentialID"] = []byte("app-cred-id")
						secret.Data["applicationCredentialName"] = []byte("app-cred-name")
						secret.Data["applicationCredentialSecret"] = []byte("")
					})

					It("should use keystone authentication instead of application credentials", func() {
						Expect(options.PrintProviderEnv(shoot, secret, cloudProfile)).To(Succeed())
						output := options.String()
						Expect(output).To(ContainSubstring("export OS_AUTH_STRATEGY='keystone'"))
						Expect(output).NotTo(ContainSubstring("export OS_AUTH_TYPE='v3applicationcredential'"))
						Expect(output).To(ContainSubstring("export OS_USERNAME='user'"))
						Expect(output).To(ContainSubstring("export OS_PASSWORD='secret'"))
						Expect(output).To(ContainSubstring("export OS_AUTH_TYPE=''"))
						Expect(output).To(ContainSubstring("export OS_APPLICATION_CREDENTIAL_ID=''"))
						Expect(output).To(ContainSubstring("export OS_APPLICATION_CREDENTIAL_NAME=''"))
						Expect(output).To(ContainSubstring("export OS_APPLICATION_CREDENTIAL_SECRET=''"))
					})
				})

				Context("when applicationCredentialSecret has a valid value", func() {
					JustBeforeEach(func() {
						secret.Data["applicationCredentialID"] = []byte("app-cred-id")
						secret.Data["applicationCredentialName"] = []byte("app-cred-name")
						secret.Data["applicationCredentialSecret"] = []byte("app-cred-secret")
					})

					It("should use application credential authentication", func() {
						Expect(options.PrintProviderEnv(shoot, secret, cloudProfile)).To(Succeed())
						output := options.String()
						Expect(output).To(ContainSubstring("export OS_AUTH_TYPE='v3applicationcredential'"))
						Expect(output).To(ContainSubstring("export OS_APPLICATION_CREDENTIAL_ID='app-cred-id'"))
						Expect(output).To(ContainSubstring("export OS_APPLICATION_CREDENTIAL_NAME='app-cred-name'"))
						Expect(output).To(ContainSubstring("export OS_APPLICATION_CREDENTIAL_SECRET='app-cred-secret'"))
						Expect(output).To(ContainSubstring("export OS_AUTH_STRATEGY=''"))
						Expect(output).To(ContainSubstring("export OS_TENANT_NAME=''"))
						Expect(output).To(ContainSubstring("export OS_USERNAME=''"))
						Expect(output).To(ContainSubstring("export OS_PASSWORD=''"))
					})
				})

				Context("output is json", func() {
					BeforeEach(func() {
						output = "json"
						shell = ""
					})

					It("should render the json successfully", func() {
						Expect(options.PrintProviderEnv(shoot, secret, cloudProfile)).To(Succeed())
						expected := strings.NewReplacer("PLACEHOLDER_CONFIG_DIR", filepath.Join(sessionDir, ".config", "openstack")).Replace(readTestFile("openstack/export.json"))
						Expect(options.String()).To(Equal(expected))
					})
				})
			})

			Context("when the cloudprovider is azure", func() {
				const (
					clientID       = "client-id"
					clientSecret   = "client-secret"
					tenantID       = "tenant-id"
					subscriptionID = "subscription-id"
				)

				BeforeEach(func() {
					shell = "fish"
					providerType = "azure"
				})

				JustBeforeEach(func() {
					secret.Data["clientID"] = []byte(clientID)
					secret.Data["clientSecret"] = []byte(clientSecret)
					secret.Data["tenantID"] = []byte(tenantID)
					secret.Data["subscriptionID"] = []byte(subscriptionID)
				})

				It("should render the template successfully", func() {
					Expect(options.PrintProviderEnv(shoot, secret, cloudProfile)).To(Succeed())
					Expect(options.String()).To(Equal(fmt.Sprintf(readTestFile("azure/export.fish"), filepath.Join(sessionDir, ".config", "az"))))
				})

				It("should fail with mkdir error", func() {
					options.SessionDir = string([]byte{0})
					Expect(options.PrintProviderEnv(shoot, secret, cloudProfile)).To(MatchError(MatchRegexp("^failed to create az configuration directory:")))
				})

				Context("output is json", func() {
					BeforeEach(func() {
						output = "json"
						shell = ""
					})

					It("should render the json successfully", func() {
						Expect(options.PrintProviderEnv(shoot, secret, cloudProfile)).To(Succeed())
						expected := strings.NewReplacer("PLACEHOLDER_CONFIG_DIR", filepath.Join(sessionDir, ".config", "az")).Replace(readTestFile("azure/export.json"))
						Expect(options.String()).To(Equal(expected))
					})
				})
			})
		})

		Describe("rendering the usage hint", func() {
			var (
				targetFlags,
				cli string
				meta map[string]interface{}
				t    = target.NewTarget("test", "project", "seed", "shoot")
			)

			BeforeEach(func() {
				providerType = "alicloud"
				shell = "bash"
				unset = false
				options.Target = t.WithSeedName("")
			})

			JustBeforeEach(func() {
				cli = providerenv.GetProviderCLI(providerType)
				meta = options.GenerateMetadata(cli)
				targetFlags = providerenv.GetTargetFlags(t)
				Expect(env.NewTemplate("helpers").ExecuteTemplate(options.IOStreams.Out, "usage-hint", meta)).To(Succeed())
			})

			Context("when configuring the shell", func() {
				It("should generate the metadata and render the export hint", func() {
					Expect(meta["unset"]).To(BeFalse())
					Expect(meta["shell"]).To(Equal(shell))
					Expect(meta["cli"]).To(Equal(cli))
					Expect(meta["commandPath"]).To(Equal(options.CmdPath))
					Expect(meta["targetFlags"]).To(Equal(targetFlags))
					regex := regexp.MustCompile(`(?m)\A\n(.*)\n(.*)\n\z`)
					match := regex.FindStringSubmatch(options.String())
					Expect(match).NotTo(BeNil())
					Expect(len(match)).To(Equal(3))
					Expect(match[1]).To(Equal(fmt.Sprintf("# Run this command to configure %s for your shell:", cli)))
					Expect(match[2]).To(Equal(fmt.Sprintf("# eval $(%s %s)", options.CmdPath, shell)))
				})
			})

			Context("when resetting the shell configuration", func() {
				BeforeEach(func() {
					unset = true
				})

				It("should generate the metadata and render the unset", func() {
					Expect(meta["unset"]).To(BeTrue())
					regex := regexp.MustCompile(`(?m)\A\n(.*)\n(.*)\n\z`)
					match := regex.FindStringSubmatch(options.String())
					Expect(match).NotTo(BeNil())
					Expect(len(match)).To(Equal(3))
					Expect(match[1]).To(Equal(fmt.Sprintf("# Run this command to reset the %s configuration for your shell:", cli)))
					Expect(match[2]).To(Equal(fmt.Sprintf("# eval $(%s -u %s)", options.CmdPath, shell)))
				})
			})
		})
	})

	Describe("getting the keyStoneURL", func() {
		var (
			cloudProfileName   = "cloud-profile-name"
			region             = "europe"
			cloudProfile       *clientgarden.CloudProfileUnion
			cloudProfileConfig *openstackv1alpha1.CloudProfileConfig
		)

		BeforeEach(func() {
			cloudProfileConfig = &openstackv1alpha1.CloudProfileConfig{
				KeyStoneURLs: []openstackv1alpha1.KeyStoneURL{
					{URL: "bar", Region: region},
				},
			}
			cloudProfile = &clientgarden.CloudProfileUnion{
				CloudProfile: &gardencorev1beta1.CloudProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name: cloudProfileName,
					},
					Spec: gardencorev1beta1.CloudProfileSpec{
						ProviderConfig: &runtime.RawExtension{
							Object: cloudProfileConfig,
							Raw:    nil,
						},
					},
				},
			}
		})

		It("should return a global url", func() {
			cloudProfileConfig.KeyStoneURL = "foo"
			url, err := providerenv.GetKeyStoneURL(cloudProfile, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(url).To(Equal(cloudProfileConfig.KeyStoneURL))
		})

		It("should return region specific url", func() {
			url, err := providerenv.GetKeyStoneURL(cloudProfile, region)
			Expect(err).NotTo(HaveOccurred())
			Expect(url).To(Equal("bar"))
		})

		It("should fail with not found", func() {
			cloudProfile.GetCloudProfileSpec().ProviderConfig = nil
			_, err := providerenv.GetKeyStoneURL(cloudProfile, region)
			Expect(err).To(MatchError(MatchRegexp("^failed to get openstack provider config:")))
		})

		It("should fail with not found", func() {
			region = "asia"
			_, err := providerenv.GetKeyStoneURL(cloudProfile, region)
			Expect(err).To(MatchError(fmt.Sprintf("cannot find keystone URL for region %q in cloudprofile %q", region, cloudProfileName)))
		})
	})

	DescribeTable("getting the provider CLI",
		func(providerType string, cli string) {
			Expect(providerenv.GetProviderCLI(providerType)).To(Equal(cli))
		},
		Entry("when provider is aws", "aws", "aws"),
		Entry("when provider is azure", "azure", "az"),
		Entry("when provider is alicloud", "alicloud", "aliyun"),
		Entry("when provider is gcp", "gcp", "gcloud"),
	)
})
