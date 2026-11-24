/*
SPDX-FileCopyrightText: Contributors to the Gardener project

SPDX-License-Identifier: Apache-2.0
*/

package config

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/gardener/gardenctl-v2/internal/util"
)

// NewCmdConfigAddSeedKey returns the command to trust a seed kubeconfig signing key.
func NewCmdConfigAddSeedKey(f util.Factory, streams util.IOStreams) *cobra.Command {
	var filename, inlineKey string

	cmd := &cobra.Command{
		Use:   "add-seed-key GARDEN (--file PUBLIC_KEY.pem | --key PEM)",
		Short: "Add a trusted seed kubeconfig signing key for a Garden",
		Long: `Add the PEM-encoded ECDSA P-256 public key from your landscape administrator to a configured Garden by identity or alias.
Run 'gardenctl config view' to find the Garden's identity or alias.
Use --file for a PEM file or --key for the full PEM block with line breaks.
Adding the same key again succeeds without changing the configuration.

Add a key only after confirming with the landscape administrator that it is theirs.
The holder of the matching private key decides which kubeconfig gardenctl loads for the Garden's non-managed seeds,
including the commands that kubeconfig runs on your machine to authenticate.`,
		Example: `gardenctl config add-seed-key landscape-dev --file public-key.pem

# Replace the placeholder with the actual public key and keep the line breaks.
gardenctl config add-seed-key landscape-dev --key '-----BEGIN PUBLIC KEY-----
<public key from your landscape administrator>
-----END PUBLIC KEY-----'`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("provide one Garden identity or alias. See '%s --help'", cmd.CommandPath())
			}

			return nil
		},
		ValidArgsFunction: validGardenArgsFunctionWrapper(f, streams),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := getConfiguration(f)
			if err != nil {
				return err
			}

			garden, err := cfg.Garden(args[0])
			if err != nil {
				return fmt.Errorf("%w. Check the identities and aliases under gardens in %q, or add the Garden with 'gardenctl config set-garden'", err, cfg.Filename)
			}

			publicKey := inlineKey

			if cmd.Flags().Changed("file") {
				data, err := os.ReadFile(filename) // #nosec G304 -- The user supplies the public key file.
				if err != nil {
					return fmt.Errorf("failed to read public key: %w", err)
				}

				publicKey = string(data)
			}

			previousKeys := garden.SeedKubeconfigPublicKeys

			added, err := garden.SeedKubeconfigPublicKeys.Add(publicKey)
			if err != nil {
				return fmt.Errorf("failed to add public key: %w. Provide the full PEM public key from your landscape administrator, including line breaks", err)
			}

			if !added {
				fmt.Fprintf(streams.Out, "Public key is already trusted for garden %q\n", garden.Name)
				return nil
			}

			if err := cfg.Save(); err != nil {
				garden.SeedKubeconfigPublicKeys = previousKeys
				return fmt.Errorf("failed to save garden configuration: %w", err)
			}

			fmt.Fprintf(streams.Out, "Added trusted public key for garden %q\n", garden.Name)

			return nil
		},
	}
	cmd.Flags().StringVarP(&filename, "file", "f", "", "Path to the PEM public key from your landscape administrator")
	cmd.Flags().StringVar(&inlineKey, "key", "", "Full PEM public key, including line breaks")
	cmd.MarkFlagsOneRequired("file", "key")
	cmd.MarkFlagsMutuallyExclusive("file", "key")
	utilruntime.Must(cmd.MarkFlagFilename("file"))
	utilruntime.Must(cmd.RegisterFlagCompletionFunc("key", cobra.NoFileCompletions))

	return cmd
}
