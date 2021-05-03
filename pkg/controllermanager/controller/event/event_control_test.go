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

package event

import (
	"context"
	"time"

	fakeclientmap "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/fake"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("#reconcileEvent", func() {
	var (
		ctx                    context.Context
		ctrl                   *gomock.Controller
		clientMap              *fakeclientmap.ClientMap
		k8sGardenRuntimeClient *mockclient.MockClient
		oldTimeFunc            = nowFunc

		namespaceName = "garden-foo"
		requestName   = "request"

		ttl = &metav1.Duration{Duration: 1 * time.Hour}

		eventController            *Controller
		request                    reconcile.Request
		shootEvent                 *corev1.Event
		nonShootEvent              *corev1.Event
		nonGardenerAPIGroupEvent   *corev1.Event
		eventWithoutInvolvedObject *corev1.Event
		cfg                        *config.EventControllerConfiguration
	)

	BeforeEach(func() {
		ctx = context.TODO()
		ctrl = gomock.NewController(GinkgoT())
		k8sGardenRuntimeClient = mockclient.NewMockClient(ctrl)
		clientMap = fakeclientmap.NewClientMap().AddRuntimeClient(keys.ForGarden(), k8sGardenRuntimeClient)

		logger.Logger = logger.NewNopLogger()

		shootEvent = &corev1.Event{
			LastTimestamp:  metav1.Time{Time: time.Date(0, 0, 0, 0, 0, 0, 0, time.UTC)},
			InvolvedObject: corev1.ObjectReference{Kind: "Shoot", APIVersion: "core.gardener.cloud/v1beta1"},
		}
		nonShootEvent = &corev1.Event{
			LastTimestamp:  metav1.Time{Time: time.Date(0, 0, 0, 0, 0, 0, 0, time.UTC)},
			InvolvedObject: corev1.ObjectReference{Kind: "Project", APIVersion: "core.gardener.cloud/v1beta1"},
		}
		eventWithoutInvolvedObject = &corev1.Event{
			LastTimestamp: metav1.Time{Time: time.Date(0, 0, 0, 0, 0, 0, 0, time.UTC)},
		}
		nonGardenerAPIGroupEvent = &corev1.Event{
			LastTimestamp:  metav1.Time{Time: time.Date(0, 0, 0, 0, 0, 0, 0, time.UTC)},
			InvolvedObject: corev1.ObjectReference{Kind: "Shoot", APIVersion: "v1"},
		}
		request = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: namespaceName,
				Name:      requestName,
			}}
		cfg = &config.EventControllerConfiguration{
			TTLNonShootEvents: ttl,
		}

		eventController = NewController(clientMap, cfg)
	})

	AfterEach(func() {
		nowFunc = oldTimeFunc
		ctrl.Finish()
	})

	Context("Shoot Events", func() {
		It("should ignore them", func() {
			mockClientGet(k8sGardenRuntimeClient, request.NamespacedName, shootEvent)
			expected := reconcile.Result{}
			actual, err := eventController.reconcileEvent(ctx, request)
			Expect(err).NotTo(HaveOccurred())
			Expect(actual).To(Equal(expected))
		})
	})

	Context("Non-Shoot events", func() {
		Context("ttl is not yet reached", func() {
			BeforeEach(func() {
				nowFunc = func() time.Time {
					return time.Date(0, 0, 0, 0, 0, 0, 0, time.UTC)
				}
			})

			It("should requeue non-shoot events", func() {
				mockClientGet(k8sGardenRuntimeClient, request.NamespacedName, nonShootEvent)
				expected := reconcile.Result{Requeue: false, RequeueAfter: ttl.Duration}
				actual, err := eventController.reconcileEvent(ctx, request)
				Expect(err).NotTo(HaveOccurred())
				Expect(actual).To(Equal(expected))
			})
			It("should requeue events with an empty involvedObject", func() {
				mockClientGet(k8sGardenRuntimeClient, request.NamespacedName, eventWithoutInvolvedObject)
				expected := reconcile.Result{Requeue: false, RequeueAfter: ttl.Duration}
				actual, err := eventController.reconcileEvent(ctx, request)
				Expect(err).NotTo(HaveOccurred())
				Expect(actual).To(Equal(expected))
			})
			It("should requeue events with non Gardener APIGroup", func() {
				mockClientGet(k8sGardenRuntimeClient, request.NamespacedName, nonGardenerAPIGroupEvent)
				expected := reconcile.Result{Requeue: false, RequeueAfter: ttl.Duration}
				actual, err := eventController.reconcileEvent(ctx, request)
				Expect(err).NotTo(HaveOccurred())
				Expect(actual).To(Equal(expected))
			})
		})

		Context("ttl is reached", func() {
			BeforeEach(func() {
				nowFunc = func() time.Time {
					return time.Date(0, 0, 0, 0, 0, 0, 0, time.UTC).
						Add(ttl.Duration)
				}
			})

			It("should delete the event", func() {
				mockClientGet(k8sGardenRuntimeClient, request.NamespacedName, nonShootEvent)
				k8sGardenRuntimeClient.
					EXPECT().
					Delete(context.TODO(), nonShootEvent).
					Return(nil).
					Times(1)
				expected := reconcile.Result{}
				actual, err := eventController.reconcileEvent(ctx, request)
				Expect(err).NotTo(HaveOccurred())
				Expect(actual).To(Equal(expected))
			})
		})
	})
})

var _ = Describe("#enqueueEvent", func() {
	var (
		queue *fakeQueue
		c     *Controller

		eventName = "foo"
	)

	BeforeEach(func() {
		logger.Logger = logger.NewNopLogger()
		queue = &fakeQueue{}
		c = &Controller{
			eventQueue: queue,
		}
	})

	It("should do nothing because it cannot compute the object key", func() {
		c.enqueueEvent("foo")

		Expect(queue.Len()).To(BeZero())
	})

	It("should add events to the workqueue", func() {
		event := &corev1.Event{
			ObjectMeta: metav1.ObjectMeta{Name: eventName},
		}
		c.enqueueEvent(event)

		Expect(queue.Len()).To(Equal(1))
		Expect(queue.items[0]).To(Equal(eventName))
	})
})

func mockClientGet(k8sGardenRuntimeClient *mockclient.MockClient, key client.ObjectKey, result *corev1.Event) {
	k8sGardenRuntimeClient.
		EXPECT().
		Get(context.TODO(), key, &corev1.Event{}).
		DoAndReturn(func(_ context.Context, _ client.ObjectKey, o client.Object) error {
			event, ok := o.(*corev1.Event)
			Expect(ok).To(BeTrue())
			result.DeepCopyInto(event)
			return nil
		})
}
