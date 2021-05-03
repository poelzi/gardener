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
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	mockkubernetes "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/clusterautoscaler"
	mockclusterautoscaler "github.com/gardener/gardener/pkg/operation/botanist/component/clusterautoscaler/mock"
	mockworker "github.com/gardener/gardener/pkg/operation/botanist/component/extensions/worker/mock"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/imagevector"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("ClusterAutoscaler", func() {
	var (
		ctrl     *gomock.Controller
		botanist *Botanist
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		botanist = &Botanist{Operation: &operation.Operation{}}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DefaultClusterAutoscaler", func() {
		var kubernetesClient *mockkubernetes.MockInterface

		BeforeEach(func() {
			kubernetesClient = mockkubernetes.NewMockInterface(ctrl)
			kubernetesClient.EXPECT().Version()

			botanist.K8sSeedClient = kubernetesClient
			botanist.Shoot = &shootpkg.Shoot{Info: &gardencorev1beta1.Shoot{}}
		})

		It("should successfully create a cluster-autoscaler interface", func() {
			kubernetesClient.EXPECT().Client()
			botanist.ImageVector = imagevector.ImageVector{{Name: "cluster-autoscaler"}}

			clusterAutoscaler, err := botanist.DefaultClusterAutoscaler()
			Expect(clusterAutoscaler).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error because the image cannot be found", func() {
			botanist.ImageVector = imagevector.ImageVector{}

			clusterAutoscaler, err := botanist.DefaultClusterAutoscaler()
			Expect(clusterAutoscaler).To(BeNil())
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#DeployClusterAutoscaler", func() {
		var (
			clusterAutoscaler *mockclusterautoscaler.MockClusterAutoscaler
			worker            *mockworker.MockInterface

			ctx                = context.TODO()
			fakeErr            = fmt.Errorf("fake err")
			secretName         = "cluster-autoscaler"
			checksum           = "1234"
			namespaceUID       = types.UID("5678")
			machineDeployments = []extensionsv1alpha1.MachineDeployment{{}}
		)

		BeforeEach(func() {
			clusterAutoscaler = mockclusterautoscaler.NewMockClusterAutoscaler(ctrl)
			worker = mockworker.NewMockInterface(ctrl)

			botanist.CheckSums = map[string]string{
				secretName: checksum,
			}
			botanist.SeedNamespaceObject = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					UID: namespaceUID,
				},
			}
			botanist.Shoot = &shootpkg.Shoot{
				Components: &shootpkg.Components{
					ControlPlane: &shootpkg.ControlPlane{
						ClusterAutoscaler: clusterAutoscaler,
					},
					Extensions: &shootpkg.Extensions{
						Worker: worker,
					},
				},
			}
		})

		Context("CA wanted", func() {
			BeforeEach(func() {
				botanist.Shoot.WantsClusterAutoscaler = true

				clusterAutoscaler.EXPECT().SetSecrets(clusterautoscaler.Secrets{
					Kubeconfig: component.Secret{Name: secretName, Checksum: checksum},
				})
				clusterAutoscaler.EXPECT().SetNamespaceUID(namespaceUID)
				worker.EXPECT().MachineDeployments().Return(machineDeployments)
				clusterAutoscaler.EXPECT().SetMachineDeployments(machineDeployments)
			})

			It("should set the secrets, namespace uid, machine deployments, and deploy", func() {
				clusterAutoscaler.EXPECT().Deploy(ctx)
				Expect(botanist.DeployClusterAutoscaler(ctx)).To(Succeed())
			})

			It("should fail when the deploy function fails", func() {
				clusterAutoscaler.EXPECT().Deploy(ctx).Return(fakeErr)
				Expect(botanist.DeployClusterAutoscaler(ctx)).To(Equal(fakeErr))
			})
		})

		Context("CA unwanted", func() {
			BeforeEach(func() {
				botanist.Shoot.WantsClusterAutoscaler = false
			})

			It("should destroy", func() {
				clusterAutoscaler.EXPECT().Destroy(ctx)
				Expect(botanist.DeployClusterAutoscaler(ctx)).To(Succeed())
			})

			It("should fail when the destroy function fails", func() {
				clusterAutoscaler.EXPECT().Destroy(ctx).Return(fakeErr)
				Expect(botanist.DeployClusterAutoscaler(ctx)).To(Equal(fakeErr))
			})
		})
	})
})
