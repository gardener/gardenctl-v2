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
	"github.com/gardener/gardenctl-v2/pkg/env"
)

type testCase struct {
	bastionHost                  string
	bastionPort                  string
	sshPrivateKeyFile            ssh.PrivateKeyFile
	bastionUserKnownHostsFiles   []string
	bastionStrictHostKeyChecking ssh.StrictHostKeyChecking
	nodeUserKnownHostsFiles      []string
	nodeStrictHostKeyChecking    ssh.StrictHostKeyChecking
	nodeHostname                 string
	nodePrivateKeyFiles          []ssh.PrivateKeyFile
	expectedArgs                 []string
	user                         string
	shell                        string
}

func newTestCase() testCase {
	return testCase{
		bastionHost:                  "bastion.example.com",
		bastionPort:                  "22",
		sshPrivateKeyFile:            "path/to/private/key",
		bastionUserKnownHostsFiles:   []string{},
		bastionStrictHostKeyChecking: "ask",
		nodeUserKnownHostsFiles:      []string{},
		nodeStrictHostKeyChecking:    "ask",
		nodeHostname:                 "node.example.com",
		nodePrivateKeyFiles:          []ssh.PrivateKeyFile{"path/to/node/private/key"},
		user:                         "gardener",
		shell:                        "bash",
	}
}

var _ = Describe("Arguments", func() {
	Describe("sshCommandArguments", func() {
		DescribeTable("should match the expected arguments as string",
			func(tc testCase) {
				shellEscapeFn, err := env.ShellEscapeFor(tc.shell)
				Expect(err).NotTo(HaveOccurred())

				args := ssh.SSHCommandArguments(
					tc.bastionHost,
					tc.bastionPort,
					tc.sshPrivateKeyFile,
					tc.bastionUserKnownHostsFiles,
					tc.bastionStrictHostKeyChecking,
					tc.nodeUserKnownHostsFiles,
					tc.nodeStrictHostKeyChecking,
					tc.nodeHostname,
					tc.nodePrivateKeyFiles,
					tc.user,
					shellEscapeFn,
				)
				res := args.String()
				exp := strings.Join(tc.expectedArgs, " ")
				Expect(res).To(Equal(exp))
			},
			Entry("basic case", func() testCase {
				tc := newTestCase()
				tc.expectedArgs = []string{
					"-oIdentitiesOnly=yes",
					"-oStrictHostKeyChecking=ask",
					"'-ipath/to/node/private/key'",
					`'-oProxyCommand=ssh '"'"'-W[%h]:%p'"'"' -oStrictHostKeyChecking=ask -oIdentitiesOnly=yes '"'"'-ipath/to/private/key'"'"' '"'"'gardener@bastion.example.com'"'"' '"'"'-p22'"'"''`,
					"'gardener@node.example.com'",
				}
				return tc
			}()),
			Entry("basic case with other ssh username", func() testCase {
				tc := newTestCase()
				tc.user = "aaa"
				tc.expectedArgs = []string{
					"-oIdentitiesOnly=yes",
					"-oStrictHostKeyChecking=ask",
					"'-ipath/to/node/private/key'",
					`'-oProxyCommand=ssh '"'"'-W[%h]:%p'"'"' -oStrictHostKeyChecking=ask -oIdentitiesOnly=yes '"'"'-ipath/to/private/key'"'"' '"'"'gardener@bastion.example.com'"'"' '"'"'-p22'"'"''`,
					"'aaa@node.example.com'",
				}
				return tc
			}()),
			Entry("no bastion port", func() testCase {
				tc := newTestCase()
				tc.bastionPort = ""
				tc.expectedArgs = []string{
					"-oIdentitiesOnly=yes",
					"-oStrictHostKeyChecking=ask",
					"'-ipath/to/node/private/key'",
					`'-oProxyCommand=ssh '"'"'-W[%h]:%p'"'"' -oStrictHostKeyChecking=ask -oIdentitiesOnly=yes '"'"'-ipath/to/private/key'"'"' '"'"'gardener@bastion.example.com'"'"''`,
					"'gardener@node.example.com'",
				}
				return tc
			}()),
			Entry("multiple known hosts files", func() testCase {
				tc := newTestCase()
				tc.bastionUserKnownHostsFiles = []string{"path/to/known_hosts1", "path/to/known_hosts2"}
				tc.nodeUserKnownHostsFiles = []string{"path/to/node_known_hosts"}
				tc.bastionStrictHostKeyChecking = "no"
				tc.nodeStrictHostKeyChecking = "yes"
				tc.expectedArgs = []string{
					"-oIdentitiesOnly=yes",
					"-oStrictHostKeyChecking=yes",
					`'-oUserKnownHostsFile='"'"'path/to/node_known_hosts'"'"''`,
					"'-ipath/to/node/private/key'",
					`'-oProxyCommand=ssh '"'"'-W[%h]:%p'"'"' -oStrictHostKeyChecking=no -oIdentitiesOnly=yes '"'"'-ipath/to/private/key'"'"' '"'"'-oUserKnownHostsFile='"'"'"'"'"'"'"'"'path/to/known_hosts1'"'"'"'"'"'"'"'"' '"'"'"'"'"'"'"'"'path/to/known_hosts2'"'"'"'"'"'"'"'"''"'"' '"'"'gardener@bastion.example.com'"'"' '"'"'-p22'"'"''`,
					"'gardener@node.example.com'",
				}
				return tc
			}()),
			Entry("no private key files", func() testCase {
				tc := newTestCase()
				tc.sshPrivateKeyFile = ""
				tc.nodePrivateKeyFiles = []ssh.PrivateKeyFile{}
				tc.expectedArgs = []string{
					"-oIdentitiesOnly=yes",
					"-oStrictHostKeyChecking=ask",
					`'-oProxyCommand=ssh '"'"'-W[%h]:%p'"'"' -oStrictHostKeyChecking=ask '"'"'gardener@bastion.example.com'"'"' '"'"'-p22'"'"''`,
					"'gardener@node.example.com'",
				}
				return tc
			}()),
		)
	})
})
