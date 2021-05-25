/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package target_test

import (
	internalfake "github.com/gardener/gardenctl-v2/internal/fake"
	"github.com/gardener/gardenctl-v2/internal/util"
	. "github.com/gardener/gardenctl-v2/pkg/cmd/target"
	"github.com/gardener/gardenctl-v2/pkg/target"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

var _ = Describe("Command", func() {
	It("should reject bad options", func() {
		streams, _, _, _ := genericclioptions.NewTestIOStreams()
		o := NewOptions(streams)
		cmd := NewCommand(&util.FactoryImpl{}, o)

		Expect(cmd.RunE(cmd, nil)).NotTo(Succeed())
	})

	It("should be able to target a garden", func() {
		streams, _, _, _ := genericclioptions.NewTestIOStreams()

		gardenName := "mygarden"
		config := &target.Config{
			Gardens: []target.Garden{{
				Name:       gardenName,
				Kubeconfig: "",
			}},
		}
		targetProvider := internalfake.NewFakeTargetProvider(target.NewTarget("", "", "", ""))
		factory := internalfake.NewFakeFactory(config, nil, nil, targetProvider)
		cmd := NewCommand(factory, NewOptions(streams))

		Expect(cmd.RunE(cmd, []string{"garden", gardenName})).To(Succeed())

		currentTarget, err := targetProvider.Read()
		Expect(err).NotTo(HaveOccurred())
		Expect(currentTarget.GardenName()).To(Equal(gardenName))
	})
})
