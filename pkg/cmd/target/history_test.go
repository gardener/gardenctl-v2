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
		currentTarget  target.Target
		factory        *internalfake.Factory
		targetProvider *internalfake.TargetProvider
	)

	BeforeEach(func() {
		streams, _, out, _ = util.NewTestIOStreams()
		options = base.NewOptions(streams)
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

	Describe("PersistentPostRunE", func() {
		It("should PersistentPostRunE succeed", func() {
			cmd := cmdtarget.NewCmdTarget(factory, streams)
			Expect(cmd.PersistentPostRunE(cmd, nil)).To(Succeed())
		})
	})

	Describe("#HistoryWrite", func() {
		It("should write history file", func() {
			err := cmdtarget.HistoryWrite(historyPath, "hello")
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("#ToHistoryOutput", func() {
		It("should print history output", func() {
			err := cmdtarget.ToHistoryOutput(historyPath, *options)
			Expect(err).NotTo(HaveOccurred())
			Expect(out.String()).Should(ContainSubstring("hello"))
		})

		It("should print history output from command level", func() {
			o := cmdtarget.NewHistoryOptions(streams)
			cmd := cmdtarget.NewCmdTarget(factory, streams)
			err := o.Complete(factory, cmd, os.Args)
			Expect(err).NotTo(HaveOccurred())
			err = o.Run(factory)
			Expect(err).NotTo(HaveOccurred())
			Expect(out.String()).Should(ContainSubstring("target --garden mygarden --project myproject --shoot myshoot"))
		})
	})

	Describe("#ToHistoryParse", func() {
		It("should succeed execute history parse", func() {
			string, err := cmdtarget.ToHistoryParse(currentTarget)
			Expect(err).NotTo(HaveOccurred())
			Expect(string).Should((ContainSubstring("target --garden mygarden --project myproject --shoot myshoot")))
		})
	})
})
