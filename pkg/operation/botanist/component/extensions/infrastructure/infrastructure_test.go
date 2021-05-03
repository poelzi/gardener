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

package infrastructure_test

import (
	"context"
	"errors"
	"time"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	mocktime "github.com/gardener/gardener/pkg/mock/go/time"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/infrastructure"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("#Interface", func() {
	const (
		namespace    = "test-namespace"
		name         = "test-deploy"
		providerType = "foo"
	)

	var (
		ctx     context.Context
		log     logrus.FieldLogger
		fakeErr = errors.New("fake")

		ctrl    *gomock.Controller
		c       *mockclient.MockClient
		mockNow *mocktime.MockNow
		now     time.Time

		region         string
		sshPublicKey   []byte
		providerConfig *runtime.RawExtension
		providerStatus *runtime.RawExtension
		nodesCIDR      *string

		infra        *extensionsv1alpha1.Infrastructure
		infraSpec    extensionsv1alpha1.InfrastructureSpec
		values       *infrastructure.Values
		deployWaiter infrastructure.Interface
		waiter       *retryfake.Ops

		cleanupFunc func()
	)

	BeforeEach(func() {
		ctx = context.TODO()
		log = logger.NewNopLogger()

		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
		mockNow = mocktime.NewMockNow(ctrl)

		region = "europe"
		sshPublicKey = []byte("secure")
		providerConfig = &runtime.RawExtension{Raw: []byte(`{"very":"provider-specific"}`)}
		providerStatus = &runtime.RawExtension{Raw: []byte(`{"very":"provider-specific-status"}`)}
		nodesCIDR = pointer.StringPtr("1.2.3.4/5")

		values = &infrastructure.Values{
			Namespace:      namespace,
			Name:           name,
			Type:           providerType,
			ProviderConfig: providerConfig,
			Region:         region,
		}
		infra = &extensionsv1alpha1.Infrastructure{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
		}
		infraSpec = extensionsv1alpha1.InfrastructureSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type:           providerType,
				ProviderConfig: providerConfig,
			},
			Region:       region,
			SSHPublicKey: sshPublicKey,
			SecretRef: corev1.SecretReference{
				Name:      v1beta1constants.SecretNameCloudProvider,
				Namespace: namespace,
			},
		}

		waiter = &retryfake.Ops{MaxAttempts: 1}
		cleanupFunc = test.WithVars(
			&retry.Until, waiter.Until,
			&retry.UntilTimeout, waiter.UntilTimeout,
		)

		deployWaiter = infrastructure.New(log, c, values, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond)
	})

	AfterEach(func() {
		ctrl.Finish()
		cleanupFunc()
	})

	Describe("#Deploy", func() {
		DescribeTable("correct Infrastructure is created", func(mutator func()) {
			defer test.WithVars(
				&infrastructure.TimeNow, mockNow.Do,
			)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			c.
				EXPECT().
				Get(ctx, kutil.Key(namespace, name), infra.DeepCopy()).
				Return(apierrors.NewNotFound(schema.GroupResource{}, name))

			deployWaiter.SetSSHPublicKey(sshPublicKey)
			infra.Spec = infraSpec
			mutator()

			c.
				EXPECT().
				Create(ctx, infra)

			Expect(deployWaiter.Deploy(ctx)).To(Succeed())
		},
			Entry("with no modification", func() {}),
			Entry("without provider config", func() {
				values.ProviderConfig = nil
				infra.Spec.ProviderConfig = nil
			}),
			Entry("annotate operation", func() {
				values.AnnotateOperation = true
				infra.Annotations = map[string]string{
					v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile,
					v1beta1constants.GardenerTimestamp: now.UTC().String(),
				}
			}),
		)
	})

	Describe("#Wait", func() {
		It("should return error when it's not found", func() {
			c.
				EXPECT().
				Get(gomock.Any(), kutil.Key(namespace, name), &extensionsv1alpha1.Infrastructure{}).
				Return(apierrors.NewNotFound(schema.GroupResource{}, name)).
				AnyTimes()

			Expect(deployWaiter.Wait(ctx)).To(HaveOccurred())
		})

		It("should return unexpected errors", func() {
			c.
				EXPECT().
				Get(gomock.Any(), kutil.Key(namespace, name), gomock.AssignableToTypeOf(&extensionsv1alpha1.Infrastructure{})).
				Return(fakeErr)

			Expect(deployWaiter.Wait(ctx)).To(MatchError(ContainSubstring(fakeErr.Error())))
		})

		It("should return error when it's not ready", func() {
			c.
				EXPECT().
				Get(gomock.Any(), kutil.Key(namespace, name), &extensionsv1alpha1.Infrastructure{}).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *extensionsv1alpha1.Infrastructure) error {
					obj.Status.LastError = &gardencorev1beta1.LastError{
						Description: fakeErr.Error(),
					}
					return nil
				}).
				AnyTimes()

			Expect(deployWaiter.Wait(ctx)).To(MatchError(ContainSubstring(fakeErr.Error())))
		})

		It("should return no error when is ready", func() {

			c.
				EXPECT().
				Get(gomock.Any(), kutil.Key(namespace, name), &extensionsv1alpha1.Infrastructure{}).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *extensionsv1alpha1.Infrastructure) error {
					obj.Status.LastError = nil
					obj.ObjectMeta.Annotations = map[string]string{}
					obj.Status.LastOperation = &gardencorev1beta1.LastOperation{
						State: gardencorev1beta1.LastOperationStateSucceeded,
					}
					obj.Status.NodesCIDR = nodesCIDR
					obj.Status.ProviderStatus = providerStatus
					return nil
				})

			Expect(deployWaiter.Wait(ctx)).To(Succeed())
			Expect(deployWaiter.ProviderStatus()).To(Equal(providerStatus))
			Expect(deployWaiter.NodesCIDR()).To(Equal(nodesCIDR))
		})

		It("should poll until it's ready", func() {
			waiter.MaxAttempts = 2

			c.
				EXPECT().
				Get(gomock.Any(), kutil.Key(namespace, name), &extensionsv1alpha1.Infrastructure{}).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *extensionsv1alpha1.Infrastructure) error {
					obj.Status.LastError = &gardencorev1beta1.LastError{
						Description: fakeErr.Error(),
					}
					return nil
				})

			c.
				EXPECT().
				Get(gomock.Any(), kutil.Key(namespace, name), &extensionsv1alpha1.Infrastructure{}).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *extensionsv1alpha1.Infrastructure) error {
					obj.Status.LastError = nil
					obj.ObjectMeta.Annotations = map[string]string{}
					obj.Status.LastOperation = &gardencorev1beta1.LastOperation{
						State: gardencorev1beta1.LastOperationStateSucceeded,
					}
					obj.Status.NodesCIDR = nodesCIDR
					obj.Status.ProviderStatus = providerStatus
					return nil
				})

			Expect(deployWaiter.Wait(ctx)).To(Succeed())
			Expect(deployWaiter.ProviderStatus()).To(Equal(providerStatus))
			Expect(deployWaiter.NodesCIDR()).To(Equal(nodesCIDR))
		})

		It("should poll until it times out", func() {
			waiter.MaxAttempts = 3

			c.
				EXPECT().
				Get(gomock.Any(), kutil.Key(namespace, name), &extensionsv1alpha1.Infrastructure{}).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *extensionsv1alpha1.Infrastructure) error {
					obj.Status.LastError = &gardencorev1beta1.LastError{
						Description: fakeErr.Error(),
					}
					return nil
				}).
				AnyTimes()

			Expect(deployWaiter.Wait(ctx)).To(MatchError(ContainSubstring(fakeErr.Error())))
		})
	})

	Describe("#Destroy", func() {
		It("should not return error when it's not found", func() {
			c.
				EXPECT().
				Patch(gomock.Any(), gomock.AssignableToTypeOf(infra), gomock.Any()).
				Return(apierrors.NewNotFound(schema.GroupResource{}, name))

			Expect(deployWaiter.Destroy(ctx)).To(Succeed())
		})

		It("should not return error when it's deleted successfully", func() {
			defer test.WithVars(
				&extensions.TimeNow, mockNow.Do,
				&gutil.TimeNow, mockNow.Do,
			)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			infraCopy := infra.DeepCopy()
			infraCopy.Annotations = map[string]string{
				gutil.ConfirmationDeletion:         "true",
				v1beta1constants.GardenerTimestamp: now.UTC().String(),
			}

			c.
				EXPECT().
				Patch(ctx, infraCopy, gomock.Any())
			c.
				EXPECT().
				Delete(ctx, infraCopy)

			Expect(deployWaiter.Destroy(ctx)).To(Succeed())
		})

		It("should return error when it's not deleted successfully", func() {
			defer test.WithVars(
				&extensions.TimeNow, mockNow.Do,
				&gutil.TimeNow, mockNow.Do,
			)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			infraCopy := infra.DeepCopy()
			infraCopy.Annotations = map[string]string{
				gutil.ConfirmationDeletion:         "true",
				v1beta1constants.GardenerTimestamp: now.UTC().String(),
			}

			c.
				EXPECT().
				Patch(ctx, infraCopy, gomock.Any())
			c.
				EXPECT().
				Delete(ctx, infraCopy).
				Return(fakeErr)

			Expect(deployWaiter.Destroy(ctx)).To(MatchError(ContainSubstring(fakeErr.Error())))
		})
	})

	Describe("#WaitCleanup", func() {
		It("should not return error when it's already removed", func() {
			c.
				EXPECT().
				Get(gomock.Any(), kutil.Key(namespace, name), gomock.AssignableToTypeOf(&extensionsv1alpha1.Infrastructure{})).
				Return(apierrors.NewNotFound(schema.GroupResource{}, name))

			Expect(deployWaiter.WaitCleanup(ctx)).To(Succeed())
		})

		It("should return error when it's not deleted successfully", func() {
			description := "some error"

			c.
				EXPECT().
				Get(gomock.Any(), kutil.Key(namespace, name), &extensionsv1alpha1.Infrastructure{}).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *extensionsv1alpha1.Infrastructure) error {
					obj.Status.LastError = &gardencorev1beta1.LastError{
						Description: description,
					}
					return nil
				})

			Expect(deployWaiter.WaitCleanup(ctx)).To(MatchError(ContainSubstring(description)))
		})

		It("should poll until it's removed", func() {
			waiter.MaxAttempts = 2
			description := "some error"

			c.
				EXPECT().
				Get(gomock.Any(), kutil.Key(namespace, name), &extensionsv1alpha1.Infrastructure{}).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *extensionsv1alpha1.Infrastructure) error {
					obj.Status.LastError = &gardencorev1beta1.LastError{
						Description: description,
					}
					return nil
				})
			c.
				EXPECT().
				Get(gomock.Any(), kutil.Key(namespace, name), gomock.AssignableToTypeOf(&extensionsv1alpha1.Infrastructure{})).
				Return(apierrors.NewNotFound(schema.GroupResource{}, name))

			Expect(deployWaiter.WaitCleanup(ctx)).To(Succeed())
		})

		It("should poll until it times out", func() {
			waiter.MaxAttempts = 3
			description := "some error"

			c.
				EXPECT().
				Get(gomock.Any(), kutil.Key(namespace, name), &extensionsv1alpha1.Infrastructure{}).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *extensionsv1alpha1.Infrastructure) error {
					obj.Status.LastError = &gardencorev1beta1.LastError{
						Description: description,
					}
					return nil
				}).
				AnyTimes()

			Expect(deployWaiter.WaitCleanup(ctx)).To(MatchError(ContainSubstring(description)))
		})

		It("should return unexpected errors", func() {
			waiter.MaxAttempts = 2

			c.
				EXPECT().
				Get(gomock.Any(), kutil.Key(namespace, name), gomock.AssignableToTypeOf(&extensionsv1alpha1.Infrastructure{})).
				Return(fakeErr)

			Expect(deployWaiter.WaitCleanup(ctx)).To(MatchError(ContainSubstring(fakeErr.Error())))
		})
	})

	Describe("#Restore", func() {
		var (
			state      = &runtime.RawExtension{Raw: []byte(`{"dummy":"state"}`)}
			shootState = &gardencorev1alpha1.ShootState{
				Spec: gardencorev1alpha1.ShootStateSpec{
					Extensions: []gardencorev1alpha1.ExtensionResourceState{
						{
							Name:  pointer.StringPtr(name),
							Kind:  extensionsv1alpha1.InfrastructureResource,
							State: state,
						},
					},
				},
			}
		)

		It("should properly restore the infrastructure state if it exists", func() {
			defer test.WithVars(
				&infrastructure.TimeNow, mockNow.Do,
				&extensions.TimeNow, mockNow.Do,
			)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			values.SSHPublicKey = sshPublicKey
			values.AnnotateOperation = true

			obj := infra.DeepCopy()
			obj.Spec = infraSpec
			metav1.SetMetaDataAnnotation(&obj.ObjectMeta, "gardener.cloud/operation", "wait-for-state")
			metav1.SetMetaDataAnnotation(&obj.ObjectMeta, "gardener.cloud/timestamp", now.UTC().String())
			expectedWithState := obj.DeepCopy()
			expectedWithState.Status.State = state
			expectedWithRestore := expectedWithState.DeepCopy()
			expectedWithRestore.Annotations["gardener.cloud/operation"] = "restore"

			mc := mockclient.NewMockClient(ctrl)
			mc.EXPECT().Get(ctx, kutil.Key(namespace, name), gomock.AssignableToTypeOf(&extensionsv1alpha1.Infrastructure{})).Return(apierrors.NewNotFound(extensionsv1alpha1.Resource("Infrastructure"), name))
			mc.EXPECT().Create(ctx, obj)
			mc.EXPECT().Status().Return(mc)
			mc.EXPECT().Update(ctx, expectedWithState)
			test.EXPECTPatch(ctx, mc, expectedWithRestore, expectedWithState, types.MergePatchType)

			Expect(infrastructure.New(log, mc, values, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond).Restore(ctx, shootState)).To(Succeed())
		})
	})

	Describe("#Migrate", func() {
		It("should migrate the resources", func() {
			defer test.WithVars(
				&extensions.TimeNow, mockNow.Do,
			)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			obj := infra.DeepCopy()
			obj.Annotations = map[string]string{
				"gardener.cloud/operation": "migrate",
				"gardener.cloud/timestamp": now.UTC().String(),
			}

			gomock.InOrder(
				c.
					EXPECT().
					Get(ctx, kutil.Key(infra.Namespace, infra.Name), gomock.AssignableToTypeOf(&extensionsv1alpha1.Infrastructure{})).
					DoAndReturn(func(_ context.Context, _ client.ObjectKey, o *extensionsv1alpha1.Infrastructure) error {
						infra.DeepCopyInto(o)
						return nil
					}),
				test.
					EXPECTPatch(ctx, c, obj, infra, types.MergePatchType),
			)

			Expect(deployWaiter.Migrate(ctx)).To(Succeed())
		})

		It("should not return error if resource does not exist", func() {
			c.
				EXPECT().
				Get(ctx, kutil.Key(infra.Namespace, infra.Name), gomock.AssignableToTypeOf(&extensionsv1alpha1.Infrastructure{})).
				Return(apierrors.NewNotFound(schema.GroupResource{}, ""))

			Expect(deployWaiter.Migrate(ctx)).To(Succeed())
		})
	})

	Describe("#WaitMigrate", func() {
		It("should not return error when resource is missing", func() {
			c.
				EXPECT().
				Get(ctx, kutil.Key(infra.Namespace, infra.Name), gomock.AssignableToTypeOf(&extensionsv1alpha1.Infrastructure{})).
				Return(apierrors.NewNotFound(schema.GroupResource{}, ""))

			Expect(deployWaiter.WaitMigrate(ctx)).To(Succeed())
		})

		It("should return error if resource is not yet migrated successfully", func() {
			obj := infra.DeepCopy()
			obj.Status.LastError = &gardencorev1beta1.LastError{
				Description: "Some error",
			}
			obj.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateError,
				Type:  gardencorev1beta1.LastOperationTypeMigrate,
			}

			c.
				EXPECT().
				Get(ctx, kutil.Key(obj.Namespace, obj.Name), gomock.AssignableToTypeOf(&extensionsv1alpha1.Infrastructure{})).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, o *extensionsv1alpha1.Infrastructure) error {
					obj.DeepCopyInto(o)
					return nil
				})

			Expect(deployWaiter.WaitMigrate(ctx)).To(HaveOccurred())
		})

		It("should not return error if resource gets migrated successfully", func() {
			obj := infra.DeepCopy()
			obj.Status.LastError = nil
			obj.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateSucceeded,
				Type:  gardencorev1beta1.LastOperationTypeMigrate,
			}

			c.
				EXPECT().
				Get(ctx, kutil.Key(obj.Namespace, obj.Name), gomock.AssignableToTypeOf(&extensionsv1alpha1.Infrastructure{})).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, o *extensionsv1alpha1.Infrastructure) error {
					obj.DeepCopyInto(o)
					return nil
				})

			Expect(deployWaiter.WaitMigrate(ctx)).To(Succeed(), "infrastructure is ready, should not return an error")
		})
	})

	Describe("#Get", func() {
		It("should return an error when the retrieval fails", func() {
			c.
				EXPECT().
				Get(ctx, kutil.Key(infra.Namespace, infra.Name), gomock.AssignableToTypeOf(&extensionsv1alpha1.Infrastructure{})).
				Return(fakeErr)

			res, err := deployWaiter.Get(ctx)
			Expect(res).To(BeNil())
			Expect(err).To(MatchError(fakeErr))
		})

		It("should retrieve the object and extract the status", func() {
			Expect(deployWaiter.ProviderStatus()).To(BeNil())
			Expect(deployWaiter.NodesCIDR()).To(BeNil())

			var (
				providerStatus = &runtime.RawExtension{Raw: []byte(`{"some":"status"}`)}
				nodesCIDR      = pointer.StringPtr("1.2.3.4")
			)

			obj := infra.DeepCopy()
			obj.Status.ProviderStatus = providerStatus
			obj.Status.NodesCIDR = nodesCIDR

			c.
				EXPECT().
				Get(ctx, kutil.Key(obj.Namespace, obj.Name), gomock.AssignableToTypeOf(&extensionsv1alpha1.Infrastructure{})).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, o *extensionsv1alpha1.Infrastructure) error {
					obj.DeepCopyInto(o)
					return nil
				})

			res, err := deployWaiter.Get(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(Equal(obj))

			Expect(deployWaiter.ProviderStatus()).To(Equal(providerStatus))
			Expect(deployWaiter.NodesCIDR()).To(Equal(nodesCIDR))
		})
	})
})
