/*
SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package testdata

import "embed"

//go:embed templates azure gcp openstack test stackit
var FS embed.FS
