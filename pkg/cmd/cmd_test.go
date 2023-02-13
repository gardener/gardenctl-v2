/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package cmd_test

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd"
	"github.com/gardener/gardenctl-v2/pkg/target"
)

var _ = Describe("Gardenctl command", func() {
	const (
		projectName  = "prod1"
		shootName    = "test-shoot1"
		nodeHostname = "example.host.invalid"
	)

	var (
		gardenName string
		streams    util.IOStreams
		out        *util.SafeBytesBuffer
	)

	BeforeEach(func() {
		gardenName = cfg.Gardens[0].Name

		streams, _, out, _ = util.NewTestIOStreams()

		targetProvider := target.NewTargetProvider(filepath.Join(sessionDir, "target.yaml"), nil)
		Expect(targetProvider.Write(target.NewTarget(gardenName, projectName, "", shootName))).To(Succeed())
	})

	Describe("Depending on the factory implementation", func() {
		var factory *util.FactoryImpl

		BeforeEach(func() {
			factory = util.NewFactoryImpl()
		})

		Context("when running the completion command", func() {
			It("should initialize command and factory correctly", func() {
				args := []string{
					"completion",
					"zsh",
				}

				cmd := cmd.NewGardenctlCommand(factory, streams)
				cmd.SetArgs(args)
				Expect(cmd.Execute()).To(Succeed())

				head := strings.Split(out.String(), "\n")[0]
				Expect(head).To(Equal("#compdef gardenctl"))
				Expect(factory.ConfigFile).To(Equal(configFile))
				Expect(factory.GardenHomeDirectory).To(Equal(gardenHomeDir))

				manager, err := factory.Manager()
				Expect(err).NotTo(HaveOccurred())

				// check target flags
				tf := manager.TargetFlags()
				Expect(tf).To(BeIdenticalTo(factory.TargetFlags()))

				// check current target values
				current, err := manager.CurrentTarget()
				Expect(err).NotTo(HaveOccurred())
				Expect(current.GardenName()).To(Equal(gardenName))
				Expect(current.ProjectName()).To(Equal(projectName))
				Expect(current.SeedName()).To(BeEmpty())
				Expect(current.ShootName()).To(Equal(shootName))
			})
		})
	})

	Context("when running the help command", func() {
		var (
			sessionID     string
			termSessionID string
		)

		BeforeEach(func() {
			sessionID = os.Getenv("GCTL_SESSION_ID")
			termSessionID = os.Getenv("TERM_SESSION_ID")

			Expect(os.Unsetenv("GCTL_SESSION_ID")).To(Succeed())
			Expect(os.Unsetenv("TERM_SESSION_ID")).To(Succeed())
		})

		AfterEach(func() {
			// restore env variables
			Expect(os.Setenv("GCTL_SESSION_ID", sessionID)).To(Succeed())
			Expect(os.Setenv("TERM_SESSION_ID", termSessionID)).To(Succeed())
		})

		It("should succeed without session IDs", func() {
			args := []string{
				"help",
			}

			cmd := cmd.NewGardenctlCommand(util.NewFactoryImpl(), streams)
			cmd.SetArgs(args)
			Expect(cmd.Execute()).To(Succeed())
		})
	})
})
