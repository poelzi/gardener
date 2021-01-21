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

package shoot_test

import (
	"context"
	"errors"
	"sync"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	fakeclientmap "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/fake"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	fakeclientset "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/shoot"
	"github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Shoot References", func() {
	const (
		shootNamespace = "shoot--foo--bar"
		shootName      = "bar"
	)

	var (
		ctx             context.Context
		cl              *mockclient.MockClient
		namespacedName  types.NamespacedName
		reconciler      reconcile.Reconciler
		shoot           gardencorev1beta1.Shoot
		clientMap       clientmap.ClientMap
		secretLister    SecretLister
		configMapLister ConfigMapLister
		cfg             = &config.ShootReferenceControllerConfiguration{}
	)

	BeforeEach(func() {
		ctx = context.TODO()
		ctrl := gomock.NewController(GinkgoT())
		cl = mockclient.NewMockClient(ctrl)
		clientMap = fakeclientmap.NewClientMap().AddClient(keys.ForGarden(), fakeclientset.NewClientSetBuilder().WithClient(cl).Build())
		namespacedName = types.NamespacedName{
			Namespace: shootNamespace,
			Name:      shootName,
		}

		shoot = gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      shootName,
				Namespace: shootNamespace,
			},
		}
	})

	JustBeforeEach(func() {
		reconciler = NewShootReferenceReconciler(logger.NewNopLogger(), clientMap, secretLister, configMapLister, cfg)
	})

	Context("Common controller tests", func() {
		BeforeEach(func() {
			secretLister = func(_ context.Context, _ *corev1.SecretList, _ ...client.ListOption) error {
				return nil
			}
			configMapLister = func(_ context.Context, _ *corev1.ConfigMapList, _ ...client.ListOption) error {
				return nil
			}
		})

		It("should do nothing because shoot in request cannot be found", func() {
			cl.EXPECT().Get(gomock.Any(), namespacedName, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).Return(apierrors.NewNotFound(gardencorev1beta1.SchemeGroupVersion.WithResource("shoots").GroupResource(), namespacedName.Name))

			request := reconcile.Request{NamespacedName: namespacedName}
			result, err := reconciler.Reconcile(ctx, request)

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(emptyResult()))
		})

		It("should error because shoot in request cannot be requested", func() {
			cl.EXPECT().Get(gomock.Any(), namespacedName, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).Return(errors.New("foo"))

			request := reconcile.Request{NamespacedName: namespacedName}
			result, err := reconciler.Reconcile(ctx, request)

			Expect(err).To(HaveOccurred())
			Expect(result).To(Equal(emptyResult()))
		})
	})

	Context("DNS secret reference test", func() {
		var (
			secrets []corev1.Secret
		)

		BeforeEach(func() {
			secretLister = func(ctx context.Context, secrets *corev1.SecretList, opts ...client.ListOption) error {
				return cl.List(ctx, secrets, opts...)
			}
			configMapLister = func(ctx context.Context, configMaps *corev1.ConfigMapList, opts ...client.ListOption) error {
				return cl.List(ctx, configMaps, opts...)
			}

			secrets = []corev1.Secret{
				{ObjectMeta: metav1.ObjectMeta{
					Name:      "secret-1",
					Namespace: shootNamespace},
				},
				{ObjectMeta: metav1.ObjectMeta{
					Name:      "secret-2",
					Namespace: shootNamespace},
				},
				{ObjectMeta: metav1.ObjectMeta{
					Name:      "secret-3",
					Namespace: shootNamespace},
				},
			}

			cl.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.ConfigMapList{}), client.InNamespace(shootNamespace)).DoAndReturn(
				func(_ context.Context, list *corev1.ConfigMapList, _ ...client.ListOption) error {
					list.Items = []corev1.ConfigMap{}
					return nil
				})
		})

		It("should not add finalizers because shoot does not define a DNS section", func() {
			shoot.Spec.DNS = nil

			cl.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{}), client.InNamespace(shootNamespace)).DoAndReturn(
				func(_ context.Context, list *gardencorev1beta1.ShootList, _ ...client.ListOption) error {
					list.Items = append(list.Items, shoot)
					return nil
				})

			cl.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.SecretList{}), client.InNamespace(shootNamespace), UserManagedSelector).DoAndReturn(
				func(_ context.Context, list *corev1.SecretList, _ ...client.ListOption) error {
					list.Items = append(list.Items, secrets...)
					return nil
				})

			cl.EXPECT().Get(gomock.Any(), namespacedName, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).DoAndReturn(
				func(_ context.Context, _ types.NamespacedName, s *gardencorev1beta1.Shoot) error {
					*s = shoot
					return nil
				})

			request := reconcile.Request{NamespacedName: namespacedName}
			result, err := reconciler.Reconcile(ctx, request)

			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(Equal(emptyResult()))
			Expect(shoot.ObjectMeta.Finalizers).To(BeEmpty())
		})

		It("should not add finalizers because shoot does not refer to any secret", func() {
			shoot.Spec.DNS = &gardencorev1beta1.DNS{
				Domain: pointer.StringPtr("shoot.example.com"),
				Providers: []gardencorev1beta1.DNSProvider{
					{Type: pointer.StringPtr("managed-dns")},
				},
			}

			cl.EXPECT().Get(gomock.Any(), namespacedName, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).DoAndReturn(
				func(_ context.Context, _ types.NamespacedName, s *gardencorev1beta1.Shoot) error {
					*s = shoot
					return nil
				})

			cl.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.SecretList{}), client.InNamespace(shootNamespace), UserManagedSelector).DoAndReturn(
				func(_ context.Context, list *corev1.SecretList, _ ...client.ListOption) error {
					list.Items = append(list.Items, secrets...)
					return nil
				})

			cl.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{}), client.InNamespace(shootNamespace)).DoAndReturn(
				func(_ context.Context, list *gardencorev1beta1.ShootList, _ ...client.ListOption) error {
					list.Items = append(list.Items, shoot)
					return nil
				})

			request := reconcile.Request{NamespacedName: namespacedName}
			result, err := reconciler.Reconcile(ctx, request)

			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(Equal(emptyResult()))
			Expect(shoot.ObjectMeta.Finalizers).To(BeEmpty())
		})

		It("should add finalizer to shoot and secrets", func() {
			secretName := secrets[0].Name
			secretName2 := secrets[1].Name
			shoot.Spec.DNS = &gardencorev1beta1.DNS{
				Domain: pointer.StringPtr("shoot.example.com"),
				Providers: []gardencorev1beta1.DNSProvider{
					{Type: pointer.StringPtr("managed-dns"), SecretName: pointer.StringPtr(secretName)},
					{Type: pointer.StringPtr("managed-dns2"), SecretName: pointer.StringPtr(secretName2)},
				},
			}

			cl.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{}), client.InNamespace(shootNamespace)).DoAndReturn(
				func(_ context.Context, list *gardencorev1beta1.ShootList, _ ...client.ListOption) error {
					list.Items = append(list.Items, shoot)
					return nil
				})

			cl.EXPECT().Get(gomock.Any(), namespacedName, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).DoAndReturn(
				func(_ context.Context, _ types.NamespacedName, s *gardencorev1beta1.Shoot) error {
					*s = shoot
					return nil
				}).Times(2)

			cl.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.SecretList{}), client.InNamespace(shootNamespace), UserManagedSelector).DoAndReturn(
				func(_ context.Context, list *corev1.SecretList, _ ...client.ListOption) error {
					list.Items = append(list.Items, secrets...)
					return nil
				})

			var (
				m              sync.Mutex
				updatedSecrets []*corev1.Secret
			)
			cl.EXPECT().Get(gomock.Any(), kutil.Key(secrets[0].Namespace, secretName), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
				func(_ context.Context, _ types.NamespacedName, s *corev1.Secret) error {
					*s = secrets[0]
					return nil
				})
			cl.EXPECT().Get(gomock.Any(), kutil.Key(secrets[1].Namespace, secretName2), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
				func(_ context.Context, _ types.NamespacedName, s *corev1.Secret) error {
					*s = secrets[1]
					return nil
				})
			cl.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, secret *corev1.Secret, _ client.Patch) error {
					defer m.Unlock()
					m.Lock()
					updatedSecrets = append(updatedSecrets, secret)
					return nil
				})
			cl.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, secret *corev1.Secret, _ client.Patch) error {
					defer m.Unlock()
					m.Lock()
					updatedSecrets = append(updatedSecrets, secret)
					return nil
				})

			var updatedShoot *gardencorev1beta1.Shoot
			cl.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, shoot *gardencorev1beta1.Shoot, _ client.Patch) error {
					updatedShoot = shoot
					return nil
				})

			request := reconcile.Request{NamespacedName: namespacedName}
			result, err := reconciler.Reconcile(ctx, request)

			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(Equal(emptyResult()))
			Expect(updatedShoot.ObjectMeta.Finalizers).To(ConsistOf(Equal(FinalizerName)))
			Expect(updatedSecrets).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"ObjectMeta": MatchFields(IgnoreExtras, Fields{
						"Finalizers": ConsistOf(FinalizerName),
						"Name":       Equal(secretName),
					}),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"ObjectMeta": MatchFields(IgnoreExtras, Fields{
						"Finalizers": ConsistOf(FinalizerName),
						"Name":       Equal(secretName2),
					}),
				})),
			))
		})

		It("should remove finalizer from shoot and secret because shoot is in deletion", func() {
			secretName := secrets[0].Name
			secrets[0].Finalizers = []string{FinalizerName}

			now := metav1.Now()
			shoot.ObjectMeta.DeletionTimestamp = &now
			shoot.Finalizers = []string{FinalizerName}

			shoot.Spec.DNS = &gardencorev1beta1.DNS{
				Domain: pointer.StringPtr("shoot.example.com"),
				Providers: []gardencorev1beta1.DNSProvider{
					{Type: pointer.StringPtr("managed-dns"), SecretName: pointer.StringPtr(secretName)},
				},
			}

			cl.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{}), client.InNamespace(shootNamespace)).DoAndReturn(
				func(_ context.Context, list *gardencorev1beta1.ShootList, _ ...client.ListOption) error {
					list.Items = append(list.Items, shoot)
					return nil
				})

			cl.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.SecretList{}), client.InNamespace(shootNamespace), UserManagedSelector).DoAndReturn(
				func(_ context.Context, list *corev1.SecretList, _ ...client.ListOption) error {
					list.Items = append(list.Items, secrets...)
					return nil
				})

			cl.EXPECT().Get(gomock.Any(), namespacedName, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).DoAndReturn(
				func(_ context.Context, _ types.NamespacedName, s *gardencorev1beta1.Shoot) error {
					*s = shoot
					return nil
				}).Times(2)

			var updatedSecret *corev1.Secret
			cl.EXPECT().Get(gomock.Any(), kutil.Key(secrets[0].Namespace, secretName), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
				func(_ context.Context, _ types.NamespacedName, s *corev1.Secret) error {
					*s = secrets[0]
					return nil
				})
			cl.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, secret *corev1.Secret, _ client.Patch) error {
					updatedSecret = secret
					return nil
				})

			var updatedShoot *gardencorev1beta1.Shoot
			cl.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, shoot *gardencorev1beta1.Shoot, _ client.Patch) error {
					updatedShoot = shoot
					return nil
				})

			request := reconcile.Request{NamespacedName: namespacedName}
			result, err := reconciler.Reconcile(ctx, request)

			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(Equal(emptyResult()))
			Expect(updatedShoot.ObjectMeta.Finalizers).To(BeEmpty())
			Expect(updatedSecret.Finalizers).To(BeEmpty())
			Expect(updatedSecret.ObjectMeta.Name).To(Equal(secrets[0].Name))
		})

		It("should remove finalizer only from shoot because secret is still referenced by another shoot", func() {
			secretName := secrets[0].Name
			secrets[0].Finalizers = []string{FinalizerName}

			now := metav1.Now()
			shoot.ObjectMeta.DeletionTimestamp = &now
			shoot.Finalizers = []string{FinalizerName}

			dnsProvider := gardencorev1beta1.DNSProvider{Type: pointer.StringPtr("managed-dns"), SecretName: pointer.StringPtr(secretName)}

			shoot.Spec.DNS = &gardencorev1beta1.DNS{
				Domain:    pointer.StringPtr("shoot.example.com"),
				Providers: []gardencorev1beta1.DNSProvider{dnsProvider},
			}

			shoot2 := gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bar2",
					Namespace: shootNamespace,
				},
				Spec: gardencorev1beta1.ShootSpec{
					DNS: &gardencorev1beta1.DNS{
						Providers: []gardencorev1beta1.DNSProvider{dnsProvider},
					},
				},
			}

			cl.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{}), client.InNamespace(shootNamespace)).DoAndReturn(
				func(_ context.Context, list *gardencorev1beta1.ShootList, _ ...client.ListOption) error {
					list.Items = append(list.Items, shoot, shoot2)
					return nil
				})

			cl.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.SecretList{}), client.InNamespace(shootNamespace), UserManagedSelector).DoAndReturn(
				func(_ context.Context, list *corev1.SecretList, _ ...client.ListOption) error {
					list.Items = append(list.Items, secrets...)
					return nil
				})

			cl.EXPECT().Get(gomock.Any(), namespacedName, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).DoAndReturn(
				func(_ context.Context, _ types.NamespacedName, s *gardencorev1beta1.Shoot) error {
					*s = shoot
					return nil
				}).Times(2)

			var updatedShoot *gardencorev1beta1.Shoot
			cl.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, shoot *gardencorev1beta1.Shoot, _ client.Patch) error {
					updatedShoot = shoot
					return nil
				})

			request := reconcile.Request{NamespacedName: namespacedName}
			result, err := reconciler.Reconcile(ctx, request)

			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(Equal(emptyResult()))
			Expect(updatedShoot.ObjectMeta.Finalizers).To(BeEmpty())
			Expect(updatedShoot.Name).To(Equal(shoot.Name))
		})

		It("should remove finalizer from secret because it is not referenced any more", func() {
			secretName := secrets[1].Name
			secrets[0].Finalizers = []string{FinalizerName}
			secrets[1].Finalizers = []string{FinalizerName}

			shoot.Finalizers = []string{FinalizerName}

			shoot.Spec.DNS = &gardencorev1beta1.DNS{
				Domain: pointer.StringPtr("shoot.example.com"),
				Providers: []gardencorev1beta1.DNSProvider{
					{Type: pointer.StringPtr("managed-dns"), SecretName: pointer.StringPtr(secrets[1].Name)},
				},
			}

			cl.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{}), client.InNamespace(shootNamespace)).DoAndReturn(
				func(_ context.Context, list *gardencorev1beta1.ShootList, _ ...client.ListOption) error {
					list.Items = append(list.Items, shoot)
					return nil
				})

			cl.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.SecretList{}), client.InNamespace(shootNamespace), UserManagedSelector).DoAndReturn(
				func(_ context.Context, list *corev1.SecretList, _ ...client.ListOption) error {
					list.Items = append(list.Items, secrets...)
					return nil
				})

			cl.EXPECT().Get(gomock.Any(), namespacedName, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).DoAndReturn(
				func(_ context.Context, _ types.NamespacedName, s *gardencorev1beta1.Shoot) error {
					*s = shoot
					return nil
				}).Times(2)

			cl.EXPECT().Get(gomock.Any(), kutil.Key(shootNamespace, secretName), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
				func(_ context.Context, _ types.NamespacedName, s *corev1.Secret) error {
					*s = secrets[1]
					return nil
				})

			var updatedSecret *corev1.Secret
			cl.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, secret *corev1.Secret, _ client.Patch) error {
					updatedSecret = secret
					return nil
				})

			request := reconcile.Request{NamespacedName: namespacedName}
			result, err := reconciler.Reconcile(ctx, request)

			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(Equal(emptyResult()))
			Expect(updatedSecret.Finalizers).To(BeEmpty())
			Expect(updatedSecret.ObjectMeta.Name).To(Equal(secrets[0].Name))
		})
	})

	Context("Audit policy ConfigMap reference test", func() {
		var configMaps []corev1.ConfigMap

		BeforeEach(func() {
			cfg.ProtectAuditPolicyConfigMaps = pointer.BoolPtr(true)
			reconciler = NewShootReferenceReconciler(logger.NewNopLogger(), clientMap, secretLister, configMapLister, cfg)

			secretLister = func(ctx context.Context, secrets *corev1.SecretList, opts ...client.ListOption) error {
				return cl.List(ctx, secrets, opts...)
			}
			configMapLister = func(ctx context.Context, configMaps *corev1.ConfigMapList, opts ...client.ListOption) error {
				return cl.List(ctx, configMaps, opts...)
			}

			configMaps = []corev1.ConfigMap{
				{ObjectMeta: metav1.ObjectMeta{
					Name:      "configmap-1",
					Namespace: shootNamespace},
				},
				{ObjectMeta: metav1.ObjectMeta{
					Name:      "configmap-2",
					Namespace: shootNamespace},
				},
				{ObjectMeta: metav1.ObjectMeta{
					Name:      "configmap-3",
					Namespace: shootNamespace},
				},
			}

			cl.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.SecretList{}), client.InNamespace(shootNamespace), UserManagedSelector).DoAndReturn(
				func(_ context.Context, list *corev1.SecretList, _ ...client.ListOption) error {
					list.Items = []corev1.Secret{}
					return nil
				})
		})

		It("should not add finalizers because shoot does not define an audit config section", func() {
			shoot.Spec.Kubernetes.KubeAPIServer = nil

			cl.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{}), client.InNamespace(shootNamespace)).DoAndReturn(
				func(_ context.Context, list *gardencorev1beta1.ShootList, _ ...client.ListOption) error {
					list.Items = append(list.Items, shoot)
					return nil
				}).Times(2)

			cl.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.ConfigMapList{}), client.InNamespace(shootNamespace)).DoAndReturn(
				func(_ context.Context, list *corev1.ConfigMapList, _ ...client.ListOption) error {
					list.Items = append(list.Items, configMaps...)
					return nil
				})

			cl.EXPECT().Get(gomock.Any(), namespacedName, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).DoAndReturn(
				func(_ context.Context, _ types.NamespacedName, s *gardencorev1beta1.Shoot) error {
					*s = shoot
					return nil
				})

			request := reconcile.Request{NamespacedName: namespacedName}
			result, err := reconciler.Reconcile(ctx, request)

			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(Equal(emptyResult()))
			Expect(shoot.ObjectMeta.Finalizers).To(BeEmpty())
		})

		It("should add finalizer to shoot and configmap", func() {
			configMapName := configMaps[1].Name
			shoot.Spec.Kubernetes.KubeAPIServer = &gardencorev1beta1.KubeAPIServerConfig{
				AuditConfig: &gardencorev1beta1.AuditConfig{
					AuditPolicy: &gardencorev1beta1.AuditPolicy{
						ConfigMapRef: &corev1.ObjectReference{
							Name: configMapName,
						},
					},
				},
			}

			cl.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{}), client.InNamespace(shootNamespace)).DoAndReturn(
				func(_ context.Context, list *gardencorev1beta1.ShootList, _ ...client.ListOption) error {
					list.Items = append(list.Items, shoot)
					return nil
				}).Times(2)

			cl.EXPECT().Get(gomock.Any(), namespacedName, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).DoAndReturn(
				func(_ context.Context, _ types.NamespacedName, s *gardencorev1beta1.Shoot) error {
					*s = shoot
					return nil
				}).Times(2)

			cl.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.ConfigMapList{}), client.InNamespace(shootNamespace)).DoAndReturn(
				func(_ context.Context, list *corev1.ConfigMapList, _ ...client.ListOption) error {
					list.Items = append(list.Items, configMaps...)
					return nil
				})

			cl.EXPECT().Get(gomock.Any(), kutil.Key(configMaps[1].Namespace, configMapName), gomock.AssignableToTypeOf(&corev1.ConfigMap{})).DoAndReturn(
				func(_ context.Context, _ types.NamespacedName, cm *corev1.ConfigMap) error {
					*cm = configMaps[1]
					return nil
				})

			var updatedConfigMap *corev1.ConfigMap
			cl.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&corev1.ConfigMap{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, configMap *corev1.ConfigMap, _ client.Patch) error {
					updatedConfigMap = configMap
					return nil
				})

			var updatedShoot *gardencorev1beta1.Shoot
			cl.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, shoot *gardencorev1beta1.Shoot, _ client.Patch) error {
					updatedShoot = shoot
					return nil
				})

			request := reconcile.Request{NamespacedName: namespacedName}
			result, err := reconciler.Reconcile(ctx, request)

			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(Equal(emptyResult()))
			Expect(updatedShoot.ObjectMeta.Finalizers).To(ConsistOf(Equal(FinalizerName)))
			Expect(updatedConfigMap).To(PointTo(
				MatchFields(IgnoreExtras, Fields{
					"ObjectMeta": MatchFields(IgnoreExtras, Fields{
						"Finalizers": ConsistOf(FinalizerName),
						"Name":       Equal(configMapName),
					}),
				})),
			)
		})

		It("should remove finalizer from shoot and configmap because shoot is in deletion", func() {
			configMapName := configMaps[0].Name
			configMaps[0].Finalizers = []string{FinalizerName}

			now := metav1.Now()
			shoot.ObjectMeta.DeletionTimestamp = &now
			shoot.Finalizers = []string{FinalizerName}

			shoot.Spec.Kubernetes.KubeAPIServer = &gardencorev1beta1.KubeAPIServerConfig{
				AuditConfig: &gardencorev1beta1.AuditConfig{
					AuditPolicy: &gardencorev1beta1.AuditPolicy{
						ConfigMapRef: &corev1.ObjectReference{
							Name: configMapName,
						},
					},
				},
			}

			cl.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{}), client.InNamespace(shootNamespace)).DoAndReturn(
				func(_ context.Context, list *gardencorev1beta1.ShootList, _ ...client.ListOption) error {
					list.Items = append(list.Items, shoot)
					return nil
				}).Times(2)

			cl.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.ConfigMapList{}), client.InNamespace(shootNamespace)).DoAndReturn(
				func(_ context.Context, list *corev1.ConfigMapList, _ ...client.ListOption) error {
					list.Items = append(list.Items, configMaps...)
					return nil
				})

			cl.EXPECT().Get(gomock.Any(), namespacedName, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).DoAndReturn(
				func(_ context.Context, _ types.NamespacedName, s *gardencorev1beta1.Shoot) error {
					*s = shoot
					return nil
				}).Times(2)

			var updatedConfigMap *corev1.ConfigMap
			cl.EXPECT().Get(gomock.Any(), kutil.Key(configMaps[0].Namespace, configMapName), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
				func(_ context.Context, _ types.NamespacedName, cm *corev1.ConfigMap) error {
					*cm = configMaps[0]
					return nil
				})
			cl.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&corev1.ConfigMap{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, cm *corev1.ConfigMap, _ client.Patch) error {
					updatedConfigMap = cm
					return nil
				})

			var updatedShoot *gardencorev1beta1.Shoot
			cl.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, shoot *gardencorev1beta1.Shoot, _ client.Patch) error {
					updatedShoot = shoot
					return nil
				})

			request := reconcile.Request{NamespacedName: namespacedName}
			result, err := reconciler.Reconcile(ctx, request)

			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(Equal(emptyResult()))
			Expect(updatedShoot.ObjectMeta.Finalizers).To(BeEmpty())
			Expect(updatedConfigMap.Finalizers).To(BeEmpty())
			Expect(updatedConfigMap.ObjectMeta.Name).To(Equal(configMaps[0].Name))
		})

		It("should remove finalizer only from shoot because configmap is still referenced by another shoot", func() {
			configMapName := configMaps[0].Name
			configMaps[0].Finalizers = []string{FinalizerName}

			now := metav1.Now()
			shoot.ObjectMeta.DeletionTimestamp = &now
			shoot.Finalizers = []string{FinalizerName}

			apiServerConfig := &gardencorev1beta1.KubeAPIServerConfig{
				AuditConfig: &gardencorev1beta1.AuditConfig{
					AuditPolicy: &gardencorev1beta1.AuditPolicy{
						ConfigMapRef: &corev1.ObjectReference{
							Name: configMapName,
						},
					},
				},
			}

			shoot.Spec.Kubernetes.KubeAPIServer = apiServerConfig

			shoot2 := gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bar2",
					Namespace: shootNamespace,
				},
				Spec: gardencorev1beta1.ShootSpec{
					Kubernetes: gardencorev1beta1.Kubernetes{
						KubeAPIServer: apiServerConfig,
					},
				},
			}

			cl.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{}), client.InNamespace(shootNamespace)).DoAndReturn(
				func(_ context.Context, list *gardencorev1beta1.ShootList, _ ...client.ListOption) error {
					list.Items = append(list.Items, shoot, shoot2)
					return nil
				}).Times(2)

			cl.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.ConfigMapList{}), client.InNamespace(shootNamespace)).DoAndReturn(
				func(_ context.Context, list *corev1.ConfigMapList, _ ...client.ListOption) error {
					list.Items = append(list.Items, configMaps...)
					return nil
				})

			cl.EXPECT().Get(gomock.Any(), namespacedName, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).DoAndReturn(
				func(_ context.Context, _ types.NamespacedName, s *gardencorev1beta1.Shoot) error {
					*s = shoot
					return nil
				}).Times(2)

			var updatedShoot *gardencorev1beta1.Shoot
			cl.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, shoot *gardencorev1beta1.Shoot, _ client.Patch) error {
					updatedShoot = shoot
					return nil
				})

			request := reconcile.Request{NamespacedName: namespacedName}
			result, err := reconciler.Reconcile(ctx, request)

			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(Equal(emptyResult()))
			Expect(updatedShoot.ObjectMeta.Finalizers).To(BeEmpty())
			Expect(updatedShoot.Name).To(Equal(shoot.Name))
		})

		It("should remove finalizer from configmap because it is not referenced any more", func() {
			configMapName := configMaps[1].Name
			configMaps[0].Finalizers = []string{FinalizerName}
			configMaps[1].Finalizers = []string{FinalizerName}

			shoot.Finalizers = []string{FinalizerName}

			shoot.Spec.Kubernetes.KubeAPIServer = &gardencorev1beta1.KubeAPIServerConfig{
				AuditConfig: &gardencorev1beta1.AuditConfig{
					AuditPolicy: &gardencorev1beta1.AuditPolicy{
						ConfigMapRef: &corev1.ObjectReference{
							Name: configMapName,
						},
					},
				},
			}

			cl.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{}), client.InNamespace(shootNamespace)).DoAndReturn(
				func(_ context.Context, list *gardencorev1beta1.ShootList, _ ...client.ListOption) error {
					list.Items = append(list.Items, shoot)
					return nil
				}).Times(2)

			cl.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.ConfigMapList{}), client.InNamespace(shootNamespace)).DoAndReturn(
				func(_ context.Context, list *corev1.ConfigMapList, _ ...client.ListOption) error {
					list.Items = append(list.Items, configMaps...)
					return nil
				})

			cl.EXPECT().Get(gomock.Any(), namespacedName, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).DoAndReturn(
				func(_ context.Context, _ types.NamespacedName, s *gardencorev1beta1.Shoot) error {
					*s = shoot
					return nil
				}).Times(2)

			cl.EXPECT().Get(gomock.Any(), kutil.Key(shootNamespace, configMapName), gomock.AssignableToTypeOf(&corev1.ConfigMap{})).DoAndReturn(
				func(_ context.Context, _ types.NamespacedName, cm *corev1.ConfigMap) error {
					*cm = configMaps[1]
					return nil
				})

			var updatedConfigMap *corev1.ConfigMap
			cl.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&corev1.ConfigMap{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, configMap *corev1.ConfigMap, _ client.Patch) error {
					updatedConfigMap = configMap
					return nil
				})

			request := reconcile.Request{NamespacedName: namespacedName}
			result, err := reconciler.Reconcile(ctx, request)

			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(Equal(emptyResult()))
			Expect(updatedConfigMap.Finalizers).To(BeEmpty())
			Expect(updatedConfigMap.ObjectMeta.Name).To(Equal(configMaps[0].Name))
		})
	})
})

func emptyResult() reconcile.Result {
	return reconcile.Result{}
}
