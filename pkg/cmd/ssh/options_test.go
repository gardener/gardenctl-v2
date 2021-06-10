/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package ssh_test

import (
	"io"
	"os"

	. "github.com/gardener/gardenctl-v2/pkg/cmd/ssh"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

var _ = Describe("Options", func() {
	It("should validate", func() {
		tmpFile, err := os.CreateTemp("", "")
		Expect(err).NotTo(HaveOccurred())
		defer tmpFile.Close()

		// write dummy SSH public key
		_, err = io.WriteString(tmpFile, "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDouNkxsNuApuKVIfgL6Yz3Ep+DqX84Yde9DArwLBSWgLnl/pH9AbbcDcAmdB2CPVXAATo4qxK7xprvyyZp52SQRCcAZpAy4D6gAWwAG3OfzrRbxRiB5pQDaaWATSzNbLtoy0ecVwFeTJe2w71q+wxbI7tfxbvo9XbXIN4I0cQy2KLICzkYkQmygGnHztv1Mvi338+sgcG7Gwq2tdSyggDaAggwDIuT39S4/L7QpR27tWH79J4Ls8tTHud2eRbkOcF98vXlQAIzb6w8iHBXylOjMM/oODwoA7V4mtRL9o13AoocvZSsD1UvfOjGxDHuLrCfFXN+/rEw0hEiYo0cnj7F")
		Expect(err).NotTo(HaveOccurred())

		streams, _, _, _ := genericclioptions.NewTestIOStreams()
		o := NewOptions(streams)
		o.CIDRs = []string{"8.8.8.8/32"}
		o.SSHPublicKeyFile = tmpFile.Name()

		Expect(o.Validate()).To(Succeed())
	})
})
