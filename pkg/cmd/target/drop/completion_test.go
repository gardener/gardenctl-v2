/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package drop

import (
	. "github.com/gardener/gardenctl-v2/pkg/cmd/common/target"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Completion", func() {
	Describe("validArgsFunction", func() {
		It("should return the allowed target types when no kind was given", func() {
			values, err := validArgsFunction(nil, nil, nil, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal([]string{
				string(TargetKindGarden),
				string(TargetKindProject),
				string(TargetKindSeed),
				string(TargetKindShoot),
			}))
		})
	})
})
