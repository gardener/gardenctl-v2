/*
SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors
SPDX-License-Identifier: Apache-2.0
*/

package info_test

import (
	"fmt"

	internalfake "github.com/gardener/gardenctl-v2/internal/fake"
	"github.com/gardener/gardenctl-v2/internal/util"
	cmdinfo "github.com/gardener/gardenctl-v2/pkg/cmd/info"
	"github.com/gardener/gardenctl-v2/pkg/config"
	"github.com/gardener/gardenctl-v2/pkg/target"
	targetmocks "github.com/gardener/gardenctl-v2/pkg/target/mocks"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Info Command", func() {
	const (
		gardenName = "mygarden"
	)

	var (
		streams              util.IOStreams
		out                  *util.SafeBytesBuffer
		factory              *internalfake.Factory
		targetProvider       *internalfake.TargetProvider
		ctrl                 *gomock.Controller
		clientProvider       *targetmocks.MockClientProvider
		cfg                  *config.Config
		gardenClient         client.Client
		testShootUnscheduled *gardencorev1beta1.Shoot
		testShootAws         *gardencorev1beta1.Shoot
		testShootGCP         *gardencorev1beta1.Shoot
	)

	BeforeEach(func() {
		cfg = &config.Config{
			LinkKubeconfig: pointer.Bool(false),
			Gardens: []config.Garden{{
				Name: gardenName,
			}},
		}

		seedAws := "aws"
		seedGcp := "gcp"
		brokenShoot := "brokenShoot"

		testShootUnscheduled = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{Name: brokenShoot},
			Spec: gardencorev1beta1.ShootSpec{
				SeedName: nil,
			},
		}

		testShootAws = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{Name: seedAws},
			Spec: gardencorev1beta1.ShootSpec{
				SeedName: &seedAws,
			},
		}

		testShootGCP = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{Name: seedGcp},
			Spec: gardencorev1beta1.ShootSpec{
				SeedName: &seedGcp,
			},
		}

		streams, _, out, _ = util.NewTestIOStreams()

		ctrl = gomock.NewController(GinkgoT())

		clientProvider = targetmocks.NewMockClientProvider(ctrl)
		targetProvider = internalfake.NewFakeTargetProvider(target.NewTarget(gardenName, "", "", ""))

		factory = internalfake.NewFakeFactory(cfg, nil, clientProvider, targetProvider)
	})

	JustBeforeEach(func() {
		clientConfig, err := cfg.ClientConfig(gardenName)
		Expect(err).ToNot(HaveOccurred())
		clientProvider.EXPECT().FromClientConfig(gomock.Eq(clientConfig)).Return(gardenClient, nil).AnyTimes()
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("RunE", func() {
		BeforeEach(func() {
			gardenClient = internalfake.NewClientWithObjects(
				testShootUnscheduled,
				testShootAws,
				testShootGCP,
			)
		})

		It("should print info information", func() {
			o := cmdinfo.NewInfoOptions(streams)
			cmd := cmdinfo.NewCmdInfo(factory, o)

			Expect(cmd.RunE(cmd, nil)).To(Succeed())
			Expect(out.String()).To(Equal(fmt.Sprintf("Garden: mygarden\nSeed                                Total                    Active                    Hibernated                    Allocatable                    Capacity\n----                                -----                    ------                    ----------                    -----------                    --------\n----                                -----                    ------                    ----------                    -----------                    --------\nTOTAL                               3                        2                         0                             -                              -\nUnscheduled                         1\nUnscheduled List                    brokenShoot\n\n")))
		})
	})
})
