/*
SPDX-FileCopyrightText: Contributors to the Gardener project

SPDX-License-Identifier: Apache-2.0
*/

package config

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
)

// SeedKubeconfigPublicKeys lists the PEM-encoded ECDSA P-256 keys trusted by a Garden.
type SeedKubeconfigPublicKeys []string

// UnmarshalJSON decodes the key list and reports the expected format on error.
func (keys *SeedKubeconfigPublicKeys) UnmarshalJSON(data []byte) error {
	var decoded []string

	if err := json.Unmarshal(data, &decoded); err != nil {
		return fmt.Errorf("seedKubeconfigPublicKeys must be a YAML list of PEM public keys: %w", err)
	}

	*keys = decoded

	return nil
}

// Add appends a validated public key and reports whether the list changed.
func (keys *SeedKubeconfigPublicKeys) Add(encoded string) (bool, error) {
	key, err := decodeSeedKubeconfigPublicKey(encoded)
	if err != nil {
		return false, err
	}

	trusted, err := keys.Parse()
	if err != nil {
		return false, err
	}

	for _, existing := range trusted {
		if key.Equal(existing) {
			return false, nil
		}
	}

	der, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return false, err
	}

	publicKey := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
	*keys = append(*keys, string(publicKey))

	return true, nil
}

// Parse decodes each configured public key and identifies malformed entries by list index.
func (keys SeedKubeconfigPublicKeys) Parse() ([]*ecdsa.PublicKey, error) {
	parsed := make([]*ecdsa.PublicKey, 0, len(keys))

	for i, encoded := range keys {
		key, err := decodeSeedKubeconfigPublicKey(encoded)
		if err != nil {
			return nil, fmt.Errorf("invalid public key at index %d: %w", i, err)
		}

		parsed = append(parsed, key)
	}

	return parsed, nil
}

func decodeSeedKubeconfigPublicKey(encodedPublicKey string) (*ecdsa.PublicKey, error) {
	der, err := decodeSinglePublicKeyPEM(encodedPublicKey)
	if err != nil {
		return nil, err
	}

	parsed, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return nil, fmt.Errorf("invalid SubjectPublicKeyInfo: %w. Expected an ECDSA P-256 public key that identifies its curve by name", err)
	}

	key, ok := parsed.(*ecdsa.PublicKey)
	if !ok {
		return nil, errors.New("expected an ECDSA public key")
	}

	if key.Curve != elliptic.P256() {
		return nil, errors.New("expected an ECDSA P-256 public key")
	}

	return key, nil
}

func decodeSinglePublicKeyPEM(encodedPublicKey string) ([]byte, error) {
	encodedPublicKey = strings.TrimSpace(encodedPublicKey)

	header, _, _ := strings.Cut(encodedPublicKey, "\n")
	if strings.TrimSpace(header) != "-----BEGIN PUBLIC KEY-----" {
		return nil, errors.New("expected a PEM PUBLIC KEY block")
	}

	// pem.Decode skips malformed blocks while looking for a valid one.
	if strings.Count(encodedPublicKey, "-----BEGIN ") != 1 {
		return nil, errors.New("expected exactly one PEM block")
	}

	block, rest := pem.Decode([]byte(encodedPublicKey))
	if block == nil {
		return nil, errors.New("invalid PUBLIC KEY PEM encoding")
	}

	if len(bytes.TrimSpace(rest)) != 0 {
		return nil, errors.New("unexpected data after the PEM public key")
	}

	return block.Bytes, nil
}
