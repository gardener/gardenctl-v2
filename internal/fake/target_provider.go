/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/
package fake

import (
	"errors"

	"github.com/gardener/gardenctl-v2/pkg/target"
)

type fakeTargetProvider struct {
	t target.Target
}

var _ target.TargetProvider = &fakeTargetProvider{}

// NewFakeTargetProvider returns a new TargetProvider that
// reads and writes from memory.
func NewFakeTargetProvider(t target.Target) target.TargetProvider {
	return &fakeTargetProvider{
		t: t,
	}
}

// Read returns the current target.
func (p *fakeTargetProvider) Read() (target.Target, error) {
	if p.t == nil {
		return nil, errors.New("no target set")
	}

	return p.t, nil
}

// Write takes a target and saves it permanently.
func (p *fakeTargetProvider) Write(t target.Target) error {
	p.t = t

	return nil
}
