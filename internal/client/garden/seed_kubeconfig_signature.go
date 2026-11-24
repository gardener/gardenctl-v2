/*
SPDX-FileCopyrightText: Contributors to the Gardener project

SPDX-License-Identifier: Apache-2.0
*/

package garden

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"errors"
	"fmt"

	"sigs.k8s.io/json"

	"github.com/gardener/gardenctl-v2/pkg/config"
)

const (
	// SeedKubeconfigManifestDataKey is the Secret data key containing the signed manifest.
	SeedKubeconfigManifestDataKey = "kubeconfig-signing-manifest"
	// SeedKubeconfigSignatureDataKey is the Secret data key containing the DER-encoded ECDSA signature.
	SeedKubeconfigSignatureDataKey = "kubeconfig-signature"

	// SeedKubeconfigSignatureAPIVersion identifies the manifest schema.
	SeedKubeconfigSignatureAPIVersion = "gardenctl.gardener.cloud/v1alpha1"
	// SeedKubeconfigSignatureKind identifies a seed kubeconfig signature manifest.
	SeedKubeconfigSignatureKind = "SeedKubeconfigSignature"
)

// SeedKubeconfigSignatureManifest binds a kubeconfig hash to a seed.
// The signature covers the exact JSON bytes stored in SeedKubeconfigManifestDataKey.
type SeedKubeconfigSignatureManifest struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	SeedName   string `json:"seedName"`

	// KubeconfigHash is the SHA-256 digest of the exact kubeconfig bytes, formatted as "sha256:<lowercase hex>".
	KubeconfigHash string `json:"kubeconfigHash"`
}

func verifySeedKubeconfigSignature(seedName string, data map[string][]byte, publicKeys config.SeedKubeconfigPublicKeys) error {
	for _, key := range []string{"kubeconfig", SeedKubeconfigManifestDataKey, SeedKubeconfigSignatureDataKey} {
		if _, ok := data[key]; !ok {
			return fmt.Errorf("missing data key %q", key)
		}
	}

	manifestData := data[SeedKubeconfigManifestDataKey]
	if err := verifyManifestSignature(manifestData, data[SeedKubeconfigSignatureDataKey], publicKeys); err != nil {
		return err
	}

	var manifest SeedKubeconfigSignatureManifest

	fieldErrors, err := json.UnmarshalStrict(manifestData, &manifest)
	if err != nil {
		return fmt.Errorf("failed to decode signing manifest: %w", err)
	}

	if len(fieldErrors) > 0 {
		return fmt.Errorf("invalid signing manifest: %w", errors.Join(fieldErrors...))
	}

	if manifest.APIVersion != SeedKubeconfigSignatureAPIVersion {
		return fmt.Errorf("unsupported signing manifest apiVersion %q", manifest.APIVersion)
	}

	if manifest.Kind != SeedKubeconfigSignatureKind {
		return fmt.Errorf("unsupported signing manifest kind %q", manifest.Kind)
	}

	if manifest.SeedName != seedName {
		return errors.New("signing manifest seed name does not match the targeted seed")
	}

	calculatedHash := fmt.Sprintf("sha256:%x", sha256.Sum256(data["kubeconfig"]))
	if manifest.KubeconfigHash != calculatedHash {
		return errors.New("kubeconfig hash does not match the signed manifest")
	}

	return nil
}

func verifyManifestSignature(manifest, signature []byte, trustedKeys config.SeedKubeconfigPublicKeys) error {
	if len(trustedKeys) == 0 {
		return errors.New("no trusted keys configured: set seedKubeconfigPublicKeys as described in docs/config/seed-kubeconfig-signatures.md")
	}

	keys, err := trustedKeys.Parse()
	if err != nil {
		return err
	}

	digest := sha256.Sum256(manifest)

	for _, key := range keys {
		if ecdsa.VerifyASN1(key, digest[:], signature) {
			return nil
		}
	}

	return errors.New("ECDSA P-256 signature verification failed with all configured keys")
}
