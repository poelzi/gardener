// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package worker

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("health", func() {
	DescribeTable("#CheckMachineDeployment",
		func(machineDeployment *machinev1alpha1.MachineDeployment, matcher types.GomegaMatcher) {
			err := CheckMachineDeployment(machineDeployment)
			Expect(err).To(matcher)
		},
		Entry("healthy", &machinev1alpha1.MachineDeployment{
			Status: machinev1alpha1.MachineDeploymentStatus{Conditions: []machinev1alpha1.MachineDeploymentCondition{
				{
					Type:   machinev1alpha1.MachineDeploymentAvailable,
					Status: machinev1alpha1.ConditionTrue,
				},
				{
					Type:   machinev1alpha1.MachineDeploymentProgressing,
					Status: machinev1alpha1.ConditionTrue,
				},
			}},
		}, BeNil()),
		Entry("healthy without progressing", &machinev1alpha1.MachineDeployment{
			Status: machinev1alpha1.MachineDeploymentStatus{Conditions: []machinev1alpha1.MachineDeploymentCondition{
				{
					Type:   machinev1alpha1.MachineDeploymentAvailable,
					Status: machinev1alpha1.ConditionTrue,
				},
			}},
		}, BeNil()),
		Entry("unhealthy without available", &machinev1alpha1.MachineDeployment{}, HaveOccurred()),
		Entry("unhealthy with false available", &machinev1alpha1.MachineDeployment{
			Status: machinev1alpha1.MachineDeploymentStatus{Conditions: []machinev1alpha1.MachineDeploymentCondition{
				{
					Type:   machinev1alpha1.MachineDeploymentAvailable,
					Status: machinev1alpha1.ConditionFalse,
				},
				{
					Type:   machinev1alpha1.MachineDeploymentProgressing,
					Status: machinev1alpha1.ConditionTrue,
				},
			}},
		}, HaveOccurred()),
		Entry("unhealthy with false progressing", &machinev1alpha1.MachineDeployment{
			Status: machinev1alpha1.MachineDeploymentStatus{Conditions: []machinev1alpha1.MachineDeploymentCondition{
				{
					Type:   machinev1alpha1.MachineDeploymentAvailable,
					Status: machinev1alpha1.ConditionTrue,
				},
				{
					Type:   machinev1alpha1.MachineDeploymentProgressing,
					Status: machinev1alpha1.ConditionFalse,
				},
			}},
		}, HaveOccurred()),
		Entry("unhealthy with bad condition", &machinev1alpha1.MachineDeployment{
			Status: machinev1alpha1.MachineDeploymentStatus{Conditions: []machinev1alpha1.MachineDeploymentCondition{
				{
					Type:   machinev1alpha1.MachineDeploymentAvailable,
					Status: machinev1alpha1.ConditionTrue,
				},
				{
					Type:   machinev1alpha1.MachineDeploymentProgressing,
					Status: machinev1alpha1.ConditionFalse,
				},
				{
					Type:   machinev1alpha1.MachineDeploymentReplicaFailure,
					Status: machinev1alpha1.ConditionTrue,
				},
			}},
		}, HaveOccurred()),
		Entry("not observed at latest version", &machinev1alpha1.MachineDeployment{
			ObjectMeta: metav1.ObjectMeta{Generation: 1},
		}, HaveOccurred()),
		Entry("not enough updated replicas", &machinev1alpha1.MachineDeployment{
			Spec: machinev1alpha1.MachineDeploymentSpec{Replicas: 1},
		}, HaveOccurred()),
	)

	Describe("#checkMachineDeploymentsHealthy", func() {
		It("should  return true for nil", func() {
			isHealthy, err := checkMachineDeploymentsHealthy(nil)

			Expect(isHealthy).To(BeTrue())
			Expect(err).To(Succeed())
		})

		It("should  return true for an empty list", func() {
			isHealthy, err := checkMachineDeploymentsHealthy([]machinev1alpha1.MachineDeployment{})

			Expect(isHealthy).To(BeTrue())
			Expect(err).To(Succeed())
		})

		It("should  return true when all machine deployments healthy", func() {
			machineDeployments := []machinev1alpha1.MachineDeployment{
				{
					Status: machinev1alpha1.MachineDeploymentStatus{
						Conditions: []machinev1alpha1.MachineDeploymentCondition{
							{
								Type:   machinev1alpha1.MachineDeploymentAvailable,
								Status: machinev1alpha1.ConditionTrue,
							},
							{
								Type:   machinev1alpha1.MachineDeploymentProgressing,
								Status: machinev1alpha1.ConditionTrue,
							},
						},
					},
				},
			}

			isHealthy, err := checkMachineDeploymentsHealthy(machineDeployments)

			Expect(isHealthy).To(BeTrue())
			Expect(err).To(Succeed())
		})

		It("should return an error due to failed machines", func() {
			var (
				machineName        = "foo"
				machineDescription = "error"
				machineDeployments = []machinev1alpha1.MachineDeployment{
					{
						Status: machinev1alpha1.MachineDeploymentStatus{
							FailedMachines: []*machinev1alpha1.MachineSummary{
								{
									Name:          machineName,
									LastOperation: machinev1alpha1.LastOperation{Description: machineDescription},
								},
							},
						},
					},
				}
			)

			isHealthy, err := checkMachineDeploymentsHealthy(machineDeployments)

			Expect(isHealthy).To(BeFalse())
			Expect(err).ToNot(Succeed())
		})

		It("should return an error because machine deployment is not available", func() {
			machineDeployments := []machinev1alpha1.MachineDeployment{
				{
					Status: machinev1alpha1.MachineDeploymentStatus{
						Conditions: []machinev1alpha1.MachineDeploymentCondition{
							{
								Type:   machinev1alpha1.MachineDeploymentAvailable,
								Status: machinev1alpha1.ConditionFalse,
							},
						},
					},
				},
			}

			isHealthy, err := checkMachineDeploymentsHealthy(machineDeployments)

			Expect(isHealthy).To(BeFalse())
			Expect(err).ToNot(Succeed())
		})
	})

	Describe("#checkNodesScalingUp", func() {
		It("should return true if number of ready nodes equal number of desired machines", func() {
			status, err := checkNodesScalingUp(nil, 1, 1)

			Expect(status).To(Equal(gardencorev1beta1.ConditionTrue))
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return an error if not enough machine objects as desired were created", func() {
			status, err := checkNodesScalingUp(&machinev1alpha1.MachineList{}, 0, 1)

			Expect(status).To(Equal(gardencorev1beta1.ConditionFalse))
			Expect(err).To(HaveOccurred())
		})

		It("should return an error when detecting erroneous machines", func() {
			machineList := &machinev1alpha1.MachineList{
				Items: []machinev1alpha1.Machine{
					{
						Status: machinev1alpha1.MachineStatus{
							CurrentStatus: machinev1alpha1.CurrentStatus{Phase: machinev1alpha1.MachineUnknown},
						},
					},
				},
			}

			status, err := checkNodesScalingUp(machineList, 0, 1)

			Expect(status).To(Equal(gardencorev1beta1.ConditionFalse))
			Expect(err).To(HaveOccurred())
		})

		It("should return an error when not detecting erroneous machines", func() {
			machineList := &machinev1alpha1.MachineList{
				Items: []machinev1alpha1.Machine{
					{
						Status: machinev1alpha1.MachineStatus{
							CurrentStatus: machinev1alpha1.CurrentStatus{Phase: machinev1alpha1.MachineRunning},
						},
					},
				},
			}

			status, err := checkNodesScalingUp(machineList, 0, 1)

			Expect(status).To(Equal(gardencorev1beta1.ConditionFalse))
			Expect(err).To(HaveOccurred())
		})

		It("should return progressing when detecting a regular scale up (pending status)", func() {
			machineList := &machinev1alpha1.MachineList{
				Items: []machinev1alpha1.Machine{
					{
						Status: machinev1alpha1.MachineStatus{
							CurrentStatus: machinev1alpha1.CurrentStatus{Phase: machinev1alpha1.MachinePending},
						},
					},
				},
			}

			status, err := checkNodesScalingUp(machineList, 0, 1)

			Expect(status).To(Equal(gardencorev1beta1.ConditionProgressing))
			Expect(err).To(HaveOccurred())
		})

		It("should return progressing when detecting a regular scale up (no status)", func() {
			machineList := &machinev1alpha1.MachineList{
				Items: []machinev1alpha1.Machine{
					{},
				},
			}

			status, err := checkNodesScalingUp(machineList, 0, 1)

			Expect(status).To(Equal(gardencorev1beta1.ConditionProgressing))
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#checkNodesScalingDown", func() {
		It("should return true if number of registered nodes equal number of desired machines", func() {
			status, err := checkNodesScalingDown(nil, nil, 1, 1)

			Expect(status).To(Equal(gardencorev1beta1.ConditionTrue))
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return an error if the machine for a cordoned node is not found", func() {
			nodeList := &corev1.NodeList{
				Items: []corev1.Node{
					{Spec: corev1.NodeSpec{Unschedulable: true}},
				},
			}

			status, err := checkNodesScalingDown(&machinev1alpha1.MachineList{}, nodeList, 2, 1)

			Expect(status).To(Equal(gardencorev1beta1.ConditionFalse))
			Expect(err).To(HaveOccurred())
		})

		It("should return an error if the machine for a cordoned node is not deleted", func() {
			var (
				nodeName = "foo"

				machineList = &machinev1alpha1.MachineList{
					Items: []machinev1alpha1.Machine{
						{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"node": nodeName}}},
					},
				}
				nodeList = &corev1.NodeList{
					Items: []corev1.Node{
						{
							ObjectMeta: metav1.ObjectMeta{Name: nodeName},
							Spec:       corev1.NodeSpec{Unschedulable: true},
						},
					},
				}
			)

			status, err := checkNodesScalingDown(machineList, nodeList, 2, 1)

			Expect(status).To(Equal(gardencorev1beta1.ConditionFalse))
			Expect(err).To(HaveOccurred())
		})

		It("should return an error if there are more nodes then machines", func() {
			status, err := checkNodesScalingDown(&machinev1alpha1.MachineList{}, &corev1.NodeList{Items: []corev1.Node{{}}}, 2, 1)

			Expect(status).To(Equal(gardencorev1beta1.ConditionFalse))
			Expect(err).To(HaveOccurred())
		})

		It("should return progressing for a regular scale down", func() {
			var (
				nodeName          = "foo"
				deletionTimestamp = metav1.Now()

				machineList = &machinev1alpha1.MachineList{
					Items: []machinev1alpha1.Machine{
						{ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: &deletionTimestamp, Labels: map[string]string{"node": nodeName}}},
					},
				}
				nodeList = &corev1.NodeList{
					Items: []corev1.Node{
						{
							ObjectMeta: metav1.ObjectMeta{Name: nodeName},
							Spec:       corev1.NodeSpec{Unschedulable: true},
						},
					},
				}
			)

			status, err := checkNodesScalingDown(machineList, nodeList, 2, 1)

			Expect(status).To(Equal(gardencorev1beta1.ConditionProgressing))
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#getDesiredMachineCount", func() {
		It("should return zero for nil", func() {
			Expect(getDesiredMachineCount(nil)).To(BeZero())
		})

		It("should return zero for an empty list", func() {
			Expect(getDesiredMachineCount([]machinev1alpha1.MachineDeployment{})).To(BeZero())
		})

		It("should return the correct machine count", func() {
			var (
				deletionTimestamp  = metav1.Now()
				machineDeployments = []machinev1alpha1.MachineDeployment{
					{Spec: machinev1alpha1.MachineDeploymentSpec{Replicas: 4}},
					{Spec: machinev1alpha1.MachineDeploymentSpec{Replicas: 5}},
					{
						ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: &deletionTimestamp},
						Spec:       machinev1alpha1.MachineDeploymentSpec{Replicas: 6},
					},
				}
			)

			Expect(getDesiredMachineCount(machineDeployments)).To(Equal(9))
		})
	})
})
