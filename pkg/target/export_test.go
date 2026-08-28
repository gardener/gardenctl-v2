/*
SPDX-FileCopyrightText: Contributors to the Gardener project

SPDX-License-Identifier: Apache-2.0
*/

package target

import (
	"context"

	"github.com/gardener/gardenctl-v2/pkg/config"
)

var Merge = merge

// ResolvePatternOverlay exposes managerImpl.resolvePatternOverlay for tests.
func ResolvePatternOverlay(ctx context.Context, m Manager, tf TargetFlags, persistedTarget Target, value string) (TargetFlags, error) {
	return m.(*managerImpl).resolvePatternOverlay(ctx, tf, persistedTarget, value)
}

// ResolveAccessLevel exposes managerImpl.resolveAccessLevel for tests.
func ResolveAccessLevel(m Manager, t Target, scope AccessScope) config.KubeconfigAccessLevel {
	return m.(*managerImpl).resolveAccessLevel(t, scope)
}
