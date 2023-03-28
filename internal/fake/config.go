/*
SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package fake

import (
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// NewConfigData generates a Kubernetes client configuration as a byte slice.
func NewConfigData(name string) ([]byte, error) {
	return clientcmd.Write(*NewTokenConfig(name))
}

// NewTokenConfig generates a new Kubernetes client configuration
// with token authentication info.
func NewTokenConfig(name string) *clientcmdapi.Config {
	config := clientcmdapi.NewConfig()
	config.Clusters["cluster"] = &clientcmdapi.Cluster{
		Server: "https://kubernetes:6443/",
	}
	config.AuthInfos["user"] = &clientcmdapi.AuthInfo{
		Token: "token",
	}
	config.Contexts[name] = &clientcmdapi.Context{
		Namespace: "default",
		AuthInfo:  "user",
		Cluster:   "cluster",
	}
	config.CurrentContext = name

	return config
}

func NewCertConfig(name string, clientCert []byte) *clientcmdapi.Config {
	config := clientcmdapi.NewConfig()
	config.Clusters["cluster"] = &clientcmdapi.Cluster{
		Server: "https://kubernetes:6443/",
	}
	config.AuthInfos["user"] = &clientcmdapi.AuthInfo{
		ClientCertificateData: clientCert,
	}
	config.Contexts[name] = &clientcmdapi.Context{
		Namespace: "default",
		AuthInfo:  "user",
		Cluster:   "cluster",
	}
	config.CurrentContext = name

	return config
}
