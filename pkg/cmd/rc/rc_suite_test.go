/*
SPDX-FileCopyrightText: Contributors to the Gardener project

SPDX-License-Identifier: Apache-2.0
*/

package rc_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestRCCommand(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "RC Command Test Suite")
}
