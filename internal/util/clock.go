/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package util

import "time"

// Clock provides the current time
type Clock interface {
	// Now returns the current time
	Now() time.Time
}

// RealClock implements Clock interface
type RealClock struct{}

func (RealClock) Now() time.Time {
	return time.Now()
}
