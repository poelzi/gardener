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
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	seedmanagementv1alpha1constants "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1/constants"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/managedseedset"
	configv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	operationshoot "github.com/gardener/gardener/pkg/operation/shoot"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
)

const (
	ordinal     = 42
	replicaName = name + "-42"
)

var _ = Describe("Replica", func() {
	var (
		ctrl *gomock.Controller
		c    *mockclient.MockClient
		ctx  context.Context

		set *seedmanagementv1alpha1.ManagedSeedSet

		now = metav1.Now()
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
		ctx = context.TODO()

		set = &seedmanagementv1alpha1.ManagedSeedSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: seedmanagementv1alpha1.ManagedSeedSetSpec{
				Template: seedmanagementv1alpha1.ManagedSeedTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"foo": "bar",
						},
					},
					Spec: seedmanagementv1alpha1.ManagedSeedSpec{
						Gardenlet: &seedmanagementv1alpha1.Gardenlet{
							Config: runtime.RawExtension{
								Object: &configv1alpha1.GardenletConfiguration{
									SeedConfig: &configv1alpha1.SeedConfig{
										SeedTemplate: gardencorev1beta1.SeedTemplate{
											Spec: gardencorev1beta1.SeedSpec{
												DNS: gardencorev1beta1.SeedDNS{
													IngressDomain: pointer.StringPtr("ingress.replica-name.example.com"),
												},
											},
										},
									},
								},
							},
						},
					},
				},
				ShootTemplate: gardencorev1beta1.ShootTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"foo": "bar",
						},
					},
					Spec: gardencorev1beta1.ShootSpec{
						DNS: &gardencorev1beta1.DNS{
							Domain: pointer.StringPtr("replica-name.example.com"),
						},
					},
				},
			},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	var (
		shoot = func(
			deletionTimestamp *metav1.Time,
			lastOperationType gardencorev1beta1.LastOperationType,
			lastOperationState gardencorev1beta1.LastOperationState,
			shs operationshoot.Status,
			protected bool,
		) *gardencorev1beta1.Shoot {
			labels := make(map[string]string)
			if shs != "" {
				labels[v1beta1constants.ShootStatus] = string(shs)
			}
			annotations := make(map[string]string)
			if protected {
				annotations[seedmanagementv1alpha1constants.AnnotationProtectFromDeletion] = "true"
			}
			var lastOperation *gardencorev1beta1.LastOperation
			if lastOperationType != "" && lastOperationState != "" {
				lastOperation = &gardencorev1beta1.LastOperation{
					Type:  lastOperationType,
					State: lastOperationState,
				}
			}
			return &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:              replicaName,
					Namespace:         namespace,
					DeletionTimestamp: deletionTimestamp,
					Labels:            labels,
					Annotations:       annotations,
				},
				Status: gardencorev1beta1.ShootStatus{
					LastOperation: lastOperation,
				},
			}
		}
		managedSeed = func(deletionTimestamp *metav1.Time, seedRegistered, protected bool) *seedmanagementv1alpha1.ManagedSeed {
			annotations := make(map[string]string)
			if protected {
				annotations[seedmanagementv1alpha1constants.AnnotationProtectFromDeletion] = "true"
			}
			var conditions []gardencorev1beta1.Condition
			if seedRegistered {
				conditions = append(conditions, gardencorev1beta1.Condition{
					Type:   seedmanagementv1alpha1.ManagedSeedSeedRegistered,
					Status: gardencorev1beta1.ConditionTrue,
				})
			}
			return &seedmanagementv1alpha1.ManagedSeed{
				ObjectMeta: metav1.ObjectMeta{
					Name:              replicaName,
					Namespace:         namespace,
					DeletionTimestamp: deletionTimestamp,
					Annotations:       annotations,
				},
				Status: seedmanagementv1alpha1.ManagedSeedStatus{
					Conditions: conditions,
				},
			}
		}
		seed = func(deletionTimestamp *metav1.Time, gardenletReady, bootstrapped bool, backupBucketsReady bool) *gardencorev1beta1.Seed {
			var conditions []gardencorev1beta1.Condition
			if gardenletReady {
				conditions = append(conditions, gardencorev1beta1.Condition{
					Type:   gardencorev1beta1.SeedGardenletReady,
					Status: gardencorev1beta1.ConditionTrue,
				})
			}
			if bootstrapped {
				conditions = append(conditions, gardencorev1beta1.Condition{
					Type:   gardencorev1beta1.SeedBootstrapped,
					Status: gardencorev1beta1.ConditionTrue,
				})
			}
			if backupBucketsReady {
				conditions = append(conditions, gardencorev1beta1.Condition{
					Type:   gardencorev1beta1.SeedBackupBucketsReady,
					Status: gardencorev1beta1.ConditionTrue,
				})
			}
			return &gardencorev1beta1.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Name:              replicaName,
					DeletionTimestamp: deletionTimestamp,
				},
				Status: gardencorev1beta1.SeedStatus{
					Conditions: conditions,
				},
			}
		}
	)

	DescribeTable("#GetName",
		func(shoot *gardencorev1beta1.Shoot, name string) {
			replica := NewReplica(set, shoot, nil, nil, false)
			Expect(replica.GetName()).To(Equal(name))
		},
		Entry("should return an empty string", nil, ""),
		Entry("should return the shoot name", shoot(nil, "", "", "", false), replicaName),
	)

	DescribeTable("#GetFullName",
		func(shoot *gardencorev1beta1.Shoot, fullName string) {
			replica := NewReplica(set, shoot, nil, nil, false)
			Expect(replica.GetFullName()).To(Equal(fullName))
		},
		Entry("should return an empty string", nil, ""),
		Entry("should return the shoot full name", shoot(nil, "", "", "", false), namespace+"/"+replicaName),
	)

	DescribeTable("#GetOrdinal",
		func(shoot *gardencorev1beta1.Shoot, ordinal int) {
			replica := NewReplica(set, shoot, nil, nil, false)
			Expect(replica.GetOrdinal()).To(Equal(ordinal))
		},
		Entry("should return -1", nil, -1),
		Entry("should return the ordinal from the shoot name", shoot(nil, "", "", "", false), ordinal),
	)

	DescribeTable("#GetStatus",
		func(shoot *gardencorev1beta1.Shoot, managedSeed *seedmanagementv1alpha1.ManagedSeed, status ReplicaStatus) {
			replica := NewReplica(set, shoot, managedSeed, nil, false)
			Expect(replica.GetStatus()).To(Equal(status))
		},
		Entry("should return Unknown", nil, nil, StatusUnknown),
		Entry("should return ShootReconciled",
			shoot(nil, gardencorev1beta1.LastOperationTypeReconcile, gardencorev1beta1.LastOperationStateSucceeded, "", false), nil, StatusShootReconciled),
		Entry("should return ShootReconcileFailed",
			shoot(nil, gardencorev1beta1.LastOperationTypeReconcile, gardencorev1beta1.LastOperationStateFailed, "", false), nil, StatusShootReconcileFailed),
		Entry("should return ShootDeleteFailed",
			shoot(&now, gardencorev1beta1.LastOperationTypeDelete, gardencorev1beta1.LastOperationStateFailed, "", false), nil, StatusShootDeleteFailed),
		Entry("should return ShootReconciling",
			shoot(nil, "", "", "", false), nil, StatusShootReconciling),
		Entry("should return ShootDeleting",
			shoot(&now, "", "", "", false), nil, StatusShootDeleting),
		Entry("should return ManagedSeedRegistered",
			shoot(nil, "", "", "", false), managedSeed(nil, true, false), StatusManagedSeedRegistered),
		Entry("should return ManagedSeedPreparing",
			shoot(nil, "", "", "", false), managedSeed(nil, false, false), StatusManagedSeedPreparing),
		Entry("should return ManagedSeedDeleting",
			shoot(nil, "", "", "", false), managedSeed(&now, false, false), StatusManagedSeedDeleting),
	)

	DescribeTable("#IsSeedReady",
		func(seed *gardencorev1beta1.Seed, seedReady bool) {
			replica := NewReplica(set, shoot(nil, "", "", "", false),
				managedSeed(nil, true, false), seed, false)
			Expect(replica.IsSeedReady()).To(Equal(seedReady))
		},
		Entry("should return false", seed(nil, false, false, false), false),
		Entry("should return false", seed(nil, true, false, false), false),
		Entry("should return true", seed(nil, true, true, false), true),
		Entry("should return true", seed(nil, true, true, true), true),
		Entry("should return false", seed(&now, true, true, true), false),
	)

	DescribeTable("#GetShootHealthStatus",
		func(shoot *gardencorev1beta1.Shoot, shs operationshoot.Status) {
			replica := NewReplica(set, shoot, nil, nil, false)
			Expect(replica.GetShootHealthStatus()).To(Equal(shs))
		},
		Entry("should return unhealthy",
			nil, operationshoot.StatusUnhealthy),
		Entry("should return progressing",
			shoot(nil, "", "", "", false), operationshoot.StatusProgressing),
		Entry("should return healthy",
			shoot(nil, "", "", operationshoot.StatusHealthy, false), operationshoot.StatusHealthy),
		Entry("should return progressing",
			shoot(nil, "", "", operationshoot.StatusProgressing, false), operationshoot.StatusProgressing),
		Entry("should return unhealthy",
			shoot(nil, "", "", operationshoot.StatusUnhealthy, false), operationshoot.StatusUnhealthy),
	)

	DescribeTable("#IsDeletable",
		func(shoot *gardencorev1beta1.Shoot, managedSeed *seedmanagementv1alpha1.ManagedSeed, hasScheduledShoots, deletable bool) {
			replica := NewReplica(set, shoot, managedSeed, nil, hasScheduledShoots)
			Expect(replica.IsDeletable()).To(Equal(deletable))
		},
		Entry("should return true",
			nil, nil, false, true),
		Entry("should return true",
			shoot(nil, "", "", "", false), nil, false, true),
		Entry("should return true",
			shoot(nil, "", "", "", false), managedSeed(nil, false, false), false, true),
		Entry("should return false",
			shoot(nil, "", "", "", true), nil, false, false),
		Entry("should return false",
			shoot(nil, "", "", "", false), managedSeed(nil, false, true), false, false),
		Entry("should return false",
			shoot(nil, "", "", "", false), managedSeed(nil, false, false), true, false),
	)

	Describe("#CreateShoot", func() {
		It("should create the shoot", func() {
			c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).DoAndReturn(
				func(_ context.Context, s *gardencorev1beta1.Shoot) error {
					Expect(s).To(Equal(&gardencorev1beta1.Shoot{
						ObjectMeta: metav1.ObjectMeta{
							Name:      replicaName,
							Namespace: namespace,
							Labels: map[string]string{
								"foo": "bar",
							},
							OwnerReferences: []metav1.OwnerReference{
								*metav1.NewControllerRef(set, seedmanagementv1alpha1.SchemeGroupVersion.WithKind("ManagedSeedSet")),
							},
						},
						Spec: gardencorev1beta1.ShootSpec{
							DNS: &gardencorev1beta1.DNS{
								Domain: pointer.StringPtr(replicaName + ".example.com"),
							},
						},
					}))
					return nil
				},
			)

			replica := NewReplica(set, nil, nil, nil, false)
			err := replica.CreateShoot(ctx, c, ordinal)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("#CreateManagedSeed", func() {
		It("should create the managed seed", func() {
			shoot := shoot(nil, "", "", "", false)
			c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&seedmanagementv1alpha1.ManagedSeed{})).DoAndReturn(
				func(_ context.Context, ms *seedmanagementv1alpha1.ManagedSeed) error {
					Expect(ms).To(Equal(&seedmanagementv1alpha1.ManagedSeed{
						ObjectMeta: metav1.ObjectMeta{
							Name:      replicaName,
							Namespace: namespace,
							Labels: map[string]string{
								"foo": "bar",
							},
							OwnerReferences: []metav1.OwnerReference{
								*metav1.NewControllerRef(set, seedmanagementv1alpha1.SchemeGroupVersion.WithKind("ManagedSeedSet")),
							},
						},
						Spec: seedmanagementv1alpha1.ManagedSeedSpec{
							Shoot: &seedmanagementv1alpha1.Shoot{
								Name: replicaName,
							},
							Gardenlet: &seedmanagementv1alpha1.Gardenlet{
								Config: runtime.RawExtension{
									Object: &configv1alpha1.GardenletConfiguration{
										SeedConfig: &configv1alpha1.SeedConfig{
											SeedTemplate: gardencorev1beta1.SeedTemplate{
												Spec: gardencorev1beta1.SeedSpec{
													DNS: gardencorev1beta1.SeedDNS{
														IngressDomain: pointer.StringPtr("ingress." + replicaName + ".example.com"),
													},
												},
											},
										},
									},
								},
							},
						},
					}))
					return nil
				},
			)

			replica := NewReplica(set, shoot, nil, nil, false)
			err := replica.CreateManagedSeed(ctx, c)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("#DeleteShoot", func() {
		It("should clean the retries, confirm the deletion, and delete the shoot", func() {
			shoot := shoot(nil, "", "", "", false)
			c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, s *gardencorev1beta1.Shoot, _ client.Patch) error {
					Expect(s.Annotations).To(HaveKeyWithValue(gutil.ConfirmationDeletion, "true"))
					*shoot = *s
					return nil
				},
			)
			c.EXPECT().Delete(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).DoAndReturn(
				func(_ context.Context, s *gardencorev1beta1.Shoot) error {
					Expect(s).To(Equal(shoot))
					return nil
				},
			)

			replica := NewReplica(set, shoot, nil, nil, false)
			err := replica.DeleteShoot(ctx, c)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("#DeleteManagedSeed", func() {
		It("should delete the managed seed", func() {
			managedSeed := managedSeed(nil, false, false)
			c.EXPECT().Delete(ctx, gomock.AssignableToTypeOf(&seedmanagementv1alpha1.ManagedSeed{})).DoAndReturn(
				func(_ context.Context, ms *seedmanagementv1alpha1.ManagedSeed) error {
					Expect(ms).To(Equal(managedSeed))
					return nil
				},
			)

			replica := NewReplica(set, nil, managedSeed, nil, false)
			err := replica.DeleteManagedSeed(ctx, c)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("#RetryShoot", func() {
		It("should set the operation to retry and the retries to 1", func() {
			shoot := shoot(nil, "", "", "", false)
			c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, s *gardencorev1beta1.Shoot, _ client.Patch) error {
					Expect(s.Annotations).To(HaveKeyWithValue(v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationRetry))
					return nil
				},
			)

			replica := NewReplica(set, shoot, nil, nil, false)
			err := replica.RetryShoot(ctx, c)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
