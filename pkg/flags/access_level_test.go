/*
SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package flags_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"

	"github.com/gardener/gardenctl-v2/pkg/config"
	"github.com/gardener/gardenctl-v2/pkg/flags"
)

var _ = Describe("AddKubeconfigAccessLevelFlag", func() {
	parse := func(args []string) (config.KubeconfigAccessLevel, error) {
		var level config.KubeconfigAccessLevel

		root := &cobra.Command{Use: "root", RunE: func(*cobra.Command, []string) error { return nil }}
		flags.AddKubeconfigAccessLevelFlag(root, &level)

		// Subcommand to verify persistent inheritance + cross-subcommand mutex.
		root.AddCommand(&cobra.Command{Use: "child", RunE: func(*cobra.Command, []string) error { return nil }})

		root.SetArgs(args)
		root.SetOut(GinkgoWriter)
		root.SetErr(GinkgoWriter)

		return level, root.Execute()
	}

	DescribeTable("parses each form",
		func(args []string, want config.KubeconfigAccessLevel) {
			level, err := parse(args)
			Expect(err).NotTo(HaveOccurred())
			Expect(level).To(Equal(want))
		},
		Entry("--access-level=admin", []string{"--access-level=admin"}, config.KubeconfigAccessLevelAdmin),
		Entry("--access-level=viewer", []string{"--access-level=viewer"}, config.KubeconfigAccessLevelViewer),
		Entry("--access-level=auto", []string{"--access-level=auto"}, config.KubeconfigAccessLevelAuto),
		Entry("--admin (NoOptDefVal accepts no =true)", []string{"--admin"}, config.KubeconfigAccessLevelAdmin),
		Entry("--viewer (NoOptDefVal accepts no =true)", []string{"--viewer"}, config.KubeconfigAccessLevelViewer),
		Entry("subcommand inherits persistent flag", []string{"child", "--viewer"}, config.KubeconfigAccessLevelViewer),
	)

	DescribeTable("rejects invalid combinations",
		func(args []string, errSubstring string) {
			_, err := parse(args)
			Expect(err).To(MatchError(ContainSubstring(errSubstring)))
		},
		Entry("--admin --viewer (mutually exclusive)", []string{"--admin", "--viewer"}, "none of the others can be"),
		Entry("--admin --access-level=viewer (mutually exclusive)", []string{"--admin", "--access-level=viewer"}, "none of the others can be"),
		Entry("--viewer --access-level=admin (mutually exclusive)", []string{"--viewer", "--access-level=admin"}, "none of the others can be"),
		Entry("subcommand --admin --viewer still mutex (persistent flags + annotation propagate)", []string{"child", "--admin", "--viewer"}, "none of the others can be"),
		Entry("--admin=false (rejected, switches not toggles)", []string{"--admin=false"}, "does not accept false"),
		Entry("--viewer=false (rejected)", []string{"--viewer=false"}, "does not accept false"),
		Entry("--admin=garbage (not a bool)", []string{"--admin=garbage"}, "invalid boolean value"),
		Entry("--access-level=guest (not in enum)", []string{"--access-level=guest"}, "invalid kubeconfig access level"),
	)

	It("leaves the value empty when no flag is set", func() {
		level, err := parse([]string{})
		Expect(err).NotTo(HaveOccurred())
		Expect(level).To(BeEmpty())
	})
})
