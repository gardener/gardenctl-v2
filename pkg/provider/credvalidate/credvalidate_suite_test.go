/*
SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package credvalidate_test

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCredvalidate(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Credvalidate Suite")
}

var _ = BeforeSuite(func() {
	// Ensure a stable default for redaction across the suite; specs can override with Setenv.
	prev, had := os.LookupEnv("GCTL_UNSAFE_DEBUG")
	Expect(os.Setenv("GCTL_UNSAFE_DEBUG", "false")).To(Succeed())

	DeferCleanup(func() {
		if had {
			_ = os.Setenv("GCTL_UNSAFE_DEBUG", prev)
		} else {
			_ = os.Unsetenv("GCTL_UNSAFE_DEBUG")
		}
	})
})
