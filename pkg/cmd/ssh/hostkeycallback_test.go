/*
SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package ssh

import (
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/crypto/ed25519"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/gardener/gardenctl-v2/internal/util"
)

var _ = Describe("realHostKeyCallbackFactory", func() {
	var (
		factory      *realHostKeyCallbackFactory
		ioStreams    util.IOStreams
		inBuffer     *util.SafeBytesBuffer
		outBuffer    *util.SafeBytesBuffer
		errOutBuffer *util.SafeBytesBuffer
		publicKey    ssh.PublicKey
		homeDir      string
	)

	BeforeEach(func() {
		ioStreams, inBuffer, outBuffer, errOutBuffer = util.NewTestIOStreams()

		var err error
		homeDir, err = os.MkdirTemp("", "testuserdir")
		Expect(err).NotTo(HaveOccurred())

		sshDir := filepath.Join(homeDir, ".ssh")
		err = os.Mkdir(sshDir, 0o700)
		Expect(err).NotTo(HaveOccurred())

		factory = &realHostKeyCallbackFactory{
			isTerminalFunc: func(in io.Reader) bool {
				return true
			},
			getUserHomeDirFunc: func() (string, error) {
				return homeDir, nil
			},
		}

		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		Expect(err).NotTo(HaveOccurred())

		publicKey, err = ssh.NewPublicKey(&privateKey.PublicKey)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		err := os.RemoveAll(homeDir)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("New", func() {
		Context("when strictHostKeyChecking is StrictHostKeyCheckingNo", func() {
			It("should return a callback that ignores host key verification", func() {
				callback, err := factory.New(StrictHostKeyCheckingNo, nil, ioStreams)
				Expect(err).NotTo(HaveOccurred())
				Expect(callback).NotTo(BeNil())

				// Test that the callback returns nil, ignoring host keys
				err = callback("hostname:22", &net.TCPAddr{}, nil)
				Expect(err).NotTo(HaveOccurred())

				// Ensure no messages are written to stderr or stdout
				Expect(errOutBuffer.String()).To(BeEmpty())
				Expect(outBuffer.String()).To(BeEmpty())
			})
		})

		Context("when strictHostKeyChecking is StrictHostKeyCheckingYes", func() {
			var (
				knownHostsFiles []string
				tempFile        *os.File
				err             error
			)

			BeforeEach(func() {
				tempFile, err = os.CreateTemp("", "known_hosts")
				Expect(err).NotTo(HaveOccurred())

				knownHostsFiles = []string{tempFile.Name()}
			})

			AfterEach(func() {
				os.Remove(tempFile.Name())
			})

			It("should return a non-nil HostKeyCallback and no error", func() {
				callback, err := factory.New(StrictHostKeyCheckingYes, knownHostsFiles, ioStreams)
				Expect(err).NotTo(HaveOccurred())
				Expect(callback).NotTo(BeNil())

				// Since we haven't added any hosts, the callback should fail when called
				err = callback("hostname:22", &net.TCPAddr{}, publicKey)
				Expect(err).To(HaveOccurred())
				var keyErr *knownhosts.KeyError
				ok := errors.As(err, &keyErr)
				Expect(ok).To(BeTrue(), "error should be of type knownhosts.KeyError")
				Expect(len(keyErr.Want)).To(Equal(0), "keyErr.Want should be empty")

				// Ensure no messages are written to stderr or stdout
				Expect(errOutBuffer.String()).To(BeEmpty())
				Expect(outBuffer.String()).To(BeEmpty())
			})

			It("should successfully verify a known and matching host key", func() {
				// Add the known host key to the known_hosts file
				knownHostsLine := knownhosts.Line([]string{"hostname"}, publicKey)
				_, err = tempFile.WriteString(knownHostsLine + "\n")
				Expect(err).NotTo(HaveOccurred())
				tempFile.Close()

				// Create the HostKeyCallback
				callback, err := factory.New(StrictHostKeyCheckingYes, knownHostsFiles, ioStreams)
				Expect(err).NotTo(HaveOccurred())
				Expect(callback).NotTo(BeNil())

				// Call the callback with matching host key
				err = callback("hostname:22", &net.TCPAddr{}, publicKey)
				Expect(err).NotTo(HaveOccurred())

				// Ensure no messages are written to stderr or stdout
				Expect(errOutBuffer.String()).To(BeEmpty())
				Expect(outBuffer.String()).To(BeEmpty())
			})
		})

		Context("when strictHostKeyChecking is StrictHostKeyCheckingAsk", func() {
			var (
				knownHostsFiles []string
				tempFile        *os.File
				err             error
			)

			BeforeEach(func() {
				tempFile, err = os.CreateTemp("", "known_hosts")
				Expect(err).NotTo(HaveOccurred())

				knownHostsFiles = []string{tempFile.Name()}
			})

			AfterEach(func() {
				os.Remove(tempFile.Name())
			})

			It("should return a non-nil HostKeyCallback and prompt user when unknown host key is encountered", func() {
				// Prepare fake user input (e.g., "yes")
				_, err = inBuffer.Write([]byte("yes\n"))
				Expect(err).NotTo(HaveOccurred())

				callback, err := factory.New(StrictHostKeyCheckingAsk, knownHostsFiles, ioStreams)
				Expect(err).NotTo(HaveOccurred())
				Expect(callback).NotTo(BeNil())

				// Call the callback
				err = callback("hostname:22", &net.TCPAddr{}, publicKey)
				Expect(err).NotTo(HaveOccurred())

				// Check that the prompt was written to stderr
				Expect(errOutBuffer.String()).To(ContainSubstring("The authenticity of host 'hostname"))
				Expect(errOutBuffer.String()).To(ContainSubstring("Are you sure you want to continue connecting"))

				// Ensure nothing is written to stdout
				Expect(outBuffer.String()).To(BeEmpty())
			})

			It("should return an error when no terminal is available to prompt the user", func() {
				// Override isTerminalFunc to return false
				factory.isTerminalFunc = func(in io.Reader) bool {
					return false
				}

				callback, err := factory.New(StrictHostKeyCheckingAsk, knownHostsFiles, ioStreams)
				Expect(err).NotTo(HaveOccurred())
				Expect(callback).NotTo(BeNil())

				// Call the callback, which should attempt to prompt the user
				err = callback("hostname:22", &net.TCPAddr{}, publicKey)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("no terminal available to prompt user"))

				// Ensure appropriate error message is not written to stderr
				Expect(errOutBuffer.String()).To(BeEmpty())
				Expect(outBuffer.String()).To(BeEmpty())
			})
		})

		Context("when strictHostKeyChecking is StrictHostKeyCheckingAcceptNew", func() {
			var (
				knownHostsFiles []string
				tempFile        *os.File
				err             error
			)

			BeforeEach(func() {
				tempFile, err = os.CreateTemp("", "known_hosts")
				Expect(err).NotTo(HaveOccurred())

				knownHostsFiles = []string{tempFile.Name()}
			})

			AfterEach(func() {
				os.Remove(tempFile.Name())
			})

			It("should automatically accept new host keys", func() {
				callback, err := factory.New(StrictHostKeyCheckingAcceptNew, knownHostsFiles, ioStreams)
				Expect(err).NotTo(HaveOccurred())
				Expect(callback).NotTo(BeNil())

				// Call the callback
				err = callback("hostname:22", &net.TCPAddr{}, publicKey)
				Expect(err).NotTo(HaveOccurred())

				// Ensure nothing is written to stderr or stdout
				Expect(errOutBuffer.String()).To(BeEmpty())
				Expect(outBuffer.String()).To(BeEmpty())

				// Verify that the host key was added to the known_hosts file
				content, err := os.ReadFile(tempFile.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(ContainSubstring("hostname"))
			})
		})

		Context("when strictHostKeyChecking is invalid", func() {
			It("should return an error", func() {
				callback, err := factory.New("invalid", nil, ioStreams)
				Expect(err).To(HaveOccurred())
				Expect(callback).To(BeNil())
				Expect(err.Error()).To(ContainSubstring("invalid strict host key checking option"))

				// Ensure nothing is written to stderr or stdout
				Expect(errOutBuffer.String()).To(BeEmpty())
				Expect(outBuffer.String()).To(BeEmpty())
			})
		})

		Context("when getKnownHostsFiles returns an error", func() {
			BeforeEach(func() {
				// Override getUserHomeDirFunc to simulate error
				factory.getUserHomeDirFunc = func() (string, error) {
					return "", errors.New("mocked error")
				}
			})

			It("should return an error for StrictHostKeyCheckingYes", func() {
				callback, err := factory.New(StrictHostKeyCheckingYes, nil, ioStreams)
				Expect(err).To(HaveOccurred())
				Expect(callback).To(BeNil())
				Expect(err.Error()).To(ContainSubstring("failed to determine known hosts files"))

				// Ensure nothing is written to stderr or stdout
				Expect(errOutBuffer.String()).To(BeEmpty())
				Expect(outBuffer.String()).To(BeEmpty())
			})

			It("should return an error for StrictHostKeyCheckingAsk", func() {
				callback, err := factory.New(StrictHostKeyCheckingAsk, nil, ioStreams)
				Expect(err).To(HaveOccurred())
				Expect(callback).To(BeNil())
				Expect(err.Error()).To(ContainSubstring("failed to determine known hosts files"))

				// Ensure nothing is written to stderr or stdout
				Expect(errOutBuffer.String()).To(BeEmpty())
				Expect(outBuffer.String()).To(BeEmpty())
			})

			It("should return an error for StrictHostKeyCheckingAcceptNew", func() {
				callback, err := factory.New(StrictHostKeyCheckingAcceptNew, nil, ioStreams)
				Expect(err).To(HaveOccurred())
				Expect(callback).To(BeNil())
				Expect(err.Error()).To(ContainSubstring("failed to determine known hosts files"))

				// Ensure nothing is written to stderr or stdout
				Expect(errOutBuffer.String()).To(BeEmpty())
				Expect(outBuffer.String()).To(BeEmpty())
			})
		})
	})

	Describe("Host key mismatch scenario", func() {
		It("should print the host key mismatch warning", func() {
			// Create a known hosts file with an entry for a host with one key
			tempFile, err := os.CreateTemp("", "known_hosts")
			Expect(err).NotTo(HaveOccurred())
			defer os.Remove(tempFile.Name())

			knownHostsFiles := []string{tempFile.Name()}

			// Generate a host key and add it to the known hosts
			privateKey1, err := rsa.GenerateKey(rand.Reader, 2048)
			Expect(err).NotTo(HaveOccurred())
			publicKey1, err := ssh.NewPublicKey(&privateKey1.PublicKey)
			Expect(err).NotTo(HaveOccurred())

			hostnames := []string{"hostname"}
			knownHostsLine := knownhosts.Line(hostnames, publicKey1)
			_, err = tempFile.WriteString(knownHostsLine + "\n")
			Expect(err).NotTo(HaveOccurred())
			tempFile.Close()

			// Generate a different host key to simulate the mismatch
			privateKey2, err := rsa.GenerateKey(rand.Reader, 2048)
			Expect(err).NotTo(HaveOccurred())
			publicKey2, err := ssh.NewPublicKey(&privateKey2.PublicKey)
			Expect(err).NotTo(HaveOccurred())

			ioStreams, _, _, errOutBuffer := util.NewTestIOStreams()

			callback, err := factory.New(StrictHostKeyCheckingAsk, knownHostsFiles, ioStreams)
			Expect(err).NotTo(HaveOccurred())
			Expect(callback).NotTo(BeNil())

			// Call the callback with the mismatched public key
			err = callback("hostname:22", &net.TCPAddr{}, publicKey2)
			Expect(err).To(HaveOccurred())
			var keyErr *knownhosts.KeyError
			ok := errors.As(err, &keyErr)
			Expect(ok).To(BeTrue(), "error should be of type knownhosts.KeyError")
			// keyErr.Want should not be empty
			Expect(len(keyErr.Want)).To(BeNumerically(">", 0))

			// Now check that the warning was printed to errOut
			errOutString := errOutBuffer.String()
			Expect(errOutString).To(ContainSubstring("WARNING: REMOTE HOST IDENTIFICATION HAS CHANGED"))
			Expect(errOutString).To(ContainSubstring(fmt.Sprintf("Add correct host key in %s to get rid of this message.", tempFile.Name())))
			Expect(errOutString).To(ContainSubstring(fmt.Sprintf("Offending ssh-rsa key in %s:1", tempFile.Name())))
			Expect(errOutString).To(ContainSubstring("Host key verification failed."))
		})

		It("should handle host key mismatch with different key types", func() {
			// Create a known hosts file with an entry for a host with an RSA key
			tempFile, err := os.CreateTemp("", "known_hosts")
			Expect(err).NotTo(HaveOccurred())
			defer os.Remove(tempFile.Name())

			knownHostsFiles := []string{tempFile.Name()}

			// Generate an RSA key and add it to the known hosts
			privateKeyRSA, err := rsa.GenerateKey(rand.Reader, 2048)
			Expect(err).NotTo(HaveOccurred())
			publicKeyRSA, err := ssh.NewPublicKey(&privateKeyRSA.PublicKey)
			Expect(err).NotTo(HaveOccurred())

			hostnames := []string{"hostname"}
			knownHostsLine := knownhosts.Line(hostnames, publicKeyRSA)
			_, err = tempFile.WriteString(knownHostsLine + "\n")
			Expect(err).NotTo(HaveOccurred())
			tempFile.Close()

			// Generate an ED25519 key to simulate the mismatch
			_, privateKeyED25519, err := ed25519.GenerateKey(rand.Reader)
			Expect(err).NotTo(HaveOccurred())
			publicKeyED25519, err := ssh.NewPublicKey(privateKeyED25519.Public())
			Expect(err).NotTo(HaveOccurred())

			ioStreams, _, _, errOutBuffer := util.NewTestIOStreams()

			callback, err := factory.New(StrictHostKeyCheckingAsk, knownHostsFiles, ioStreams)
			Expect(err).NotTo(HaveOccurred())
			Expect(callback).NotTo(BeNil())

			// Call the callback with the ED25519 key
			err = callback("hostname:22", &net.TCPAddr{}, publicKeyED25519)
			Expect(err).To(HaveOccurred())
			var keyErr *knownhosts.KeyError
			ok := errors.As(err, &keyErr)
			Expect(ok).To(BeTrue(), "error should be of type knownhosts.KeyError")
			// keyErr.Want should not be empty
			Expect(len(keyErr.Want)).To(BeNumerically(">", 0))

			// Now check that the warning was printed to errOut
			errOutString := errOutBuffer.String()
			Expect(errOutString).To(ContainSubstring("WARNING: REMOTE HOST IDENTIFICATION HAS CHANGED"))
			Expect(errOutString).To(ContainSubstring("Remote host hostname:22 using a different, unknown key type"))
			Expect(errOutString).To(ContainSubstring("Known keys for hostname:22:"))
			Expect(errOutString).To(ContainSubstring(fmt.Sprintf("- ssh-rsa key in %s:1\n", tempFile.Name())))
			Expect(errOutString).To(ContainSubstring("Host key verification failed."))
		})
	})

	Describe("Known host key scenario", func() {
		It("should not prompt when host key is known and matches", func() {
			// Create a known hosts file with an entry for a host with a known key
			tempFile, err := os.CreateTemp("", "known_hosts")
			Expect(err).NotTo(HaveOccurred())
			defer os.Remove(tempFile.Name())

			knownHostsFiles := []string{tempFile.Name()}

			// Generate a host key and add it to the known hosts
			privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
			Expect(err).NotTo(HaveOccurred())
			publicKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
			Expect(err).NotTo(HaveOccurred())

			hostnames := []string{"hostname"}
			knownHostsLine := knownhosts.Line(hostnames, publicKey)
			_, err = tempFile.WriteString(knownHostsLine + "\n")
			Expect(err).NotTo(HaveOccurred())
			tempFile.Close()

			// Prepare ioStreams to capture outputs
			ioStreams, _, outBuffer, errOutBuffer := util.NewTestIOStreams()

			// Create the callback
			callback, err := factory.New(StrictHostKeyCheckingAsk, knownHostsFiles, ioStreams)
			Expect(err).NotTo(HaveOccurred())
			Expect(callback).NotTo(BeNil())

			// Call the callback with the matching public key
			err = callback("hostname:22", &net.TCPAddr{}, publicKey)
			Expect(err).NotTo(HaveOccurred())

			// Ensure no messages are written to stderr or stdout
			Expect(errOutBuffer.String()).To(BeEmpty())
			Expect(outBuffer.String()).To(BeEmpty())
		})
	})

	Describe("promptUserToTrustHostKey", func() {
		Context("when user enters the fingerprint", func() {
			It("should accept the connection", func() {
				// Prepare fake user input (fingerprint)
				fingerprint := ssh.FingerprintSHA256(publicKey)
				_, err := inBuffer.Write([]byte(fingerprint + "\n"))
				Expect(err).NotTo(HaveOccurred())

				// Create the callback
				callback, err := factory.New(StrictHostKeyCheckingAsk, nil, ioStreams)
				Expect(err).NotTo(HaveOccurred())
				Expect(callback).NotTo(BeNil())

				// Call the callback, which should accept based on fingerprint
				err = callback("hostname:22", &net.TCPAddr{}, publicKey)
				Expect(err).NotTo(HaveOccurred())

				// Check that the prompt was written to stderr
				Expect(errOutBuffer.String()).To(ContainSubstring("The authenticity of host 'hostname"))
				Expect(errOutBuffer.String()).To(ContainSubstring("Are you sure you want to continue connecting"))

				// Ensure nothing is written to stdout
				Expect(outBuffer.String()).To(BeEmpty())
			})
		})

		Context("when user responds with 'no'", func() {
			It("should reject the connection and return an error", func() {
				// Prepare fake user input ("no")
				_, err := inBuffer.Write([]byte("no\n"))
				Expect(err).NotTo(HaveOccurred())

				// Create the callback
				callback, err := factory.New(StrictHostKeyCheckingAsk, nil, ioStreams)
				Expect(err).NotTo(HaveOccurred())
				Expect(callback).NotTo(BeNil())

				// Call the callback, which should reject the connection
				err = callback("hostname:22", &net.TCPAddr{}, publicKey)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("host key verification failed for host hostname"))

				// Check that the prompt was written to stderr
				Expect(errOutBuffer.String()).To(ContainSubstring("The authenticity of host 'hostname"))
				Expect(errOutBuffer.String()).To(ContainSubstring("Are you sure you want to continue connecting"))

				// Ensure nothing is written to stdout
				Expect(outBuffer.String()).To(BeEmpty())
			})
		})

		Context("when user enters invalid input followed by 'yes'", func() {
			It("should handle invalid input and accept the connection after valid input", func() {
				// Prepare fake user input (invalid input followed by 'yes')
				_, err := inBuffer.Write([]byte("maybe\nyes\n"))
				Expect(err).NotTo(HaveOccurred())

				// Create the callback
				callback, err := factory.New(StrictHostKeyCheckingAsk, nil, ioStreams)
				Expect(err).NotTo(HaveOccurred())
				Expect(callback).NotTo(BeNil())

				// Call the callback, which should handle invalid input and then accept
				err = callback("hostname:22", &net.TCPAddr{}, publicKey)
				Expect(err).NotTo(HaveOccurred())

				// Check that the prompt was written to stderr
				Expect(errOutBuffer.String()).To(ContainSubstring("Please type 'yes', 'no' or the fingerprint"))

				// Ensure nothing is written to stdout
				Expect(outBuffer.String()).To(BeEmpty())
			})
		})

		Context("when user enters invalid input and does not provide valid confirmation", func() {
			It("should keep prompting until valid input is received or fail", func() {
				// Prepare fake user input (invalid inputs without 'yes', 'no' or fingerprint)
				_, err := inBuffer.Write([]byte("yup\nunknown\ny\n"))
				Expect(err).NotTo(HaveOccurred())

				// Create the callback
				callback, err := factory.New(StrictHostKeyCheckingAsk, nil, ioStreams)
				Expect(err).NotTo(HaveOccurred())
				Expect(callback).NotTo(BeNil())

				// Call the callback, which should eventually fail after invalid inputs
				err = callback("hostname:22", &net.TCPAddr{}, publicKey)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to read user input: EOF"))

				// Check that the prompt was written to stderr multiple times
				errOutString := errOutBuffer.String()
				promptCount := strings.Count(errOutString, "Please type 'yes', 'no' or the fingerprint")
				Expect(promptCount).To(Equal(3), "Expected the prompt to be shown three times")

				// Ensure nothing is written to stdout
				Expect(outBuffer.String()).To(BeEmpty())
			})
		})
	})

	Describe("defaultGetKnownHostsFiles", func() {
		Context("when user has a home directory", func() {
			BeforeEach(func() {
				// Override getUserHomeDirFunc to return a specific home directory
				factory.getUserHomeDirFunc = func() (string, error) {
					return "/home/testuser", nil
				}
			})

			It("should return the default known hosts file", func() {
				knownHostsFiles, err := factory.getKnownHostsFiles(nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(knownHostsFiles).To(Equal([]string{"/home/testuser/.ssh/known_hosts"}))
			})
		})

		Context("when user does not have a home directory", func() {
			BeforeEach(func() {
				// Override getUserHomeDirFunc to return empty string
				factory.getUserHomeDirFunc = func() (string, error) {
					return "", nil
				}
			})

			It("should return an error", func() {
				knownHostsFiles, err := factory.getKnownHostsFiles(nil)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("user does not have a home directory"))
				Expect(knownHostsFiles).To(BeNil())
			})
		})

		Context("when getting current user fails", func() {
			BeforeEach(func() {
				// Override getUserHomeDirFunc to simulate an error
				factory.getUserHomeDirFunc = func() (string, error) {
					return "", errors.New("failed to get current user")
				}
			})

			It("should return an error", func() {
				knownHostsFiles, err := factory.getKnownHostsFiles(nil)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to get current user"))
				Expect(knownHostsFiles).To(BeNil())
			})
		})
	})
})
