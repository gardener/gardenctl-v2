/*
SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package credvalidate_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	credvalidate "github.com/gardener/gardenctl-v2/pkg/provider/credvalidate"
)

var _ = Describe("STACKIT Validator", func() {
	var validator *credvalidate.STACKITValidator

	BeforeEach(func() {
		validator = credvalidate.NewSTACKITValidator(context.Background())
	})
	Describe("Secret Validation", func() {
		var secret, secretWithOpenstack *corev1.Secret

		BeforeEach(func() {
			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "stackit-secret",
					Namespace: "test-namespace",
				},
				Data: map[string][]byte{
					"project-id":          []byte("7971c1b2-7ae1-4094-9375-db08dee917d1"),
					"serviceaccount.json": []byte("{}"),
				},
			}
			secretWithOpenstack = secret.DeepCopy()
			secretWithOpenstack.Data["domainName"] = []byte("default")
			secretWithOpenstack.Data["tenantName"] = []byte("7971c1b2-7ae1-4094-9375-db08dee917d1")
			secretWithOpenstack.Data["username"] = []byte("myuser")
			secretWithOpenstack.Data["password"] = []byte("mypassword")
		})

		Context("Valid credentials", func() {
			It("should succeed with STACKIT credentials", func() {
				creds, err := validator.ValidateSecret(secret)
				Expect(err).NotTo(HaveOccurred())
				Expect(creds).To(HaveKeyWithValue("project-id", "7971c1b2-7ae1-4094-9375-db08dee917d1"))
				Expect(creds).To(HaveKeyWithValue("serviceaccount.json", "{}"))
				Expect(len(creds)).To(Equal(2))
			})

			It("should succeed with STACKIT andopenstack credentials", func() {
				creds, err := validator.ValidateSecret(secretWithOpenstack)
				Expect(err).NotTo(HaveOccurred())
				Expect(creds).To(HaveKeyWithValue("domainName", "default"))
				Expect(creds).To(HaveKeyWithValue("tenantName", "7971c1b2-7ae1-4094-9375-db08dee917d1"))
				Expect(creds).To(HaveKeyWithValue("username", "myuser"))
				Expect(creds).To(HaveKeyWithValue("password", "mypassword"))
				Expect(creds).To(HaveKeyWithValue("project-id", "7971c1b2-7ae1-4094-9375-db08dee917d1"))
				Expect(creds).To(HaveKeyWithValue("serviceaccount.json", "{}"))
				Expect(len(creds)).To(Equal(6))
			})

			It("should ignore extra top-level secret keys (Permissive mode)", func() {
				secret.Data["foo"] = []byte("bar")
				creds, err := validator.ValidateSecret(secretWithOpenstack)
				Expect(err).NotTo(HaveOccurred())
				Expect(creds).To(HaveKey("domainName"))
				Expect(creds).To(HaveKey("tenantName"))
				Expect(creds).To(HaveKey("username"))
				Expect(creds).To(HaveKey("password"))
				Expect(creds).To(HaveKey("project-id"))
				Expect(creds).To(HaveKey("serviceaccount.json"))
				Expect(creds).NotTo(HaveKey("foo"))
				Expect(len(creds)).To(Equal(6))
			})
		})

		Context("Missing fields", func() {
			DescribeTable("should fail when required fields are missing",
				func(modifySecret func(*corev1.Secret), expectedError string) {
					modifySecret(secret)
					_, err := validator.ValidateSecret(secret)
					Expect(err).To(MatchError(expectedError))
				},
				Entry("missing serviceaccount.json",
					func(s *corev1.Secret) { delete(s.Data, "serviceaccount.json") },
					"validation error in field \"serviceaccount.json\": required field is missing",
				),
				Entry("missing project-id",
					func(s *corev1.Secret) { delete(s.Data, "project-id") },
					"validation error in field \"project-id\": required field is missing",
				),
			)
		})

		Context("Empty fields", func() {
			DescribeTable("should fail when required fields are empty",
				func(modifySecret func(*corev1.Secret), expectedError string) {
					modifySecret(secret)
					_, err := validator.ValidateSecret(secret)
					Expect(err).To(MatchError(expectedError))
				},
				Entry("empty serviceaccount.json",
					func(s *corev1.Secret) { s.Data["serviceaccount.json"] = []byte("") },
					"validation error in field \"serviceaccount.json\": required field cannot be empty",
				),
				Entry("empty project-id",
					func(s *corev1.Secret) { s.Data["project-id"] = []byte("") },
					"validation error in field \"project-id\": required field cannot be empty",
				),
			)

			It("should fail when secret.Data is nil", func() {
				secret.Data = nil
				_, err := validator.ValidateSecret(secret)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("Invalid fields", func() {
			DescribeTable("should fail when required fields are invalid",
				func(modifySecret func(*corev1.Secret), expectedError string) {
					modifySecret(secret)
					_, err := validator.ValidateSecret(secret)
					Expect(err).To(MatchError(expectedError))
				},
				Entry("not json serviceaccount.json",
					func(s *corev1.Secret) { s.Data["serviceaccount.json"] = []byte("{{{") },
					"validation error in field \"serviceaccount.json\": no valid json invalid character '{' looking for beginning of object key string",
				),
				Entry("non uuid ad project-id",
					func(s *corev1.Secret) { s.Data["project-id"] = []byte("foobar") },
					"pattern mismatch in field \"project-id\": does not match any allowed patterns (actual: \"foobar\")",
				),
			)

			It("should fail when secret.Data is nil", func() {
				secret.Data = nil
				_, err := validator.ValidateSecret(secret)
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
