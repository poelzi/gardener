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

package kubernetes_test

import (
	"context"
	"errors"

	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	seedmanagementfake "github.com/gardener/gardener/pkg/client/seedmanagement/clientset/versioned/fake"
	. "github.com/gardener/gardener/pkg/utils/kubernetes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/testing"
)

const (
	name      = "foo"
	namespace = "garden"
)

var _ = Describe("managedseed", func() {
	Describe("#GetManagedSeed", func() {
		var (
			seedManagementClient *seedmanagementfake.Clientset
			managedSeed          *seedmanagementv1alpha1.ManagedSeed
		)

		BeforeEach(func() {
			seedManagementClient = &seedmanagementfake.Clientset{}

			managedSeed = &seedmanagementv1alpha1.ManagedSeed{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: seedmanagementv1alpha1.ManagedSeedSpec{
					Shoot: &seedmanagementv1alpha1.Shoot{
						Name: name,
					},
				},
			}
		})

		It("should return the ManagedSeed for the given shoot namespace and name, if it exists", func() {
			seedManagementClient.AddReactor("list", "managedseeds", func(action testing.Action) (bool, runtime.Object, error) {
				return true, &seedmanagementv1alpha1.ManagedSeedList{Items: []seedmanagementv1alpha1.ManagedSeed{*managedSeed}}, nil
			})

			result, err := GetManagedSeed(context.TODO(), seedManagementClient, namespace, name)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(managedSeed))
		})

		It("should return nil if a ManagedSeed does not exist", func() {
			seedManagementClient.AddReactor("list", "managedseeds", func(action testing.Action) (bool, runtime.Object, error) {
				return true, &seedmanagementv1alpha1.ManagedSeedList{Items: []seedmanagementv1alpha1.ManagedSeed{}}, nil
			})

			result, err := GetManagedSeed(context.TODO(), seedManagementClient, namespace, name)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeNil())
		})

		It("should fail if more than one ManagedSeeds exist", func() {
			seedManagementClient.AddReactor("list", "managedseeds", func(action testing.Action) (bool, runtime.Object, error) {
				return true, &seedmanagementv1alpha1.ManagedSeedList{Items: []seedmanagementv1alpha1.ManagedSeed{*managedSeed, *managedSeed}}, nil
			})

			_, err := GetManagedSeed(context.TODO(), seedManagementClient, namespace, name)
			Expect(err).To(HaveOccurred())
		})

		It("should fail if listing the ManagedSeeds fails", func() {
			seedManagementClient.AddReactor("list", "managedseeds", func(action testing.Action) (bool, runtime.Object, error) {
				return true, nil, errors.New("error")
			})

			_, err := GetManagedSeed(context.TODO(), seedManagementClient, namespace, name)
			Expect(err).To(HaveOccurred())
		})
	})
})
