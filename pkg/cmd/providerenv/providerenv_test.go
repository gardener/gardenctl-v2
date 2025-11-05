/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package providerenv_test

import (
	"context"
	"path/filepath"
	"strings"

	openstackv1alpha1 "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	corev1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardensecurityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	clientgarden "github.com/gardener/gardenctl-v2/internal/client/garden"
	"github.com/gardener/gardenctl-v2/internal/fake"
	"github.com/gardener/gardenctl-v2/internal/util"
	utilmocks "github.com/gardener/gardenctl-v2/internal/util/mocks"
	"github.com/gardener/gardenctl-v2/pkg/cmd/providerenv"
	"github.com/gardener/gardenctl-v2/pkg/config"
	"github.com/gardener/gardenctl-v2/pkg/env"
	"github.com/gardener/gardenctl-v2/pkg/target"
	targetmocks "github.com/gardener/gardenctl-v2/pkg/target/mocks"
)

var _ = Describe("Env Commands", func() {
	var (
		ctrl    *gomock.Controller
		factory *utilmocks.MockFactory
		manager *targetmocks.MockManager
		streams util.IOStreams
		out     *util.SafeBytesBuffer
		errOut  *util.SafeBytesBuffer
		parent  cobra.Command
		cmd     *cobra.Command
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		factory = utilmocks.NewMockFactory(ctrl)

		manager = targetmocks.NewMockManager(ctrl)
		factory.EXPECT().GetSessionID().Return("test-session-id", nil).AnyTimes()
		factory.EXPECT().Manager().Return(manager, nil).AnyTimes()

		targetFlags := target.NewTargetFlags("", "", "", "", false)
		factory.EXPECT().TargetFlags().Return(targetFlags).AnyTimes()

		streams, _, out, errOut = util.NewTestIOStreams()
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("given a ProviderEnv instance", func() {
		JustBeforeEach(func() {
			cmd = providerenv.NewCmdProviderEnv(factory, streams)

			// Add cmd to a dummy parent to avoid test failure when ProviderEnv command accesses the parent
			parent = cobra.Command{Use: "test-parent"}
			parent.AddCommand(cmd)
			parent.SetOut(out)
			parent.SetErr(errOut)
		})

		It("should have Use, Flags and SubCommands", func() {
			Expect(cmd.Use).To(Equal("provider-env"))
			Expect(cmd.Aliases).To(HaveLen(2))
			Expect(cmd.Aliases).To(Equal([]string{"p-env", "cloud-env"}))
			Expect(cmd.Flag("output")).NotTo(BeNil())
			flag := cmd.Flag("unset")
			Expect(flag).NotTo(BeNil())
			Expect(flag.Shorthand).To(Equal("u"))
			subCmds := cmd.Commands()
			Expect(len(subCmds)).To(Equal(4))
			for _, c := range subCmds {
				Expect(c.Flag("unset")).To(BeIdenticalTo(flag))
				Expect(c.Flag("output")).To(BeNil())
				s := env.Shell(c.Name())
				Expect(s).To(BeElementOf(env.ValidShells()))
			}
		})

		Context("flag parsing", func() {
			var zshCmd *cobra.Command

			BeforeEach(func() {
				// Initialize zshCmd once for the following tests
				for _, c := range cmd.Commands() {
					if c.Name() == "zsh" {
						zshCmd = c
						break
					}
				}
				Expect(zshCmd).NotTo(BeNil())
			})

			It("parses a single JSON object with --openstack-allowed-patterns (stringArray)", func() {
				json := `{"field":"authURL","uri":"https://keystone.example.com:5000/v3"}`

				// Parse flags on the parent command so persistent flags are recognized
				Expect(cmd.ParseFlags([]string{"--openstack-allowed-patterns", json})).To(Succeed())

				values, err := cmd.PersistentFlags().GetStringArray("openstack-allowed-patterns")
				Expect(err).NotTo(HaveOccurred())
				Expect(values).To(HaveLen(1))
				Expect(values[0]).To(Equal(json))
			})

			It("parses multiple JSON objects via repeated --openstack-allowed-patterns", func() {
				json1 := `{"field":"authURL","uri":"https://keystone.example.com:5000/v3"}`
				json2 := `{"field":"authURL","host":"keystone.example.com","path":"/v3"}`

				Expect(cmd.ParseFlags([]string{
					"--openstack-allowed-patterns", json1,
					"--openstack-allowed-patterns", json2,
				})).To(Succeed())

				values, err := cmd.PersistentFlags().GetStringArray("openstack-allowed-patterns")
				Expect(err).NotTo(HaveOccurred())
				Expect(values).To(HaveLen(2))
				Expect(values[0]).To(Equal(json1))
				Expect(values[1]).To(Equal(json2))
			})
		})

		Context("command execution", func() {
			var (
				ctx                    context.Context
				cfg                    *config.Config
				t                      target.Target
				credentialsBindingName string
				cloudProfileName       string
				region                 string
				provider               *gardencorev1beta1.Provider
				secretRef              *corev1.SecretReference
				project                *gardencorev1beta1.Project
				shoot                  *gardencorev1beta1.Shoot
				credentialsBinding     *gardensecurityv1alpha1.CredentialsBinding
				cloudProfile           *gardencorev1beta1.CloudProfile
				providerConfig         *openstackv1alpha1.CloudProfileConfig
				secret                 *corev1.Secret
			)

			BeforeEach(func() {
				t = target.NewTarget("test", "project", "seed", "shoot")
				cfg = &config.Config{
					Gardens: []config.Garden{
						{
							Name: t.GardenName(),
						},
					},
				}

				manager.EXPECT().SessionDir().Return(sessionDir)
				manager.EXPECT().CurrentTarget().Return(t, nil)
				manager.EXPECT().Configuration().Return(cfg).AnyTimes()

				factory.EXPECT().GardenHomeDir().Return(gardenHomeDir)

				ctx = context.Background()
				factory.EXPECT().Context().Return(ctx).AnyTimes()

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
				namespace := "garden-" + t.ProjectName()

				project = &gardencorev1beta1.Project{
					ObjectMeta: metav1.ObjectMeta{
						Name: t.ProjectName(),
					},
					Spec: gardencorev1beta1.ProjectSpec{
						Namespace: ptr.To(namespace),
					},
				}
				shoot = &gardencorev1beta1.Shoot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      t.ShootName(),
						Namespace: namespace,
					},
					Spec: gardencorev1beta1.ShootSpec{
						CloudProfile: &gardencorev1beta1.CloudProfileReference{
							Kind: corev1beta1constants.CloudProfileReferenceKindCloudProfile,
							Name: cloudProfileName,
						},
						Region:                 region,
						CredentialsBindingName: &credentialsBindingName,
						Provider:               *provider.DeepCopy(),
					},
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
						"serviceaccount.json": []byte(readTestFile("gcp/serviceaccount.json")),
					},
				}
				cloudProfile = &gardencorev1beta1.CloudProfile{
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
				}

				client := clientgarden.NewClient(
					nil,
					fake.NewClientWithObjects(project, shoot, credentialsBinding, secret, cloudProfile),
					t.GardenName(),
				)
				manager.EXPECT().GardenClient(t.GardenName()).Return(client, nil)
			})

			It("should output in yaml format", func() {
				parent.SetArgs([]string{"provider-env", "--output", "yaml"})
				Expect(parent.Execute()).To(Succeed())
				configDir := filepath.Join(sessionDir, ".config", "gcloud")
				hash := computeTestHash("test-session-id", t.GardenName(), shoot.Namespace, t.ShootName())
				replacer := strings.NewReplacer(
					"PLACEHOLDER_CONFIG_DIR", configDir,
					"PLACEHOLDER_SESSION_DIR", sessionDir,
					"PLACEHOLDER_HASH", hash,
				)
				expectedOutput := replacer.Replace(readTestFile("gcp/export.yaml"))
				Expect(out.String()).To(Equal(expectedOutput))
			})
		})
	})
})
