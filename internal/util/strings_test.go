/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package util_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardenctl-v2/internal/util"
)

var _ = Describe("String Utilities", func() {
	Describe("filtering string by prefix", func() {
		It("should return only strings with the given prefix", func() {
			Expect(util.FilterStringsByPrefix("", []string{"a", "c"})).To(Equal([]string{"a", "c"}))
			Expect(util.FilterStringsByPrefix("x", []string{"xa", "yb", "xc", "zx"})).To(Equal([]string{"xa", "xc"}))
		})
	})
})
