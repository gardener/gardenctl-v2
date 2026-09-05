/*
SPDX-FileCopyrightText: Contributors to the Gardener project

SPDX-License-Identifier: Apache-2.0
*/

package logout_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestLogoutCommand(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Logout Command Test Suite")
}
