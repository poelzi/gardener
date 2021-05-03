// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package controllerregistration

import (
	"context"
	"errors"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/operation/common"
	gardenpkg "github.com/gardener/gardener/pkg/operation/garden"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	dnsv1alpha1 "github.com/gardener/external-dns-management/pkg/apis/dns/v1alpha1"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/pointer"
)

var _ = Describe("controllerRegistrationReconciler", func() {
	var (
		ctx       = context.TODO()
		nopLogger = logger.NewFieldLogger(logger.NewNopLogger(), "", "")

		seedName       = "seed"
		seedLabels     = map[string]string{"foo": "bar"}
		seedObjectMeta = metav1.ObjectMeta{
			Name:   seedName,
			Labels: seedLabels,
		}

		alwaysPolicy         = gardencorev1beta1.ControllerDeploymentPolicyAlways
		alwaysIfShootsPolicy = gardencorev1beta1.ControllerDeploymentPolicyAlwaysExceptNoShoots
		onDemandPolicy       = gardencorev1beta1.ControllerDeploymentPolicyOnDemand
		now                  = metav1.Now()

		type1  = "type1"
		type2  = "type2"
		type3  = "type3"
		type4  = "type4"
		type5  = "type5"
		type6  = "type6"
		type7  = "type7"
		type8  = "type8"
		type9  = "type9"
		type10 = "type10"
		type11 = "type11"
		type12 = "type12"

		backupBucket1 = &gardencorev1beta1.BackupBucket{
			ObjectMeta: metav1.ObjectMeta{
				Name: "bb1",
			},
			Spec: gardencorev1beta1.BackupBucketSpec{
				Provider: gardencorev1beta1.BackupBucketProvider{
					Type: type1,
				},
			},
		}
		backupBucket2 = &gardencorev1beta1.BackupBucket{
			ObjectMeta: metav1.ObjectMeta{
				Name: "bb2",
			},
			Spec: gardencorev1beta1.BackupBucketSpec{
				SeedName: &seedName,
				Provider: gardencorev1beta1.BackupBucketProvider{
					Type: type2,
				},
			},
		}
		backupBucket3 = &gardencorev1beta1.BackupBucket{
			ObjectMeta: metav1.ObjectMeta{
				Name: "bb3",
			},
			Spec: gardencorev1beta1.BackupBucketSpec{
				SeedName: &seedName,
				Provider: gardencorev1beta1.BackupBucketProvider{
					Type: type3,
				},
			},
		}
		backupBucketList = &gardencorev1beta1.BackupBucketList{
			Items: []gardencorev1beta1.BackupBucket{
				*backupBucket1,
				*backupBucket2,
				*backupBucket3,
			},
		}
		buckets = map[string]gardencorev1beta1.BackupBucket{
			backupBucket1.Name: *backupBucket1,
			backupBucket2.Name: *backupBucket2,
			backupBucket3.Name: *backupBucket3,
		}

		backupEntry1 = &gardencorev1beta1.BackupEntry{
			ObjectMeta: metav1.ObjectMeta{
				Name: "be1",
			},
			Spec: gardencorev1beta1.BackupEntrySpec{
				BucketName: backupBucket1.Name,
			},
		}
		backupEntry2 = &gardencorev1beta1.BackupEntry{
			ObjectMeta: metav1.ObjectMeta{
				Name: "be2",
			},
			Spec: gardencorev1beta1.BackupEntrySpec{
				SeedName:   &seedName,
				BucketName: backupBucket1.Name,
			},
		}
		backupEntry3 = &gardencorev1beta1.BackupEntry{
			ObjectMeta: metav1.ObjectMeta{
				Name: "be3",
			},
			Spec: gardencorev1beta1.BackupEntrySpec{
				SeedName:   &seedName,
				BucketName: backupBucket2.Name,
			},
		}
		backupEntryList = &gardencorev1beta1.BackupEntryList{
			Items: []gardencorev1beta1.BackupEntry{
				*backupEntry1,
				*backupEntry2,
				*backupEntry3,
			},
		}

		seedWithShootDNSEnabled = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				Name: seedName,
			},
			Spec: gardencorev1beta1.SeedSpec{
				Provider: gardencorev1beta1.SeedProvider{
					Type: type11,
				},
				Backup: &gardencorev1beta1.SeedBackup{
					Provider: type8,
				},
				Settings: &gardencorev1beta1.SeedSettings{
					ShootDNS: &gardencorev1beta1.SeedSettingShootDNS{
						Enabled: true,
					},
				},
			},
		}
		seedWithShootDNSDisabled = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				Name: seedName,
			},
			Spec: gardencorev1beta1.SeedSpec{
				Provider: gardencorev1beta1.SeedProvider{
					Type: type11,
				},
				Backup: &gardencorev1beta1.SeedBackup{
					Provider: type8,
				},
				Settings: &gardencorev1beta1.SeedSettings{
					ShootDNS: &gardencorev1beta1.SeedSettingShootDNS{
						Enabled: false,
					},
				},
			},
		}

		shoot1 = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name: "s1",
			},
			Spec: gardencorev1beta1.ShootSpec{
				Provider: gardencorev1beta1.Provider{
					Type: type1,
				},
			},
		}
		shoot2 = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name: "s2",
			},
			Spec: gardencorev1beta1.ShootSpec{
				SeedName: &seedName,
				Provider: gardencorev1beta1.Provider{
					Type: type2,
					Workers: []gardencorev1beta1.Worker{
						{
							Machine: gardencorev1beta1.Machine{
								Image: &gardencorev1beta1.ShootMachineImage{
									Name: type5,
								},
							},
						},
					},
				},
				Networking: gardencorev1beta1.Networking{
					Type: type3,
				},
				Extensions: []gardencorev1beta1.Extension{
					{Type: type4},
				},
			},
		}
		shoot3 = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name: "s3",
			},
			Spec: gardencorev1beta1.ShootSpec{
				SeedName: &seedName,
				Provider: gardencorev1beta1.Provider{
					Type: type6,
					Workers: []gardencorev1beta1.Worker{
						{
							CRI: &gardencorev1beta1.CRI{
								ContainerRuntimes: []gardencorev1beta1.ContainerRuntime{
									{Type: type12},
								},
							},
						},
					},
				},
				Networking: gardencorev1beta1.Networking{
					Type: type3,
				},
				DNS: &gardencorev1beta1.DNS{
					Providers: []gardencorev1beta1.DNSProvider{
						{Type: &type7},
					},
				},
			},
		}
		shootList = []gardencorev1beta1.Shoot{
			*shoot1,
			*shoot2,
			*shoot3,
		}

		internalDomain = &gardenpkg.Domain{
			Provider: type9,
		}

		controllerRegistration1 = &gardencorev1beta1.ControllerRegistration{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cr1",
			},
			Spec: gardencorev1beta1.ControllerRegistrationSpec{
				Resources: []gardencorev1beta1.ControllerResource{
					{
						Kind: extensionsv1alpha1.BackupBucketResource,
						Type: type1,
					},
					{
						Kind:            extensionsv1alpha1.ExtensionResource,
						GloballyEnabled: pointer.BoolPtr(true),
						Type:            type10,
					},
					{
						Kind:    extensionsv1alpha1.NetworkResource,
						Type:    type2,
						Primary: pointer.BoolPtr(false),
					},
				},
			},
		}
		controllerRegistration2 = &gardencorev1beta1.ControllerRegistration{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cr2",
			},
			Spec: gardencorev1beta1.ControllerRegistrationSpec{
				Resources: []gardencorev1beta1.ControllerResource{
					{
						Kind: extensionsv1alpha1.NetworkResource,
						Type: type2,
					},
					{
						Kind: extensionsv1alpha1.ContainerRuntimeResource,
						Type: type12,
					},
				},
				Deployment: &gardencorev1beta1.ControllerDeployment{
					Policy: &onDemandPolicy,
				},
			},
		}
		controllerRegistration3 = &gardencorev1beta1.ControllerRegistration{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cr3",
			},
			Spec: gardencorev1beta1.ControllerRegistrationSpec{
				Resources: []gardencorev1beta1.ControllerResource{
					{
						Kind: extensionsv1alpha1.ControlPlaneResource,
						Type: type3,
					},
					{
						Kind: extensionsv1alpha1.InfrastructureResource,
						Type: type3,
					},
					{
						Kind: extensionsv1alpha1.WorkerResource,
						Type: type3,
					},
				},
				Deployment: &gardencorev1beta1.ControllerDeployment{
					Policy: &onDemandPolicy,
				},
			},
		}
		controllerRegistration4 = &gardencorev1beta1.ControllerRegistration{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cr4",
			},
			Spec: gardencorev1beta1.ControllerRegistrationSpec{
				Deployment: &gardencorev1beta1.ControllerDeployment{
					Policy: &alwaysPolicy,
				},
			},
		}
		controllerRegistration5 = &gardencorev1beta1.ControllerRegistration{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cr5",
			},
			Spec: gardencorev1beta1.ControllerRegistrationSpec{
				Deployment: &gardencorev1beta1.ControllerDeployment{
					Policy: &alwaysPolicy,
					SeedSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"bar": "foo",
						},
					},
				},
			},
		}
		controllerRegistration6 = &gardencorev1beta1.ControllerRegistration{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cr6",
			},
		}
		controllerRegistration7 = &gardencorev1beta1.ControllerRegistration{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cr7",
			},
			Spec: gardencorev1beta1.ControllerRegistrationSpec{
				Deployment: &gardencorev1beta1.ControllerDeployment{
					Policy: &onDemandPolicy,
				},
			},
		}
		controllerRegistration8 = &gardencorev1beta1.ControllerRegistration{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cr8",
			},
			Spec: gardencorev1beta1.ControllerRegistrationSpec{
				Deployment: &gardencorev1beta1.ControllerDeployment{
					Policy: &alwaysIfShootsPolicy,
				},
			},
		}
		controllerRegistrationList = &gardencorev1beta1.ControllerRegistrationList{
			Items: []gardencorev1beta1.ControllerRegistration{
				*controllerRegistration1,
				*controllerRegistration2,
				*controllerRegistration3,
				*controllerRegistration4,
				*controllerRegistration5,
				*controllerRegistration6,
				*controllerRegistration7,
				*controllerRegistration8,
			},
		}
		controllerRegistrations = map[string]controllerRegistration{
			controllerRegistration1.Name: {obj: controllerRegistration1},
			controllerRegistration2.Name: {obj: controllerRegistration2},
			controllerRegistration3.Name: {obj: controllerRegistration3},
			controllerRegistration4.Name: {obj: controllerRegistration4, deployAlways: true},
			controllerRegistration5.Name: {obj: controllerRegistration5, deployAlways: true},
			controllerRegistration6.Name: {obj: controllerRegistration6},
			controllerRegistration7.Name: {obj: controllerRegistration7},
			controllerRegistration8.Name: {obj: controllerRegistration8, deployAlwaysExceptNoShoots: true},
		}

		controllerInstallation1 = &gardencorev1beta1.ControllerInstallation{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ci1",
			},
			Spec: gardencorev1beta1.ControllerInstallationSpec{
				SeedRef: corev1.ObjectReference{
					Name: "another-seed",
				},
				RegistrationRef: corev1.ObjectReference{
					Name: controllerRegistration1.Name,
				},
			},
		}
		controllerInstallation2 = &gardencorev1beta1.ControllerInstallation{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ci2",
			},
			Spec: gardencorev1beta1.ControllerInstallationSpec{
				SeedRef: corev1.ObjectReference{
					Name: seedName,
				},
				RegistrationRef: corev1.ObjectReference{
					Name: controllerRegistration2.Name,
				},
			},
		}
		controllerInstallation3 = &gardencorev1beta1.ControllerInstallation{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ci3",
			},
			Spec: gardencorev1beta1.ControllerInstallationSpec{
				SeedRef: corev1.ObjectReference{
					Name: seedName,
				},
				RegistrationRef: corev1.ObjectReference{
					Name: controllerRegistration3.Name,
				},
			},
		}
		controllerInstallation4 = &gardencorev1beta1.ControllerInstallation{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ci4",
			},
			Spec: gardencorev1beta1.ControllerInstallationSpec{
				SeedRef: corev1.ObjectReference{
					Name: seedName,
				},
				RegistrationRef: corev1.ObjectReference{
					Name: controllerRegistration4.Name,
				},
			},
		}
		controllerInstallation7 = &gardencorev1beta1.ControllerInstallation{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ci7",
			},
			Spec: gardencorev1beta1.ControllerInstallationSpec{
				SeedRef: corev1.ObjectReference{
					Name: seedName,
				},
				RegistrationRef: corev1.ObjectReference{
					Name: controllerRegistration7.Name,
				},
			},
			Status: gardencorev1beta1.ControllerInstallationStatus{
				Conditions: []gardencorev1beta1.Condition{
					{
						Type:   gardencorev1beta1.ControllerInstallationRequired,
						Status: gardencorev1beta1.ConditionTrue,
					},
				},
			},
		}
		controllerInstallationList = &gardencorev1beta1.ControllerInstallationList{
			Items: []gardencorev1beta1.ControllerInstallation{
				*controllerInstallation1,
				*controllerInstallation2,
				*controllerInstallation3,
				*controllerInstallation4,
				*controllerInstallation7,
			},
		}
	)

	Describe("#computeKindTypesForBackupBuckets", func() {
		It("should return empty results for empty input", func() {
			kindTypes, bs := computeKindTypesForBackupBuckets(&gardencorev1beta1.BackupBucketList{}, seedName)

			Expect(kindTypes.Len()).To(BeZero())
			Expect(bs).To(BeEmpty())
		})

		It("should correctly compute the result", func() {
			kindTypes, bs := computeKindTypesForBackupBuckets(backupBucketList, seedName)

			Expect(kindTypes).To(Equal(sets.NewString(
				extensionsv1alpha1.BackupBucketResource+"/"+backupBucket2.Spec.Provider.Type,
				extensionsv1alpha1.BackupBucketResource+"/"+backupBucket3.Spec.Provider.Type,
			)))
			Expect(bs).To(Equal(buckets))
		})
	})

	Describe("#computeKindTypesForBackupEntries", func() {
		It("should return empty results for empty input", func() {
			kindTypes := computeKindTypesForBackupEntries(nopLogger, &gardencorev1beta1.BackupEntryList{}, nil, seedName)

			Expect(kindTypes.Len()).To(BeZero())
		})

		It("should correctly compute the result", func() {
			kindTypes := computeKindTypesForBackupEntries(nopLogger, backupEntryList, buckets, seedName)

			Expect(kindTypes).To(Equal(sets.NewString(
				extensionsv1alpha1.BackupEntryResource+"/"+backupBucket1.Spec.Provider.Type,
				extensionsv1alpha1.BackupEntryResource+"/"+backupBucket2.Spec.Provider.Type,
			)))
		})
	})

	Describe("#computeKindTypesForShoots", func() {
		It("should correctly compute the result for a seed without DNS taint", func() {
			kindTypes := computeKindTypesForShoots(ctx, nopLogger, nil, shootList, seedWithShootDNSEnabled, controllerRegistrationList, internalDomain, nil)

			Expect(kindTypes).To(Equal(sets.NewString(
				// seedWithShootDNSEnabled types
				extensionsv1alpha1.BackupBucketResource+"/"+type8,
				extensionsv1alpha1.BackupEntryResource+"/"+type8,
				extensionsv1alpha1.ControlPlaneResource+"/"+type11,

				// shoot2 types
				extensionsv1alpha1.ControlPlaneResource+"/"+type2,
				extensionsv1alpha1.InfrastructureResource+"/"+type2,
				extensionsv1alpha1.WorkerResource+"/"+type2,
				extensionsv1alpha1.OperatingSystemConfigResource+"/"+type5,
				extensionsv1alpha1.NetworkResource+"/"+type3,
				extensionsv1alpha1.ExtensionResource+"/"+type4,

				// shoot3 types
				extensionsv1alpha1.ControlPlaneResource+"/"+type6,
				extensionsv1alpha1.InfrastructureResource+"/"+type6,
				extensionsv1alpha1.WorkerResource+"/"+type6,
				dnsv1alpha1.DNSProviderKind+"/"+type7,
				extensionsv1alpha1.ContainerRuntimeResource+"/"+type12,

				// internal domain + globally enabled extensions
				extensionsv1alpha1.ExtensionResource+"/"+type10,
				dnsv1alpha1.DNSProviderKind+"/"+type9,
			)))
		})

		It("should correctly compute the result for a seed with DNS taint", func() {
			kindTypes := computeKindTypesForShoots(ctx, nopLogger, nil, shootList, seedWithShootDNSDisabled, controllerRegistrationList, internalDomain, nil)

			Expect(kindTypes).To(Equal(sets.NewString(
				// seedWithShootDNSDisabled types
				extensionsv1alpha1.BackupBucketResource+"/"+type8,
				extensionsv1alpha1.BackupEntryResource+"/"+type8,
				extensionsv1alpha1.ControlPlaneResource+"/"+type11,

				// shoot2 types
				extensionsv1alpha1.ControlPlaneResource+"/"+type2,
				extensionsv1alpha1.InfrastructureResource+"/"+type2,
				extensionsv1alpha1.WorkerResource+"/"+type2,
				extensionsv1alpha1.OperatingSystemConfigResource+"/"+type5,
				extensionsv1alpha1.NetworkResource+"/"+type3,
				extensionsv1alpha1.ExtensionResource+"/"+type4,
				extensionsv1alpha1.ContainerRuntimeResource+"/"+type12,

				// shoot3 types
				extensionsv1alpha1.ControlPlaneResource+"/"+type6,
				extensionsv1alpha1.InfrastructureResource+"/"+type6,
				extensionsv1alpha1.WorkerResource+"/"+type6,

				// globally enabled extensions
				extensionsv1alpha1.ExtensionResource+"/"+type10,
			)))
		})

		It("should correctly compute types for shoot that has the Seed`s name as status not spec", func() {
			shootList := []gardencorev1beta1.Shoot{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "s4",
					},
					Spec: gardencorev1beta1.ShootSpec{
						SeedName: pointer.StringPtr("anotherSeed"),
						Provider: gardencorev1beta1.Provider{
							Type: type2,
							Workers: []gardencorev1beta1.Worker{
								{
									Machine: gardencorev1beta1.Machine{
										Image: &gardencorev1beta1.ShootMachineImage{
											Name: type5,
										},
									},
								},
							},
						},
						Networking: gardencorev1beta1.Networking{
							Type: type3,
						},
						Extensions: []gardencorev1beta1.Extension{
							{Type: type4},
						},
					},
					Status: gardencorev1beta1.ShootStatus{
						SeedName: &seedName,
					},
				},
			}

			kindTypes := computeKindTypesForShoots(ctx, nopLogger, nil, shootList, seedWithShootDNSDisabled, controllerRegistrationList, internalDomain, nil)

			Expect(kindTypes).To(Equal(sets.NewString(
				// seedWithShootDNSDisabled types
				extensionsv1alpha1.BackupBucketResource+"/"+type8,
				extensionsv1alpha1.BackupEntryResource+"/"+type8,
				extensionsv1alpha1.ControlPlaneResource+"/"+type11,

				// shoot4 types
				extensionsv1alpha1.ControlPlaneResource+"/"+type2,
				extensionsv1alpha1.InfrastructureResource+"/"+type2,
				extensionsv1alpha1.WorkerResource+"/"+type2,
				extensionsv1alpha1.OperatingSystemConfigResource+"/"+type5,
				extensionsv1alpha1.NetworkResource+"/"+type3,
				extensionsv1alpha1.ExtensionResource+"/"+type4,

				// globally enabled extensions
				extensionsv1alpha1.ExtensionResource+"/"+type10,
			)))
		})
	})

	Describe("#computeKindTypesForSeed", func() {
		var providerType = "fake-provider-type"

		It("should add the DNSProvider extension", func() {
			seed := &gardencorev1beta1.Seed{
				Spec: gardencorev1beta1.SeedSpec{
					DNS: gardencorev1beta1.SeedDNS{
						Provider: &gardencorev1beta1.SeedDNSProvider{
							Type: providerType,
						},
					},
				},
			}

			expected := sets.NewString(extensions.Id(dnsv1alpha1.DNSProviderKind, providerType))
			actual := computeKindTypesForSeed(seed)
			Expect(actual).To(Equal(expected))
		})

		It("should not add an extension if Seed has a deletion timestamp", func() {
			deletionTimestamp := metav1.Now()
			seed := &gardencorev1beta1.Seed{
				ObjectMeta: metav1.ObjectMeta{
					DeletionTimestamp: &deletionTimestamp,
				},
				Spec: gardencorev1beta1.SeedSpec{
					DNS: gardencorev1beta1.SeedDNS{
						Provider: &gardencorev1beta1.SeedDNSProvider{
							Type: providerType,
						},
					},
				},
			}

			expected := sets.NewString()
			actual := computeKindTypesForSeed(seed)
			Expect(actual).To(Equal(expected))
		})

		It("should not add an extension if no provider configured", func() {
			seed := &gardencorev1beta1.Seed{
				Spec: gardencorev1beta1.SeedSpec{},
			}

			expected := sets.NewString()
			actual := computeKindTypesForSeed(seed)
			Expect(actual).To(Equal(expected))
		})
	})

	Describe("#computeControllerRegistrationMaps", func() {
		It("should correctly compute the result", func() {
			registrations := computeControllerRegistrationMaps(controllerRegistrationList)

			Expect(registrations).To(Equal(controllerRegistrations))
		})
	})

	Describe("#computeWantedControllerRegistrationNames", func() {
		It("should correctly compute the result w/o error", func() {
			wantedKindTypeCombinations := sets.NewString(
				extensionsv1alpha1.NetworkResource+"/"+type2,
				extensionsv1alpha1.ControlPlaneResource+"/"+type3,
			)

			names, err := computeWantedControllerRegistrationNames(wantedKindTypeCombinations, controllerInstallationList, controllerRegistrations, len(shootList), seedObjectMeta)

			Expect(names).To(Equal(sets.NewString(controllerRegistration1.Name, controllerRegistration2.Name, controllerRegistration3.Name, controllerRegistration4.Name, controllerRegistration7.Name, controllerRegistration8.Name)))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not consider 'always-deploy-if-shoots' registrations when seed has no shoots", func() {
			wantedKindTypeCombinations := sets.NewString()

			names, err := computeWantedControllerRegistrationNames(wantedKindTypeCombinations, controllerInstallationList, controllerRegistrations, 0, seedObjectMeta)

			Expect(names).To(Equal(sets.NewString(controllerRegistration4.Name, controllerRegistration7.Name)))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should consider 'always-deploy' registrations when seed has no shoots but no deletion timestamp", func() {
			wantedKindTypeCombinations := sets.NewString()

			names, err := computeWantedControllerRegistrationNames(wantedKindTypeCombinations, controllerInstallationList, controllerRegistrations, 0, seedObjectMeta)

			Expect(names).To(Equal(sets.NewString(controllerRegistration4.Name, controllerRegistration7.Name)))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not consider 'always-deploy' registrations when seed has no shoots and deletion timestamp", func() {
			seedObjectMetaCopy := seedObjectMeta.DeepCopy()
			time := metav1.Time{}
			seedObjectMetaCopy.DeletionTimestamp = &time
			wantedKindTypeCombinations := sets.NewString()

			names, err := computeWantedControllerRegistrationNames(wantedKindTypeCombinations, controllerInstallationList, controllerRegistrations, 0, *seedObjectMetaCopy)

			Expect(names).To(Equal(sets.NewString(controllerRegistration7.Name)))
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("#computeRegistrationNameToInstallationNameMap", func() {
		It("should correctly compute the result w/o error", func() {
			regNameToInstallationName, err := computeRegistrationNameToInstallationNameMap(controllerInstallationList, controllerRegistrations, seedName)

			Expect(err).NotTo(HaveOccurred())
			Expect(regNameToInstallationName).To(Equal(map[string]string{
				controllerRegistration2.Name: controllerInstallation2.Name,
				controllerRegistration3.Name: controllerInstallation3.Name,
				controllerRegistration4.Name: controllerInstallation4.Name,
				controllerRegistration7.Name: controllerInstallation7.Name,
			}))
		})

		It("should fail to compute the result and return error", func() {
			regNameToInstallationName, err := computeRegistrationNameToInstallationNameMap(controllerInstallationList, map[string]controllerRegistration{}, seedName)

			Expect(err).To(HaveOccurred())
			Expect(regNameToInstallationName).To(BeNil())
		})
	})

	Context("deployment and deletion", func() {
		var (
			ctrl      *gomock.Controller
			k8sClient *mockclient.MockClient
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())

			k8sClient = mockclient.NewMockClient(ctrl)
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		Describe("#deployNeededInstallations", func() {
			It("should return an error", func() {
				var (
					wantedControllerRegistrations      = sets.NewString(controllerRegistration2.Name)
					registrationNameToInstallationName = map[string]string{
						controllerRegistration1.Name: controllerInstallation1.Name,
						controllerRegistration2.Name: controllerInstallation2.Name,
						controllerRegistration3.Name: controllerInstallation3.Name,
					}
					fakeErr = errors.New("err")
				)

				k8sClient.EXPECT().Get(ctx, kutil.Key(controllerInstallation2.Name), gomock.AssignableToTypeOf(&gardencorev1beta1.ControllerInstallation{})).Return(fakeErr)

				err := deployNeededInstallations(ctx, nopLogger, k8sClient, seedWithShootDNSEnabled, wantedControllerRegistrations, controllerRegistrations, registrationNameToInstallationName)

				Expect(err).To(Equal(fakeErr))
			})

			It("should correctly deploy needed controller installations", func() {
				var (
					wantedControllerRegistrations      = sets.NewString(controllerRegistration2.Name, controllerRegistration3.Name, controllerRegistration4.Name)
					registrationNameToInstallationName = map[string]string{
						controllerRegistration1.Name: controllerInstallation1.Name,
						controllerRegistration2.Name: controllerInstallation2.Name,
						controllerRegistration3.Name: controllerInstallation3.Name,
						controllerRegistration4.Name: "",
					}
				)

				installation2 := controllerInstallation2.DeepCopy()
				installation2.Labels = map[string]string{
					common.RegistrationSpecHash: "b24405c0d68a538e",
					common.SeedSpecHash:         "a5e0943b25bc6cab",
				}

				installation3 := controllerInstallation3.DeepCopy()
				installation3.Labels = map[string]string{
					common.RegistrationSpecHash: "b24405c0d68a538e",
					common.SeedSpecHash:         "a5e0943b25bc6cab",
				}

				k8sClient.EXPECT().Get(ctx, kutil.Key(controllerInstallation2.Name), gomock.AssignableToTypeOf(&gardencorev1beta1.ControllerInstallation{}))
				k8sClient.EXPECT().Update(ctx, installation2)

				k8sClient.EXPECT().Get(ctx, kutil.Key(controllerInstallation3.Name), gomock.AssignableToTypeOf(&gardencorev1beta1.ControllerInstallation{}))
				k8sClient.EXPECT().Update(ctx, installation3)

				k8sClient.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.ControllerInstallation{}))

				err := deployNeededInstallations(ctx, nopLogger, k8sClient, seedWithShootDNSEnabled, wantedControllerRegistrations, controllerRegistrations, registrationNameToInstallationName)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should not skip the controller registration that is after one in deletion", func() {
				registration1 := controllerRegistration1.DeepCopy()
				registration1.DeletionTimestamp = &now
				var (
					wantedControllerRegistrations      = sets.NewString(registration1.Name, controllerRegistration2.Name)
					registrationNameToInstallationName = map[string]string{
						registration1.Name:           controllerInstallation1.Name,
						controllerRegistration2.Name: controllerInstallation2.Name,
					}
					registrations = map[string]controllerRegistration{
						registration1.Name:           {obj: registration1, deployAlways: false},
						controllerRegistration2.Name: {obj: controllerRegistration2, deployAlways: false},
					}
				)

				installation2 := controllerInstallation2.DeepCopy()
				installation2.Labels = map[string]string{
					common.RegistrationSpecHash: "b24405c0d68a538e",
					common.SeedSpecHash:         "a5e0943b25bc6cab",
				}

				k8sClient.EXPECT().Get(ctx, kutil.Key(controllerInstallation2.Name), gomock.AssignableToTypeOf(&gardencorev1beta1.ControllerInstallation{}))
				k8sClient.EXPECT().Update(ctx, installation2)

				err := deployNeededInstallations(ctx, nopLogger, k8sClient, seedWithShootDNSEnabled, wantedControllerRegistrations, registrations, registrationNameToInstallationName)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should not create or update controller installation for controller registration in deletion", func() {
				registration1 := controllerRegistration1.DeepCopy()
				registration1.DeletionTimestamp = &now
				registration2 := controllerRegistration2.DeepCopy()
				registration2.DeletionTimestamp = &now
				var (
					wantedControllerRegistrations      = sets.NewString(registration1.Name, registration2.Name)
					registrationNameToInstallationName = map[string]string{
						registration1.Name: controllerInstallation1.Name,
						registration2.Name: "",
					}
					registrations = map[string]controllerRegistration{
						registration1.Name: {obj: registration1, deployAlways: false},
						registration2.Name: {obj: registration2, deployAlways: false},
					}
				)

				err := deployNeededInstallations(ctx, nopLogger, k8sClient, seedWithShootDNSEnabled, wantedControllerRegistrations, registrations, registrationNameToInstallationName)

				Expect(err).NotTo(HaveOccurred())
			})
		})

		Describe("#deleteUnneededInstallations", func() {
			It("should return an error", func() {
				var (
					wantedControllerRegistrationNames  = sets.NewString()
					registrationNameToInstallationName = map[string]string{"": controllerInstallation1.Name}
					fakeErr                            = errors.New("err")
				)

				k8sClient.EXPECT().Delete(ctx, &gardencorev1beta1.ControllerInstallation{ObjectMeta: metav1.ObjectMeta{Name: controllerInstallation1.Name}}).Return(fakeErr)

				err := deleteUnneededInstallations(ctx, nopLogger, k8sClient, wantedControllerRegistrationNames, registrationNameToInstallationName)

				Expect(err).To(Equal(fakeErr))
			})

			It("should correctly delete unneeded controller installations", func() {
				var (
					wantedControllerRegistrationNames  = sets.NewString(controllerRegistration2.Name)
					registrationNameToInstallationName = map[string]string{
						controllerRegistration1.Name: controllerInstallation1.Name,
						controllerRegistration2.Name: controllerInstallation2.Name,
						controllerRegistration3.Name: controllerInstallation3.Name,
					}
				)

				k8sClient.EXPECT().Delete(ctx, &gardencorev1beta1.ControllerInstallation{ObjectMeta: metav1.ObjectMeta{Name: controllerInstallation1.Name}})
				k8sClient.EXPECT().Delete(ctx, &gardencorev1beta1.ControllerInstallation{ObjectMeta: metav1.ObjectMeta{Name: controllerInstallation3.Name}})

				err := deleteUnneededInstallations(ctx, nopLogger, k8sClient, wantedControllerRegistrationNames, registrationNameToInstallationName)

				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})
