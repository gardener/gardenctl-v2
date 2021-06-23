/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package version_test

import (
	"github.com/gardener/gardenctl-v2/internal/util"
	. "github.com/gardener/gardenctl-v2/pkg/cmd/version"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Options", func() {
	It("should validate", func() {
		streams, _, _, _ := util.NewTestIOStreams()
		o := NewOptions(streams)
		err := o.Validate()
		Expect(err).ToNot(HaveOccurred())
	})
})
