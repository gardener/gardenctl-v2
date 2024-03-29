/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package version_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestVersionCommand(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Version Command Test Suite")
}
