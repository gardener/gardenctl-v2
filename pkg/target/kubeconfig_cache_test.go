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

var _ = Describe("Kubeconfig Cache", func() {
	var (
		tmpDir string
		cache  target.KubeconfigCache
		t      target.Target
	)

	BeforeEach(func() {
		var err error

		tmpDir, err = ioutil.TempDir("", "gardenertarget*")
		Expect(err).NotTo(HaveOccurred())

		cache = target.NewFilesystemKubeconfigCache(tmpDir)
		t = target.NewTarget("g", "p", "s", "shoot", false)
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	It("should error when no file exists", func() {
		_, err := cache.Read(t)
		Expect(err).To(HaveOccurred())
	})

	It("should require a garden to be targeted", func() {
		badTarget := target.NewTarget("", "", "", "", false)

		data := []byte("hello world")
		Expect(cache.Write(badTarget, data)).NotTo(Succeed())
	})

	It("should be able to write files to disk", func() {
		data := []byte("hello world")
		Expect(cache.Write(t, data)).To(Succeed())

		readBack, err := cache.Read(t)
		Expect(err).NotTo(HaveOccurred())
		Expect(readBack).To(Equal(data))
	})

	It("should separate different targets", func() {
		targetA := target.NewTarget("garden", "project", "", "shootA", false)
		targetB := target.NewTarget("garden", "project", "", "shootB", false)
		dataA := []byte("hello A")
		dataB := []byte("hello B")

		Expect(cache.Write(targetA, dataA)).To(Succeed())
		Expect(cache.Write(targetB, dataB)).To(Succeed())

		readBack, err := cache.Read(targetA)
		Expect(err).NotTo(HaveOccurred())
		Expect(readBack).To(Equal(dataA))

		readBack, err = cache.Read(targetB)
		Expect(err).NotTo(HaveOccurred())
		Expect(readBack).To(Equal(dataB))
	})

	It("should separate based on the path", func() {
		targetA := target.NewTarget("garden", "project", "", "shoot", false)
		targetB := target.NewTarget("garden", "", "seed", "shoot", false)
		dataA := []byte("hello A")
		dataB := []byte("hello B")

		Expect(cache.Write(targetA, dataA)).To(Succeed())
		Expect(cache.Write(targetB, dataB)).To(Succeed())

		readBack, err := cache.Read(targetA)
		Expect(err).NotTo(HaveOccurred())
		Expect(readBack).To(Equal(dataA))

		readBack, err = cache.Read(targetB)
		Expect(err).NotTo(HaveOccurred())
		Expect(readBack).To(Equal(dataB))
	})
})
