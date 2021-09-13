/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package drop_test

import (
	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/cmd/common/target"
	"github.com/gardener/gardenctl-v2/pkg/cmd/target/drop"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Options", func() {
	It("should validate", func() {
		streams, _, _, _ := util.NewTestIOStreams()
		o := drop.NewOptions(streams)
		o.Kind = target.TargetKindGarden

		Expect(o.Validate()).To(Succeed())
	})

	It("should reject invalid kinds", func() {
		streams, _, _, _ := util.NewTestIOStreams()
		o := drop.NewOptions(streams)
		o.Kind = target.TargetKind("not a kind")

		Expect(o.Validate()).NotTo(Succeed())
	})
})
