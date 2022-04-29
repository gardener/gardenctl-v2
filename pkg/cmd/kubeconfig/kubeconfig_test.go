/*
SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package kubeconfig_test

import (
	"context"
	"errors"
	"io"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardenctl-v2/internal/util"
	utilmocks "github.com/gardener/gardenctl-v2/internal/util/mocks"
	cmdkubeconfig "github.com/gardener/gardenctl-v2/pkg/cmd/kubeconfig"
	"github.com/gardener/gardenctl-v2/pkg/target"
	targetmocks "github.com/gardener/gardenctl-v2/pkg/target/mocks"
)

var _ = Describe("Kubeconfig Command - Options", func() {
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

	Describe("Having a Kubeconfig command instance", func() {
		var (
			streams util.IOStreams
			cmd     *cobra.Command
			out     *util.SafeBytesBuffer
			ctx     context.Context
			config  clientcmd.ClientConfig
		)

		BeforeEach(func() {
			streams, _, out, _ = util.NewTestIOStreams()
			cmd = cmdkubeconfig.NewCmdKubeconfig(factory, streams)

			ctx = context.Background()
			config = &clientcmd.DirectClientConfig{}

			factory.EXPECT().Manager().Return(manager, nil).AnyTimes()
			factory.EXPECT().Context().Return(ctx)

			manager.EXPECT().CurrentTarget().Return(t, nil)
			manager.EXPECT().ClientConfig(ctx, t).Return(config, nil)
		})

		It("should execute the kubeconfig subcommand", func() {
			cmd = cmdkubeconfig.NewCmdKubeconfig(factory, streams)
			cmd.SetArgs([]string{"--output", "yaml"})
			Expect(cmd.Execute()).To(Succeed())
			Expect(out.String()).To(Equal(`apiVersion: v1
clusters: null
contexts: null
current-context: ""
kind: Config
preferences: {}
users: null
`))
		})
	})

	Describe("having an Options instance", func() {
		var (
			err       error
			ctx       context.Context
			config    clientcmd.ClientConfig
			rawConfig clientcmdapi.Config
		)

		BeforeEach(func() {
			ctx = context.Background()
			config = &clientcmd.DirectClientConfig{}
			rawConfig, err = config.RawConfig()
			Expect(err).To(Succeed())
		})

		Describe("completing the command options", func() {
			It("should complete options", func() {
				factory.EXPECT().Manager().Return(manager, nil)
				factory.EXPECT().Context().Return(ctx)
				manager.EXPECT().CurrentTarget().Return(t, nil)
				manager.EXPECT().ClientConfig(ctx, t).Return(config, nil)

				Expect(options.Complete(factory, nil, nil)).To(Succeed())
				Expect(options.PrintObject).NotTo(BeNil())
				Expect(options.RawConfig).To(Equal(rawConfig))
			})

			It("should fail to complete options when the target is empty", func() {
				currentTarget := target.NewTarget("", "", "", "")

				factory.EXPECT().Manager().Return(manager, nil)
				manager.EXPECT().CurrentTarget().Return(currentTarget, nil)

				Expect(options.Complete(factory, nil, nil)).To(MatchError(target.ErrNoGardenTargeted))
			})
		})

		Describe("validating the command options", func() {
			It("should successfully validate the options", func() {
				Expect(options.Validate()).To(Succeed())
			})
		})

		Describe("running the kubeconfig command with the given options", func() {
			JustBeforeEach(func() {
				printer, err := options.PrintFlags.ToPrinter()
				Expect(err).To(Succeed())

				options.PrintObject = printer.PrintObj
			})

			Context("when the command runs successfully", func() {
				It("should return the kubeconfig in yaml format", func() {
					Expect(options.Run(nil)).To(Succeed())
					Expect(options.String()).To(Equal(`apiVersion: v1
clusters: null
contexts: null
current-context: ""
kind: Config
preferences: {}
users: null
`))
				})

				Context("json format", func() {
					BeforeEach(func() {
						options.PrintFlags.OutputFormat = pointer.StringPtr("json")
					})

					It("should return the kubeconfig ", func() {
						Expect(options.Run(nil)).To(Succeed())
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
				})

				It("should return the kubeconfig minified", func() {
					options.Minify = true
					options.RawByteData = true
					options.Context = "context2"

					config = clientcmd.NewDefaultClientConfig(*createTestKubeconfig(), nil)
					rawConfig, err = config.RawConfig()
					Expect(err).To(Succeed())
					options.RawConfig = rawConfig

					Expect(options.Run(nil)).To(Succeed())
					Expect(options.String()).To(Equal(`apiVersion: v1
clusters:
- cluster:
    insecure-skip-tls-verify: true
    server: https://kubernetes:6443/
  name: cluster
contexts:
- context:
    cluster: cluster
    namespace: default2
    user: user2
  name: context2
current-context: context2
kind: Config
preferences: {}
users:
- name: user2
  user:
    token: token2
`))
				})

				Context("when an error occurs during PrintObject", func() {
					var err = errors.New("error")

					It("should fail with an error", func() {
						options.PrintObject = func(_ runtime.Object, _ io.Writer) error {
							return err
						}

						Expect(options.Run(nil)).To(BeIdenticalTo(err))
					})
				})
			})
		})
	})
})

func createTestKubeconfig() *clientcmdapi.Config {
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
	config.Contexts["context"] = &clientcmdapi.Context{
		Namespace: "default",
		AuthInfo:  "user",
		Cluster:   "cluster",
	}
	config.Contexts["context2"] = &clientcmdapi.Context{
		Namespace: "default2",
		AuthInfo:  "user2",
		Cluster:   "cluster",
	}
	config.CurrentContext = "context"

	return config
}
