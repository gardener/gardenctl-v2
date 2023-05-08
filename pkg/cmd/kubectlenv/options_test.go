/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package kubectlenv_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/pointer"

	utilmocks "github.com/gardener/gardenctl-v2/internal/util/mocks"
	"github.com/gardener/gardenctl-v2/pkg/cmd/kubectlenv"
	"github.com/gardener/gardenctl-v2/pkg/config"
	"github.com/gardener/gardenctl-v2/pkg/env"
	envmocks "github.com/gardener/gardenctl-v2/pkg/env/mocks"
	"github.com/gardener/gardenctl-v2/pkg/target"
	targetmocks "github.com/gardener/gardenctl-v2/pkg/target/mocks"
)

var _ = Describe("Env Commands - Options", func() {
	Describe("having an Options instance", func() {
		var (
			ctrl    *gomock.Controller
			factory *utilmocks.MockFactory
			manager *targetmocks.MockManager
			options *kubectlenv.TestOptions
			cmdPath,
			shell string
			unset        bool
			baseTemplate env.Template
			cfg          *config.Config
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			factory = utilmocks.NewMockFactory(ctrl)
			manager = targetmocks.NewMockManager(ctrl)
			options = kubectlenv.NewOptions()
			cmdPath = "gardenctl provider-env"
			baseTemplate = env.NewTemplate("helpers")
			shell = "default"
			cfg = &config.Config{
				LinkKubeconfig: pointer.Bool(false),
				Gardens:        []config.Garden{{Name: "test"}},
			}
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		JustBeforeEach(func() {
			options.Shell = shell
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
				root.SetArgs([]string{"alias", "child"})
				Expect(root.Execute()).To(Succeed())
				baseTemplate = nil
			})

			It("should complete options for providerType kubernetes", func() {
				factory.EXPECT().Manager().Return(manager, nil)
				manager.EXPECT().SessionDir().Return(sessionDir)
				manager.EXPECT().Configuration().Return(cfg)
				Expect(options.Template).To(BeNil())
				Expect(options.Complete(factory, child, nil)).To(Succeed())
				Expect(options.Template).NotTo(BeNil())
				t, ok := options.Template.(kubectlenv.TestTemplate)
				Expect(ok).To(BeTrue())
				Expect(t.Delegate().Lookup("usage-hint")).NotTo(BeNil())
				Expect(t.Delegate().Lookup("bash")).NotTo(BeNil())
			})

			It("should fail to complete options for providerType kubernetes", func() {
				writeTempFile(filepath.Join("templates", "kubernetes.tmpl"), "{{define")
				Expect(options.Complete(factory, child, nil)).To(MatchError(MatchRegexp("^parsing template \\\"kubernetes\\\" failed:")))
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

		Describe("running the kubectl-env command with the given options", func() {
			var (
				ctx              context.Context
				mockTemplate     *envmocks.MockTemplate
				t                target.Target
				pathToKubeconfig string
				config           clientcmd.ClientConfig
			)

			BeforeEach(func() {
				ctx = context.Background()
				mockTemplate = envmocks.NewMockTemplate(ctrl)
				baseTemplate = mockTemplate
				t = target.NewTarget("test", "project", "seed", "shoot")
				cmdPath = "gardenctl kubectl-env"
				shell = "bash"
				pathToKubeconfig = "/path/to/kube/config"
				config = &clientcmd.DirectClientConfig{}

				factory.EXPECT().Context().Return(ctx)
				factory.EXPECT().Manager().Return(manager, nil)
			})

			Context("when the command runs successfully", func() {
				Context("and the shoot is targeted via project", func() {
					It("does the work when the shoot is targeted via project", func() {
						currentTarget := t.WithSeedName("")
						manager.EXPECT().CurrentTarget().Return(currentTarget, nil)
						manager.EXPECT().ClientConfig(ctx, currentTarget).Return(config, nil)
						manager.EXPECT().WriteClientConfig(config).Return(pathToKubeconfig, nil)
						mockTemplate.EXPECT().ExecuteTemplate(options.IOStreams.Out, shell, gomock.Any()).
							Do(func(_ io.Writer, _ string, data map[string]interface{}) {
								Expect(data["filename"]).To(Equal(pathToKubeconfig))
								metadata, ok := data["__meta"].(map[string]interface{})
								Expect(ok).To(BeTrue())
								Expect(metadata["cli"]).To(Equal("kubectl"))
								Expect(metadata["commandPath"]).To(Equal(cmdPath))
								Expect(metadata["shell"]).To(Equal(shell))
								Expect(metadata["unset"]).To(Equal(unset))
							}).Return(nil)
						Expect(options.Run(factory)).To(Succeed())
					})
				})
			})

			Context("when an error occurs", func() {
				var currentTarget target.Target

				JustBeforeEach(func() {
					manager.EXPECT().CurrentTarget().Return(currentTarget, nil)
				})

				Context("because no garden is targeted", func() {
					BeforeEach(func() {
						currentTarget = t.WithGardenName("")
					})

					It("should fail with ErrNoShootTargeted", func() {
						Expect(options.Run(factory)).To(BeIdenticalTo(target.ErrNoGardenTargeted))
					})
				})

				Context("because reading kubeconfig fails", func() {
					err := errors.New("error")

					BeforeEach(func() {
						currentTarget = t.WithGardenName("test")
					})

					It("should fail with a read error", func() {
						manager.EXPECT().ClientConfig(ctx, currentTarget).Return(nil, err)
						Expect(options.Run(factory)).To(BeIdenticalTo(err))
					})

					It("should fail with a write error", func() {
						manager.EXPECT().ClientConfig(ctx, currentTarget).Return(config, nil)
						manager.EXPECT().WriteClientConfig(config).Return("", err)
						Expect(options.Run(factory)).To(BeIdenticalTo(err))
					})
				})
			})
		})

		Describe("rendering the usage hint", func() {
			var (
				targetFlags string
				meta        map[string]interface{}
				t           = target.NewTarget("test", "project", "seed", "shoot")
			)

			BeforeEach(func() {
				shell = "bash"
				unset = false
				options.Target = t.WithSeedName("")
			})

			JustBeforeEach(func() {
				meta = options.GenerateMetadata()
				targetFlags = kubectlenv.GetTargetFlags(t)
				Expect(env.NewTemplate("helpers").ExecuteTemplate(options.IOStreams.Out, "usage-hint", meta)).To(Succeed())
			})

			Context("when configuring the shell", func() {
				It("should generate the metadata and render the export hint", func() {
					Expect(meta["unset"]).To(BeFalse())
					Expect(meta["shell"]).To(Equal(shell))
					Expect(meta["cli"]).To(Equal("kubectl"))
					Expect(meta["commandPath"]).To(Equal(options.CmdPath))
					Expect(meta["targetFlags"]).To(Equal(targetFlags))
					regex := regexp.MustCompile(`(?m)\A\n(.*)\n(.*)\n\z`)
					match := regex.FindStringSubmatch(options.String())
					Expect(match).NotTo(BeNil())
					Expect(len(match)).To(Equal(3))
					Expect(match[1]).To(Equal("# Run this command to configure kubectl for your shell:"))
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
					Expect(match[1]).To(Equal("# Run this command to reset the kubectl configuration for your shell:"))
					Expect(match[2]).To(Equal(fmt.Sprintf("# eval $(%s -u %s)", options.CmdPath, shell)))
				})
			})
		})
	})
})
