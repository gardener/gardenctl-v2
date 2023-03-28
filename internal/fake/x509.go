/*
SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package fake

import (
	gardensecrets "github.com/gardener/gardener/pkg/utils/secrets"
)

func NewCaCert() (*gardensecrets.Certificate, error) {
	caCertCSC := &gardensecrets.CertificateSecretConfig{
		Name:       "test-issuer-name",
		CommonName: "test-issuer-cn",
		CertType:   gardensecrets.CACert,
	}

	return caCertCSC.GenerateCertificate()
}

func NewClientCert(caCert *gardensecrets.Certificate, commonName string, organization []string) (*gardensecrets.Certificate, error) {
	csc := &gardensecrets.CertificateSecretConfig{
		Name:         "client-name",
		CommonName:   commonName,
		Organization: organization,
		CertType:     gardensecrets.ClientCert,
		SigningCA:    caCert,
	}

	return csc.GenerateCertificate()
}
