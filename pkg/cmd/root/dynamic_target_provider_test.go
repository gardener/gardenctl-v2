/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package root_test

import (
	"io/ioutil"
	"os"

	"github.com/gardener/gardenctl-v2/pkg/cmd/root"
	"github.com/gardener/gardenctl-v2/pkg/target"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

func expectEqualTargets(actual, expected target.Target) {
	ExpectWithOffset(1, actual.GardenName()).To(Equal(expected.GardenName()))
	ExpectWithOffset(1, actual.ProjectName()).To(Equal(expected.ProjectName()))
	ExpectWithOffset(1, actual.SeedName()).To(Equal(expected.SeedName()))
	ExpectWithOffset(1, actual.ShootName()).To(Equal(expected.ShootName()))
}

var _ = Describe("Dynamic Target Provider", func() {
	var (
		tmpFile  *os.File
		provider target.TargetProvider
	)

	BeforeEach(func() {
		var err error

		tmpFile, err = ioutil.TempFile("", "gardenertarget*")
		Expect(err).NotTo(HaveOccurred())

		provider = target.NewFilesystemTargetProvider(tmpFile.Name())
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

		dtp := root.DynamicTargetProvider{
			TargetFile: tmpFile.Name(),
		}

		readBack, err := dtp.Read()
		Expect(err).NotTo(HaveOccurred())
		expectEqualTargets(readBack, dummy)
	})

	DescribeTable(
		"should return a new target for complete flags",
		func(dtp root.DynamicTargetProvider) {
			// prepare target that should never be read
			dummy := target.NewTarget("mygarden", "myproject", "", "myshoot")
			Expect(provider.Write(dummy)).To(Succeed())

			dtp.TargetFile = tmpFile.Name()

			readBack, err := dtp.Read()
			Expect(err).NotTo(HaveOccurred())
			Expect(readBack.GardenName()).To(Equal(dtp.GardenNameFlag))
			Expect(readBack.ProjectName()).To(Equal(dtp.ProjectNameFlag))
			Expect(readBack.SeedName()).To(Equal(dtp.SeedNameFlag))
			Expect(readBack.ShootName()).To(Equal(dtp.ShootNameFlag))
		},
		Entry("just garden", root.DynamicTargetProvider{GardenNameFlag: "newgarden"}),
		Entry("garden->project", root.DynamicTargetProvider{GardenNameFlag: "newgarden", ProjectNameFlag: "newproject"}),
		Entry("garden->seed", root.DynamicTargetProvider{GardenNameFlag: "newgarden", SeedNameFlag: "newseed"}),
		Entry("garden->project->shoot", root.DynamicTargetProvider{GardenNameFlag: "newgarden", ProjectNameFlag: "newproject", ShootNameFlag: "newshoot"}),
	)

	DescribeTable(
		"should augment existing target with CLI flags",
		func(dtp root.DynamicTargetProvider, expected target.Target) {
			dummy := target.NewTarget("mygarden", "myproject", "", "myshoot")
			Expect(provider.Write(dummy)).To(Succeed())

			dtp.TargetFile = tmpFile.Name()

			readBack, err := dtp.Read()
			Expect(err).NotTo(HaveOccurred())
			expectEqualTargets(readBack, expected)
		},
		Entry(
			"target garden cluster",
			root.DynamicTargetProvider{GardenNameFlag: "newgarden"},
			target.NewTarget("newgarden", "", "", ""),
		),
		Entry(
			"target project",
			root.DynamicTargetProvider{ProjectNameFlag: "newproject"},
			target.NewTarget("mygarden", "newproject", "", ""),
		),
		Entry(
			"target seed",
			root.DynamicTargetProvider{SeedNameFlag: "newseed"},
			target.NewTarget("mygarden", "", "newseed", ""),
		),
		Entry(
			"target shoot",
			root.DynamicTargetProvider{ShootNameFlag: "newshoot"},
			target.NewTarget("mygarden", "myproject", "", "newshoot"),
		),
		Entry(
			"target shoot in a different project",
			root.DynamicTargetProvider{ProjectNameFlag: "newproject", ShootNameFlag: "newshoot"},
			target.NewTarget("mygarden", "newproject", "", "newshoot"),
		),
		Entry(
			"target shoot in a seed",
			root.DynamicTargetProvider{SeedNameFlag: "newseed", ShootNameFlag: "newshoot"},
			target.NewTarget("mygarden", "", "newseed", "newshoot"),
		),
		Entry(
			"complete re-target",
			root.DynamicTargetProvider{GardenNameFlag: "newgarden", SeedNameFlag: "newseed", ShootNameFlag: "newshoot"},
			target.NewTarget("newgarden", "", "newseed", "newshoot"),
		),
	)

	DescribeTable(
		"should not allow syntactically wrong targets",
		func(dtp root.DynamicTargetProvider) {
			dummy := target.NewTarget("mygarden", "myproject", "", "myshoot")
			Expect(provider.Write(dummy)).To(Succeed())

			dtp.TargetFile = tmpFile.Name()

			readBack, err := dtp.Read()
			Expect(readBack).To(BeNil())
			Expect(err).To(HaveOccurred())
		},
		Entry("seed and project", root.DynamicTargetProvider{ProjectNameFlag: "newproject", SeedNameFlag: "newseed"}),
	)

	It("should allow writes if no flags are given", func() {
		// prepare target
		dummy := target.NewTarget("mygarden", "myproject", "", "myshoot")

		dtp := root.DynamicTargetProvider{TargetFile: tmpFile.Name()}
		Expect(dtp.Write(dummy)).To(Succeed())

		readBack, err := provider.Read()
		Expect(err).NotTo(HaveOccurred())
		expectEqualTargets(readBack, dummy)
	})

	It("should forbid writes if flags are given", func() {
		dtp := root.DynamicTargetProvider{
			TargetFile:     tmpFile.Name(),
			GardenNameFlag: "oops-a-flag",
		}
		Expect(dtp.Write(target.NewTarget("g", "p", "", "sh"))).NotTo(Succeed())
	})
})
