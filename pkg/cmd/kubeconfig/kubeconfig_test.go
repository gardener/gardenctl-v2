/*
SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package kubeconfig_test

import (
	"context"
	"errors"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/utils/pointer"

	utilmocks "github.com/gardener/gardenctl-v2/internal/util/mocks"
	cmdkubeconfig "github.com/gardener/gardenctl-v2/pkg/cmd/kubeconfig"
	"github.com/gardener/gardenctl-v2/pkg/target"
	targetmocks "github.com/gardener/gardenctl-v2/pkg/target/mocks"
)

var _ = Describe("Target Commands - Options", func() {
	Describe("having an Options instance", func() {
		var (
			ctrl    *gomock.Controller
			factory *utilmocks.MockFactory
			manager *targetmocks.MockManager
			options *cmdkubeconfig.TestOptions
			t       target.Target
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			factory = utilmocks.NewMockFactory(ctrl)
			manager = targetmocks.NewMockManager(ctrl)
			options = cmdkubeconfig.NewOptions()
			t = target.NewTarget("test", "project", "seed", "shoot")
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		Describe("completing the command options", func() {
			var (
				root,
				parent *cobra.Command
			)

			BeforeEach(func() {
				root = &cobra.Command{Use: "root"}
				parent = &cobra.Command{Use: "parent", Aliases: []string{"alias"}}
				root.AddCommand(parent)
				root.SetArgs([]string{"alias", "child"})
				Expect(root.Execute()).To(Succeed())
			})

			It("should complete options with current target and print object", func() {
				factory.EXPECT().Manager().Return(manager, nil)
				manager.EXPECT().CurrentTarget().Return(t, nil)
				Expect(options.Complete(factory, parent, nil)).To(Succeed())
				Expect(options.CurrentTarget).To(Equal(t))
				Expect(options.PrintObject).NotTo(BeNil())
			})

			It("should fail to complete options", func() {
				err := errors.New("error")
				factory.EXPECT().Manager().Return(nil, err)
				Expect(options.Complete(factory, parent, nil)).To(MatchError(err))
			})
		})

		Describe("validating the command options", func() {
			It("should successfully validate the options", func() {
				options.CurrentTarget = target.NewTarget("my-garden", "", "", "")
				Expect(options.Validate()).To(Succeed())
			})

			It("should return an error when the target is empty", func() {
				options.CurrentTarget = target.NewTarget("", "", "", "")
				Expect(options.Validate()).To(MatchError(target.ErrNoGardenTargeted))
			})
		})

		Describe("running the kubeconfig command with the given options", func() {
			var (
				ctx    context.Context
				config clientcmd.ClientConfig
			)

			BeforeEach(func() {
				ctx = context.Background()
				config = &clientcmd.DirectClientConfig{}
			})

			Context("when the command runs successfully", func() {
				BeforeEach(func() {
					factory.EXPECT().Manager().Return(manager, nil).AnyTimes()
					factory.EXPECT().Context().Return(ctx)
				})

				It("should return the kubeconfig in yaml format", func() {
					manager.EXPECT().CurrentTarget().Return(t, nil)
					manager.EXPECT().ClientConfig(ctx, t).Return(config, nil)
					Expect(options.Complete(factory, nil, nil)).To(Succeed())
					Expect(options.Run(factory)).To(Succeed())
					Expect(options.String()).To(Equal(`apiVersion: v1
clusters: null
contexts: null
current-context: ""
kind: Config
preferences: {}
users: null
`))
				})

				It("should return the kubeconfig in json format", func() {
					options.PrintFlags.OutputFormat = pointer.StringPtr("json")

					manager.EXPECT().CurrentTarget().Return(t, nil)
					manager.EXPECT().ClientConfig(ctx, t).Return(config, nil)

					Expect(options.Complete(factory, nil, nil)).To(Succeed())
					Expect(options.Run(factory)).To(Succeed())
					Expect(options.String()).To(Equal(`{
    "kind": "Config",
    "apiVersion": "v1",
    "preferences": {},
    "clusters": null,
    "users": null,
    "contexts": null,
    "current-context": ""
}
`))
				})

				It("should return the kubeconfig minified", func() {
					options.Minify = true
					options.RawByteData = true
					config = clientcmd.NewDefaultClientConfig(*createTestKubeconfig("context-a"), nil)

					manager.EXPECT().CurrentTarget().Return(t, nil)
					manager.EXPECT().ClientConfig(ctx, t).Return(config, nil)

					Expect(options.Complete(factory, nil, nil)).To(Succeed())
					Expect(options.Run(factory)).To(Succeed())
					Expect(options.String()).To(Equal(`apiVersion: v1
clusters:
- cluster:
    insecure-skip-tls-verify: true
    server: https://kubernetes:6443/
  name: cluster
contexts:
- context:
    cluster: cluster
    namespace: default
    user: user
  name: context-a
current-context: context-a
kind: Config
preferences: {}
users:
- name: user
  user:
    token: token
`))
				})

				Context("when an error occurs", func() {
					var currentTarget target.Target

					BeforeEach(func() {
						factory.EXPECT().Manager().Return(manager, nil).AnyTimes()
					})

					JustBeforeEach(func() {
						manager.EXPECT().CurrentTarget().Return(currentTarget, nil)
					})

					Context("because fetching kubeconfig fails", func() {
						var err = errors.New("error")

						BeforeEach(func() {
							currentTarget = t.WithGardenName("test")
						})

						It("should fail with a read error", func() {
							manager.EXPECT().ClientConfig(ctx, currentTarget).Return(nil, err)
							Expect(options.Complete(factory, nil, nil)).To(Succeed())
							Expect(options.Run(factory)).To(BeIdenticalTo(err))
						})
					})
				})
			})
		})
	})
})

var _ = Describe("Kubeconfig Options", func() {
	It("should validate", func() {
		o := cmdkubeconfig.NewOptions()
		o.CurrentTarget = target.NewTarget("my-garden", "", "", "")

		Expect(o.Validate()).To(Succeed())
	})

	It("should reject empty target", func() {
		o := cmdkubeconfig.NewOptions()
		o.CurrentTarget = target.NewTarget("", "", "", "")

		Expect(o.Validate()).ToNot(Succeed())
	})
})

func createTestKubeconfig(name string) *clientcmdapi.Config {
	config := clientcmdapi.NewConfig()
	config.Clusters["cluster"] = &clientcmdapi.Cluster{
		Server:                "https://kubernetes:6443/",
		InsecureSkipTLSVerify: true,
	}
	config.AuthInfos["user"] = &clientcmdapi.AuthInfo{
		Token: "token",
	}
	config.AuthInfos["user2"] = &clientcmdapi.AuthInfo{
		Token: "token2",
	}
	config.Contexts[name] = &clientcmdapi.Context{
		Namespace: "default",
		AuthInfo:  "user",
		Cluster:   "cluster",
	}
	config.Contexts["context2"] = &clientcmdapi.Context{
		Namespace: "default2",
		AuthInfo:  "user2",
		Cluster:   "cluster",
	}
	config.CurrentContext = name

	return config
}
