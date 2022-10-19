/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package target_test

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardenctl-v2/pkg/target"
)

func expectEqualTargets(actual, expected target.Target) {
	ExpectWithOffset(1, actual.GardenName()).To(Equal(expected.GardenName()))
	ExpectWithOffset(1, actual.ProjectName()).To(Equal(expected.ProjectName()))
	ExpectWithOffset(1, actual.SeedName()).To(Equal(expected.SeedName()))
	ExpectWithOffset(1, actual.ShootName()).To(Equal(expected.ShootName()))
	ExpectWithOffset(1, actual.ControlPlane()).To(Equal(expected.ControlPlane()))
}

var _ = Describe("Target Provider", func() {
	var (
		tmpFile  *os.File
		provider target.TargetProvider
	)

	BeforeEach(func() {
		var err error

		tmpFile, err = os.CreateTemp("", "gardenertarget*")
		Expect(err).NotTo(HaveOccurred())

		provider = target.NewTargetProvider(tmpFile.Name(), nil)
	})

	AfterEach(func() {
		if tmpFile != nil {
			tmpFile.Close()
			os.Remove(tmpFile.Name())
		}
	})

	It("should return a default target if the file is empty", func() {
		target, err := provider.Read()
		Expect(err).To(Succeed())
		Expect(target).NotTo(BeNil())
	})

	It("should return a default target if the file does not exist", func() {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		tmpFile = nil

		target, err := provider.Read()
		Expect(err).To(Succeed())
		Expect(target).NotTo(BeNil())
	})

	It("should be able to write a target", func() {
		t := target.NewTarget("garden", "project", "", "shoot").WithControlPlane(true)

		// write it
		Expect(provider.Write(t)).To(Succeed())

		// read it back
		target, err := provider.Read()
		Expect(err).To(Succeed())
		Expect(target).NotTo(BeNil())
		Expect(target.GardenName()).To(Equal(t.GardenName()))
		Expect(target.ProjectName()).To(Equal(t.ProjectName()))
		Expect(target.SeedName()).To(Equal(t.SeedName()))
		Expect(target.ShootName()).To(Equal(t.ShootName()))
		Expect(target.ControlPlane()).To(Equal(t.ControlPlane()))

	})
})

var _ = Describe("Dynamic Target Provider", func() {
	var (
		tmpFile  *os.File
		provider target.TargetProvider
	)

	BeforeEach(func() {
		var err error

		tmpFile, err = os.CreateTemp("", "gardenertarget*")
		Expect(err).NotTo(HaveOccurred())

		provider = target.NewTargetProvider(tmpFile.Name(), nil)
	})

	AfterEach(func() {
		if tmpFile != nil {
			tmpFile.Close()
			os.Remove(tmpFile.Name())
		}
	})

	It("should just return the file content if no flags are given", func() {
		// prepare target
		dummy := target.NewTarget("mygarden", "myproject", "", "myshoot")
		Expect(provider.Write(dummy)).To(Succeed())

		tf := target.NewTargetFlags("", "", "", "", false)
		dtp := target.NewTargetProvider(tmpFile.Name(), tf)

		readBack, err := dtp.Read()
		Expect(err).NotTo(HaveOccurred())
		expectEqualTargets(readBack, dummy)
	})

	DescribeTable(
		"should return a new target for complete flags",
		func(tf target.TargetFlags) {
			// prepare target that should never be read
			dummy := target.NewTarget("mygarden", "myproject", "", "myshoot")
			Expect(provider.Write(dummy)).To(Succeed())

			dtp := target.NewTargetProvider(tmpFile.Name(), tf)

			readBack, err := dtp.Read()
			Expect(err).NotTo(HaveOccurred())
			expectEqualTargets(readBack, tf.ToTarget())
		},
		Entry("just garden", target.NewTargetFlags("newgarden", "", "", "", false)),
		Entry("garden->project", target.NewTargetFlags("newgarden", "newproject", "", "", false)),
		Entry("garden->seed", target.NewTargetFlags("newgarden", "", "newseed", "", false)),
		Entry("garden->project->shoot", target.NewTargetFlags("newgarden", "newproject", "", "newshoot", false)),
		Entry("garden->project->shoot->control-plane", target.NewTargetFlags("newgarden", "newproject", "", "newshoot", true)),
	)

	DescribeTable(
		"should augment existing target with CLI flags",
		func(tf target.TargetFlags, expected target.Target) {
			dummy := target.NewTarget("mygarden", "myproject", "", "myshoot")
			Expect(provider.Write(dummy)).To(Succeed())

			dtp := target.NewTargetProvider(tmpFile.Name(), tf)

			readBack, err := dtp.Read()
			Expect(err).NotTo(HaveOccurred())
			expectEqualTargets(readBack, expected)
		},
		Entry(
			"target garden cluster",
			target.NewTargetFlags("newgarden", "", "", "", false),
			target.NewTarget("newgarden", "", "", ""),
		),
		Entry(
			"target project",
			target.NewTargetFlags("", "newproject", "", "", false),
			target.NewTarget("mygarden", "newproject", "", ""),
		),
		Entry(
			"target seed",
			target.NewTargetFlags("", "", "newseed", "", false),
			target.NewTarget("mygarden", "", "newseed", ""),
		),
		Entry(
			"target shoot",
			target.NewTargetFlags("", "", "", "newshoot", false),
			target.NewTarget("mygarden", "myproject", "", "newshoot"),
		),
		Entry(
			"target shoot control plane",
			target.NewTargetFlags("", "", "", "myshoot", true),
			target.NewTarget("mygarden", "myproject", "", "myshoot").WithControlPlane(true),
		),
		Entry(
			"target shoot in a different project",
			target.NewTargetFlags("", "newproject", "", "newshoot", false),
			target.NewTarget("mygarden", "newproject", "", "newshoot"),
		),
		Entry(
			"target shoot in a seed",
			target.NewTargetFlags("", "", "newseed", "newshoot", false),
			target.NewTarget("mygarden", "", "newseed", "newshoot"),
		),
		Entry(
			"complete re-target",
			target.NewTargetFlags("newgarden", "", "newseed", "newshoot", false),
			target.NewTarget("newgarden", "", "newseed", "newshoot"),
		),
	)

	DescribeTable(
		"should not allow syntactically wrong targets",
		func(tf target.TargetFlags) {
			dummy := target.NewTarget("mygarden", "myproject", "", "myshoot")
			Expect(provider.Write(dummy)).To(Succeed())

			dtp := target.NewTargetProvider(tmpFile.Name(), tf)

			readBack, err := dtp.Read()
			Expect(readBack).To(BeNil())
			Expect(err).To(HaveOccurred())
		},
		Entry("seed and project", target.NewTargetFlags("", "newproject", "newseed", "", false)),
	)

	It("should write changes as expected", func() {
		// prepare target
		dummy := target.NewTarget("mygarden", "myproject", "", "myshoot")

		tf := target.NewTargetFlags("", "", "", "", false)
		dtp := target.NewTargetProvider(tmpFile.Name(), tf)
		Expect(dtp.Write(dummy)).To(Succeed())

		readBack, err := provider.Read()
		Expect(err).NotTo(HaveOccurred())
		expectEqualTargets(readBack, dummy)
	})
})
