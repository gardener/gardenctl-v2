/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/spf13/pflag"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardenctl-v2/internal/util"
	utilmocks "github.com/gardener/gardenctl-v2/internal/util/mocks"
	"github.com/gardener/gardenctl-v2/pkg/config"
	targetmocks "github.com/gardener/gardenctl-v2/pkg/target/mocks"
)

const (
	gardenIdentity1 = "fooGarden"
	gardenIdentity2 = "barGarden"
	gardenIdentity3 = "bazGarden"
	gardenContext1  = "my-context"
	kubeconfig      = "not/a/file"
)

var (
	gardenHomeDir string
	cfg           *config.Config
	ctrl          *gomock.Controller
	factory       *utilmocks.MockFactory
	manager       *targetmocks.MockManager
	streams       util.IOStreams
	out           *util.SafeBytesBuffer
	errOut        *util.SafeBytesBuffer
	patterns      []string
)

func TestCommand(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Config Command Test Suite")
}

var _ = BeforeSuite(func() {
	var err error

	gardenHomeDir, err = os.MkdirTemp("", "gctlv2-*")
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	Expect(os.RemoveAll(gardenHomeDir)).To(Succeed())
})

var _ = BeforeEach(func() {
	patterns = []string{
		"^shoot--(?P<project>.+)--(?P<shoot>.+)$",
		"^namespace:(?P<namespace>[^/]+)$",
	}
	cfg = &config.Config{
		Filename:       filepath.Join(gardenHomeDir, "gardenctl-testconfig.yaml"),
		LinkKubeconfig: pointer.Bool(false),
		Gardens: []config.Garden{
			{
				Name:       gardenIdentity1,
				Kubeconfig: kubeconfig,
				Context:    gardenContext1,
			},
			{
				Name:       gardenIdentity2,
				Kubeconfig: kubeconfig,
				Patterns:   patterns,
			}},
	}

	streams, _, out, errOut = util.NewTestIOStreams()
	ctrl = gomock.NewController(GinkgoT())
	factory = utilmocks.NewMockFactory(ctrl)
	manager = targetmocks.NewMockManager(ctrl)
})

var _ = AfterEach(func() {
	ctrl.Finish()
})

func assertAllFlagNames(flags *pflag.FlagSet, expNames ...string) {
	var actNames []string

	flags.VisitAll(func(flag *pflag.Flag) {
		actNames = append(actNames, flag.Name)
	})

	ExpectWithOffset(1, actNames).To(Equal(expNames))
}

func assertGardenNames(cfg *config.Config, names ...string) {
	ExpectWithOffset(1, cfg.GardenNames()).To(Equal(names))
}

func assertGarden(cfg *config.Config, garden *config.Garden) {
	g, err := cfg.Garden(garden.Name)
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	ExpectWithOffset(1, g).To(BeEquivalentTo(garden))
}

func assertConfigHasBeenSaved(cfg *config.Config) {
	c, err := config.LoadFromFile(cfg.Filename)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	ExpectWithOffset(1, c).To(BeEquivalentTo(cfg))
}
