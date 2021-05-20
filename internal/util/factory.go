/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package util

// Factory provides abstractions that allow the command to be extended across multiple types of resources and different API sets.
type Factory interface {
	// Clock returns a clock that provides access to the current time.
	Clock() Clock
	// HomeDir returns the home directory for the executing user.
	HomeDir() string
}

// FactoryImpl implements util.Factory interface
type FactoryImpl struct {
	HomeDirectory string
}

func (f *FactoryImpl) HomeDir() string {
	return f.HomeDirectory
}

func (f *FactoryImpl) Clock() Clock {
	return &RealClock{}
}
