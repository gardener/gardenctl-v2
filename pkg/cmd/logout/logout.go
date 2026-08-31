/*
SPDX-FileCopyrightText: Contributors to the Gardener project

SPDX-License-Identifier: Apache-2.0
*/

package logout

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
	"github.com/gardener/gardenctl-v2/pkg/config"
)

const defaultOIDCCacheDir = "~/.kube/cache/oidc-login"

// NewCmdLogout returns a new logout command.
func NewCmdLogout(f util.Factory, ioStreams util.IOStreams) *cobra.Command {
	o := newOptions(ioStreams)

	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Clear cached OIDC login tokens for garden cluster(s)",
		Long: `Clear cached OIDC login tokens for one or all configured garden clusters.

This command reads the exec plugin configuration from each garden cluster's
kubeconfig, determines the OIDC token cache directory (defaulting to
~/.kube/cache/oidc-login/), and removes cached token files whose issuer
matches the garden's OIDC provider.

Use this when your OIDC token has expired or is no longer refreshing and you
need to force a fresh login on the next kubectl/gardenctl invocation.`,
		Example: `# Clear cached OIDC tokens for all configured garden clusters
gardenctl logout

# Clear cached OIDC tokens for a specific garden cluster
gardenctl logout --garden my-garden`,
		RunE: base.WrapRunE(o, f),
	}

	o.AddFlags(cmd.Flags())

	return cmd
}

// options is a struct to support the logout command.
type options struct {
	base.Options

	// GardenName optionally restricts logout to a single garden.
	GardenName string

	// cfg is loaded during Complete.
	cfg *config.Config

	// entries holds the resolved (gardenName, cacheDir, issuerURL) per garden.
	entries []cacheEntry
}

type cacheEntry struct {
	gardenName string
	cacheDir   string
	issuerURL  string // empty when no OIDC exec plugin was found
}

// tokenCacheFile is the on-disk structure written by kubelogin.
type tokenCacheFile struct {
	IDToken string `json:"id_token"`
}

// jwtPayload holds only the claims we care about.
type jwtPayload struct {
	Iss string `json:"iss"`
}

func newOptions(ioStreams util.IOStreams) *options {
	return &options{
		Options: base.Options{
			IOStreams: ioStreams,
		},
	}
}

// AddFlags binds command flags to the options struct.
func (o *options) AddFlags(flags *pflag.FlagSet) {
	flags.StringVar(&o.GardenName, "garden", o.GardenName, "Name of the garden cluster to log out of. Defaults to all configured gardens.")
}

// Complete resolves configuration and OIDC exec plugin settings for each garden.
func (o *options) Complete(f util.Factory, _ *cobra.Command, _ []string) error {
	manager, err := f.Manager()
	if err != nil {
		return fmt.Errorf("failed to create manager: %w", err)
	}

	cfg := manager.Configuration()
	o.cfg = cfg

	gardens := cfg.Gardens
	if o.GardenName != "" {
		g, err := cfg.Garden(o.GardenName)
		if err != nil {
			return err
		}

		gardens = []config.Garden{*g}
	}

	for _, g := range gardens {
		entry := cacheEntry{gardenName: g.Name}

		rawCfg, err := g.LoadRawConfig()
		if err != nil {
			fmt.Fprintf(o.IOStreams.ErrOut, "Warning: could not load kubeconfig for garden %q: %v\n", g.Name, err)
			o.entries = append(o.entries, entry)

			continue
		}

		// Search all authInfos for an OIDC exec plugin.
		for _, authInfo := range rawCfg.AuthInfos {
			if authInfo.Exec == nil {
				continue
			}

			cmd := authInfo.Exec.Command
			if !strings.Contains(cmd, "kubelogin") && !strings.Contains(cmd, "oidc-login") {
				continue
			}

			entry.cacheDir, entry.issuerURL = parseExecArgs(authInfo.Exec.Args)

			break
		}

		o.entries = append(o.entries, entry)
	}

	return nil
}

// parseExecArgs extracts --token-cache-dir and --oidc-issuer-url from exec plugin args.
// Returns the resolved cache directory and issuer URL.
func parseExecArgs(args []string) (cacheDir, issuerURL string) {
	for i, arg := range args {
		switch {
		case strings.HasPrefix(arg, "--token-cache-dir="):
			cacheDir = strings.TrimPrefix(arg, "--token-cache-dir=")
		case arg == "--token-cache-dir" && i+1 < len(args):
			cacheDir = args[i+1]
		case strings.HasPrefix(arg, "--oidc-issuer-url="):
			issuerURL = strings.TrimPrefix(arg, "--oidc-issuer-url=")
		case arg == "--oidc-issuer-url" && i+1 < len(args):
			issuerURL = args[i+1]
		}
	}

	if cacheDir == "" {
		cacheDir = defaultOIDCCacheDir
	}

	expanded, err := homedir.Expand(cacheDir)
	if err == nil {
		cacheDir = expanded
	}

	return cacheDir, issuerURL
}

// Validate checks that options are consistent.
func (o *options) Validate() error {
	return nil
}

// Run deletes cached OIDC token files matching each garden's issuer URL.
func (o *options) Run(_ util.Factory) error {
	for _, entry := range o.entries {
		if entry.issuerURL == "" {
			fmt.Fprintf(o.IOStreams.ErrOut, "Warning: no OIDC exec plugin found in kubeconfig for garden %q, skipping\n", entry.gardenName)

			continue
		}

		removed, err := removeCachedTokens(entry.cacheDir, entry.issuerURL)
		if err != nil {
			fmt.Fprintf(o.IOStreams.ErrOut, "Warning: error clearing tokens for garden %q: %v\n", entry.gardenName, err)

			continue
		}

		if removed == 0 {
			fmt.Fprintf(o.IOStreams.Out, "No cached tokens found for garden %q (issuer: %s)\n", entry.gardenName, entry.issuerURL)
		} else {
			fmt.Fprintf(o.IOStreams.Out, "Removed %d cached token file(s) for garden %q (issuer: %s)\n", removed, entry.gardenName, entry.issuerURL)
		}
	}

	return nil
}

// removeCachedTokens deletes token cache files in dir whose id_token issuer matches issuerURL.
// Returns the number of files removed.
func removeCachedTokens(dir, issuerURL string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}

		return 0, fmt.Errorf("failed to read cache directory %q: %w", dir, err)
	}

	removed := 0

	for _, e := range entries {
		if e.IsDir() || strings.HasSuffix(e.Name(), ".lock") {
			continue
		}

		path := filepath.Join(dir, e.Name())

		iss, err := readIssuerFromCacheFile(path)
		if err != nil {
			// Not a token cache file we understand; skip silently.
			continue
		}

		if iss != issuerURL {
			continue
		}

		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return removed, fmt.Errorf("failed to remove %q: %w", path, err)
		}

		removed++
	}

	return removed, nil
}

// readIssuerFromCacheFile reads a kubelogin token cache file and returns the OIDC issuer URL.
// The file contains {"id_token": "<jwt>", ...}; the issuer is in the JWT payload's "iss" claim.
func readIssuerFromCacheFile(path string) (string, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- path is constructed from os.ReadDir output
	if err != nil {
		return "", err
	}

	var f tokenCacheFile
	if err := json.Unmarshal(data, &f); err != nil || f.IDToken == "" {
		return "", fmt.Errorf("not a token cache file")
	}

	return issuerFromJWT(f.IDToken)
}

// issuerFromJWT extracts the "iss" claim from a JWT's payload segment.
func issuerFromJWT(token string) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid JWT format")
	}

	// JWT payload is base64url-encoded without padding.
	payload := parts[1]
	// Add padding if needed.
	switch len(payload) % 4 {
	case 2:
		payload += "=="
	case 3:
		payload += "="
	}

	decoded, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		return "", fmt.Errorf("failed to decode JWT payload: %w", err)
	}

	var claims jwtPayload
	if err := json.Unmarshal(decoded, &claims); err != nil {
		return "", fmt.Errorf("failed to unmarshal JWT payload: %w", err)
	}

	if claims.Iss == "" {
		return "", fmt.Errorf("JWT payload has no iss claim")
	}

	return claims.Iss, nil
}
