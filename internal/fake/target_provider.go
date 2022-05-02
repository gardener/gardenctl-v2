/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package fake

import (
	"errors"

	"github.com/gardener/gardenctl-v2/pkg/target"
)

type TargetProvider struct {
	Target target.Target
}

var _ target.TargetProvider = &TargetProvider{}

// NewFakeTargetProvider returns a new TargetProvider that
// reads and writes from memory.
func NewFakeTargetProvider(t target.Target) *TargetProvider {
	return &TargetProvider{
		Target: t,
	}
}

// Read returns the current target.
func (p *TargetProvider) Read() (target.Target, error) {
	if p.Target == nil {
		return nil, errors.New("no target set")
	}

	return p.Target, nil
}

// Write takes a target and saves it permanently.
func (p *TargetProvider) Write(t target.Target) error {
	p.Target = t

	return nil
}
