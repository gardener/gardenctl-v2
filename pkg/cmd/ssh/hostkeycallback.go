/*
SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package ssh

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"golang.org/x/term"

	"github.com/gardener/gardenctl-v2/internal/util"
)

// HostKeyCallbackFactory interface allows creation of new HostKeyCallback instances.
type HostKeyCallbackFactory interface {
	New(strictHostKeyChecking StrictHostKeyChecking, knownHostsFiles []string, ioStreams util.IOStreams) (ssh.HostKeyCallback, error)
}

// realHostKeyCallbackFactory is the concrete implementation of HostKeyCallbackFactory.
type realHostKeyCallbackFactory struct {
	isTerminalFunc     func(io.Reader) bool
	getUserHomeDirFunc func() (string, error)
}

// NewRealHostKeyCallbackFactory creates a new instance of realHostKeyCallbackFactory with default functions.
func NewRealHostKeyCallbackFactory() HostKeyCallbackFactory {
	return &realHostKeyCallbackFactory{
		isTerminalFunc:     defaultIsTerminalFunc,
		getUserHomeDirFunc: defaultGetUserHomeDirFunc,
	}
}

// New creates a new HostKeyCallback based on the strictHostKeyChecking option.
func (f *realHostKeyCallbackFactory) New(strictHostKeyChecking StrictHostKeyChecking, knownHostsFiles []string, ioStreams util.IOStreams) (ssh.HostKeyCallback, error) {
	switch strictHostKeyChecking {
	case StrictHostKeyCheckingNo, StrictHostKeyCheckingOff:
		// #nosec G106 -- InsecureIgnoreHostKey is used based on user configuration via flags to disable strict host key checking.
		return ssh.InsecureIgnoreHostKey(), nil
	case StrictHostKeyCheckingYes:
		knownHostsFiles, err := f.getKnownHostsFiles(knownHostsFiles)
		if err != nil {
			return nil, fmt.Errorf("failed to determine known hosts files: %w", err)
		}

		hostKeyCallback, err := knownhosts.New(knownHostsFiles...)
		if err != nil {
			return nil, fmt.Errorf("could not create hostkey callback: %w", err)
		}

		return hostKeyCallback, nil
	case StrictHostKeyCheckingAsk:
		knownHostsFiles, err := f.getKnownHostsFiles(knownHostsFiles)
		if err != nil {
			return nil, fmt.Errorf("failed to determine known hosts files: %w", err)
		}

		return f.newInteractiveHostKeyVerifier(knownHostsFiles, ioStreams, false), nil
	case StrictHostKeyCheckingAcceptNew:
		knownHostsFiles, err := f.getKnownHostsFiles(knownHostsFiles)
		if err != nil {
			return nil, fmt.Errorf("failed to determine known hosts files: %w", err)
		}

		return f.newInteractiveHostKeyVerifier(knownHostsFiles, ioStreams, true), nil
	default:
		return nil, fmt.Errorf("invalid strict host key checking option: %s", strictHostKeyChecking)
	}
}

// getKnownHostsFiles determines the known hosts files to use.
func (f *realHostKeyCallbackFactory) getKnownHostsFiles(customFiles []string) ([]string, error) {
	if len(customFiles) > 0 {
		return customFiles, nil
	}

	homeDir, err := f.getUserHomeDirFunc()
	if err != nil {
		return nil, err
	}

	if homeDir == "" {
		return nil, errors.New("user does not have a home directory")
	}

	defaultKnownHostsFile := filepath.Join(homeDir, ".ssh", "known_hosts")

	return []string{defaultKnownHostsFile}, nil
}

// defaultGetUserHomeDirFunc retrieves the current user's home directory.
func defaultGetUserHomeDirFunc() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("failed to get current user: %w", err)
	}

	return usr.HomeDir, nil
}

// defaultIsTerminalFunc checks if the io.Reader is connected to a terminal.
func defaultIsTerminalFunc(in io.Reader) bool {
	file, ok := in.(*os.File)
	if !ok {
		return false
	}

	return term.IsTerminal(int(file.Fd()))
}

// newInteractiveHostKeyVerifier returns a HostKeyCallback that handles interactive verification.
func (f *realHostKeyCallbackFactory) newInteractiveHostKeyVerifier(knownHostsFiles []string, ioStreams util.IOStreams, autoAccept bool) ssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, remoteKey ssh.PublicKey) error {
		hostKeyCallback, err := newKnownHostsVerifier(knownHostsFiles)
		if err != nil {
			return err
		}

		err = hostKeyCallback(hostname, remote, remoteKey)
		if err == nil {
			// Host key is known and matches
			return nil
		}

		var keyErr *knownhosts.KeyError
		if errors.As(err, &keyErr) {
			if len(keyErr.Want) != 0 {
				printHostKeyMismatchWarning(hostname, remoteKey, keyErr, ioStreams)
				return keyErr
			}

			var proceed bool
			if autoAccept {
				proceed = true
			} else {
				proceed, err = f.promptUserToTrustHostKey(hostname, remote, remoteKey, ioStreams)
				if err != nil {
					return err
				}
			}

			if !proceed {
				return fmt.Errorf("host key verification failed for host %s", hostname)
			}

			if err := addHostKeyToKnownHosts(knownHostsFiles, hostname, remoteKey); err != nil {
				return err
			}

			// Recreate the hostKeyCallback with the updated known hosts file
			hostKeyCallback, err = newKnownHostsVerifier(knownHostsFiles)
			if err != nil {
				return err
			}
			// Re-check the host key
			return hostKeyCallback(hostname, remote, remoteKey)
		}

		// Some other error occurred
		return err
	}
}

// printHostKeyMismatchWarning prints warnings related to host key mismatches.
func printHostKeyMismatchWarning(hostname string, remoteKey ssh.PublicKey, keyErr *knownhosts.KeyError, ioStreams util.IOStreams) {
	fmt.Fprintf(ioStreams.ErrOut, "@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@\n")
	fmt.Fprintf(ioStreams.ErrOut, "@    WARNING: REMOTE HOST IDENTIFICATION HAS CHANGED!     @\n")
	fmt.Fprintf(ioStreams.ErrOut, "@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@\n")
	fmt.Fprintf(ioStreams.ErrOut, "IT IS POSSIBLE THAT SOMEONE IS DOING SOMETHING NASTY!\n")
	fmt.Fprintf(ioStreams.ErrOut, "Someone could be eavesdropping on you right now (man-in-the-middle attack)!\n")
	fmt.Fprintf(ioStreams.ErrOut, "It is also possible that a host key has just been changed.\n")
	fmt.Fprintf(ioStreams.ErrOut, "The fingerprint for the %s key sent by the remote host is %s.\n", remoteKey.Type(), ssh.FingerprintSHA256(remoteKey))

	offendingKeyFound := false

	for _, wantKey := range keyErr.Want {
		if wantKey.Key.Type() == remoteKey.Type() {
			offendingKeyFound = true

			fmt.Fprintf(ioStreams.ErrOut, "Add correct host key in %s to get rid of this message.\n", wantKey.Filename)
			fmt.Fprintf(ioStreams.ErrOut, "Offending %s key in %s:%d\n", wantKey.Key.Type(), wantKey.Filename, wantKey.Line)
		}
	}

	if !offendingKeyFound {
		fmt.Fprintf(ioStreams.ErrOut, "Remote host %s using a different, unknown key type %s.\n", hostname, remoteKey.Type())
		fmt.Fprintf(ioStreams.ErrOut, "Known keys for %s:\n", hostname)

		for _, wantKey := range keyErr.Want {
			fmt.Fprintf(ioStreams.ErrOut, "- %s key in %s:%d\n", wantKey.Key.Type(), wantKey.Filename, wantKey.Line)
		}

		fmt.Fprintf(ioStreams.ErrOut, "Please verify the host key manually and update the known hosts file accordingly.\n")
	}

	fmt.Fprintf(ioStreams.ErrOut, "Host key for %s has changed and you have requested strict checking.\n", hostname)
	fmt.Fprintf(ioStreams.ErrOut, "Host key verification failed.\n")
}

// newKnownHostsVerifier creates a HostKeyCallback that strictly verifies host keys against known hosts files.
func newKnownHostsVerifier(knownHostsFiles []string) (ssh.HostKeyCallback, error) {
	existingFiles := filterExistingFiles(knownHostsFiles)

	hostKeyCallback, err := knownhosts.New(existingFiles...)
	if err != nil {
		return nil, fmt.Errorf("could not create hostkey callback: %w", err)
	}

	return hostKeyCallback, nil
}

// filterExistingFiles filters out known hosts files that do not exist.
func filterExistingFiles(files []string) []string {
	var existingFiles []string

	for _, file := range files {
		if _, err := os.Stat(file); err == nil {
			existingFiles = append(existingFiles, file)
		}
	}

	return existingFiles
}

// promptUserToTrustHostKey prompts the user to trust an unknown host key.
func (f *realHostKeyCallbackFactory) promptUserToTrustHostKey(hostname string, remote net.Addr, key ssh.PublicKey, ioStreams util.IOStreams) (bool, error) {
	if !f.isTerminalFunc(ioStreams.In) {
		return false, errors.New("no terminal available to prompt user")
	}

	fingerprint := ssh.FingerprintSHA256(key)

	// Prompt the user
	fmt.Fprintf(ioStreams.ErrOut, "The authenticity of host '%s (%s)' can't be established.\n", hostname, remote.String())
	fmt.Fprintf(ioStreams.ErrOut, "%s key fingerprint is %s.\n", key.Type(), fingerprint)
	fmt.Fprintf(ioStreams.ErrOut, "This key is not known by any other names.\n")
	fmt.Fprintf(ioStreams.ErrOut, "Are you sure you want to continue connecting (yes/no/[fingerprint])? ")

	// Read user input
	var response string

	for {
		_, err := fmt.Fscanln(ioStreams.In, &response)
		if err != nil {
			return false, fmt.Errorf("failed to read user input: %w", err)
		}

		response = strings.TrimSpace(response)

		switch strings.ToLower(response) {
		case "yes":
			return true, nil
		case "no":
			return false, nil
		default:
			if response == fingerprint {
				// User typed the correct fingerprint
				return true, nil
			}

			fmt.Fprintf(ioStreams.ErrOut, "Please type 'yes', 'no' or the fingerprint: ")
		}
	}
}

// addHostKeyToKnownHosts adds a new host key to the known hosts file.
func addHostKeyToKnownHosts(knownHostsFiles []string, hostname string, key ssh.PublicKey) error {
	if len(knownHostsFiles) == 0 {
		return errors.New("no known hosts file available")
	}

	knownHostsFile := knownHostsFiles[0]

	dir := filepath.Dir(knownHostsFile)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("known hosts directory does not exist: %s", dir)
	} else if err != nil {
		return fmt.Errorf("failed to stat known hosts directory %s: %w", dir, err)
	}

	knownHostsLine := knownhosts.Line([]string{hostname}, key)

	fileHandle, err := os.OpenFile(knownHostsFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600) // #nosec G304 -- Accepting user-provided known hosts file path by design
	if err != nil {
		return fmt.Errorf("failed to open known hosts file: %w", err)
	}
	defer fileHandle.Close()

	// Write to the known hosts file
	if _, err := fileHandle.WriteString(knownHostsLine + "\n"); err != nil {
		return fmt.Errorf("failed to write to known hosts file: %w", err)
	}

	return nil
}
