/*
SPDX-FileCopyrightText: Contributors to the Gardener project

SPDX-License-Identifier: Apache-2.0
*/

package ac_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAccessControl(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "AccessControl Package Test Suite")
}
