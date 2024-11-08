/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
SPDX-License-Identifier: Apache-2.0
*/

package target_test

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardenctl-v2/internal/util"
	utilmocks "github.com/gardener/gardenctl-v2/internal/util/mocks"
	cmdtarget "github.com/gardener/gardenctl-v2/pkg/cmd/target"
	"github.com/gardener/gardenctl-v2/pkg/target"
	targetmocks "github.com/gardener/gardenctl-v2/pkg/target/mocks"
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
		ctrl           *gomock.Controller
		factory        *utilmocks.MockFactory
		manager        *targetmocks.MockManager
		targetProvider target.TargetProvider
		currentTarget  target.Target
		sessionDir     string
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		factory = utilmocks.NewMockFactory(ctrl)
		manager = targetmocks.NewMockManager(ctrl)

		factory.EXPECT().Manager().Return(manager, nil)

		streams, _, out, _ = util.NewTestIOStreams()

		targetFlags := target.NewTargetFlags("", "", "", "", false)
		factory.EXPECT().TargetFlags().Return(targetFlags).AnyTimes()

		sessionID := uuid.New().String()
		sessionDir = filepath.Join(os.TempDir(), "garden", "sessions", sessionID)
		Expect(os.MkdirAll(sessionDir, os.ModePerm))
		currentTarget = target.NewTarget(gardenName, projectName, "", shootName)
		targetProvider = target.NewTargetProvider(filepath.Join(sessionDir, "target.yaml"), targetFlags)
		Expect(targetProvider.Write(currentTarget)).To(Succeed())

		manager.EXPECT().CurrentTarget().DoAndReturn(func() (target.Target, error) {
			return targetProvider.Read()
		})
	})

	AfterEach(func() {
		Expect(os.RemoveAll(sessionDir)).To(Succeed())
		ctrl.Finish()
	})

	It("should print current target information in yaml format", func() {
		// user has already targeted a garden, project and shoot
		cmd := cmdtarget.NewCmdView(factory, streams)
		Expect(cmd.RunE(cmd, nil)).To(Succeed())
		Expect(out.String()).To(Equal(fmt.Sprintf("garden: %s\nproject: %s\nshoot: %s\n", gardenName, projectName, shootName)))
	})

	Context("when target flags given", func() {
		It("should print current target information in yaml format", func() {
			cmd := cmdtarget.NewCmdView(factory, streams)
			cmd.SetArgs([]string{"--shoot", "myshoot2"})
			Expect(cmd.Execute()).To(Succeed())
			Expect(cmd.Flag("shoot").Value.String()).To(Equal("myshoot2"))

			// here we need the real manager
			Expect(out.String()).To(Equal(fmt.Sprintf("garden: %s\nproject: %s\nshoot: %s\n", gardenName, projectName, "myshoot2")))
		})
	})
})

var _ = Describe("Target View Options", func() {
	It("should validate", func() {
		o := &cmdtarget.TargetViewOptions{}
		Expect(o.Validate()).ToNot(HaveOccurred())
	})
})
