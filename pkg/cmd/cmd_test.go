/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package cmd_test

import (
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd"
	"github.com/gardener/gardenctl-v2/pkg/target"
)

var _ = Describe("Root Command", func() {
	var (
		factory     *util.FactoryImpl
		streams     util.IOStreams
		out         *util.SafeBytesBuffer
		targetFlags target.TargetFlags
	)

	BeforeEach(func() {
		streams, _, out, _ = util.NewTestIOStreams()

		targetFlags = target.NewTargetFlags("", "", "", "", false)

		factory = &util.FactoryImpl{
			TargetFlags: targetFlags,
			ConfigFile:  configFile,
		}
	})

	It("should initialize command and factory correctly", func() {
		args := []string{
			"completion",
			"zsh",
		}

		cmd := cmd.NewGardenctlCommand(factory, streams)
		cmd.SetArgs(args)
		Expect(cmd.Execute()).To(Succeed())

		head := strings.Split(out.String(), "\n")[0]
		Expect(head).To(Equal("#compdef _gardenctl gardenctl"))
		Expect(factory.ConfigFile).To(Equal(configFile))
		Expect(factory.TargetFile).To(Equal(targetFile))
		Expect(factory.GardenHomeDirectory).To(Equal(gardenHomeDir))
	})
})
