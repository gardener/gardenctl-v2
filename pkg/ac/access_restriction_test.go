/*
SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package ac_test

import (
	"bytes"
	"context"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardenctl-v2/pkg/ac"
)

var _ = Describe("AccessRestriction", func() {
	Describe("Checking access restrictions", func() {
		var accessRestrictions []ac.AccessRestriction
		var shoot *gardencorev1beta1.Shoot

		BeforeEach(func() {
			accessRestrictions = []ac.AccessRestriction{
				{
					Key:      "a",
					NotifyIf: true,
					Msg:      "A",
					Options: []ac.AccessRestrictionOption{
						{
							Key:      "a1",
							NotifyIf: true,
							Msg:      "A1",
						},
						{
							Key:      "a2",
							NotifyIf: false,
							Msg:      "A2",
						},
					},
				},
				{
					Key:      "b",
					NotifyIf: false,
					Msg:      "B",
					Options: []ac.AccessRestrictionOption{
						{
							Key:      "b1",
							NotifyIf: false,
							Msg:      "B1",
						},
						{
							Key:      "b2",
							NotifyIf: true,
							Msg:      "B2",
						},
					},
				},
			}
			shoot = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"a1": "true",
						"a2": "false",
						"b1": "false",
						"b2": "true",
					},
				},
				Spec: gardencorev1beta1.ShootSpec{
					SeedSelector: &gardencorev1beta1.SeedSelector{
						LabelSelector: metav1.LabelSelector{
							MatchLabels: map[string]string{
								"a": "true",
								"b": "false",
							},
						},
					},
				},
			}
		})

		It("should match all access restrictions and options", func() {
			messages := ac.CheckAccessRestrictions(accessRestrictions, shoot)
			Expect(messages).To(HaveLen(2))
			Expect(messages).To(Equal([]*ac.AccessRestrictionMessage{
				{Header: "A", Items: []string{"A1", "A2"}},
				{Header: "B", Items: []string{"B1", "B2"}},
			}))
		})

		It("should match no access restriction", func() {
			matchLabels := shoot.Spec.SeedSelector.MatchLabels
			matchLabels["a"] = "false"
			matchLabels["b"] = "true"
			messages := ac.CheckAccessRestrictions(accessRestrictions, shoot)
			Expect(messages).To(HaveLen(0))
		})

		It("should match all access restriction but no options", func() {
			annotations := shoot.Annotations
			annotations["a1"] = "0"
			annotations["a2"] = "1"
			annotations["b1"] = "TRUE"
			annotations["b2"] = "Faux"
			messages := ac.CheckAccessRestrictions(accessRestrictions, shoot)
			Expect(messages).To(HaveLen(2))
			Expect(messages).To(Equal([]*ac.AccessRestrictionMessage{
				{Header: "A"},
				{Header: "B"},
			}))
		})

		It("should not return messages if seed selector is nil", func() {
			shoot.Spec.SeedSelector = nil
			messages := ac.CheckAccessRestrictions(accessRestrictions, shoot)
			Expect(messages).To(HaveLen(0))
		})
	})

	Describe("Handling an access restriction message", func() {
		It("should add and get a handler function from the context", func() {

			message := &ac.AccessRestrictionMessage{}
			ctx := ac.WithAccessRestrictionHandler(context.Background(), func(msg *ac.AccessRestrictionMessage) {
				Expect(msg).To(BeIdenticalTo(message))
			})
			ac.AccessRestrictionHandlerFromContext(ctx)(message)
		})

		It("should return nil if no handler function has been added", func() {
			ctx := context.Background()
			Expect(ac.AccessRestrictionHandlerFromContext(ctx)).To(BeNil())
		})
	})

	Describe("Rendering an access restriction message", func() {
		var out *bytes.Buffer

		BeforeEach(func() {
			out = &bytes.Buffer{}
		})

		It("should return nil if no handler function has been added", func() {
			message := &ac.AccessRestrictionMessage{Header: "A", Items: []string{"A1", "A2"}}
			message.Render(out)
			Expect(out.String()).To(Equal(`┌─ Access Restriction ─────────────────────────────────────────────────────────┐
│ A                                                                            │
│ * A1                                                                         │
│ * A2                                                                         │
└──────────────────────────────────────────────────────────────────────────────┘
`))
		})
	})
})
