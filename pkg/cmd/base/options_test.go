/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package base_test

import (
	"errors"
	"fmt"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"

	"github.com/gardener/gardenctl-v2/internal/util"
	utilmocks "github.com/gardener/gardenctl-v2/internal/util/mocks"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
	basemocks "github.com/gardener/gardenctl-v2/pkg/cmd/base/mocks"
)

var _ = Describe("Base Options", func() {
	Describe("given an instance", func() {
		type barType struct {
			Baz string
		}

		type fooType struct {
			Foo string
			Bar barType
		}

		var (
			options *base.Options
			buf     *util.SafeBytesBuffer
			foo     *fooType
		)

		BeforeEach(func() {
			streams, _, out, _ := util.NewTestIOStreams()
			buf = out
			options = base.NewOptions(streams)
			foo = &fooType{
				Foo: "foo",
				Bar: barType{
					Baz: "baz",
				},
			}
		})

		Context("when the output is empty", func() {
			BeforeEach(func() {
				options.Output = ""
			})

			It("validate should succeed", func() {
				Expect(options.Validate()).To(Succeed())
			})

			It("should print without format", func() {
				Expect(options.PrintObject(foo)).To(Succeed())
				Expect(buf.String()).To(Equal(fmt.Sprintf("&{%s %s}", foo.Foo, foo.Bar)))
			})
		})

		Context("when the output is json", func() {
			BeforeEach(func() {
				options.Output = "json"
			})

			It("validate should succeed", func() {
				Expect(options.Validate()).To(Succeed())
			})

			It("should print with json format", func() {
				Expect(options.PrintObject(foo)).To(Succeed())
				Expect(buf.String()).To(Equal(fmt.Sprintf(
					`{
  "Foo": %q,
  "Bar": {
    "Baz": %q
  }
}
`, foo.Foo, foo.Bar.Baz)))
			})
		})

		Context("when the output is yaml", func() {
			BeforeEach(func() {
				options.Output = "yaml"
			})

			It("validate should succeed", func() {
				Expect(options.Validate()).To(Succeed())
			})

			It("should print with yaml format", func() {
				Expect(options.PrintObject(foo)).To(Succeed())
				Expect(buf.String()).To(Equal(fmt.Sprintf(
					`foo: %s
bar:
  baz: %s
`,
					foo.Foo, foo.Bar.Baz)))
			})
		})

		Context("when the output is invalid", func() {
			BeforeEach(func() {
				options.Output = "invalid"
			})

			It("validate should fail", func() {
				Expect(options.Validate()).To(MatchError(ContainSubstring("--output must be either 'yaml' or 'json")))
			})
		})
	})

	Describe("wrapping the run function", func() {
		var (
			ctrl        *gomock.Controller
			mockFactory *utilmocks.MockFactory
			mockOptions *basemocks.MockCommandOptions
			runE        func(cmd *cobra.Command, args []string) error
			cmd         *cobra.Command
			args        []string
			err         error
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			mockOptions = basemocks.NewMockCommandOptions(ctrl)
			mockFactory = utilmocks.NewMockFactory(ctrl)
			cmd = &cobra.Command{}
			args = []string{"foo", "bar"}
			err = errors.New("error")
		})

		JustBeforeEach(func() {
			runE = base.WrapRunE(mockOptions, mockFactory)
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		Context("when completion and validation passes successfully", func() {
			BeforeEach(func() {
				mockOptions.EXPECT().Complete(mockFactory, cmd, args)
				mockOptions.EXPECT().Validate()
			})

			It("should do the work", func() {
				mockOptions.EXPECT().Run(mockFactory)
				Expect(runE(cmd, args)).To(Succeed())
			})

			It("should fail to do the work", func() {
				mockOptions.EXPECT().Run(mockFactory).Return(err)
				Expect(runE(cmd, args)).To(BeIdenticalTo(err))
			})
		})

		Context("when completion fails", func() {
			It("should fail to run the wrapped options with a completion error", func() {
				mockOptions.EXPECT().Complete(mockFactory, cmd, args).Return(err)
				Expect(runE(cmd, args)).To(MatchError("failed to complete command options: error"))
			})
		})

		Context("when validation fails", func() {
			BeforeEach(func() {
				mockOptions.EXPECT().Complete(mockFactory, cmd, args)
			})

			It("should fail to run the wrapped options with a validation error", func() {
				mockOptions.EXPECT().Validate().Return(err)
				Expect(runE(cmd, args)).To(BeIdenticalTo(err))
			})
		})
	})
})
