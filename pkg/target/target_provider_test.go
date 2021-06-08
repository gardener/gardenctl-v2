/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package target_test

import (
	"io/ioutil"
	"os"

	"github.com/gardener/gardenctl-v2/pkg/target"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Target Provider", func() {
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
		t := target.NewTarget("garden", "project", "", "shoot")

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
	})
})
