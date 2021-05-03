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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	mockkubernetes "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	"github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubecontrollermanager"
	mockkubecontrollermanager "github.com/gardener/gardener/pkg/operation/botanist/component/kubecontrollermanager/mock"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("KubeControllerManager", func() {
	var (
		ctrl             *gomock.Controller
		botanist         *Botanist
		kubernetesClient *mockkubernetes.MockInterface
		c                *mockclient.MockClient

		ctx       = context.TODO()
		fakeErr   = fmt.Errorf("fake err")
		namespace = "foo"
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		botanist = &Botanist{Operation: &operation.Operation{}}
		kubernetesClient = mockkubernetes.NewMockInterface(ctrl)
		c = mockclient.NewMockClient(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DefaultKubeControllerManager", func() {
		BeforeEach(func() {
			kubernetesClient.EXPECT().Version()

			botanist.Logger = logger.NewFieldLogger(logger.NewNopLogger(), "", "")
			botanist.K8sSeedClient = kubernetesClient
			botanist.Shoot = &shootpkg.Shoot{
				Info:     &gardencorev1beta1.Shoot{},
				Networks: &shootpkg.Networks{},
			}
		})

		It("should successfully create a kube-controller-manager interface", func() {
			kubernetesClient.EXPECT().Client()
			botanist.ImageVector = imagevector.ImageVector{{Name: "kube-controller-manager"}}

			kubeControllerManager, err := botanist.DefaultKubeControllerManager()
			Expect(kubeControllerManager).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error because the image cannot be found", func() {
			botanist.ImageVector = imagevector.ImageVector{}

			kubeControllerManager, err := botanist.DefaultKubeControllerManager()
			Expect(kubeControllerManager).To(BeNil())
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#DeployKubeControllerManager", func() {
		var (
			kubeControllerManager *mockkubecontrollermanager.MockKubeControllerManager

			secretName                  = "kube-controller-manager"
			secretNameServer            = "kube-controller-manager-server"
			secretNameCA                = "ca"
			secretNameServiceAccountKey = "service-account-key"
			checksum                    = "12"
			checksumServer              = "34"
			checksumCA                  = "56"
			checksumServiceAccountKey   = "78"
			secrets                     = kubecontrollermanager.Secrets{
				Kubeconfig:        component.Secret{Name: secretName, Checksum: checksum},
				Server:            component.Secret{Name: secretNameServer, Checksum: checksumServer},
				CA:                component.Secret{Name: secretNameCA, Checksum: checksumCA},
				ServiceAccountKey: component.Secret{Name: secretNameServiceAccountKey, Checksum: checksumServiceAccountKey},
			}
		)

		BeforeEach(func() {
			kubeControllerManager = mockkubecontrollermanager.NewMockKubeControllerManager(ctrl)

			botanist.K8sSeedClient = kubernetesClient
			botanist.CheckSums = map[string]string{
				secretName:                  checksum,
				secretNameServer:            checksumServer,
				secretNameCA:                checksumCA,
				secretNameServiceAccountKey: checksumServiceAccountKey,
			}
			botanist.Shoot = &shootpkg.Shoot{
				Components: &shootpkg.Components{
					ControlPlane: &shootpkg.ControlPlane{
						KubeControllerManager: kubeControllerManager,
					},
				},
				Info:          &gardencorev1beta1.Shoot{},
				SeedNamespace: namespace,
			}
		})

		Context("successfully deployment", func() {
			BeforeEach(func() {
				kubeControllerManager.EXPECT().SetSecrets(secrets)
				kubeControllerManager.EXPECT().Deploy(ctx)
			})

			Context("last operation is nil or not of type 'create'", func() {
				BeforeEach(func() {
					botanist.Shoot.Info.Status.LastOperation = nil
				})

				It("hibernation status unequal (true/false)", func() {
					botanist.Shoot.HibernationEnabled = true
					botanist.Shoot.Info.Status.IsHibernated = false

					kubeControllerManager.EXPECT().SetReplicaCount(int32(1))

					Expect(botanist.DeployKubeControllerManager(ctx)).To(Succeed())
				})

				It("hibernation status unequal (false/true)", func() {
					botanist.Shoot.HibernationEnabled = false
					botanist.Shoot.Info.Status.IsHibernated = true

					kubeControllerManager.EXPECT().SetReplicaCount(int32(1))

					Expect(botanist.DeployKubeControllerManager(ctx)).To(Succeed())
				})

				It("hibernation status equal (true/true)", func() {
					botanist.Shoot.HibernationEnabled = true
					botanist.Shoot.Info.Status.IsHibernated = true

					var replicas int32 = 4
					kubernetesClient.EXPECT().Client().Return(c)
					c.EXPECT().Get(ctx, kutil.Key(namespace, "kube-controller-manager"), gomock.AssignableToTypeOf(&appsv1.Deployment{})).DoAndReturn(func(_ context.Context, _ types.NamespacedName, obj *appsv1.Deployment) error {
						obj.Spec.Replicas = pointer.Int32Ptr(replicas)
						return nil
					})
					kubeControllerManager.EXPECT().SetReplicaCount(replicas)

					Expect(botanist.DeployKubeControllerManager(ctx)).To(Succeed())
				})

				It("hibernation status equal (false/false)", func() {
					botanist.Shoot.HibernationEnabled = false
					botanist.Shoot.Info.Status.IsHibernated = false

					var replicas int32 = 4
					kubernetesClient.EXPECT().Client().Return(c)
					c.EXPECT().Get(ctx, kutil.Key(namespace, "kube-controller-manager"), gomock.AssignableToTypeOf(&appsv1.Deployment{})).DoAndReturn(func(_ context.Context, _ types.NamespacedName, obj *appsv1.Deployment) error {
						obj.Spec.Replicas = pointer.Int32Ptr(replicas)
						return nil
					})
					kubeControllerManager.EXPECT().SetReplicaCount(replicas)

					Expect(botanist.DeployKubeControllerManager(ctx)).To(Succeed())
				})
			})

			Context("last operation is not nil and of type 'create'", func() {
				BeforeEach(func() {
					botanist.Shoot.Info.Status.LastOperation = &gardencorev1beta1.LastOperation{Type: gardencorev1beta1.LastOperationTypeCreate}
				})

				It("hibernation enabled", func() {
					botanist.Shoot.HibernationEnabled = true

					kubeControllerManager.EXPECT().SetReplicaCount(int32(0))

					Expect(botanist.DeployKubeControllerManager(ctx)).To(Succeed())
				})

				It("hibernation disabled", func() {
					botanist.Shoot.HibernationEnabled = false

					kubeControllerManager.EXPECT().SetReplicaCount(int32(1))

					Expect(botanist.DeployKubeControllerManager(ctx)).To(Succeed())
				})
			})
		})

		It("should fail when the replicas cannot be determined", func() {
			kubernetesClient.EXPECT().Client().Return(c)
			c.EXPECT().Get(ctx, kutil.Key(namespace, "kube-controller-manager"), gomock.AssignableToTypeOf(&appsv1.Deployment{})).Return(fakeErr)

			Expect(botanist.DeployKubeControllerManager(ctx)).To(Equal(fakeErr))
		})

		It("should fail when the deploy function fails", func() {
			kubernetesClient.EXPECT().Client().Return(c)
			c.EXPECT().Get(ctx, kutil.Key(namespace, "kube-controller-manager"), gomock.AssignableToTypeOf(&appsv1.Deployment{}))
			kubeControllerManager.EXPECT().SetSecrets(secrets)
			kubeControllerManager.EXPECT().SetReplicaCount(int32(0))
			kubeControllerManager.EXPECT().Deploy(ctx).Return(fakeErr)

			Expect(botanist.DeployKubeControllerManager(ctx)).To(Equal(fakeErr))
		})
	})

	Describe("#ScaleKubeControllerManagerToOne", func() {
		BeforeEach(func() {
			botanist.K8sSeedClient = kubernetesClient
			botanist.Shoot = &shootpkg.Shoot{
				SeedNamespace: namespace,
			}

			kubernetesClient.EXPECT().Client().Return(c)
		})

		var patch = client.RawPatch(types.MergePatchType, []byte(`{"spec":{"replicas":1}}`))

		It("should scale the KCM deployment", func() {
			c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), patch)
			Expect(botanist.ScaleKubeControllerManagerToOne(ctx)).To(Succeed())
		})

		It("should fail when the scale call fails", func() {
			c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), patch).Return(fakeErr)
			Expect(botanist.ScaleKubeControllerManagerToOne(ctx)).To(MatchError(fakeErr))
		})
	})
})
