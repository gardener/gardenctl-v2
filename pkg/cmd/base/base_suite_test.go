/*
SPDX-FileCopyrightText: Copyright Contributors to the Gardener project

SPDX-License-Identifier: Apache-2.0
*/

package base_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestBaseCommand(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Base Options Test Suite")
}
