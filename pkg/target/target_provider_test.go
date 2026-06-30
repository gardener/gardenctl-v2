/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package target_test

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/pflag"

	"github.com/gardener/gardenctl-v2/pkg/target"
)

func expectEqualTargets(actual, expected target.Target) {
	ExpectWithOffset(1, actual.GardenName()).To(Equal(expected.GardenName()))
	ExpectWithOffset(1, actual.ProjectName()).To(Equal(expected.ProjectName()))
	ExpectWithOffset(1, actual.SeedName()).To(Equal(expected.SeedName()))
	ExpectWithOffset(1, actual.ShootName()).To(Equal(expected.ShootName()))
	ExpectWithOffset(1, actual.ControlPlane()).To(Equal(expected.ControlPlane()))
}

func mustParseTargetFlags(args []string) target.TargetFlags {
	tf := target.NewTargetFlags("", "", "", "", false)
	flags := &pflag.FlagSet{}
	tf.AddFlags(flags)
	ExpectWithOffset(1, flags.Parse(args)).To(Succeed())

	return tf
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

	It("should override a target with target flags", func() {
		tf := target.NewTargetFlags("garden", "project", "", "shoot", true)

		t, err := target.Merge(target.NewTarget("a", "b", "c", "d"), tf)
		Expect(err).NotTo(HaveOccurred())
		Expect(t.GardenName()).To(Equal("garden"))
		Expect(t.ProjectName()).To(Equal("project"))
		Expect(t.SeedName()).To(BeEmpty())
		Expect(t.ShootName()).To(Equal("shoot"))
		Expect(t.ControlPlane()).To(BeTrue())
	})

	It("should allow to override a target", func() {
		tf := target.NewTargetFlags("", "", "", "shoot", false)
		_, err := target.Merge(target.NewTarget("", "b", "c", "d"), tf)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should clear control-plane when the flag is explicitly false", func() {
		tf := mustParseTargetFlags([]string{"--control-plane=false"})

		t, err := target.Merge(target.NewTarget("garden", "project", "", "shoot").WithControlPlane(true), tf)
		Expect(err).NotTo(HaveOccurred())
		expectEqualTargets(t, target.NewTarget("garden", "project", "", "shoot"))
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

	It("should apply target flags consistently across consecutive reads after a write", func() {
		// `gardenctl target shoot newshoot --garden mygarden` writes the full
		// resolved target, while regular dynamic reads continue to apply the
		// explicit --garden selector.
		written := target.NewTarget("mygarden", "myproject", "myseed", "newshoot")
		expectedRead := target.NewTarget("mygarden", "", "", "")

		tf := target.NewTargetFlags("mygarden", "", "", "", false)
		dtp := target.NewTargetProvider(tmpFile.Name(), tf)

		Expect(dtp.Write(written)).To(Succeed())

		first, err := dtp.Read()
		Expect(err).NotTo(HaveOccurred())
		expectEqualTargets(first, expectedRead)

		second, err := dtp.Read()
		Expect(err).NotTo(HaveOccurred())
		expectEqualTargets(second, first)
	})

	It("should clear an inherited control-plane target when the flag is explicitly false", func() {
		tf := mustParseTargetFlags([]string{"--control-plane=false"})
		dtp := target.NewTargetProvider(tmpFile.Name(), tf)

		Expect(provider.Write(target.NewTarget("mygarden", "myproject", "", "myshoot").WithControlPlane(true))).To(Succeed())

		readBack, err := dtp.Read()
		Expect(err).NotTo(HaveOccurred())
		expectEqualTargets(readBack, target.NewTarget("mygarden", "myproject", "", "myshoot"))
	})

	DescribeTable(
		"should return a new target for complete flags",
		func(tf target.TargetFlags) {
			// prepare a persisted target that complete flags should override
			// through the normal read-and-merge path.
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
		"should merge target flags as selector paths",
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
		Entry(
			"--garden matching the persisted garden still starts at garden scope",
			target.NewTargetFlags("mygarden", "", "", "", false),
			target.NewTarget("mygarden", "", "", ""),
		),
		Entry(
			"--project matching the persisted project still starts at project scope",
			target.NewTargetFlags("", "myproject", "", "", false),
			target.NewTarget("mygarden", "myproject", "", ""),
		),
		Entry(
			"same --garden plus --shoot starts at garden scope",
			target.NewTargetFlags("mygarden", "", "", "newshoot", false),
			target.NewTarget("mygarden", "", "", "newshoot"),
		),
	)

	It("should clear persisted seed metadata when --project and --shoot select the project branch", func() {
		dummy := target.NewTarget("mygarden", "myproject", "myseed", "myshoot")
		Expect(provider.Write(dummy)).To(Succeed())

		tf := target.NewTargetFlags("", "myproject", "", "newshoot", false)
		dtp := target.NewTargetProvider(tmpFile.Name(), tf)

		readBack, err := dtp.Read()
		Expect(err).NotTo(HaveOccurred())
		expectEqualTargets(readBack, target.NewTarget("mygarden", "myproject", "", "newshoot"))
	})

	It("should start at seed scope when --seed matches the persisted seed", func() {
		dummy := target.NewTarget("mygarden", "", "myseed", "myshoot")
		Expect(provider.Write(dummy)).To(Succeed())

		tf := target.NewTargetFlags("", "", "myseed", "", false)
		dtp := target.NewTargetProvider(tmpFile.Name(), tf)

		readBack, err := dtp.Read()
		Expect(err).NotTo(HaveOccurred())
		expectEqualTargets(readBack, target.NewTarget("mygarden", "", "myseed", ""))
	})

	It("should keep current project and seed context for bare --shoot", func() {
		dummy := target.NewTarget("mygarden", "myproject", "myseed", "myshoot")
		Expect(provider.Write(dummy)).To(Succeed())

		tf := target.NewTargetFlags("", "", "", "newshoot", false)
		dtp := target.NewTargetProvider(tmpFile.Name(), tf)

		readBack, err := dtp.Read()
		Expect(err).NotTo(HaveOccurred())
		expectEqualTargets(readBack, target.NewTarget("mygarden", "myproject", "myseed", "newshoot"))
	})

	It("should keep the seed selector for bare --shoot when no project is targeted", func() {
		dummy := target.NewTarget("mygarden", "", "myseed", "myshoot")
		Expect(provider.Write(dummy)).To(Succeed())

		tf := target.NewTargetFlags("", "", "", "newshoot", false)
		dtp := target.NewTargetProvider(tmpFile.Name(), tf)

		readBack, err := dtp.Read()
		Expect(err).NotTo(HaveOccurred())
		expectEqualTargets(readBack, target.NewTarget("mygarden", "", "myseed", "newshoot"))
	})

	DescribeTable(
		"should clear inherited control-plane when target flags change scope",
		func(current target.Target, tf target.TargetFlags, expected target.Target) {
			Expect(provider.Write(current)).To(Succeed())

			dtp := target.NewTargetProvider(tmpFile.Name(), tf)

			readBack, err := dtp.Read()
			Expect(err).NotTo(HaveOccurred())
			expectEqualTargets(readBack, expected)
		},
		Entry(
			"garden changes",
			target.NewTarget("mygarden", "myproject", "", "myshoot").WithControlPlane(true),
			target.NewTargetFlags("newgarden", "", "", "", false),
			target.NewTarget("newgarden", "", "", ""),
		),
		Entry(
			"project changes",
			target.NewTarget("mygarden", "myproject", "", "myshoot").WithControlPlane(true),
			target.NewTargetFlags("", "newproject", "", "", false),
			target.NewTarget("mygarden", "newproject", "", ""),
		),
		Entry(
			"seed changes",
			target.NewTarget("mygarden", "", "myseed", "myshoot").WithControlPlane(true),
			target.NewTargetFlags("", "", "newseed", "", false),
			target.NewTarget("mygarden", "", "newseed", ""),
		),
		Entry(
			"shoot changes",
			target.NewTarget("mygarden", "myproject", "", "myshoot").WithControlPlane(true),
			target.NewTargetFlags("", "", "", "newshoot", false),
			target.NewTarget("mygarden", "myproject", "", "newshoot"),
		),
		Entry(
			"project seed shoot scope changes",
			target.NewTarget("mygarden", "myproject", "myseed", "myshoot").WithControlPlane(true),
			target.NewTargetFlags("", "newproject", "myseed", "myshoot", false),
			target.NewTarget("mygarden", "newproject", "myseed", "myshoot"),
		),
	)

	DescribeTable(
		"should clear inherited control-plane when target path flags are provided without --control-plane",
		func(current target.Target, tf target.TargetFlags, expected target.Target) {
			Expect(provider.Write(current)).To(Succeed())

			dtp := target.NewTargetProvider(tmpFile.Name(), tf)

			readBack, err := dtp.Read()
			Expect(err).NotTo(HaveOccurred())
			expectEqualTargets(readBack, expected)
		},
		Entry(
			"garden matches",
			target.NewTarget("mygarden", "myproject", "", "myshoot").WithControlPlane(true),
			target.NewTargetFlags("mygarden", "", "", "", false),
			target.NewTarget("mygarden", "", "", ""),
		),
		Entry(
			"project matches",
			target.NewTarget("mygarden", "myproject", "", "myshoot").WithControlPlane(true),
			target.NewTargetFlags("", "myproject", "", "", false),
			target.NewTarget("mygarden", "myproject", "", ""),
		),
		Entry(
			"seed matches",
			target.NewTarget("mygarden", "", "myseed", "myshoot").WithControlPlane(true),
			target.NewTargetFlags("", "", "myseed", "", false),
			target.NewTarget("mygarden", "", "myseed", ""),
		),
		Entry(
			"shoot matches",
			target.NewTarget("mygarden", "myproject", "", "myshoot").WithControlPlane(true),
			target.NewTargetFlags("", "", "", "myshoot", false),
			target.NewTarget("mygarden", "myproject", "", "myshoot"),
		),
		Entry(
			"project seed shoot scope matches",
			target.NewTarget("mygarden", "myproject", "myseed", "myshoot").WithControlPlane(true),
			target.NewTargetFlags("", "myproject", "myseed", "myshoot", false),
			target.NewTarget("mygarden", "myproject", "myseed", "myshoot"),
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
		Entry("seed and project without shoot", target.NewTargetFlags("", "newproject", "newseed", "", false)),
		Entry("control-plane without shoot", target.NewTargetFlags("", "newproject", "", "", true)),
	)

	It("should allow --project, --seed, and --shoot together", func() {
		dummy := target.NewTarget("mygarden", "myproject", "", "myshoot")
		Expect(provider.Write(dummy)).To(Succeed())

		tf := target.NewTargetFlags("", "newproject", "newseed", "newshoot", false)
		dtp := target.NewTargetProvider(tmpFile.Name(), tf)

		readBack, err := dtp.Read()
		Expect(err).NotTo(HaveOccurred())
		Expect(readBack.GardenName()).To(Equal("mygarden"))
		Expect(readBack.ProjectName()).To(Equal("newproject"))
		Expect(readBack.SeedName()).To(Equal("newseed"))
		Expect(readBack.ShootName()).To(Equal("newshoot"))
	})

	It("should retarget to the specified seed and drop project and shoot", func() {
		dummy := target.NewTarget("mygarden", "myproject", "actualseed", "myshoot")
		Expect(provider.Write(dummy)).To(Succeed())

		tf := target.NewTargetFlags("", "", "otherseed", "", false)
		dtp := target.NewTargetProvider(tmpFile.Name(), tf)

		readBack, err := dtp.Read()
		Expect(err).NotTo(HaveOccurred())
		Expect(readBack.GardenName()).To(Equal("mygarden"))
		Expect(readBack.SeedName()).To(Equal("otherseed"))
		Expect(readBack.ProjectName()).To(BeEmpty())
		Expect(readBack.ShootName()).To(BeEmpty())
	})

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
