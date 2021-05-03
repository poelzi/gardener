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

package managedseed_test

import (
	"context"

	"github.com/gardener/gardener/pkg/apis/core"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/seedmanagement"
	. "github.com/gardener/gardener/pkg/registry/seedmanagement/managedseed"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
)

var _ = Describe("Strategy", func() {
	var (
		ctx      = context.TODO()
		strategy = Strategy{}
	)

	Describe("#PrepareForUpdate", func() {
		var oldManagedSeed, newManagedSeed *seedmanagement.ManagedSeed

		BeforeEach(func() {
			oldManagedSeed = &seedmanagement.ManagedSeed{}
			newManagedSeed = &seedmanagement.ManagedSeed{}
		})

		It("should increase the generation if the spec has changed", func() {
			newManagedSeed.Spec.Shoot = &seedmanagement.Shoot{Name: "foo"}

			strategy.PrepareForUpdate(ctx, newManagedSeed, oldManagedSeed)
			Expect(newManagedSeed.Generation).To(Equal(oldManagedSeed.Generation + 1))
		})

		It("should increase the generation if the deletion timestamp is set", func() {
			deletionTimestamp := metav1.Now()
			newManagedSeed.DeletionTimestamp = &deletionTimestamp

			strategy.PrepareForUpdate(ctx, newManagedSeed, oldManagedSeed)
			Expect(newManagedSeed.Generation).To(Equal(oldManagedSeed.Generation + 1))
		})

		It("should increase the generation if the operation annotation with value reconcile was added", func() {
			newManagedSeed.Annotations = map[string]string{
				v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile,
			}

			strategy.PrepareForUpdate(ctx, newManagedSeed, oldManagedSeed)
			Expect(newManagedSeed.Generation).To(Equal(oldManagedSeed.Generation + 1))
			Expect(newManagedSeed.Annotations).To(BeEmpty())
		})

		It("should not increase the generation if neither the spec has changed nor the deletion timestamp is set", func() {
			strategy.PrepareForUpdate(ctx, newManagedSeed, oldManagedSeed)
			Expect(newManagedSeed.Generation).To(Equal(oldManagedSeed.Generation))
		})
	})
})

var _ = Describe("ToSelectableFields", func() {
	It("should return correct fields", func() {
		result := ToSelectableFields(newManagedSeed("foo"))

		Expect(result).To(HaveLen(3))
		Expect(result.Has(seedmanagement.ManagedSeedShootName)).To(BeTrue())
		Expect(result.Get(seedmanagement.ManagedSeedShootName)).To(Equal("foo"))
	})
})

var _ = Describe("GetAttrs", func() {
	It("should return error when object is not ManagedSeed", func() {
		_, _, err := GetAttrs(&core.Seed{})
		Expect(err).To(HaveOccurred())
	})

	It("should return correct result", func() {
		ls, fs, err := GetAttrs(newManagedSeed("foo"))

		Expect(ls).To(HaveLen(1))
		Expect(ls.Get("foo")).To(Equal("bar"))
		Expect(fs.Get(seedmanagement.ManagedSeedShootName)).To(Equal("foo"))
		Expect(err).NotTo(HaveOccurred())
	})
})

var _ = Describe("ShootNameTriggerFunc", func() {
	It("should return spec.shoot.name", func() {
		actual := ShootNameTriggerFunc(newManagedSeed("foo"))
		Expect(actual).To(Equal("foo"))
	})
})

var _ = Describe("MatchManagedSeed", func() {
	It("should return correct predicate", func() {
		ls, _ := labels.Parse("app=test")
		fs := fields.OneTermEqualSelector(seedmanagement.ManagedSeedShootName, "foo")

		result := MatchManagedSeed(ls, fs)

		Expect(result.Label).To(Equal(ls))
		Expect(result.Field).To(Equal(fs))
		Expect(result.IndexFields).To(ConsistOf(seedmanagement.ManagedSeedShootName))
	})
})

func newManagedSeed(shootName string) *seedmanagement.ManagedSeed {
	return &seedmanagement.ManagedSeed{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test-namespace",
			Labels:    map[string]string{"foo": "bar"},
		},
		Spec: seedmanagement.ManagedSeedSpec{
			Shoot: &seedmanagement.Shoot{
				Name: shootName,
			},
		},
	}
}
