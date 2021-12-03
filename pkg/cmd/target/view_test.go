/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
SPDX-License-Identifier: Apache-2.0
*/

package target_test

import (
	"fmt"

	internalfake "github.com/gardener/gardenctl-v2/internal/fake"
	"github.com/gardener/gardenctl-v2/internal/util"
	cmdtarget "github.com/gardener/gardenctl-v2/pkg/cmd/target"
	"github.com/gardener/gardenctl-v2/pkg/config"
	"github.com/gardener/gardenctl-v2/pkg/target"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Command", func() {
	It("should print current target information", func() {
		// user has already targeted a garden, project and shoot
		gardenName := "mygarden"
		projectName := "myproject"
		shootName := "myshoot"
		cfg := &config.Config{
			Gardens: []config.Garden{{
				ClusterIdentity: gardenName,
				Kubeconfig:      "",
			}},
		}
		currentTarget := target.NewTarget(gardenName, projectName, "", shootName)

		// setup command
		targetProvider := internalfake.NewFakeTargetProvider(currentTarget)
		clientProvider := internalfake.NewFakeClientProvider()

		factory := internalfake.NewFakeFactory(cfg, nil, clientProvider, nil, targetProvider)
		streams, _, out, _ := util.NewTestIOStreams()
		o := cmdtarget.NewViewOptions(streams)
		cmd := cmdtarget.NewCmdView(factory, o)

		Expect(cmd.RunE(cmd, nil)).To(Succeed())
		Expect(out.String()).To(Equal(fmt.Sprintf("garden:\"%s\", project:\"%s\", shoot:\"%s\"", gardenName, projectName, shootName)))
	})
})

var _ = Describe("ViewOptions", func() {
	It("should validate", func() {
		streams, _, _, _ := util.NewTestIOStreams()
		o := cmdtarget.NewViewOptions(streams)
		err := o.Validate()
		Expect(err).ToNot(HaveOccurred())
	})
})
