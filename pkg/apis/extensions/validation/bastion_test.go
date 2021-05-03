// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package validation_test

import (
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/apis/extensions/validation"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

var _ = Describe("Bastion validation tests", func() {
	var bastion *extensionsv1alpha1.Bastion

	BeforeEach(func() {
		bastion = &extensionsv1alpha1.Bastion{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-bastion",
				Namespace: "test-namespace",
			},
			Spec: extensionsv1alpha1.BastionSpec{
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					Type: "provider",
				},
				UserData: []byte("echo hello world"),
				Ingress: []extensionsv1alpha1.BastionIngressPolicy{
					{
						IPBlock: networkingv1.IPBlock{
							CIDR: "1.2.3.4/8",
						},
					},
				},
			},
		}
	})

	Describe("#ValidBastion", func() {
		It("should forbid empty Bastion resources", func() {
			errorList := ValidateBastion(&extensionsv1alpha1.Bastion{})

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("metadata.name"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("metadata.namespace"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.type"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.userData"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.ingress"),
			}))))
		})

		It("should allow valid Bastion resources", func() {
			errorList := ValidateBastion(bastion)

			Expect(errorList).To(BeEmpty())
		})
	})

	Describe("#ValidBastionUpdate", func() {
		It("should prevent updating anything if deletion timestamp is set", func() {
			now := metav1.Now()
			bastion.DeletionTimestamp = &now

			newBastion := prepareBastionForUpdate(bastion)
			newBastion.DeletionTimestamp = &now
			newBastion.Spec.Ingress[0].IPBlock.CIDR = "8.8.8.8/8"

			errorList := ValidateBastionUpdate(newBastion, bastion)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec"),
			}))))
		})

		It("should prevent updating the type or userData", func() {
			newBastion := prepareBastionForUpdate(bastion)
			newBastion.Spec.Type = "changed-type"
			newBastion.Spec.UserData = []byte("echo goodbye")

			errorList := ValidateBastionUpdate(newBastion, bastion)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.type"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.userData"),
			}))))
		})

		It("should allow updating the the ingress", func() {
			newBastion := prepareBastionForUpdate(bastion)
			newBastion.Spec.Ingress[0].IPBlock.CIDR = "8.8.8.8/8"

			errorList := ValidateBastionUpdate(newBastion, bastion)

			Expect(errorList).To(BeEmpty())
		})
	})
})

func prepareBastionForUpdate(obj *extensionsv1alpha1.Bastion) *extensionsv1alpha1.Bastion {
	newObj := obj.DeepCopy()
	newObj.ResourceVersion = "1"
	return newObj
}
