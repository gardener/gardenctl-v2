/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package base

import "k8s.io/cli-runtime/pkg/genericclioptions"

// Options contains all settings that are used across all commands in gardenctl.
type Options struct {
	// IOStreams provides the standard names for iostreams
	IOStreams genericclioptions.IOStreams
}

// NewOptions returns initialized Options
func NewOptions(ioStreams genericclioptions.IOStreams) *Options {
	return &Options{
		IOStreams: ioStreams,
	}
}
