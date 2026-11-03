/*
SPDX-FileCopyrightText: Contributors to the Gardener project

SPDX-License-Identifier: Apache-2.0
*/

package ssh_test

import (
	"os/exec"
	"path/filepath"
	"runtime"
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
				tc.bastionUserKnownHostsFiles = []string{"/path/to/known_hosts1", "/path/to/known_hosts2"}
				tc.nodeUserKnownHostsFiles = []string{"/path/to/node_known_hosts"}
				tc.bastionStrictHostKeyChecking = "no"
				tc.nodeStrictHostKeyChecking = "yes"
				tc.expectedArgs = []string{
					"-oIdentitiesOnly=yes",
					"-oStrictHostKeyChecking=yes",
					"'-oUserKnownHostsFile=/path/to/node_known_hosts'",
					"'-ipath/to/node/private/key'",
					`'-oProxyCommand=ssh '"'"'-W[%h]:%p'"'"' -oStrictHostKeyChecking=no -oIdentitiesOnly=yes '"'"'-ipath/to/private/key'"'"' '"'"'-oUserKnownHostsFile=/path/to/known_hosts1 /path/to/known_hosts2'"'"' '"'"'gardener@bastion.example.com'"'"' '"'"'-p22'"'"''`,
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

	Describe("known hosts files resolved by OpenSSH", func() {
		var (
			sshPath                string
			bastionKnownHostsFiles []string
			nodeKnownHostsFiles    []string
		)

		BeforeEach(func() {
			if runtime.GOOS == "windows" {
				Skip("OpenSSH ProxyCommand is not run through a shell on windows")
			}

			var err error

			sshPath, err = exec.LookPath("ssh")
			if err != nil {
				Skip("ssh not found in PATH")
			}

			dir := filepath.Join(GinkgoT().TempDir(), "Known-Hosts_dir.1")
			bastionKnownHostsFiles = []string{filepath.Join(dir, "bastion_known_hosts1"), filepath.Join(dir, "bastion_known_hosts2")}
			nodeKnownHostsFiles = []string{filepath.Join(dir, "node_known_hosts")}

			for _, file := range append(bastionKnownHostsFiles, nodeKnownHostsFiles...) {
				Expect(ssh.NormalizeKnownHostsFile(file)).To(Equal(file))
			}
		})

		commandArguments := func(shell string) []string {
			shellEscapeFn, err := env.ShellEscapeFor(shell)
			Expect(err).NotTo(HaveOccurred())

			return ssh.SSHCommandArguments(
				"bastion.example.com",
				"22",
				"",
				bastionKnownHostsFiles,
				"ask",
				nodeKnownHostsFiles,
				"ask",
				"node.example.com",
				nil,
				"gardener",
				shellEscapeFn,
			).Values()
		}

		DescribeTable("should use the node known hosts files for the node connection",
			func(shell string) {
				cfg := resolveSSHConfig(exec.Command(sshPath, append([]string{"-F", "/dev/null", "-G"}, commandArguments(shell)...)...))

				Expect(cfg).To(HaveKeyWithValue("userknownhostsfile", strings.Join(nodeKnownHostsFiles, " ")))
			},
			Entry("bash", "bash"),
			Entry("zsh", "zsh"),
			Entry("fish", "fish"),
			Entry("powershell", "powershell"),
		)

		// The proxy command is escaped for the configured shell, but OpenSSH runs it with "$SHELL -c",
		// which may be a different shell.
		DescribeTable("should use the bastion known hosts files for the proxy command with any executing shell",
			func(shell string) {
				cfg := resolveSSHConfig(exec.Command(sshPath, append([]string{"-F", "/dev/null", "-G"}, commandArguments(shell)...)...))
				Expect(cfg).To(HaveKey("proxycommand"))

				proxyCommand := strings.NewReplacer("%h", "node.example.com", "%p", "22").Replace(cfg["proxycommand"])
				Expect(proxyCommand).To(HavePrefix("ssh "))
				proxyCommand = sshPath + " -F /dev/null -G " + strings.TrimPrefix(proxyCommand, "ssh ")

				for _, executor := range []string{"sh", "bash", "zsh", "fish", "pwsh"} {
					executorPath, err := exec.LookPath(executor)
					if err != nil {
						continue
					}

					By("running the proxy command with " + executor)

					cfg := resolveSSHConfig(exec.Command(executorPath, "-c", proxyCommand))

					Expect(cfg).To(HaveKeyWithValue("hostname", "bastion.example.com"))
					Expect(cfg).To(HaveKeyWithValue("userknownhostsfile", strings.Join(bastionKnownHostsFiles, " ")))
				}
			},
			Entry("bash", "bash"),
			Entry("zsh", "zsh"),
			Entry("fish", "fish"),
			Entry("powershell", "powershell"),
		)
	})
})

// resolveSSHConfig runs an "ssh -G" command, which prints the resolved client configuration without connecting.
func resolveSSHConfig(cmd *exec.Cmd) map[string]string {
	GinkgoHelper()

	out, err := cmd.Output()
	Expect(err).NotTo(HaveOccurred(), "command %v failed: %s", cmd.Args, out)

	cfg := map[string]string{}

	for _, line := range strings.Split(string(out), "\n") {
		key, value, found := strings.Cut(line, " ")
		if _, exists := cfg[key]; found && !exists {
			cfg[key] = value
		}
	}

	return cfg
}
