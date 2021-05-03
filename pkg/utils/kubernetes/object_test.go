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

package kubernetes_test

import (
	"context"
	"fmt"

	"github.com/golang/mock/gomock"
	"github.com/hashicorp/go-multierror"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	. "github.com/gardener/gardener/pkg/utils/kubernetes"
)

var _ = Describe("Object", func() {
	var (
		ctrl *gomock.Controller
		c    *mockclient.MockClient

		ctx     = context.TODO()
		fakeErr = fmt.Errorf("fake err")

		obj1 = &corev1.Secret{}
		obj2 = &appsv1.Deployment{}
		objs = []client.Object{obj1, obj2}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DeleteObjects", func() {
		It("should fail because an object fails to delete", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, obj1).Return(fakeErr),
			)

			Expect(DeleteObjects(ctx, c, objs...)).To(MatchError(fakeErr))
		})

		It("should fail because an object fails to delete", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, obj1),
				c.EXPECT().Delete(ctx, obj2).Return(fakeErr),
			)

			Expect(DeleteObjects(ctx, c, objs...)).To(MatchError(fakeErr))
		})

		It("should successfully delete all objects", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, obj1),
				c.EXPECT().Delete(ctx, obj2),
			)

			Expect(DeleteObjects(ctx, c, objs...)).To(Succeed())
		})
	})

	Describe("#DeleteObject", func() {
		It("should fail to delete the object", func() {
			c.EXPECT().Delete(ctx, obj1).Return(fakeErr)

			Expect(DeleteObject(ctx, c, obj1)).To(MatchError(fakeErr))
		})

		It("should not fail to delete the object (not found error)", func() {
			c.EXPECT().Delete(ctx, obj1).Return(apierrors.NewNotFound(schema.GroupResource{}, ""))

			Expect(DeleteObject(ctx, c, obj1)).To(Succeed())
		})

		It("should not fail to delete the object (no match error)", func() {
			c.EXPECT().Delete(ctx, obj1).Return(&meta.NoResourceMatchError{PartialResource: schema.GroupVersionResource{}})

			Expect(DeleteObject(ctx, c, obj1)).To(Succeed())
		})

		It("should successfully delete the object", func() {
			c.EXPECT().Delete(ctx, obj1)

			Expect(DeleteObject(ctx, c, obj1)).To(Succeed())
		})
	})

	Describe("#DeleteObjectsFromListConditionally", func() {
		var (
			obj1       = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "1"}}
			obj2       = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "2"}}
			obj3       = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "3"}}
			listObject = &corev1.NamespaceList{Items: []corev1.Namespace{*obj1, *obj2, *obj3}}

			predicateFn = func(obj runtime.Object) bool {
				acc, _ := meta.Accessor(obj)
				return acc.GetName() != "2"
			}
		)

		It("should return an error if deleting an object failed", func() {
			c.EXPECT().Delete(ctx, obj1)
			c.EXPECT().Delete(ctx, obj3).Return(fakeErr)

			err := DeleteObjectsFromListConditionally(ctx, c, listObject, predicateFn)
			Expect(err).To(BeAssignableToTypeOf(&multierror.Error{}))
			Expect(err.(*multierror.Error).Errors).To(ConsistOf(Equal(fakeErr)))
		})

		It("should successfully delete the relevant objects", func() {
			c.EXPECT().Delete(ctx, obj1)
			c.EXPECT().Delete(ctx, obj3)

			Expect(DeleteObjectsFromListConditionally(ctx, c, listObject, predicateFn)).To(Succeed())
		})
	})

	Describe("#IsNamespaceInUse", func() {
		var (
			group                     = "group"
			version                   = "v43"
			kind                      = "kind"
			gvk                       = schema.GroupVersionKind{Group: group, Version: version, Kind: kind}
			partialObjectMetadataList = &metav1.PartialObjectMetadataList{TypeMeta: metav1.TypeMeta{APIVersion: group + "/" + version, Kind: kind}}
		)

		It("should return an error because the listing failed", func() {
			c.EXPECT().List(ctx, partialObjectMetadataList, client.InNamespace(namespace), client.Limit(1)).Return(fakeErr)

			inUse, err := IsNamespaceInUse(ctx, c, namespace, gvk)
			Expect(err).To(MatchError(fakeErr))
			Expect(inUse).To(BeTrue())
		})

		It("should return true because objects found", func() {
			c.EXPECT().List(ctx, partialObjectMetadataList, client.InNamespace(namespace), client.Limit(1)).DoAndReturn(func(_ context.Context, list *metav1.PartialObjectMetadataList, _ ...client.ListOption) error {
				(&metav1.PartialObjectMetadataList{Items: []metav1.PartialObjectMetadata{{}}}).DeepCopyInto(list)
				return nil
			})

			inUse, err := IsNamespaceInUse(ctx, c, namespace, gvk)
			Expect(err).NotTo(HaveOccurred())
			Expect(inUse).To(BeTrue())
		})

		It("should return false because no objects found", func() {
			c.EXPECT().List(ctx, partialObjectMetadataList, client.InNamespace(namespace), client.Limit(1))

			inUse, err := IsNamespaceInUse(ctx, c, namespace, gvk)
			Expect(err).NotTo(HaveOccurred())
			Expect(inUse).To(BeFalse())
		})
	})
})
