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

package botanist_test

import (
	"context"
	"fmt"
	"time"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	mockkubernetes "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	extensionpkg "github.com/gardener/gardener/pkg/operation/botanist/component/extensions/extension"
	mockextension "github.com/gardener/gardener/pkg/operation/botanist/component/extensions/extension/mock"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Extensions", func() {
	var (
		ctrl                  *gomock.Controller
		extension             *mockextension.MockInterface
		gardenClientInterface *mockkubernetes.MockInterface
		gardenClient          *mockclient.MockClient
		botanist              *Botanist

		ctx        = context.TODO()
		fakeErr    = fmt.Errorf("fake")
		shootState = &gardencorev1alpha1.ShootState{}
		namespace  = "shoot--name--space"
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		extension = mockextension.NewMockInterface(ctrl)
		gardenClientInterface = mockkubernetes.NewMockInterface(ctrl)
		gardenClient = mockclient.NewMockClient(ctrl)
		botanist = &Botanist{Operation: &operation.Operation{
			K8sGardenClient: gardenClientInterface,
			Shoot: &shootpkg.Shoot{
				Components: &shootpkg.Components{
					Extensions: &shootpkg.Extensions{
						Extension: extension,
					},
				},
				Info:          &gardencorev1beta1.Shoot{},
				SeedNamespace: namespace,
			},
			ShootState: shootState,
		}}

		gardenClientInterface.EXPECT().Client().Return(gardenClient).AnyTimes()
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DefaultExtension", func() {
		var (
			extensionKind  = extensionsv1alpha1.ExtensionResource
			providerConfig = runtime.RawExtension{
				Raw: []byte("key: value"),
			}

			fooExtensionType         = "foo"
			fooReconciliationTimeout = metav1.Duration{Duration: 5 * time.Minute}
			fooRegistration          = gardencorev1beta1.ControllerRegistration{
				Spec: gardencorev1beta1.ControllerRegistrationSpec{
					Resources: []gardencorev1beta1.ControllerResource{
						{
							Kind:             extensionKind,
							Type:             fooExtensionType,
							ReconcileTimeout: &fooReconciliationTimeout,
						},
					},
				},
			}
			fooExtension = gardencorev1beta1.Extension{
				Type:           fooExtensionType,
				ProviderConfig: &providerConfig,
			}

			barExtensionType = "bar"
			barRegistration  = gardencorev1beta1.ControllerRegistration{
				Spec: gardencorev1beta1.ControllerRegistrationSpec{
					Resources: []gardencorev1beta1.ControllerResource{
						{
							Kind:            extensionKind,
							Type:            barExtensionType,
							GloballyEnabled: pointer.BoolPtr(true),
						},
					},
				},
			}
			barExtension = gardencorev1beta1.Extension{
				Type:           barExtensionType,
				ProviderConfig: &providerConfig,
			}
			barExtensionDisabled = gardencorev1beta1.Extension{
				Type:           barExtensionType,
				ProviderConfig: &providerConfig,
				Disabled:       pointer.BoolPtr(true),
			}
		)

		It("should return the error because listing failed", func() {
			gardenClient.EXPECT().List(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.ControllerRegistrationList{})).Return(fakeErr)

			ext, err := botanist.DefaultExtension(ctx, nil)
			Expect(ext).To(BeNil())
			Expect(err).To(MatchError(fakeErr))
		})

		DescribeTable("#DefaultExtension",
			func(registrations []gardencorev1beta1.ControllerRegistration, extensions []gardencorev1beta1.Extension, conditionMatcher gomegatypes.GomegaMatcher) {
				botanist.Shoot.Info.Spec.Extensions = extensions
				gardenClient.EXPECT().List(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.ControllerRegistrationList{})).DoAndReturn(func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
					(&gardencorev1beta1.ControllerRegistrationList{Items: registrations}).DeepCopyInto(list.(*gardencorev1beta1.ControllerRegistrationList))
					return nil
				})

				ext, err := botanist.DefaultExtension(ctx, nil)
				Expect(err).To(BeNil())
				Expect(ext.Extensions()).To(conditionMatcher)
			},

			Entry(
				"No extensions",
				nil,
				nil,
				BeEmpty(),
			),
			Entry(
				"Extension w/o registration",
				nil,
				[]gardencorev1beta1.Extension{{Type: fooExtensionType}},
				BeEmpty(),
			),
			Entry(
				"Extensions w/ registration",
				[]gardencorev1beta1.ControllerRegistration{fooRegistration},
				[]gardencorev1beta1.Extension{fooExtension},
				HaveKeyWithValue(
					Equal(fooExtensionType),
					MatchAllFields(
						Fields{
							"Extension": MatchFields(IgnoreExtras, Fields{
								"Spec": MatchFields(IgnoreExtras, Fields{
									"DefaultSpec": MatchAllFields(Fields{
										"Type":           Equal(fooExtensionType),
										"ProviderConfig": PointTo(Equal(providerConfig)),
									}),
								}),
							}),
							"Timeout": Equal(fooReconciliationTimeout.Duration),
						},
					),
				),
			),
			Entry(
				"Registration w/o extension",
				[]gardencorev1beta1.ControllerRegistration{fooRegistration},
				nil,
				BeEmpty(),
			),
			Entry(
				"Globally enabled extension registration, w/o extension",
				[]gardencorev1beta1.ControllerRegistration{barRegistration},
				nil,
				HaveKeyWithValue(
					Equal(barExtensionType),
					MatchAllFields(
						Fields{
							"Extension": MatchFields(IgnoreExtras, Fields{
								"Spec": MatchAllFields(Fields{
									"DefaultSpec": MatchAllFields(Fields{
										"Type":           Equal(barExtensionType),
										"ProviderConfig": BeNil(),
									}),
								}),
							}),
							"Timeout": Equal(extensionpkg.DefaultTimeout),
						},
					),
				),
			),
			Entry(
				"Globally enabled extension registration but explicitly disabled",
				[]gardencorev1beta1.ControllerRegistration{barRegistration},
				[]gardencorev1beta1.Extension{barExtensionDisabled},
				BeEmpty(),
			),
			Entry(
				"Multiple registration but a globally one is explicitly disabled",
				[]gardencorev1beta1.ControllerRegistration{fooRegistration, barRegistration},
				[]gardencorev1beta1.Extension{fooExtension, barExtensionDisabled},
				SatisfyAll(
					HaveLen(1),
					HaveKeyWithValue(
						Equal(fooExtensionType),
						MatchAllFields(
							Fields{
								"Extension": MatchFields(IgnoreExtras, Fields{
									"Spec": MatchFields(IgnoreExtras, Fields{
										"DefaultSpec": MatchAllFields(Fields{
											"Type":           Equal(fooExtensionType),
											"ProviderConfig": PointTo(Equal(providerConfig)),
										}),
									}),
								}),
								"Timeout": Equal(fooReconciliationTimeout.Duration),
							},
						),
					),
				),
			),
			Entry(
				"Multiple registrations, w/ one extension",
				[]gardencorev1beta1.ControllerRegistration{
					fooRegistration,
					barRegistration,
					{
						Spec: gardencorev1beta1.ControllerRegistrationSpec{
							Resources: []gardencorev1beta1.ControllerResource{
								{
									Kind: "kind",
									Type: "type",
								},
							},
						},
					},
				},
				[]gardencorev1beta1.Extension{barExtension},
				HaveKeyWithValue(
					Equal(barExtensionType),
					MatchAllFields(
						Fields{
							"Extension": MatchFields(IgnoreExtras, Fields{
								"Spec": MatchAllFields(Fields{
									"DefaultSpec": MatchAllFields(Fields{
										"Type":           Equal(barExtensionType),
										"ProviderConfig": PointTo(Equal(providerConfig)),
									}),
								}),
							}),
							"Timeout": Equal(extensionpkg.DefaultTimeout),
						},
					),
				),
			),
		)
	})

	Describe("#DeployExtensions", func() {
		Context("deploy", func() {
			It("should deploy successfully", func() {
				extension.EXPECT().Deploy(ctx)
				Expect(botanist.DeployExtensions(ctx)).To(Succeed())
			})

			It("should return the error during deployment", func() {
				extension.EXPECT().Deploy(ctx).Return(fakeErr)
				Expect(botanist.DeployExtensions(ctx)).To(MatchError(fakeErr))
			})
		})

		Context("restore", func() {
			BeforeEach(func() {
				botanist.Shoot.Info = &gardencorev1beta1.Shoot{
					Status: gardencorev1beta1.ShootStatus{
						LastOperation: &gardencorev1beta1.LastOperation{
							Type: gardencorev1beta1.LastOperationTypeRestore,
						},
					},
				}
			})

			It("should restore successfully", func() {
				extension.EXPECT().Restore(ctx, shootState)
				Expect(botanist.DeployExtensions(ctx)).To(Succeed())
			})

			It("should return the error during restoration", func() {
				extension.EXPECT().Restore(ctx, shootState).Return(fakeErr)
				Expect(botanist.DeployExtensions(ctx)).To(MatchError(fakeErr))
			})
		})
	})
})
