/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package target_test

import (
	"github.com/gardener/gardenctl-v2/internal/util"
	. "github.com/gardener/gardenctl-v2/pkg/cmd/common/target"
	. "github.com/gardener/gardenctl-v2/pkg/cmd/target"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Options", func() {
	It("should validate", func() {
		streams, _, _, _ := util.NewTestIOStreams()
		o := NewOptions(streams)
		o.Kind = TargetKindGarden
		o.TargetName = "foo"

		Expect(o.Validate()).To(Succeed())
	})

	It("should reject invalid kinds", func() {
		streams, _, _, _ := util.NewTestIOStreams()
		o := NewOptions(streams)
		o.Kind = TargetKind("not a kind")
		o.TargetName = "foo"

		Expect(o.Validate()).NotTo(Succeed())
	})
})
