/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
SPDX-License-Identifier: Apache-2.0
*/

package target_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	internalfake "github.com/gardener/gardenctl-v2/internal/fake"
	"github.com/gardener/gardenctl-v2/internal/util"
	cmdtarget "github.com/gardener/gardenctl-v2/pkg/cmd/target"
	"github.com/gardener/gardenctl-v2/pkg/target"
)

var _ = Describe("Target View Command", func() {
	const (
		gardenName  = "mygarden"
		projectName = "myproject"
		shootName   = "myshoot"
	)

	var (
		streams        util.IOStreams
		out            *util.SafeBytesBuffer
		factory        *internalfake.Factory
		targetProvider *internalfake.TargetProvider
		currentTarget  target.Target
	)

	BeforeEach(func() {
		streams, _, out, _ = util.NewTestIOStreams()
		currentTarget = target.NewTarget(gardenName, projectName, "", shootName)
	})

	JustBeforeEach(func() {
		targetProvider = internalfake.NewFakeTargetProvider(currentTarget)
		factory = internalfake.NewFakeFactory(nil, nil, nil, targetProvider)
	})

	It("should print current target information", func() {
		// user has already targeted a garden, project and shoot
		o := cmdtarget.NewViewOptions(streams)
		cmd := cmdtarget.NewCmdView(factory, o)

		Expect(cmd.RunE(cmd, nil)).To(Succeed())
		Expect(out.String()).To(Equal(fmt.Sprintf("garden:\"%s\", project:\"%s\", shoot:\"%s\"", gardenName, projectName, shootName)))
	})
})

var _ = Describe("Target View Options", func() {
	It("should validate", func() {
		streams, _, _, _ := util.NewTestIOStreams()
		o := cmdtarget.NewViewOptions(streams)
		Expect(o.Validate()).ToNot(HaveOccurred())
	})
})
