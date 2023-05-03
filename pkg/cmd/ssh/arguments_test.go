/*
SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package ssh_test

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardenctl-v2/pkg/cmd/ssh"
)

type testCase struct {
	bastionHost                string
	bastionPort                string
	sshPrivateKeyFile          ssh.PrivateKeyFile
	bastionUserKnownHostsFiles []string
	nodeHostname               string
	nodePrivateKeyFiles        []ssh.PrivateKeyFile
	expectedArgs               []string
}

func newTestCase() testCase {
	return testCase{
		bastionHost:         "bastion.example.com",
		bastionPort:         "22",
		sshPrivateKeyFile:   "path/to/private/key",
		nodeHostname:        "node.example.com",
		nodePrivateKeyFiles: []ssh.PrivateKeyFile{"path/to/node/private/key"},
	}
}

var _ = Describe("Arguments", func() {
	Describe("sshCommandArguments", func() {
		DescribeTable("should match the expected arguments as string",
			func(tc testCase) {
				args := ssh.SSHCommandArguments(tc.bastionHost, tc.bastionPort, tc.sshPrivateKeyFile, tc.bastionUserKnownHostsFiles, tc.nodeHostname, tc.nodePrivateKeyFiles)
				Expect(args.String()).To(Equal(strings.Join(tc.expectedArgs, " ")))
			},
			Entry("basic case", func() testCase {
				tc := newTestCase()
				tc.expectedArgs = []string{
					"-oStrictHostKeyChecking=no",
					"-oIdentitiesOnly=yes",
					"'-ipath/to/node/private/key'",
					`'-oProxyCommand=ssh -W%h:%p -oStrictHostKeyChecking=no -oIdentitiesOnly=yes '"'"'-ipath/to/private/key'"'"' '"'"'gardener@bastion.example.com'"'"' '"'"'-p22'"'"''`,
					"'gardener@node.example.com'",
				}
				return tc
			}()),
			Entry("no bastion port", func() testCase {
				tc := newTestCase()
				tc.bastionPort = ""
				tc.expectedArgs = []string{
					"-oStrictHostKeyChecking=no",
					"-oIdentitiesOnly=yes",
					"'-ipath/to/node/private/key'",
					`'-oProxyCommand=ssh -W%h:%p -oStrictHostKeyChecking=no -oIdentitiesOnly=yes '"'"'-ipath/to/private/key'"'"' '"'"'gardener@bastion.example.com'"'"''`,
					"'gardener@node.example.com'",
				}
				return tc
			}()),
			Entry("multiple known hosts files", func() testCase {
				tc := newTestCase()
				tc.bastionUserKnownHostsFiles = []string{"path/to/known_hosts1", "path/to/known_hosts2"}
				tc.expectedArgs = []string{
					"-oStrictHostKeyChecking=no",
					"-oIdentitiesOnly=yes",
					"'-ipath/to/node/private/key'",
					`'-oProxyCommand=ssh -W%h:%p -oStrictHostKeyChecking=no -oIdentitiesOnly=yes '"'"'-ipath/to/private/key'"'"' '"'"'-oUserKnownHostsFile='"'"'"'"'"'"'"'"'path/to/known_hosts1'"'"'"'"'"'"'"'"' '"'"'"'"'"'"'"'"'path/to/known_hosts2'"'"'"'"'"'"'"'"''"'"' '"'"'gardener@bastion.example.com'"'"' '"'"'-p22'"'"''`,
					"'gardener@node.example.com'",
				}
				return tc
			}()),
			Entry("no private key files", func() testCase {
				tc := newTestCase()
				tc.sshPrivateKeyFile = ""
				tc.nodePrivateKeyFiles = []ssh.PrivateKeyFile{}
				tc.expectedArgs = []string{
					"-oStrictHostKeyChecking=no",
					"-oIdentitiesOnly=yes",
					`'-oProxyCommand=ssh -W%h:%p -oStrictHostKeyChecking=no '"'"'gardener@bastion.example.com'"'"' '"'"'-p22'"'"''`,
					"'gardener@node.example.com'",
				}
				return tc
			}()),
		)
	})
})
