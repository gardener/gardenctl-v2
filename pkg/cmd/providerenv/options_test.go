/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package providerenv_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
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
	allowpattern "github.com/gardener/gardenctl-v2/pkg/provider/common/allowpattern"
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
			ctx          context.Context
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			ctx = context.Background()
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
			var root,
				parent,
				child *cobra.Command

			BeforeEach(func() {
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
					factory.EXPECT().GetSessionID().Return("test-session-id", nil)
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
					factory.EXPECT().GetSessionID().Return("", err)
					Expect(options.Complete(factory, child, nil)).To(MatchError(err))
				})
			})

			Context("when the providerType is azure", func() {
				BeforeEach(func() {
					providerType = "azure"
				})

				It("should complete options", func() {
					factory.EXPECT().GetSessionID().Return("test-session-id", nil)
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
				manager                *targetmocks.MockManager
				client                 *gardenclientmocks.MockClient
				t                      target.Target
				secretBindingName      string
				credentialsBindingName string
				cloudProfileName       string
				region                 string
				provider               *gardencorev1beta1.Provider
				secretRef              *corev1.SecretReference
				cloudProfileRef        *gardencorev1beta1.CloudProfileReference
				shoot                  *gardencorev1beta1.Shoot
				secretBinding          *gardencorev1beta1.SecretBinding
				credentialsBinding     *gardensecurityv1alpha1.CredentialsBinding
				cloudProfile           *clientgarden.CloudProfileUnion
				providerConfig         *openstackv1alpha1.CloudProfileConfig
				secret                 *corev1.Secret
				mockCmd                *cobra.Command
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
				factory.EXPECT().Context().Return(ctx).AnyTimes()
				// Create a proper command hierarchy for Complete() to work
				parentCmd := &cobra.Command{Use: "gardenctl"}
				mockCmd = &cobra.Command{Use: "provider-env"}
				parentCmd.AddCommand(mockCmd)
			})

			JustBeforeEach(func() {
				shoot = &gardencorev1beta1.Shoot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      t.ShootName(),
						Namespace: "garden-" + t.ProjectName(),
						UID:       "00000000-0000-0000-0000-000000000000",
					},
					Spec: gardencorev1beta1.ShootSpec{
						CloudProfile:           cloudProfileRef,
						Region:                 region,
						CredentialsBindingName: &credentialsBindingName,
						Provider:               *provider.DeepCopy(),
					},
				}
				secretBinding = &gardencorev1beta1.SecretBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      secretBindingName,
						Namespace: shoot.Namespace,
						UID:       "00000000-0000-0000-0000-000000000000",
					},
					SecretRef: *secretRef.DeepCopy(),
				}
				credentialsBinding = &gardensecurityv1alpha1.CredentialsBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      credentialsBindingName,
						Namespace: shoot.Namespace,
						UID:       "00000000-0000-0000-0000-000000000000",
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
						UID:       "00000000-0000-0000-0000-000000000000",
					},
					Data: map[string][]byte{
						"serviceaccount.json": []byte(readTestFile("gcp/serviceaccount.json")),
					},
				}
				cloudProfile = &clientgarden.CloudProfileUnion{
					CloudProfile: &gardencorev1beta1.CloudProfile{
						ObjectMeta: metav1.ObjectMeta{
							Name: cloudProfileName,
							UID:  "00000000-0000-0000-0000-000000000000",
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
					factory.EXPECT().Manager().Return(manager, nil).Times(2)
					manager.EXPECT().GardenClient(t.GardenName()).Return(client, nil)

					factory.EXPECT().TargetFlags().Return(tf)
					factory.EXPECT().GardenHomeDir().Return(gardenHomeDir)
					manager.EXPECT().SessionDir().Return(sessionDir)
					manager.EXPECT().Configuration().Return(cfg).Times(2)
					factory.EXPECT().GetSessionID().Return("test-session-id", nil)
					Expect(options.Complete(factory, mockCmd, nil)).To(Succeed())
				})

				JustBeforeEach(func() {
					client.EXPECT().GetSecret(ctx, secretBinding.SecretRef.Namespace, secretBinding.SecretRef.Name).Return(secret, nil)
					client.EXPECT().GetCloudProfile(ctx, *shoot.Spec.CloudProfile, shoot.Namespace).Return(cloudProfile, nil)
				})

				Context("and the shoot is targeted via project", func() {
					JustBeforeEach(func() {
						shoot.Spec.SecretBindingName = &secretBindingName
						client.EXPECT().GetSecretBinding(ctx, shoot.Namespace, *shoot.Spec.SecretBindingName).Return(secretBinding, nil)
						currentTarget := t.WithSeedName("")
						manager.EXPECT().CurrentTarget().Return(currentTarget, nil)
						client.EXPECT().FindShoot(ctx, currentTarget.AsListOption()).Return(shoot, nil)
					})

					It("does the work when the shoot is targeted via project", func() {
						Expect(options.Run(factory)).To(Succeed())
						hash := computeTestHash("test-session-id", t.GardenName(), shoot.Namespace, t.ShootName())
						replacer := strings.NewReplacer(
							"PLACEHOLDER_CONFIG_DIR", filepath.Join(sessionDir, ".config", "gcloud"),
							"PLACEHOLDER_SESSION_DIR", sessionDir,
							"PLACEHOLDER_HASH", hash,
						)
						expected := replacer.Replace(readTestFile("gcp/export.bash"))
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
					var hash string

					JustBeforeEach(func() {
						currentTarget := t.WithProjectName("")
						hash = computeTestHash("test-session-id", t.GardenName(), shoot.Namespace, t.ShootName())
						manager.EXPECT().CurrentTarget().Return(currentTarget, nil)
						client.EXPECT().FindShoot(ctx, currentTarget.AsListOption()).Return(shoot, nil)
					})

					Context("and the shoot uses secret binding", func() {
						JustBeforeEach(func() {
							shoot.Spec.CredentialsBindingName = nil
							shoot.Spec.SecretBindingName = &secretBindingName
							client.EXPECT().GetSecretBinding(ctx, shoot.Namespace, *shoot.Spec.SecretBindingName).Return(secretBinding, nil)
						})

						It("does the work when the shoot is targeted via seed", func() {
							Expect(options.Run(factory)).To(Succeed())
							replacer := strings.NewReplacer(
								"PLACEHOLDER_CONFIG_DIR", filepath.Join(sessionDir, ".config", "gcloud"),
								"PLACEHOLDER_SESSION_DIR", sessionDir,
								"PLACEHOLDER_HASH", hash,
							)
							expected := replacer.Replace(readTestFile("gcp/export.seed.bash"))
							Expect(options.String()).To(Equal(expected))
						})
					})

					Context("and the shoot uses credentials binding", func() {
						JustBeforeEach(func() {
							client.EXPECT().GetCredentialsBinding(ctx, shoot.Namespace, *shoot.Spec.CredentialsBindingName).Return(credentialsBinding, nil)
						})

						It("does the work when the shoot is targeted via seed", func() {
							Expect(options.Run(factory)).To(Succeed())
							expected := strings.NewReplacer(
								"PLACEHOLDER_CONFIG_DIR", filepath.Join(sessionDir, ".config", "gcloud"),
								"PLACEHOLDER_SESSION_DIR", sessionDir,
								"PLACEHOLDER_HASH", hash,
							).Replace(readTestFile("gcp/export.seed.bash"))
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

					It("should fail with GetCredentialsBindingError", func() {
						currentTarget := t.WithSeedName("")
						manager.EXPECT().CurrentTarget().Return(currentTarget, nil)
						client.EXPECT().FindShoot(ctx, currentTarget.AsListOption()).Return(shoot, nil)
						client.EXPECT().GetCredentialsBinding(ctx, shoot.Namespace, *shoot.Spec.CredentialsBindingName).Return(nil, err)
						Expect(options.Run(factory)).To(BeIdenticalTo(err))
					})

					It("should fail with GetSecretError", func() {
						currentTarget := t.WithSeedName("")
						manager.EXPECT().CurrentTarget().Return(currentTarget, nil)
						client.EXPECT().FindShoot(ctx, currentTarget.AsListOption()).Return(shoot, nil)
						client.EXPECT().GetCredentialsBinding(ctx, shoot.Namespace, *shoot.Spec.CredentialsBindingName).Return(credentialsBinding, nil)
						client.EXPECT().GetCloudProfile(ctx, *shoot.Spec.CloudProfile, shoot.Namespace).Return(cloudProfile, nil)
						manager.EXPECT().Configuration().Return(cfg)
						client.EXPECT().GetSecret(ctx, credentialsBinding.CredentialsRef.Namespace, credentialsBinding.CredentialsRef.Name).Return(nil, err)
						Expect(options.Run(factory)).To(BeIdenticalTo(err))
					})

					It("should fail with GetCloudProfileError", func() {
						currentTarget := t.WithSeedName("")
						manager.EXPECT().CurrentTarget().Return(currentTarget, nil)
						client.EXPECT().FindShoot(ctx, currentTarget.AsListOption()).Return(shoot, nil)
						client.EXPECT().GetCredentialsBinding(ctx, shoot.Namespace, *shoot.Spec.CredentialsBindingName).Return(credentialsBinding, nil)
						client.EXPECT().GetCloudProfile(ctx, *shoot.Spec.CloudProfile, shoot.Namespace).Return(nil, err)
						Expect(options.Run(factory)).To(BeIdenticalTo(err))
					})
				})
			})
		})

		Describe("rendering the template", func() {
			var (
				gardenName,
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
				mockCmd         *cobra.Command
				credentialsRef  corev1.ObjectReference
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
				gardenName = "test"
				options.Target = target.NewTarget("test", "project", "", shootName)
				// Create a proper command hierarchy for Complete() to work
				parentCmd := &cobra.Command{Use: "gardenctl"}
				mockCmd = &cobra.Command{Use: "provider-env"}
				parentCmd.AddCommand(mockCmd)
			})

			JustBeforeEach(func() {
				shoot = &gardencorev1beta1.Shoot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      shootName,
						Namespace: namespace,
						UID:       "00000000-0000-0000-0000-000000000000",
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
						UID:       "00000000-0000-0000-0000-000000000000",
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
							UID:  "00000000-0000-0000-0000-000000000000",
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

			Context("with Secret credentials", func() {
				BeforeEach(func() {
					credentialsRef = corev1.ObjectReference{
						APIVersion: "v1",
						Kind:       "Secret",
						Namespace:  namespace,
						Name:       secretName,
					}
				})

				Context("when configuring the shell", func() {
					BeforeEach(func() {
						unset = false

						// Initialize options with Complete() to set up default patterns
						factory := utilmocks.NewMockFactory(ctrl)
						manager := targetmocks.NewMockManager(ctrl)
						factory.EXPECT().GetSessionID().Return("test-session-id", nil)
						factory.EXPECT().Manager().Return(manager, nil)
						factory.EXPECT().TargetFlags().Return(tf)
						factory.EXPECT().GardenHomeDir().Return(gardenHomeDir)
						factory.EXPECT().Context().Return(ctx)
						manager.EXPECT().SessionDir().Return(sessionDir)
						manager.EXPECT().Configuration().Return(cfg)
						Expect(options.Complete(factory, mockCmd, nil)).To(Succeed())
					})

					It("should render the template successfully", func() {
						client := gardenclientmocks.NewMockClient(ctrl)
						client.EXPECT().GetSecret(ctx, secret.Namespace, secret.Name).Return(secret, nil)
						Expect(options.PrintProviderEnv(ctx, client, shoot, credentialsRef, cloudProfile, nil)).To(Succeed())
						hash := computeTestHash("test-session-id", gardenName, shoot.Namespace, shoot.Name)
						expected := strings.NewReplacer(
							"PLACEHOLDER_CONFIG_DIR", filepath.Join(sessionDir, ".config", "gcloud"),
							"PLACEHOLDER_SESSION_DIR", sessionDir,
							"PLACEHOLDER_HASH", hash,
						).Replace(readTestFile("gcp/export.bash"))
						Expect(options.String()).To(Equal(expected))
					})
				})

				Context("when resetting the shell configuration", func() {
					BeforeEach(func() {
						unset = true
						shell = "powershell"

						// Initialize options with Complete() to set up default patterns
						factory := utilmocks.NewMockFactory(ctrl)
						manager := targetmocks.NewMockManager(ctrl)
						factory.EXPECT().GetSessionID().Return("test-session-id", nil)
						factory.EXPECT().Manager().Return(manager, nil)
						factory.EXPECT().TargetFlags().Return(tf)
						factory.EXPECT().GardenHomeDir().Return(gardenHomeDir)
						factory.EXPECT().Context().Return(ctx)
						manager.EXPECT().SessionDir().Return(sessionDir)
						manager.EXPECT().Configuration().Return(cfg)
						Expect(options.Complete(factory, mockCmd, nil)).To(Succeed())
					})

					It("should render the template successfully", func() {
						client := gardenclientmocks.NewMockClient(ctrl)
						client.EXPECT().GetSecret(ctx, secret.Namespace, secret.Name).Return(secret, nil)
						Expect(options.PrintProviderEnv(ctx, client, shoot, credentialsRef, cloudProfile, nil)).To(Succeed())
						Expect(options.String()).To(Equal(readTestFile("gcp/unset.pwsh")))
					})
				})

				Context("when JSON input is invalid", func() {
					JustBeforeEach(func() {
						secret.Data["serviceaccount.json"] = []byte("{")
					})

					It("should fail to render the template with JSON parse error", func() {
						client := gardenclientmocks.NewMockClient(ctrl)
						client.EXPECT().GetSecret(ctx, secret.Namespace, secret.Name).Return(secret, nil)
						Expect(options.PrintProviderEnv(ctx, client, shoot, credentialsRef, cloudProfile, nil)).To(MatchError(ContainSubstring("unexpected end of JSON input")))
					})
				})

				Context("when the shell is invalid", func() {
					BeforeEach(func() {
						shell = "cmd"

						// Initialize options with Complete() to set up default patterns
						factory := utilmocks.NewMockFactory(ctrl)
						manager := targetmocks.NewMockManager(ctrl)
						factory.EXPECT().GetSessionID().Return("test-session-id", nil)
						factory.EXPECT().Manager().Return(manager, nil)
						factory.EXPECT().TargetFlags().Return(tf)
						factory.EXPECT().GardenHomeDir().Return(gardenHomeDir)
						factory.EXPECT().Context().Return(ctx)
						manager.EXPECT().SessionDir().Return(sessionDir)
						manager.EXPECT().Configuration().Return(cfg)
						Expect(options.Complete(factory, mockCmd, nil)).To(Succeed())
					})

					It("should fail to render the template with JSON parse error", func() {
						client := gardenclientmocks.NewMockClient(ctrl)
						client.EXPECT().GetSecret(ctx, secret.Namespace, secret.Name).Return(secret, nil)
						noTemplateFmt := "template: no template %q associated with template %q"
						Expect(options.PrintProviderEnv(ctx, client, shoot, credentialsRef, cloudProfile, nil)).To(MatchError(fmt.Sprintf(noTemplateFmt, shell, "base")))
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
						client := gardenclientmocks.NewMockClient(ctrl)
						client.EXPECT().GetSecret(ctx, secret.Namespace, secret.Name).Return(secret, nil)
						Expect(options.PrintProviderEnv(ctx, client, shoot, credentialsRef, cloudProfile, nil)).To(Succeed())
						Expect(options.String()).To(Equal(readTestFile("test/export.bash")))
					})
				})

				Context("when the cloudprovider template is not found", func() {
					BeforeEach(func() {
						providerType = "not-found"
					})

					It("should fail to render the template with a not supported error", func() {
						client := gardenclientmocks.NewMockClient(ctrl)
						client.EXPECT().GetSecret(ctx, secret.Namespace, secret.Name).Return(secret, nil)
						message := "failed to generate the cloud provider CLI configuration script"
						Expect(options.PrintProviderEnv(ctx, client, shoot, credentialsRef, cloudProfile, nil)).To(MatchError(MatchRegexp(message)))
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
						client := gardenclientmocks.NewMockClient(ctrl)
						client.EXPECT().GetSecret(ctx, secret.Namespace, secret.Name).Return(secret, nil)
						message := "failed to generate the cloud provider CLI configuration script"
						Expect(options.PrintProviderEnv(ctx, client, shoot, credentialsRef, cloudProfile, nil)).To(MatchError(MatchRegexp(message)))
					})
				})

				Context("when the configuration directory could not be created", func() {
					It("should fail a mkdir error", func() {
						client := gardenclientmocks.NewMockClient(ctrl)
						options.SessionDir = string([]byte{0})
						Expect(options.PrintProviderEnv(ctx, client, shoot, credentialsRef, cloudProfile, nil)).To(MatchError(MatchRegexp("^failed to create gcloud configuration directory:")))
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
						providerConfig = &openstackv1alpha1.CloudProfileConfig{KeyStoneURL: "https://keystone.example.com:5000"}

						factory := utilmocks.NewMockFactory(ctrl)
						manager := targetmocks.NewMockManager(ctrl)
						factory.EXPECT().GetSessionID().Return("test-session-id", nil)
						factory.EXPECT().Manager().Return(manager, nil)
						factory.EXPECT().TargetFlags().Return(tf)
						factory.EXPECT().GardenHomeDir().Return(gardenHomeDir)
						factory.EXPECT().Context().Return(ctx)
						manager.EXPECT().SessionDir().Return(sessionDir)
						manager.EXPECT().Configuration().Return(cfg)
						Expect(options.Complete(factory, mockCmd, nil)).To(Succeed())

						options.MergedAllowedPatterns = &providerenv.MergedProviderPatterns{
							OpenStack: []allowpattern.Pattern{
								{Field: "authURL", URI: "https://keystone.example.com:5000"},
							},
						}
					})

					JustBeforeEach(func() {
						secret.Data["username"] = []byte(username)
						secret.Data["password"] = []byte(password)
						secret.Data["tenantName"] = []byte(tenantName)
						secret.Data["domainName"] = []byte(domainName)
					})

					It("should render the template successfully", func() {
						client := gardenclientmocks.NewMockClient(ctrl)
						client.EXPECT().GetSecret(ctx, secret.Namespace, secret.Name).Return(secret, nil)
						Expect(options.PrintProviderEnv(ctx, client, shoot, credentialsRef, cloudProfile, nil)).To(Succeed())
						hash := computeTestHash("test-session-id", gardenName, shoot.Namespace, shoot.Name)
						expected := strings.NewReplacer(
							"PLACEHOLDER_SESSION_DIR", sessionDir,
							"PLACEHOLDER_HASH", hash,
						).Replace(readTestFile("openstack/export.bash"))
						Expect(options.String()).To(Equal(expected))
					})

					It("should fail with invalid provider config", func() {
						client := gardenclientmocks.NewMockClient(ctrl)
						client.EXPECT().GetSecret(ctx, secret.Namespace, secret.Name).Return(secret, nil)
						cloudProfile.GetCloudProfileSpec().ProviderConfig = nil
						Expect(options.PrintProviderEnv(ctx, client, shoot, credentialsRef, cloudProfile, nil)).To(MatchError(MatchRegexp("^failed to get openstack provider config:")))
					})

					Context("when applicationCredentialSecret is empty", func() {
						JustBeforeEach(func() {
							secret.Data["applicationCredentialID"] = []byte("app-cred-id")
							secret.Data["applicationCredentialName"] = []byte("app-cred-name")
							secret.Data["applicationCredentialSecret"] = []byte("")
						})

						It("should use keystone authentication instead of application credentials", func() {
							client := gardenclientmocks.NewMockClient(ctrl)
							client.EXPECT().GetSecret(ctx, secret.Namespace, secret.Name).Return(secret, nil)
							Expect(options.PrintProviderEnv(ctx, client, shoot, credentialsRef, cloudProfile, nil)).To(Succeed())
							output := options.String()
							hash := computeTestHash("test-session-id", gardenName, shoot.Namespace, shoot.Name)
							providerEnvDir := filepath.Join(sessionDir, "provider-env")

							authStrategyPath := filepath.Join(providerEnvDir, hash+"-authStrategy.txt")
							usernamePath := filepath.Join(providerEnvDir, hash+"-username.txt")
							passwordPath := filepath.Join(providerEnvDir, hash+"-password.txt")
							authTypePath := filepath.Join(providerEnvDir, hash+"-authType.txt")
							appCredIDPath := filepath.Join(providerEnvDir, hash+"-applicationCredentialID.txt")
							appCredNamePath := filepath.Join(providerEnvDir, hash+"-applicationCredentialName.txt")
							appCredSecretPath := filepath.Join(providerEnvDir, hash+"-applicationCredentialSecret.txt")

							Expect(output).To(ContainSubstring(fmt.Sprintf("export OS_AUTH_STRATEGY=$(< '%s');", authStrategyPath)))
							Expect(output).To(ContainSubstring(fmt.Sprintf("export OS_USERNAME=$(< '%s');", usernamePath)))
							Expect(output).To(ContainSubstring(fmt.Sprintf("export OS_PASSWORD=$(< '%s');", passwordPath)))
							Expect(output).To(ContainSubstring(fmt.Sprintf("export OS_AUTH_TYPE=$(< '%s');", authTypePath)))
							Expect(output).To(ContainSubstring(fmt.Sprintf("export OS_APPLICATION_CREDENTIAL_ID=$(< '%s');", appCredIDPath)))
							Expect(output).To(ContainSubstring(fmt.Sprintf("export OS_APPLICATION_CREDENTIAL_NAME=$(< '%s');", appCredNamePath)))
							Expect(output).To(ContainSubstring(fmt.Sprintf("export OS_APPLICATION_CREDENTIAL_SECRET=$(< '%s');", appCredSecretPath)))

							Expect(os.ReadFile(authStrategyPath)).To(Equal([]byte("keystone")))
							Expect(os.ReadFile(usernamePath)).To(Equal([]byte("user")))
							Expect(os.ReadFile(passwordPath)).To(Equal([]byte("secret")))
							Expect(os.ReadFile(authTypePath)).To(Equal([]byte("")))
							Expect(os.ReadFile(appCredIDPath)).To(Equal([]byte("")))
							Expect(os.ReadFile(appCredNamePath)).To(Equal([]byte("")))
							Expect(os.ReadFile(appCredSecretPath)).To(Equal([]byte("")))
						})
					})

					Context("when applicationCredentialSecret has a valid value", func() {
						JustBeforeEach(func() {
							// Remove password-based auth fields when using application credentials
							delete(secret.Data, "username")
							delete(secret.Data, "password")
							secret.Data["applicationCredentialSecret"] = []byte("app-cred-secret")
						})

						Context("when applicationCredentialID is provided", func() {
							JustBeforeEach(func() {
								secret.Data["applicationCredentialID"] = []byte("app-cred-id")
							})

							It("should use application credential authentication with ID", func() {
								client := gardenclientmocks.NewMockClient(ctrl)
								client.EXPECT().GetSecret(ctx, secret.Namespace, secret.Name).Return(secret, nil)
								Expect(options.PrintProviderEnv(ctx, client, shoot, credentialsRef, cloudProfile, nil)).To(Succeed())
								output := options.String()
								hash := computeTestHash("test-session-id", gardenName, shoot.Namespace, shoot.Name)
								providerEnvDir := filepath.Join(sessionDir, "provider-env")

								authTypePath := filepath.Join(providerEnvDir, hash+"-authType.txt")
								applicationCredentialIDPath := filepath.Join(providerEnvDir, hash+"-applicationCredentialID.txt")
								applicationCredentialNamePath := filepath.Join(providerEnvDir, hash+"-applicationCredentialName.txt")
								applicationCredentialSecretPath := filepath.Join(providerEnvDir, hash+"-applicationCredentialSecret.txt")
								authStrategyPath := filepath.Join(providerEnvDir, hash+"-authStrategy.txt")
								tenantNamePath := filepath.Join(providerEnvDir, hash+"-tenantName.txt")
								usernamePath := filepath.Join(providerEnvDir, hash+"-username.txt")
								passwordPath := filepath.Join(providerEnvDir, hash+"-password.txt")

								Expect(output).To(ContainSubstring(fmt.Sprintf("export OS_AUTH_TYPE=$(< '%s');", authTypePath)))
								Expect(output).To(ContainSubstring(fmt.Sprintf("export OS_APPLICATION_CREDENTIAL_ID=$(< '%s');", applicationCredentialIDPath)))
								Expect(output).To(ContainSubstring(fmt.Sprintf("export OS_APPLICATION_CREDENTIAL_NAME=$(< '%s');", applicationCredentialNamePath)))
								Expect(output).To(ContainSubstring(fmt.Sprintf("export OS_APPLICATION_CREDENTIAL_SECRET=$(< '%s');", applicationCredentialSecretPath)))
								Expect(output).To(ContainSubstring(fmt.Sprintf("export OS_AUTH_STRATEGY=$(< '%s');", authStrategyPath)))
								Expect(output).To(ContainSubstring(fmt.Sprintf("export OS_TENANT_NAME=$(< '%s');", tenantNamePath)))
								Expect(output).To(ContainSubstring(fmt.Sprintf("export OS_USERNAME=$(< '%s');", usernamePath)))
								Expect(output).To(ContainSubstring(fmt.Sprintf("export OS_PASSWORD=$(< '%s');", passwordPath)))

								Expect(os.ReadFile(authTypePath)).To(Equal([]byte("v3applicationcredential")))
								Expect(os.ReadFile(applicationCredentialIDPath)).To(Equal([]byte("app-cred-id")))
								Expect(os.ReadFile(applicationCredentialNamePath)).To(Equal([]byte("")))
								Expect(os.ReadFile(applicationCredentialSecretPath)).To(Equal([]byte("app-cred-secret")))
								Expect(os.ReadFile(authStrategyPath)).To(Equal([]byte("")))
								Expect(os.ReadFile(tenantNamePath)).To(Equal([]byte("")))
								Expect(os.ReadFile(usernamePath)).To(Equal([]byte("")))
								Expect(os.ReadFile(passwordPath)).To(Equal([]byte("")))
							})
						})

						Context("when applicationCredentialName is provided", func() {
							JustBeforeEach(func() {
								secret.Data["applicationCredentialName"] = []byte("app-cred-name")
								secret.Data["username"] = []byte("user") // username is required when using applicationCredentialName
							})

							It("should use application credential authentication with name", func() {
								client := gardenclientmocks.NewMockClient(ctrl)
								client.EXPECT().GetSecret(ctx, secret.Namespace, secret.Name).Return(secret, nil)
								Expect(options.PrintProviderEnv(ctx, client, shoot, credentialsRef, cloudProfile, nil)).To(Succeed())
								output := options.String()
								hash := computeTestHash("test-session-id", gardenName, shoot.Namespace, shoot.Name)
								providerEnvDir := filepath.Join(sessionDir, "provider-env")

								authTypePath := filepath.Join(providerEnvDir, hash+"-authType.txt")
								applicationCredentialIDPath := filepath.Join(providerEnvDir, hash+"-applicationCredentialID.txt")
								applicationCredentialNamePath := filepath.Join(providerEnvDir, hash+"-applicationCredentialName.txt")
								applicationCredentialSecretPath := filepath.Join(providerEnvDir, hash+"-applicationCredentialSecret.txt")
								authStrategyPath := filepath.Join(providerEnvDir, hash+"-authStrategy.txt")
								tenantNamePath := filepath.Join(providerEnvDir, hash+"-tenantName.txt")
								usernamePath := filepath.Join(providerEnvDir, hash+"-username.txt")
								passwordPath := filepath.Join(providerEnvDir, hash+"-password.txt")

								Expect(output).To(ContainSubstring(fmt.Sprintf("export OS_AUTH_TYPE=$(< '%s');", authTypePath)))
								Expect(output).To(ContainSubstring(fmt.Sprintf("export OS_APPLICATION_CREDENTIAL_ID=$(< '%s');", applicationCredentialIDPath)))
								Expect(output).To(ContainSubstring(fmt.Sprintf("export OS_APPLICATION_CREDENTIAL_NAME=$(< '%s');", applicationCredentialNamePath)))
								Expect(output).To(ContainSubstring(fmt.Sprintf("export OS_APPLICATION_CREDENTIAL_SECRET=$(< '%s');", applicationCredentialSecretPath)))
								Expect(output).To(ContainSubstring(fmt.Sprintf("export OS_AUTH_STRATEGY=$(< '%s');", authStrategyPath)))
								Expect(output).To(ContainSubstring(fmt.Sprintf("export OS_TENANT_NAME=$(< '%s');", tenantNamePath)))
								Expect(output).To(ContainSubstring(fmt.Sprintf("export OS_USERNAME=$(< '%s');", usernamePath)))
								Expect(output).To(ContainSubstring(fmt.Sprintf("export OS_PASSWORD=$(< '%s');", passwordPath)))

								Expect(os.ReadFile(authTypePath)).To(Equal([]byte("v3applicationcredential")))
								Expect(os.ReadFile(applicationCredentialIDPath)).To(Equal([]byte("")))
								Expect(os.ReadFile(applicationCredentialNamePath)).To(Equal([]byte("app-cred-name")))
								Expect(os.ReadFile(applicationCredentialSecretPath)).To(Equal([]byte("app-cred-secret")))
								Expect(os.ReadFile(authStrategyPath)).To(Equal([]byte("")))
								Expect(os.ReadFile(tenantNamePath)).To(Equal([]byte("")))
								Expect(os.ReadFile(usernamePath)).To(Equal([]byte("")))
								Expect(os.ReadFile(passwordPath)).To(Equal([]byte("")))
							})
						})
					})

					Context("output is json", func() {
						BeforeEach(func() {
							output = "json"
							shell = ""
						})

						It("should render the json successfully", func() {
							client := gardenclientmocks.NewMockClient(ctrl)
							client.EXPECT().GetSecret(ctx, secret.Namespace, secret.Name).Return(secret, nil)
							Expect(options.PrintProviderEnv(ctx, client, shoot, credentialsRef, cloudProfile, nil)).To(Succeed())
							hash := computeTestHash("test-session-id", gardenName, shoot.Namespace, shoot.Name)
							expected := strings.NewReplacer(
								"PLACEHOLDER_CONFIG_DIR", filepath.Join(sessionDir, ".config", "openstack"),
								"PLACEHOLDER_SESSION_DIR", sessionDir,
								"PLACEHOLDER_HASH", hash,
							).Replace(readTestFile("openstack/export.json"))
							Expect(options.String()).To(Equal(expected))
						})
					})
				})

				Context("when the cloudprovider is azure", func() {
					const (
						clientID       = "12345678-1234-1234-1234-123456789012"
						clientSecret   = "AbCdE~fGhI.-jKlMnOpQrStUvWxYz0_123456789"
						tenantID       = "87654321-4321-4321-4321-210987654321"
						subscriptionID = "abcdef12-3456-7890-abcd-ef1234567890"
					)

					BeforeEach(func() {
						shell = "fish"
						providerType = "azure"

						factory := utilmocks.NewMockFactory(ctrl)
						manager := targetmocks.NewMockManager(ctrl)
						factory.EXPECT().GetSessionID().Return("test-session-id", nil)
						factory.EXPECT().Manager().Return(manager, nil)
						factory.EXPECT().TargetFlags().Return(tf)
						factory.EXPECT().GardenHomeDir().Return(gardenHomeDir)
						factory.EXPECT().Context().Return(ctx)
						manager.EXPECT().SessionDir().Return(sessionDir)
						manager.EXPECT().Configuration().Return(cfg)
						Expect(options.Complete(factory, mockCmd, nil)).To(Succeed())
					})

					JustBeforeEach(func() {
						secret.Data["clientID"] = []byte(clientID)
						secret.Data["clientSecret"] = []byte(clientSecret)
						secret.Data["tenantID"] = []byte(tenantID)
						secret.Data["subscriptionID"] = []byte(subscriptionID)
					})

					It("should render the template successfully", func() {
						client := gardenclientmocks.NewMockClient(ctrl)
						client.EXPECT().GetSecret(ctx, secret.Namespace, secret.Name).Return(secret, nil)
						Expect(options.PrintProviderEnv(ctx, client, shoot, credentialsRef, cloudProfile, nil)).To(Succeed())
						hash := computeTestHash("test-session-id", "test", namespace, shootName)
						replacer := strings.NewReplacer(
							"PLACEHOLDER_CONFIG_DIR", filepath.Join(sessionDir, ".config", "az"),
							"PLACEHOLDER_SESSION_DIR", sessionDir,
							"PLACEHOLDER_HASH", hash,
						)
						expected := replacer.Replace(readTestFile("azure/export.fish"))
						Expect(options.String()).To(Equal(expected))
					})

					It("should fail with mkdir error", func() {
						client := gardenclientmocks.NewMockClient(ctrl)
						options.SessionDir = string([]byte{0})
						Expect(options.PrintProviderEnv(ctx, client, shoot, credentialsRef, cloudProfile, nil)).To(MatchError(MatchRegexp("^failed to create az configuration directory:")))
					})

					Context("output is json", func() {
						BeforeEach(func() {
							output = "json"
							shell = ""
						})

						It("should render the json successfully", func() {
							client := gardenclientmocks.NewMockClient(ctrl)
							client.EXPECT().GetSecret(ctx, secret.Namespace, secret.Name).Return(secret, nil)
							Expect(options.PrintProviderEnv(ctx, client, shoot, credentialsRef, cloudProfile, nil)).To(Succeed())
							hash := computeTestHash("test-session-id", "test", namespace, shootName)
							replacer := strings.NewReplacer(
								"PLACEHOLDER_CONFIG_DIR", filepath.Join(sessionDir, ".config", "az"),
								"PLACEHOLDER_SESSION_DIR", sessionDir,
								"PLACEHOLDER_HASH", hash,
							)
							expected := replacer.Replace(readTestFile("azure/export.json"))
							Expect(options.String()).To(Equal(expected))
						})
					})
				})
			})

			Context("with WorkloadIdentity credentials", func() {
				var (
					workloadIdentity  *gardensecurityv1alpha1.WorkloadIdentity
					credentialsConfig map[string]interface{}
				)

				BeforeEach(func() {
					credentialsRef = corev1.ObjectReference{
						Kind:       "WorkloadIdentity",
						APIVersion: gardensecurityv1alpha1.SchemeGroupVersion.String(),
						Namespace:  namespace,
						Name:       "wi-gcp",
					}

					credentialsConfig = map[string]interface{}{
						"type":                              "external_account",
						"audience":                          "//iam.googleapis.com/projects/123456/locations/global/workloadIdentityPools/my-pool/providers/my-provider",
						"subject_token_type":                "urn:ietf:params:oauth:token-type:jwt",
						"service_account_impersonation_url": "https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/my-sa@project.iam.gserviceaccount.com:generateAccessToken",
						"token_url":                         "https://sts.googleapis.com/v1/token",
					}

					providerConfigRaw := map[string]interface{}{
						"projectID":         "my-gcp-project",
						"credentialsConfig": credentialsConfig,
					}

					providerConfigBytes, _ := json.Marshal(providerConfigRaw)

					workloadIdentity = &gardensecurityv1alpha1.WorkloadIdentity{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: namespace,
							Name:      "wi-gcp",
						},
						Spec: gardensecurityv1alpha1.WorkloadIdentitySpec{
							Audiences: []string{"sts.googleapis.com"},
							TargetSystem: gardensecurityv1alpha1.TargetSystem{
								Type: "gcp",
								ProviderConfig: &runtime.RawExtension{
									Raw: providerConfigBytes,
								},
							},
						},
						Status: gardensecurityv1alpha1.WorkloadIdentityStatus{
							Sub: "subject-123",
						},
					}

					// Initialize options with Complete() to set up defaults and session ID
					factory := utilmocks.NewMockFactory(ctrl)
					manager := targetmocks.NewMockManager(ctrl)
					factory.EXPECT().GetSessionID().Return("test-session-id", nil)
					factory.EXPECT().Manager().Return(manager, nil)
					factory.EXPECT().TargetFlags().Return(tf)
					factory.EXPECT().GardenHomeDir().Return(gardenHomeDir)
					factory.EXPECT().Context().Return(ctx)
					manager.EXPECT().SessionDir().Return(sessionDir)
					manager.EXPECT().Configuration().Return(cfg)
					Expect(options.Complete(factory, mockCmd, nil)).To(Succeed())
				})

				It("should render provider-env script and files for workload identity", func() {
					client := gardenclientmocks.NewMockClient(ctrl)
					// CloudProfile is not used for GCP WI provider data generation, but pass it along
					client.EXPECT().CreateWorkloadIdentityToken(ctx, workloadIdentity.Namespace, workloadIdentity.Name, gomock.Any()).Return(&gardensecurityv1alpha1.TokenRequest{
						Status: gardensecurityv1alpha1.TokenRequestStatus{
							Token:               "test-jwt-token",
							ExpirationTimestamp: metav1.Now(),
						},
					}, nil)
					client.EXPECT().GetWorkloadIdentity(ctx, workloadIdentity.Namespace, workloadIdentity.Name).Return(workloadIdentity, nil)

					Expect(options.PrintProviderEnv(ctx, client, shoot, credentialsRef, cloudProfile, nil)).To(Succeed())

					hash := computeTestHash("test-session-id", gardenName, shoot.Namespace, shoot.Name)
					expected := strings.NewReplacer(
						"PLACEHOLDER_CONFIG_DIR", filepath.Join(sessionDir, ".config", "gcloud"),
						"PLACEHOLDER_SESSION_DIR", sessionDir,
						"PLACEHOLDER_HASH", hash,
					).Replace(readTestFile("gcp/export_wi.bash"))
					Expect(options.String()).To(Equal(expected))

					providerEnvDir := filepath.Join(sessionDir, "provider-env")
					credentialsPath := filepath.Join(providerEnvDir, hash+"-credentials.txt")
					projectIDPath := filepath.Join(providerEnvDir, hash+"-project_id.txt")
					tokenPath := filepath.Join(providerEnvDir, hash+"-token.txt")
					subjectPath := filepath.Join(providerEnvDir, hash+"-subject.txt")
					audiencesPath := filepath.Join(providerEnvDir, hash+"-audiences.txt")

					// Verify baseline files
					proj, _ := os.ReadFile(projectIDPath)
					Expect(string(proj)).To(Equal("my-gcp-project"))
					tok, _ := os.ReadFile(tokenPath)
					Expect(string(tok)).To(Equal("test-jwt-token"))
					sub, _ := os.ReadFile(subjectPath)
					Expect(string(sub)).To(Equal("subject-123"))
					aud, _ := os.ReadFile(audiencesPath)
					Expect(string(aud)).To(Equal("[sts.googleapis.com]"))

					// Verify credentials JSON content and credential_source points to token file
					credBytes, _ := os.ReadFile(credentialsPath)
					var credMap map[string]interface{}
					Expect(json.Unmarshal(credBytes, &credMap)).To(Succeed())
					Expect(credMap["type"]).To(Equal("external_account"))
					Expect(credMap["audience"]).To(Equal(credentialsConfig["audience"]))
					Expect(credMap["token_url"]).To(Equal("https://sts.googleapis.com/v1/token"))
					Expect(credMap["service_account_impersonation_url"]).To(Equal("https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/my-sa@project.iam.gserviceaccount.com:generateAccessToken"))
					cs, ok := credMap["credential_source"].(map[string]interface{})
					Expect(ok).To(BeTrue())
					Expect(cs["file"]).To(Equal(tokenPath))
					format, ok := cs["format"].(map[string]interface{})
					Expect(ok).To(BeTrue())
					Expect(format["type"]).To(Equal("text"))
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
				meta = options.GenerateMetadata(cli, "Secret")
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
