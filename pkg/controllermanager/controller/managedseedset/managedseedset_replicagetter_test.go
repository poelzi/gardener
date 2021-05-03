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

package managedseedset_test

import (
	"context"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	mockkubernetes "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/managedseedset"
	controllermanagerfeatures "github.com/gardener/gardener/pkg/controllermanager/features"
	"github.com/gardener/gardener/pkg/features"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("ReplicaGetter", func() {
	var (
		ctrl *gomock.Controller

		gardenClient *mockkubernetes.MockInterface
		c            *mockclient.MockClient
		r            *mockclient.MockReader

		replicaGetter ReplicaGetter

		ctx context.Context

		set          *seedmanagementv1alpha1.ManagedSeedSet
		shoots       []gardencorev1beta1.Shoot
		managedSeeds []seedmanagementv1alpha1.ManagedSeed
		seeds        []gardencorev1beta1.Seed

		cleanup func()
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		gardenClient = mockkubernetes.NewMockInterface(ctrl)
		c = mockclient.NewMockClient(ctrl)
		r = mockclient.NewMockReader(ctrl)

		gardenClient.EXPECT().Client().Return(c).AnyTimes()
		gardenClient.EXPECT().APIReader().Return(r).AnyTimes()

		replicaGetter = NewReplicaGetter(gardenClient, ReplicaFactoryFunc(NewReplica))

		ctx = context.TODO()

		set = &seedmanagementv1alpha1.ManagedSeedSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: seedmanagementv1alpha1.ManagedSeedSetSpec{
				Selector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"name": name,
					},
				},
			},
		}
		shoots = []gardencorev1beta1.Shoot{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name + "-0",
					Namespace: namespace,
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name + "-1",
					Namespace: namespace,
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name + "-2",
					Namespace: namespace,
				},
			},
		}
		managedSeeds = []seedmanagementv1alpha1.ManagedSeed{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name + "-0",
					Namespace: namespace,
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name + "-1",
					Namespace: namespace,
				},
			},
		}
		seeds = []gardencorev1beta1.Seed{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: name + "-0",
				},
			},
		}

		cleanup = test.WithFeatureGate(controllermanagerfeatures.FeatureGate, features.CachedRuntimeClients, true)
	})

	AfterEach(func() {
		cleanup()
		ctrl.Finish()
	})

	Describe("#GetReplicas", func() {
		It("should return all existing replicas", func() {
			selector, err := metav1.LabelSelectorAsSelector(&set.Spec.Selector)
			Expect(err).ToNot(HaveOccurred())

			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{}), client.InNamespace(set.Namespace), client.MatchingLabelsSelector{Selector: selector}).DoAndReturn(
				func(_ context.Context, shootList *gardencorev1beta1.ShootList, _ ...client.ListOption) error {
					shootList.Items = shoots
					return nil
				},
			)
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&seedmanagementv1alpha1.ManagedSeedList{}), client.InNamespace(set.Namespace), client.MatchingLabelsSelector{Selector: selector}).DoAndReturn(
				func(_ context.Context, msList *seedmanagementv1alpha1.ManagedSeedList, _ ...client.ListOption) error {
					msList.Items = managedSeeds
					return nil
				},
			)
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.SeedList{}), client.MatchingLabelsSelector{Selector: selector}).DoAndReturn(
				func(_ context.Context, seedList *gardencorev1beta1.SeedList, _ ...client.ListOption) error {
					seedList.Items = seeds
					return nil
				},
			)
			r.EXPECT().List(ctx, gomock.AssignableToTypeOf(&metav1.PartialObjectMetadataList{}), client.InNamespace(set.Namespace), client.MatchingLabelsSelector{Selector: selector}).DoAndReturn(
				func(_ context.Context, pomList *metav1.PartialObjectMetadataList, _ ...client.ListOption) error {
					var items []metav1.PartialObjectMetadata
					for _, shoot := range shoots {
						items = append(items, metav1.PartialObjectMetadata{ObjectMeta: shoot.ObjectMeta})
					}
					pomList.Items = items
					return nil
				},
			)
			r.EXPECT().List(ctx, gomock.AssignableToTypeOf(&metav1.PartialObjectMetadataList{}), client.MatchingFields{gardencore.ShootSeedName: seeds[0].Name}, client.Limit(1)).DoAndReturn(
				func(_ context.Context, pomList *metav1.PartialObjectMetadataList, _ ...client.ListOption) error {
					pomList.Items = []metav1.PartialObjectMetadata{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "foo",
								Namespace: "bar",
							},
						},
					}
					return nil
				},
			)

			result, err := replicaGetter.GetReplicas(ctx, set)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal([]Replica{
				NewReplica(set, &shoots[0], &managedSeeds[0], &seeds[0], true),
				NewReplica(set, &shoots[1], &managedSeeds[1], nil, false),
				NewReplica(set, &shoots[2], nil, nil, false),
			}))
		})
	})
})
