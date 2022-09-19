/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
SPDX-License-Identifier: Apache-2.0
*/

package target_test

import (
	"os"

	internalfake "github.com/gardener/gardenctl-v2/internal/fake"
	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
	cmdtarget "github.com/gardener/gardenctl-v2/pkg/cmd/target"
	"github.com/gardener/gardenctl-v2/pkg/target"

	"github.com/spf13/cobra"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("history Command", func() {

	const (
		historyPath = "./history"
		gardenName  = "mygarden"
		projectName = "myproject"
		shootName   = "myshoot"
	)

	var (
		streams        util.IOStreams
		options        *base.Options
		out            *util.SafeBytesBuffer
		factory        *internalfake.Factory
		targetProvider *internalfake.TargetProvider
		currentTarget  target.Target
		cmd            *cobra.Command
	)

	BeforeEach(func() {
		streams, _, out, _ = util.NewTestIOStreams()
		options = base.NewOptions(streams)
		cmd = &cobra.Command{}
		currentTarget = target.NewTarget(gardenName, projectName, "", shootName)
	})

	JustBeforeEach(func() {
		targetProvider = internalfake.NewFakeTargetProvider(currentTarget)
		factory = internalfake.NewFakeFactory(nil, nil, nil, targetProvider)
	})

	AfterSuite(func() {
		err := os.Remove(historyPath)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("#HistoryWrite", func() {
		It("should write history file", func() {
			err := cmdtarget.HistoryWrite(historyPath, "hello")
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("#HistoryOutput", func() {
		It("should print history output", func() {
			err := cmdtarget.HistoryOutput(historyPath, *options)
			Expect(err).NotTo(HaveOccurred())
			Expect(out.String()).Should(ContainSubstring("hello"))
		})
	})

	Describe("#HistoryParse", func() {
		It("should succeed print history parse", func() {
			string, err := cmdtarget.HistoryParse(factory, cmd, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(string).Should(ContainSubstring("--garden mygarden --project myproject --shoot myshoot"))
		})

		It("should succeed print history parse", func() {
			string, err := cmdtarget.HistoryParse(factory, cmd, "target")
			Expect(err).NotTo(HaveOccurred())
			Expect(string).Should(ContainSubstring("--garden mygarden --project myproject --shoot myshoot"))
		})

	})
})
