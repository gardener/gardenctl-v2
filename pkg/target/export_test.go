/*
SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package target

import "github.com/gardener/gardenctl-v2/pkg/config"

var Merge = merge

// ResolveAccessLevel exposes managerImpl.resolveAccessLevel for tests.
func ResolveAccessLevel(m Manager, t Target, scope AccessScope) config.KubeconfigAccessLevel {
	return m.(*managerImpl).resolveAccessLevel(t, scope)
}
