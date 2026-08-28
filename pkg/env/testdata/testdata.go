/*
SPDX-FileCopyrightText: Contributors to the Gardener project

SPDX-License-Identifier: Apache-2.0
*/

package testdata

import "embed"

//go:embed templates azure gcp openstack test stackit
var FS embed.FS
