/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
SPDX-License-Identifier: Apache-2.0
*/

package target_test

import (
	"os"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
	cmdtarget "github.com/gardener/gardenctl-v2/pkg/cmd/target"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("history Command", func() {

	const (
		historyPath = "./history"
	)

	var (
		streams util.IOStreams
		options *base.Options
		out     *util.SafeBytesBuffer
	)

	BeforeEach(func() {
		streams, _, out, _ = util.NewTestIOStreams()
		options = base.NewOptions(streams)
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
})
