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

package v1alpha1_test

import (
	"encoding/json"
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	configv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
)

const (
	name      = "test"
	namespace = "garden"
)

var _ = Describe("Defaults", func() {
	Describe("#SetDefaults_ManagedSeed", func() {
		var obj *ManagedSeed

		BeforeEach(func() {
			obj = &ManagedSeed{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
			}
		})

		It("should default seed template secret reference", func() {
			obj.Spec.SeedTemplate = &SeedTemplate{}

			SetDefaults_ManagedSeed(obj)

			Expect(obj).To(Equal(&ManagedSeed{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: ManagedSeedSpec{
					SeedTemplate: &SeedTemplate{
						Spec: gardencorev1beta1.SeedSpec{
							SecretRef: &corev1.SecretReference{
								Name:      fmt.Sprintf("seed-%s", name),
								Namespace: namespace,
							},
						},
					},
				},
			}))
		})

		It("should default seed template backup secret reference if backup is specified", func() {
			obj.Spec.SeedTemplate = &SeedTemplate{
				Spec: gardencorev1beta1.SeedSpec{
					Backup: &gardencorev1beta1.SeedBackup{},
				},
			}

			SetDefaults_ManagedSeed(obj)

			Expect(obj).To(Equal(&ManagedSeed{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: ManagedSeedSpec{
					SeedTemplate: &SeedTemplate{
						Spec: gardencorev1beta1.SeedSpec{
							Backup: &gardencorev1beta1.SeedBackup{
								SecretRef: corev1.SecretReference{
									Name:      fmt.Sprintf("backup-%s", name),
									Namespace: namespace,
								},
							},
							SecretRef: &corev1.SecretReference{
								Name:      fmt.Sprintf("seed-%s", name),
								Namespace: namespace,
							},
						},
					},
				},
			}))
		})

		It("should default gardenlet deployment and configuration", func() {
			obj.Spec.Gardenlet = &Gardenlet{}

			SetDefaults_ManagedSeed(obj)

			Expect(obj).To(Equal(&ManagedSeed{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: ManagedSeedSpec{
					Gardenlet: &Gardenlet{
						Deployment: &GardenletDeployment{},
						Config: &runtime.RawExtension{
							Object: &configv1alpha1.GardenletConfiguration{
								TypeMeta: metav1.TypeMeta{
									APIVersion: configv1alpha1.SchemeGroupVersion.String(),
									Kind:       "GardenletConfiguration",
								},
								SeedConfig: &configv1alpha1.SeedConfig{},
							},
						},
						Bootstrap:       bootstrapPtr(BootstrapToken),
						MergeWithParent: pointer.BoolPtr(true),
					},
				},
			}))
		})

		It("should default gardenlet deployment, configuration, and backup secret reference if backup is specified", func() {
			obj.Spec.Gardenlet = &Gardenlet{
				Config: &runtime.RawExtension{
					Raw: encode(&configv1alpha1.GardenletConfiguration{
						TypeMeta: metav1.TypeMeta{
							APIVersion: configv1alpha1.SchemeGroupVersion.String(),
							Kind:       "GardenletConfiguration",
						},
						SeedConfig: &configv1alpha1.SeedConfig{
							Seed: gardencorev1beta1.Seed{
								Spec: gardencorev1beta1.SeedSpec{
									Backup: &gardencorev1beta1.SeedBackup{},
								},
							},
						},
					}),
				},
			}

			SetDefaults_ManagedSeed(obj)

			Expect(obj).To(Equal(&ManagedSeed{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: ManagedSeedSpec{
					Gardenlet: &Gardenlet{
						Deployment: &GardenletDeployment{},
						Config: &runtime.RawExtension{
							Object: &configv1alpha1.GardenletConfiguration{
								TypeMeta: metav1.TypeMeta{
									APIVersion: configv1alpha1.SchemeGroupVersion.String(),
									Kind:       "GardenletConfiguration",
								},
								SeedConfig: &configv1alpha1.SeedConfig{
									Seed: gardencorev1beta1.Seed{
										Spec: gardencorev1beta1.SeedSpec{
											Backup: &gardencorev1beta1.SeedBackup{
												SecretRef: corev1.SecretReference{
													Name:      fmt.Sprintf("backup-%s", name),
													Namespace: namespace,
												},
											},
										},
									},
								},
							},
						},
						Bootstrap:       bootstrapPtr(BootstrapToken),
						MergeWithParent: pointer.BoolPtr(true),
					},
				},
			}))
		})
	})

	Describe("#SetDefaults_GardenletDeployment", func() {
		var obj *GardenletDeployment

		BeforeEach(func() {
			obj = &GardenletDeployment{}
		})

		It("should default replica count, revision history limit, image, and vpa", func() {
			SetDefaults_GardenletDeployment(obj)

			Expect(obj).To(Equal(&GardenletDeployment{
				ReplicaCount:         pointer.Int32Ptr(1),
				RevisionHistoryLimit: pointer.Int32Ptr(1),
				Image:                &Image{},
				VPA:                  pointer.BoolPtr(true),
			}))
		})
	})

	Describe("#SetDefaults_Image", func() {
		var obj *Image

		BeforeEach(func() {
			obj = &Image{}
		})

		It("should default pull policy to IfNotPresent", func() {
			SetDefaults_Image(obj)

			Expect(obj).To(Equal(&Image{
				PullPolicy: pullPolicyPtr(corev1.PullIfNotPresent),
			}))
		})

		It("should default pull policy to Always if tag is latest", func() {
			obj.Tag = pointer.StringPtr("latest")

			SetDefaults_Image(obj)

			Expect(obj).To(Equal(&Image{
				Tag:        pointer.StringPtr("latest"),
				PullPolicy: pullPolicyPtr(corev1.PullAlways),
			}))
		})
	})
})

func encode(obj runtime.Object) []byte {
	data, _ := json.Marshal(obj)
	return data
}

func pullPolicyPtr(v corev1.PullPolicy) *corev1.PullPolicy { return &v }

func bootstrapPtr(v Bootstrap) *Bootstrap { return &v }
