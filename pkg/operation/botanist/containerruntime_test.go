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

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	mockcontainerruntime "github.com/gardener/gardener/pkg/operation/botanist/component/extensions/containerruntime/mock"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ContainerRuntime", func() {
	var (
		ctrl             *gomock.Controller
		containerRuntime *mockcontainerruntime.MockInterface
		botanist         *Botanist

		ctx        = context.TODO()
		fakeErr    = fmt.Errorf("fake")
		shootState = &gardencorev1alpha1.ShootState{}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		containerRuntime = mockcontainerruntime.NewMockInterface(ctrl)
		botanist = &Botanist{Operation: &operation.Operation{
			Shoot: &shootpkg.Shoot{
				Components: &shootpkg.Components{
					Extensions: &shootpkg.Extensions{
						ContainerRuntime: containerRuntime,
					},
				},
			},
			ShootState: shootState,
		}}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DeployContainerRuntime", func() {
		Context("deploy", func() {
			It("should deploy successfully", func() {
				containerRuntime.EXPECT().Deploy(ctx)
				Expect(botanist.DeployContainerRuntime(ctx)).To(Succeed())
			})

			It("should return the error during deployment", func() {
				containerRuntime.EXPECT().Deploy(ctx).Return(fakeErr)
				Expect(botanist.DeployContainerRuntime(ctx)).To(MatchError(fakeErr))
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
				containerRuntime.EXPECT().Restore(ctx, shootState)
				Expect(botanist.DeployContainerRuntime(ctx)).To(Succeed())
			})

			It("should return the error during restoration", func() {
				containerRuntime.EXPECT().Restore(ctx, shootState).Return(fakeErr)
				Expect(botanist.DeployContainerRuntime(ctx)).To(MatchError(fakeErr))
			})
		})
	})
})
