/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package ssh_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestCommand(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "SSH Command Test Suite")
}
