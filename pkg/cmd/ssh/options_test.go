/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package ssh_test

import (
	"io"
	"io/ioutil"
	"os"

	"github.com/gardener/gardenctl-v2/internal/util"
	. "github.com/gardener/gardenctl-v2/pkg/cmd/ssh"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Options", func() {
	var (
		publicSSHKeyFile string
	)

	BeforeEach(func() {
		tmpFile, err := os.CreateTemp("", "")
		Expect(err).NotTo(HaveOccurred())
		defer tmpFile.Close()

		// write dummy SSH public key
		_, err = io.WriteString(tmpFile, "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDouNkxsNuApuKVIfgL6Yz3Ep+DqX84Yde9DArwLBSWgLnl/pH9AbbcDcAmdB2CPVXAATo4qxK7xprvyyZp52SQRCcAZpAy4D6gAWwAG3OfzrRbxRiB5pQDaaWATSzNbLtoy0ecVwFeTJe2w71q+wxbI7tfxbvo9XbXIN4I0cQy2KLICzkYkQmygGnHztv1Mvi338+sgcG7Gwq2tdSyggDaAggwDIuT39S4/L7QpR27tWH79J4Ls8tTHud2eRbkOcF98vXlQAIzb6w8iHBXylOjMM/oODwoA7V4mtRL9o13AoocvZSsD1UvfOjGxDHuLrCfFXN+/rEw0hEiYo0cnj7F")
		Expect(err).NotTo(HaveOccurred())

		publicSSHKeyFile = tmpFile.Name()
	})

	AfterEach(func() {
		os.Remove(publicSSHKeyFile)
	})

	It("should validate", func() {
		streams, _, _, _ := util.NewTestIOStreams()
		o := NewOptions(streams)
		o.CIDRs = []string{"8.8.8.8/32"}
		o.SSHPublicKeyFile = publicSSHKeyFile

		Expect(o.Validate()).To(Succeed())
	})

	It("should require a non-zero wait time", func() {
		streams, _, _, _ := util.NewTestIOStreams()
		o := NewOptions(streams)
		o.CIDRs = []string{"8.8.8.8/32"}
		o.SSHPublicKeyFile = publicSSHKeyFile
		o.WaitTimeout = 0

		Expect(o.Validate()).NotTo(Succeed())
	})

	It("should require a public SSH key file", func() {
		streams, _, _, _ := util.NewTestIOStreams()
		o := NewOptions(streams)
		o.CIDRs = []string{"8.8.8.8/32"}

		Expect(o.Validate()).NotTo(Succeed())
	})

	It("should require a valid public SSH key file", func() {
		Expect(ioutil.WriteFile(publicSSHKeyFile, []byte("not a key"), 0644)).To(Succeed())

		streams, _, _, _ := util.NewTestIOStreams()
		o := NewOptions(streams)
		o.CIDRs = []string{"8.8.8.8/32"}
		o.SSHPublicKeyFile = publicSSHKeyFile

		Expect(o.Validate()).NotTo(Succeed())
	})

	It("should require at least one CIDR", func() {
		streams, _, _, _ := util.NewTestIOStreams()
		o := NewOptions(streams)
		o.SSHPublicKeyFile = publicSSHKeyFile

		Expect(o.Validate()).NotTo(Succeed())
	})

	It("should reject invalid CIDRs", func() {
		streams, _, _, _ := util.NewTestIOStreams()
		o := NewOptions(streams)
		o.CIDRs = []string{"8.8.8.8"}
		o.SSHPublicKeyFile = publicSSHKeyFile

		Expect(o.Validate()).NotTo(Succeed())
	})
})
