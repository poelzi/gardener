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

package botanist_test

import (
	"context"
	"fmt"
	"net"

	"github.com/gardener/gardener/charts"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	mockkubernetes "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	"github.com/gardener/gardener/pkg/features"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/vpnseedserver"
	mockvpnseedserver "github.com/gardener/gardener/pkg/operation/botanist/component/vpnseedserver/mock"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	"github.com/gardener/gardener/pkg/utils/test"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
)

var _ = Describe("VPNSeedServer", func() {
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

	Describe("#DefaultVPNSeedServer", func() {
		var kubernetesClient *mockkubernetes.MockInterface

		BeforeEach(func() {
			kubernetesClient = mockkubernetes.NewMockInterface(ctrl)
			kubernetesClient.EXPECT().Version()

			botanist.K8sSeedClient = kubernetesClient
			botanist.Shoot = &shootpkg.Shoot{
				Info: &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Networking: gardencorev1beta1.Networking{
							Nodes: pointer.StringPtr("10.0.0.0/24"),
						},
					},
				},
				DisableDNS: true,
				Networks: &shootpkg.Networks{
					Services: &net.IPNet{IP: net.IP{10, 0, 0, 1}, Mask: net.CIDRMask(10, 24)},
					Pods:     &net.IPNet{IP: net.IP{10, 0, 0, 2}, Mask: net.CIDRMask(10, 24)},
				},
			}
		})

		It("should successfully create a vpn seed server interface", func() {
			defer test.WithFeatureGate(gardenletfeatures.FeatureGate, features.APIServerSNI, true)()
			kubernetesClient.EXPECT().Client()
			kubernetesClient.EXPECT().Version()
			botanist.ImageVector = imagevector.ImageVector{{Name: charts.ImageNameVpnSeedServer}, {Name: charts.ImageNameApiserverProxy}}

			vpnSeedServer, err := botanist.DefaultVPNSeedServer()
			Expect(vpnSeedServer).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error because the images cannot be found", func() {
			botanist.ImageVector = imagevector.ImageVector{}

			vpnSeedServer, err := botanist.DefaultVPNSeedServer()
			Expect(vpnSeedServer).To(BeNil())
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#DeployVPNSeedServer", func() {
		var (
			vpnSeedServer *mockvpnseedserver.MockVPNSeedServer

			ctx     = context.TODO()
			fakeErr = fmt.Errorf("fake err")

			secretNameTLSAuth     = vpnseedserver.VpnSeedServerTLSAuth
			secretChecksumTLSAuth = "1234"
			secretNameServer      = vpnseedserver.DeploymentName
			secretChecksumServer  = "5678"
			secretNameDH          = v1beta1constants.GardenRoleOpenVPNDiffieHellman
			secretChecksumDH      = "9012"
		)

		BeforeEach(func() {
			vpnSeedServer = mockvpnseedserver.NewMockVPNSeedServer(ctrl)

			botanist.CheckSums = map[string]string{
				secretNameTLSAuth: secretChecksumTLSAuth,
				secretNameServer:  secretChecksumServer,
				secretNameDH:      secretChecksumDH,
			}
			botanist.Secrets = map[string]*corev1.Secret{
				secretNameTLSAuth: {},
				secretNameServer:  {},
				secretNameDH:      {},
			}
			botanist.Shoot = &shootpkg.Shoot{
				Components: &shootpkg.Components{
					ControlPlane: &shootpkg.ControlPlane{
						VPNSeedServer: vpnSeedServer,
					},
				},
				KonnectivityTunnelEnabled: false,
				ReversedVPNEnabled:        true,
			}
		})

		BeforeEach(func() {
			vpnSeedServer.EXPECT().SetSecrets(vpnseedserver.Secrets{
				TLSAuth:          component.Secret{Name: secretNameTLSAuth, Checksum: secretChecksumTLSAuth},
				Server:           component.Secret{Name: vpnseedserver.DeploymentName, Checksum: secretChecksumServer},
				DiffieHellmanKey: component.Secret{Name: secretNameDH, Checksum: secretChecksumDH},
			})
		})

		It("should set the secrets and deploy", func() {
			vpnSeedServer.EXPECT().Deploy(ctx)
			Expect(botanist.DeployVPNServer(ctx)).To(Succeed())
		})

		It("should fail when the deploy function fails", func() {
			vpnSeedServer.EXPECT().Deploy(ctx).Return(fakeErr)
			Expect(botanist.DeployVPNServer(ctx)).To(Equal(fakeErr))
		})
	})
})
