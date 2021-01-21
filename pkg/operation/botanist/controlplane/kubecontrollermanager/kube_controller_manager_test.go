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

package kubecontrollermanager_test

import (
	"context"
	"fmt"
	"net"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	. "github.com/gardener/gardener/pkg/operation/botanist/controlplane/kubecontrollermanager"
	"github.com/gardener/gardener/pkg/operation/common"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	versionutils "github.com/gardener/gardener/pkg/utils/version"

	"github.com/Masterminds/semver"
	resourcesv1alpha1 "github.com/gardener/gardener-resource-manager/pkg/apis/resources/v1alpha1"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	autoscalingv1beta2 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1beta2"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("KubeControllerManager", func() {
	var (
		ctx                   = context.TODO()
		testLogger            = logrus.NewEntry(logger.NewNopLogger())
		ctrl                  *gomock.Controller
		c                     *mockclient.MockClient
		kubeControllerManager KubeControllerManager

		_, podCIDR, _     = net.ParseCIDR("100.96.0.0/11")
		_, serviceCIDR, _ = net.ParseCIDR("100.64.0.0/13")
		fakeErr           = fmt.Errorf("fake error")
		namespace         = "shoot--foo--bar"
		version           = "1.17.2"
		semverVersion, _  = semver.NewVersion(version)
		image             = "k8s.gcr.io/kube-controller-manager:v1.17.2"

		hpaConfig = gardencorev1beta1.HorizontalPodAutoscalerConfig{
			CPUInitializationPeriod: &metav1.Duration{Duration: 5 * time.Minute},
			DownscaleDelay:          &metav1.Duration{Duration: 15 * time.Minute},
			DownscaleStabilization:  &metav1.Duration{Duration: 5 * time.Minute},
			InitialReadinessDelay:   &metav1.Duration{Duration: 30 * time.Second},
			SyncPeriod:              &metav1.Duration{Duration: 30 * time.Second},
			Tolerance:               pointer.Float64Ptr(0.1),
			UpscaleDelay:            &metav1.Duration{Duration: 1 * time.Minute},
		}

		nodeCIDRMask       int32 = 24
		podEvictionTimeout       = metav1.Duration{Duration: 3 * time.Minute}
		kcmConfig                = gardencorev1beta1.KubeControllerManagerConfig{
			KubernetesConfig:              gardencorev1beta1.KubernetesConfig{},
			HorizontalPodAutoscalerConfig: &hpaConfig,
			NodeCIDRMaskSize:              &nodeCIDRMask,
			PodEvictionTimeout:            &podEvictionTimeout,
		}

		// checksums
		secretChecksumKubeconfig        = "1234"
		secretChecksumServer            = "5678"
		secretChecksumCA                = "1234"
		secretChecksumServiceAccountKey = "1234"

		// vpa
		vpaName             = "kube-controller-manager-vpa"
		managedResourceName = "shoot-core-kube-controller-manager"
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Deploy", func() {
		Context("Tests expecting a failure", func() {
			BeforeEach(func() {
				kubeControllerManager = New(
					testLogger,
					c,
					namespace,
					semverVersion,
					image,
					&kcmConfig,
					podCIDR,
					serviceCIDR,
				)
			})

			Context("missing secret information", func() {
				It("should return an error because the kubeconfig secret information is not provided", func() {
					Expect(kubeControllerManager.Deploy(ctx)).To(MatchError(ContainSubstring("missing kubeconfig secret information")))
				})

				It("should return an error because the server secret information is not provided", func() {
					kubeControllerManager.SetSecrets(Secrets{Kubeconfig: component.Secret{Name: "kube-controller-manager", Checksum: secretChecksumKubeconfig}})
					Expect(kubeControllerManager.Deploy(ctx)).To(MatchError(ContainSubstring("missing server secret information")))
				})

				It("should return an error because the CA secret information is not provided", func() {
					kubeControllerManager.SetSecrets(Secrets{
						Kubeconfig: component.Secret{Name: "kube-controller-manager", Checksum: secretChecksumKubeconfig},
						Server:     component.Secret{Name: "kube-controller-manager-server", Checksum: secretChecksumServer},
					})
					Expect(kubeControllerManager.Deploy(ctx)).To(MatchError(ContainSubstring("missing CA secret information")))
				})

				It("should return an error because the ServiceAccountKey secret information is not provided", func() {
					kubeControllerManager.SetSecrets(Secrets{
						Kubeconfig: component.Secret{Name: "kube-controller-manager", Checksum: secretChecksumKubeconfig},
						Server:     component.Secret{Name: "kube-controller-manager-server", Checksum: secretChecksumServer},
						CA:         component.Secret{Name: "ca", Checksum: secretChecksumCA},
					})
					Expect(kubeControllerManager.Deploy(ctx)).To(MatchError(ContainSubstring("missing ServiceAccountKey secret information")))
				})
			})
			Context("secret information available", func() {
				BeforeEach(func() {
					kubeControllerManager.SetSecrets(Secrets{
						Kubeconfig:        component.Secret{Name: "kube-controller-manager", Checksum: secretChecksumKubeconfig},
						Server:            component.Secret{Name: "kube-controller-manager-server", Checksum: secretChecksumServer},
						CA:                component.Secret{Name: "ca", Checksum: secretChecksumCA},
						ServiceAccountKey: component.Secret{Name: "service-account-key", Checksum: secretChecksumServiceAccountKey},
					})
				})

				It("should fail when the service cannot be created", func() {
					gomock.InOrder(
						c.EXPECT().Get(ctx, kutil.Key(namespace, ServiceName), gomock.AssignableToTypeOf(&corev1.Service{})),
						c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Service{})).Return(fakeErr),
					)

					Expect(kubeControllerManager.Deploy(ctx)).To(MatchError(fakeErr))
				})

				It("should fail because the deployment cannot be created", func() {
					gomock.InOrder(
						c.EXPECT().Get(ctx, kutil.Key(namespace, ServiceName), gomock.AssignableToTypeOf(&corev1.Service{})),
						c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Service{})),
						c.EXPECT().Get(ctx, kutil.Key(namespace, v1beta1constants.DeploymentNameKubeControllerManager), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
						c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{})).Return(fakeErr),
					)

					Expect(kubeControllerManager.Deploy(ctx)).To(MatchError(fakeErr))
				})

				It("should fail because the vpa cannot be created", func() {
					gomock.InOrder(
						c.EXPECT().Get(ctx, kutil.Key(namespace, ServiceName), gomock.AssignableToTypeOf(&corev1.Service{})),
						c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Service{})),
						c.EXPECT().Get(ctx, kutil.Key(namespace, v1beta1constants.DeploymentNameKubeControllerManager), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
						c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{})),
						c.EXPECT().Get(ctx, kutil.Key(namespace, vpaName), gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{})),
						c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{})).Return(fakeErr),
					)

					Expect(kubeControllerManager.Deploy(ctx)).To(MatchError(fakeErr))
				})

				It("should fail because the managed resource cannot be deleted", func() {
					gomock.InOrder(
						c.EXPECT().Get(ctx, kutil.Key(namespace, ServiceName), gomock.AssignableToTypeOf(&corev1.Service{})),
						c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Service{})),
						c.EXPECT().Get(ctx, kutil.Key(namespace, v1beta1constants.DeploymentNameKubeControllerManager), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
						c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{})),
						c.EXPECT().Get(ctx, kutil.Key(namespace, vpaName), gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{})),
						c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{})),
						c.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: managedResourceName, Namespace: namespace}}).Return(fakeErr),
					)

					Expect(kubeControllerManager.Deploy(ctx)).To(MatchError(fakeErr))
				})

				It("should fail because the managed resource secret cannot be deleted", func() {
					gomock.InOrder(
						c.EXPECT().Get(ctx, kutil.Key(namespace, ServiceName), gomock.AssignableToTypeOf(&corev1.Service{})),
						c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Service{})),
						c.EXPECT().Get(ctx, kutil.Key(namespace, v1beta1constants.DeploymentNameKubeControllerManager), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
						c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{})),
						c.EXPECT().Get(ctx, kutil.Key(namespace, vpaName), gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{})),
						c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{})),
						c.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: managedResourceName, Namespace: namespace}}),
						c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: common.ManagedResourceSecretName(managedResourceName), Namespace: namespace}}).Return(fakeErr),
					)

					Expect(kubeControllerManager.Deploy(ctx)).To(MatchError(fakeErr))
				})

				It("should fail because the managed resource secret cannot be updated (k8s version 1.12)", func() {
					semverVersion112, err := semver.NewVersion("1.12.4")
					Expect(err).NotTo(HaveOccurred())

					kubeControllerManager = New(
						testLogger,
						c,
						namespace,
						semverVersion112,
						image,
						&kcmConfig,
						podCIDR,
						serviceCIDR,
					)

					kubeControllerManager.SetSecrets(Secrets{
						Kubeconfig:        component.Secret{Name: "kube-controller-manager", Checksum: secretChecksumKubeconfig},
						Server:            component.Secret{Name: "kube-controller-manager-server", Checksum: secretChecksumServer},
						CA:                component.Secret{Name: "ca", Checksum: secretChecksumCA},
						ServiceAccountKey: component.Secret{Name: "service-account-key", Checksum: secretChecksumServiceAccountKey},
					})

					gomock.InOrder(
						c.EXPECT().Get(ctx, kutil.Key(namespace, ServiceName), gomock.AssignableToTypeOf(&corev1.Service{})),
						c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Service{})),
						c.EXPECT().Get(ctx, kutil.Key(namespace, v1beta1constants.DeploymentNameKubeControllerManager), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
						c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{})),
						c.EXPECT().Get(ctx, kutil.Key(namespace, vpaName), gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{})),
						c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{})),
						c.EXPECT().Get(ctx, kutil.Key(namespace, common.ManagedResourceSecretName(managedResourceName)), gomock.AssignableToTypeOf(&corev1.Secret{})),
						c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Return(fakeErr),
					)

					Expect(kubeControllerManager.Deploy(ctx)).To(MatchError(fakeErr))
				})

				It("should fail because the managed resource cannot be updated (k8s version 1.13)", func() {
					semverVersion113, err := semver.NewVersion("1.13.4")
					Expect(err).NotTo(HaveOccurred())

					kubeControllerManager = New(
						testLogger,
						c,
						namespace,
						semverVersion113,
						image,
						&kcmConfig,
						podCIDR,
						serviceCIDR,
					)
					kubeControllerManager.SetSecrets(Secrets{
						Kubeconfig:        component.Secret{Name: "kube-controller-manager", Checksum: secretChecksumKubeconfig},
						Server:            component.Secret{Name: "kube-controller-manager-server", Checksum: secretChecksumServer},
						CA:                component.Secret{Name: "ca", Checksum: secretChecksumCA},
						ServiceAccountKey: component.Secret{Name: "service-account-key", Checksum: secretChecksumServiceAccountKey},
					})

					gomock.InOrder(
						c.EXPECT().Get(ctx, kutil.Key(namespace, ServiceName), gomock.AssignableToTypeOf(&corev1.Service{})),
						c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Service{})),
						c.EXPECT().Get(ctx, kutil.Key(namespace, v1beta1constants.DeploymentNameKubeControllerManager), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
						c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{})),
						c.EXPECT().Get(ctx, kutil.Key(namespace, vpaName), gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{})),
						c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{})),
						c.EXPECT().Get(ctx, kutil.Key(namespace, common.ManagedResourceSecretName(managedResourceName)), gomock.AssignableToTypeOf(&corev1.Secret{})),
						c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})),
						c.EXPECT().Get(ctx, kutil.Key(namespace, managedResourceName), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
						c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(fakeErr),
					)

					Expect(kubeControllerManager.Deploy(ctx)).To(MatchError(fakeErr))
				})
			})
		})

		Context("Tests expecting success", func() {
			var (
				vpaUpdateMode = autoscalingv1beta2.UpdateModeAuto
				vpa           = &autoscalingv1beta2.VerticalPodAutoscaler{
					ObjectMeta: metav1.ObjectMeta{Name: vpaName, Namespace: namespace},
					Spec: autoscalingv1beta2.VerticalPodAutoscalerSpec{
						TargetRef: &autoscalingv1.CrossVersionObjectReference{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Name:       v1beta1constants.DeploymentNameKubeControllerManager,
						},
						UpdatePolicy: &autoscalingv1beta2.PodUpdatePolicy{
							UpdateMode: &vpaUpdateMode,
						},
						ResourcePolicy: &autoscalingv1beta2.PodResourcePolicy{
							ContainerPolicies: []autoscalingv1beta2.ContainerResourcePolicy{{
								ContainerName: "kube-controller-manager",
								MinAllowed: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("100Mi"),
								},
							}},
						},
					},
				}

				serviceFor = func(version string) *corev1.Service {
					return &corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Name:      ServiceName,
							Namespace: namespace,
							Labels: map[string]string{
								"app":  "kubernetes",
								"role": "controller-manager",
							},
						},
						Spec: corev1.ServiceSpec{
							Selector: map[string]string{
								"app":  "kubernetes",
								"role": "controller-manager",
							},
							Type:      corev1.ServiceTypeClusterIP,
							ClusterIP: corev1.ClusterIPNone,
							Ports: []corev1.ServicePort{
								{
									Name:     "metrics",
									Protocol: corev1.ProtocolTCP,
									Port:     portForKubernetesVersion(version),
								},
							},
						},
					}
				}

				replicas      int32 = 1
				deploymentFor       = func(version string, config *gardencorev1beta1.KubeControllerManagerConfig) *appsv1.Deployment {
					return &appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{
							Name:      v1beta1constants.DeploymentNameKubeControllerManager,
							Namespace: namespace,
							Labels: map[string]string{
								"app":                 "kubernetes",
								"role":                "controller-manager",
								"gardener.cloud/role": "controlplane",
							},
						},
						Spec: appsv1.DeploymentSpec{
							RevisionHistoryLimit: pointer.Int32Ptr(1),
							Replicas:             &replicas,
							Selector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app":  "kubernetes",
									"role": "controller-manager",
								},
							},
							Template: corev1.PodTemplateSpec{
								ObjectMeta: metav1.ObjectMeta{
									Annotations: map[string]string{
										"checksum/secret-ca":                             secretChecksumCA,
										"checksum/secret-service-account-key":            secretChecksumServiceAccountKey,
										"checksum/secret-kube-controller-manager":        secretChecksumKubeconfig,
										"checksum/secret-kube-controller-manager-server": secretChecksumServer,
									},
									Labels: map[string]string{
										"app":                                "kubernetes",
										"role":                               "controller-manager",
										"gardener.cloud/role":                "controlplane",
										"garden.sapcloud.io/role":            "controlplane",
										"maintenance.gardener.cloud/restart": "true",
										"networking.gardener.cloud/to-dns":   "allowed",
										"networking.gardener.cloud/to-shoot-apiserver": "allowed",
										"networking.gardener.cloud/from-prometheus":    "allowed",
									},
								},
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{
										{
											Name:            "kube-controller-manager",
											Image:           image,
											ImagePullPolicy: corev1.PullIfNotPresent,
											Command:         commandForKubernetesVersion(version, portForKubernetesVersion(version), config.NodeCIDRMaskSize, config.PodEvictionTimeout, namespace, serviceCIDR, podCIDR, getHorizontalPodAutoscalerConfig(config.HorizontalPodAutoscalerConfig), kutil.FeatureGatesToCommandLineParameter(config.FeatureGates)),
											LivenessProbe: &corev1.Probe{
												Handler: corev1.Handler{
													HTTPGet: &corev1.HTTPGetAction{
														Path:   "/healthz",
														Scheme: probeSchemeForKubernetesVersion(version),
														Port:   intstr.FromInt(int(portForKubernetesVersion(version))),
													},
												},
												SuccessThreshold:    1,
												FailureThreshold:    2,
												InitialDelaySeconds: 15,
												PeriodSeconds:       10,
												TimeoutSeconds:      15,
											},
											Ports: []corev1.ContainerPort{
												{
													Name:          "metrics",
													ContainerPort: portForKubernetesVersion(version),
													Protocol:      corev1.ProtocolTCP,
												},
											},
											Resources: corev1.ResourceRequirements{
												Requests: corev1.ResourceList{
													corev1.ResourceCPU:    resource.MustParse("100m"),
													corev1.ResourceMemory: resource.MustParse("128Mi"),
												},
												Limits: corev1.ResourceList{
													corev1.ResourceCPU:    resource.MustParse("400m"),
													corev1.ResourceMemory: resource.MustParse("512Mi"),
												},
											},
											VolumeMounts: []corev1.VolumeMount{
												{
													Name:      "ca",
													MountPath: "/srv/kubernetes/ca",
												},
												{
													Name:      "service-account-key",
													MountPath: "/srv/kubernetes/service-account-key",
												},
												{
													Name:      "kube-controller-manager",
													MountPath: "/var/lib/kube-controller-manager",
												},
												{
													Name:      "kube-controller-manager-server",
													MountPath: "/var/lib/kube-controller-manager-server",
												},
											},
										},
									},
									Volumes: []corev1.Volume{
										{
											Name: "ca",
											VolumeSource: corev1.VolumeSource{
												Secret: &corev1.SecretVolumeSource{
													SecretName: "ca",
												},
											},
										},
										{
											Name: "service-account-key",
											VolumeSource: corev1.VolumeSource{
												Secret: &corev1.SecretVolumeSource{
													SecretName: "service-account-key",
												},
											},
										},
										{
											Name: "kube-controller-manager",
											VolumeSource: corev1.VolumeSource{
												Secret: &corev1.SecretVolumeSource{
													SecretName: "kube-controller-manager",
												},
											},
										},
										{
											Name: "kube-controller-manager-server",
											VolumeSource: corev1.VolumeSource{
												Secret: &corev1.SecretVolumeSource{
													SecretName: "kube-controller-manager-server",
												},
											},
										},
									},
								},
							},
						},
					}
				}

				clusterRoleBindingYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  creationTimestamp: null
  name: system:controller:kube-controller-manager
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:auth-delegator
subjects:
- kind: User
  name: system:kube-controller-manager
`
				roleBindingYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  creationTimestamp: null
  name: system:controller:kube-controller-manager:auth-reader
  namespace: kube-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: extension-apiserver-authentication-reader
subjects:
- kind: User
  name: system:kube-controller-manager
`

				managedResourceSecret = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      common.ManagedResourceSecretName(managedResourceName),
						Namespace: namespace,
					},
					Type: corev1.SecretTypeOpaque,
					Data: map[string][]byte{
						"clusterrolebinding____system_controller_kube-controller-manager.yaml":                 []byte(clusterRoleBindingYAML),
						"rolebinding__kube-system__system_controller_kube-controller-manager_auth-reader.yaml": []byte(roleBindingYAML),
					},
				}
				managedResource = &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      managedResourceName,
						Namespace: namespace,
						Labels:    map[string]string{"origin": "gardener"},
					},
					Spec: resourcesv1alpha1.ManagedResourceSpec{
						SecretRefs: []corev1.LocalObjectReference{
							{Name: common.ManagedResourceSecretName(managedResourceName)},
						},
						InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
						KeepObjects:  pointer.BoolPtr(false),
					},
				}

				emptyConfig                = &gardencorev1beta1.KubeControllerManagerConfig{}
				configWithAutoscalerConfig = &gardencorev1beta1.KubeControllerManagerConfig{
					// non default configuration
					HorizontalPodAutoscalerConfig: &gardencorev1beta1.HorizontalPodAutoscalerConfig{
						CPUInitializationPeriod: &metav1.Duration{Duration: 10 * time.Minute},
						DownscaleDelay:          &metav1.Duration{Duration: 10 * time.Minute},
						DownscaleStabilization:  &metav1.Duration{Duration: 10 * time.Minute},
						InitialReadinessDelay:   &metav1.Duration{Duration: 20 * time.Second},
						SyncPeriod:              &metav1.Duration{Duration: 20 * time.Second},
						Tolerance:               pointer.Float64Ptr(0.3),
						UpscaleDelay:            &metav1.Duration{Duration: 2 * time.Minute},
					},
					NodeCIDRMaskSize: nil,
				}
				configWithFeatureFlags       = &gardencorev1beta1.KubeControllerManagerConfig{KubernetesConfig: gardencorev1beta1.KubernetesConfig{FeatureGates: map[string]bool{"Foo": true, "Bar": false, "Baz": false}}}
				configWithNodeCIDRMaskSize   = &gardencorev1beta1.KubeControllerManagerConfig{NodeCIDRMaskSize: pointer.Int32Ptr(26)}
				configWithPodEvictionTimeout = &gardencorev1beta1.KubeControllerManagerConfig{PodEvictionTimeout: &podEvictionTimeout}
			)

			DescribeTable("success tests for various kubernetes versions",
				func(version string, config *gardencorev1beta1.KubeControllerManagerConfig) {
					semverVersion, err := semver.NewVersion(version)
					Expect(err).NotTo(HaveOccurred())

					kubeControllerManager = New(
						testLogger,
						c,
						namespace,
						semverVersion,
						image,
						config,
						podCIDR,
						serviceCIDR,
					)

					kubeControllerManager.SetSecrets(Secrets{
						Kubeconfig:        component.Secret{Name: "kube-controller-manager", Checksum: secretChecksumKubeconfig},
						Server:            component.Secret{Name: "kube-controller-manager-server", Checksum: secretChecksumServer},
						CA:                component.Secret{Name: "ca", Checksum: secretChecksumCA},
						ServiceAccountKey: component.Secret{Name: "service-account-key", Checksum: secretChecksumServiceAccountKey},
					})

					kubeControllerManager.SetReplicaCount(replicas)

					gomock.InOrder(
						c.EXPECT().Get(ctx, kutil.Key(namespace, ServiceName), gomock.AssignableToTypeOf(&corev1.Service{})),
						c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Service{})).Do(func(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) {
							Expect(obj).To(DeepEqual(serviceFor(version)))
						}),
						c.EXPECT().Get(ctx, kutil.Key(namespace, v1beta1constants.DeploymentNameKubeControllerManager), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
						c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{})).Do(func(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) {
							Expect(obj).To(DeepEqual(deploymentFor(version, config)))
						}),
						c.EXPECT().Get(ctx, kutil.Key(namespace, vpaName), gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{})),
						c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{})).Do(func(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) {
							Expect(obj).To(DeepEqual(vpa))
						}),
					)

					is112Version, _ := versionutils.CompareVersions(version, "~", "1.12")
					is113Version, _ := versionutils.CompareVersions(version, "~", "1.13")

					if is112Version || is113Version {
						gomock.InOrder(
							c.EXPECT().Get(ctx, kutil.Key(namespace, common.ManagedResourceSecretName(managedResourceName)), gomock.AssignableToTypeOf(&corev1.Secret{})),
							c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Do(func(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) {
								Expect(obj).To(DeepEqual(managedResourceSecret))
							}),
							c.EXPECT().Get(ctx, kutil.Key(namespace, managedResourceName), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
							c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Do(func(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) {
								Expect(obj).To(DeepEqual(managedResource))
							}),
						)
					} else {
						gomock.InOrder(
							c.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: managedResourceName, Namespace: namespace}}),
							c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: common.ManagedResourceSecretName(managedResourceName), Namespace: namespace}}),
						)
					}

					Expect(kubeControllerManager.Deploy(ctx)).To(Succeed())
				},
				Entry("kubernetes 1.19 w/o config", "1.19.0", emptyConfig),
				Entry("kubernetes 1.19 with non-default autoscaler config", "1.19.0", configWithAutoscalerConfig),
				Entry("kubernetes 1.19 with feature flags", "1.19.0", configWithFeatureFlags),
				Entry("kubernetes 1.19 with NodeCIDRMaskSize", "1.19.0", configWithNodeCIDRMaskSize),
				Entry("kubernetes 1.19 with PodEvictionTimeout", "1.19.0", configWithPodEvictionTimeout),

				Entry("kubernetes 1.18 w/o config", "1.18.0", emptyConfig),
				Entry("kubernetes 1.18 with non-default autoscaler config", "1.18.0", configWithAutoscalerConfig),
				Entry("kubernetes 1.18 with feature flags", "1.18.0", configWithFeatureFlags),
				Entry("kubernetes 1.18 with NodeCIDRMaskSize", "1.18.0", configWithNodeCIDRMaskSize),
				Entry("kubernetes 1.18 with PodEvictionTimeout", "1.18.0", configWithPodEvictionTimeout),

				Entry("kubernetes 1.17 w/o config", "1.17.0", emptyConfig),
				Entry("kubernetes 1.17 with non-default autoscaler config", "1.17.0", configWithAutoscalerConfig),
				Entry("kubernetes 1.17 with feature flags", "1.17.0", configWithFeatureFlags),
				Entry("kubernetes 1.17 with NodeCIDRMaskSize", "1.17.0", configWithNodeCIDRMaskSize),
				Entry("kubernetes 1.17 with PodEvictionTimeout", "1.17.0", configWithPodEvictionTimeout),

				Entry("kubernetes 1.16 w/o config", "1.16.0", emptyConfig),
				Entry("kubernetes 1.16 with non-default autoscaler config", "1.16.0", configWithAutoscalerConfig),
				Entry("kubernetes 1.16 with feature flags", "1.16.0", configWithFeatureFlags),
				Entry("kubernetes 1.16 with NodeCIDRMaskSize", "1.16.0", configWithNodeCIDRMaskSize),
				Entry("kubernetes 1.16 with PodEvictionTimeout", "1.16.0", configWithPodEvictionTimeout),

				Entry("kubernetes 1.15 w/o config", "1.15.0", emptyConfig),
				Entry("kubernetes 1.15 with non-default autoscaler config", "1.15.0", configWithAutoscalerConfig),
				Entry("kubernetes 1.15 with feature flags", "1.15.0", configWithFeatureFlags),
				Entry("kubernetes 1.15 with NodeCIDRMaskSize", "1.15.0", configWithNodeCIDRMaskSize),
				Entry("kubernetes 1.15 with PodEvictionTimeout", "1.15.0", configWithPodEvictionTimeout),

				Entry("kubernetes 1.14 w/o config", "1.14.0", emptyConfig),
				Entry("kubernetes 1.14 with non-default autoscaler config", "1.14.0", configWithAutoscalerConfig),
				Entry("kubernetes 1.14 with feature flags", "1.14.0", configWithFeatureFlags),
				Entry("kubernetes 1.14 with NodeCIDRMaskSize", "1.14.0", configWithNodeCIDRMaskSize),
				Entry("kubernetes 1.14 with PodEvictionTimeout", "1.14.0", configWithPodEvictionTimeout),

				Entry("kubernetes 1.13 w/o config", "1.13.0", emptyConfig),
				Entry("kubernetes 1.13 with non-default autoscaler config", "1.13.0", configWithAutoscalerConfig),
				Entry("kubernetes 1.13 with feature flags", "1.13.0", configWithFeatureFlags),
				Entry("kubernetes 1.13 with NodeCIDRMaskSize", "1.13.0", configWithNodeCIDRMaskSize),
				Entry("kubernetes 1.13 with PodEvictionTimeout", "1.13.0", configWithPodEvictionTimeout),

				Entry("kubernetes 1.12 w/o config", "1.12.0", emptyConfig),
				Entry("kubernetes 1.12 with non-default autoscaler config", "1.12.0", configWithAutoscalerConfig),
				Entry("kubernetes 1.12 with feature flags", "1.12.0", configWithFeatureFlags),
				Entry("kubernetes 1.12 with NodeCIDRMaskSize", "1.12.0", configWithNodeCIDRMaskSize),
				Entry("kubernetes 1.12 with PodEvictionTimeout", "1.12.0", configWithPodEvictionTimeout),

				Entry("kubernetes 1.11 w/ config", "1.11.0", emptyConfig),
				Entry("kubernetes 1.11 with non-default autoscaler config", "1.11.0", configWithAutoscalerConfig),
				Entry("kubernetes 1.11 with feature flags", "1.11.0", configWithFeatureFlags),
				Entry("kubernetes 1.11 with NodeCIDRMaskSize", "1.11.0", configWithNodeCIDRMaskSize),
				Entry("kubernetes 1.11 with PodEvictionTimeout", "1.11.0", configWithPodEvictionTimeout),
			)
		})
	})

	Describe("#Destroy", func() {
		It("should return nil as it's not implemented as of now", func() {
			Expect(kubeControllerManager.Destroy(ctx)).To(Succeed())
		})
	})

	Describe("#Wait", func() {
		It("should return nil as it's not implemented as of now", func() {
			Expect(kubeControllerManager.Wait(ctx)).To(Succeed())
		})
	})

	Describe("#WaitCleanup", func() {
		It("should return nil as it's not implemented as of now", func() {
			Expect(kubeControllerManager.WaitCleanup(ctx)).To(Succeed())
		})
	})
})

// Utility functions

func portForKubernetesVersion(version string) int32 {
	if k8sVersionGreaterEqual113, _ := versionutils.CompareVersions(version, ">=", "1.13"); k8sVersionGreaterEqual113 {
		return 10257
	}
	return 10252
}

func probeSchemeForKubernetesVersion(version string) corev1.URIScheme {
	if k8sVersionGreaterEqual113, _ := versionutils.CompareVersions(version, ">=", "1.13"); k8sVersionGreaterEqual113 {
		return corev1.URISchemeHTTPS
	}
	return corev1.URISchemeHTTP
}

func commandForKubernetesVersion(
	version string,
	port int32,
	nodeCIDRMaskSize *int32,
	podEvictionTimeout *metav1.Duration,
	clusterName string,
	serviceNetwork, podNetwork *net.IPNet,
	horizontalPodAutoscalerConfig *gardencorev1beta1.HorizontalPodAutoscalerConfig,
	featureGateFlags string,
) []string {
	var command []string

	if k8sVersionGreaterEqual117, _ := versionutils.CompareVersions(version, ">=", "1.17"); k8sVersionGreaterEqual117 {
		command = append(command, "/usr/local/bin/kube-controller-manager")
	} else if k8sVersionGreaterEqual115, _ := versionutils.CompareVersions(version, ">=", "1.15"); k8sVersionGreaterEqual115 {
		command = append(command, "/hyperkube", "kube-controller-manager")
	} else {
		command = append(command, "/hyperkube", "controller-manager")
	}

	command = append(command,
		"--allocate-node-cidrs=true",
		"--attach-detach-reconcile-sync-period=1m0s",
		"--controllers=*,bootstrapsigner,tokencleaner",
	)

	if nodeCIDRMaskSize != nil {
		command = append(command, fmt.Sprintf("--node-cidr-mask-size=%d", *nodeCIDRMaskSize))
	}

	command = append(command,
		fmt.Sprintf("--cluster-cidr=%s", podNetwork.String()),
		fmt.Sprintf("--cluster-name=%s", clusterName),
		"--cluster-signing-cert-file=/srv/kubernetes/ca/ca.crt",
		"--cluster-signing-key-file=/srv/kubernetes/ca/ca.key",
		"--concurrent-deployment-syncs=50",
		"--concurrent-endpoint-syncs=15",
		"--concurrent-gc-syncs=30",
		"--concurrent-namespace-syncs=50",
		"--concurrent-replicaset-syncs=50",
		"--concurrent-resource-quota-syncs=15",
	)

	if k8sVersionGreaterEqual116, _ := versionutils.CompareVersions(version, ">=", "1.16"); k8sVersionGreaterEqual116 {
		command = append(command,
			"--concurrent-service-endpoint-syncs=15",
			"--concurrent-statefulset-syncs=15",
		)
	}

	command = append(command, "--concurrent-serviceaccount-token-syncs=15")

	if len(featureGateFlags) > 0 {
		command = append(command, featureGateFlags)
	}

	if k8sVersionSmaller117, _ := versionutils.CompareVersions(version, "<", "1.12"); k8sVersionSmaller117 {
		command = append(command,
			fmt.Sprintf("--horizontal-pod-autoscaler-downscale-delay=%s", horizontalPodAutoscalerConfig.DownscaleDelay.Duration.String()),
			fmt.Sprintf("--horizontal-pod-autoscaler-upscale-delay=%s", horizontalPodAutoscalerConfig.UpscaleDelay.Duration.String()),
		)
	}

	podEvictionTimeoutSetting := "2m0s"
	if podEvictionTimeout != nil {
		podEvictionTimeoutSetting = podEvictionTimeout.Duration.String()
	}

	command = append(command,
		fmt.Sprintf("--horizontal-pod-autoscaler-sync-period=%s", horizontalPodAutoscalerConfig.SyncPeriod.Duration.String()),
		fmt.Sprintf("--horizontal-pod-autoscaler-tolerance=%v", *horizontalPodAutoscalerConfig.Tolerance),
		"--kubeconfig=/var/lib/kube-controller-manager/kubeconfig",
		"--leader-elect=true",
		"--node-monitor-grace-period=120s",
		fmt.Sprintf("--pod-eviction-timeout=%s", podEvictionTimeoutSetting),
		"--root-ca-file=/srv/kubernetes/ca/ca.crt",
		"--service-account-private-key-file=/srv/kubernetes/service-account-key/id_rsa",
		fmt.Sprintf("--service-cluster-ip-range=%s", serviceNetwork.String()),
	)

	if k8sVersionGreaterEqual113, _ := versionutils.CompareVersions(version, ">=", "1.13"); k8sVersionGreaterEqual113 {
		command = append(command,
			fmt.Sprintf("--secure-port=%d", port),
			"--port=0",
		)
	}

	if k8sVersionGreaterEqual112, _ := versionutils.CompareVersions(version, ">=", "1.12"); k8sVersionGreaterEqual112 {
		command = append(command,
			fmt.Sprintf("--horizontal-pod-autoscaler-downscale-stabilization=%s", horizontalPodAutoscalerConfig.DownscaleStabilization.Duration.String()),
			fmt.Sprintf("--horizontal-pod-autoscaler-initial-readiness-delay=%s", horizontalPodAutoscalerConfig.InitialReadinessDelay.Duration.String()),
			fmt.Sprintf("--horizontal-pod-autoscaler-cpu-initialization-period=%s", horizontalPodAutoscalerConfig.CPUInitializationPeriod.Duration.String()),
			"--authentication-kubeconfig=/var/lib/kube-controller-manager/kubeconfig",
			"--authorization-kubeconfig=/var/lib/kube-controller-manager/kubeconfig",
			"--tls-cert-file=/var/lib/kube-controller-manager-server/kube-controller-manager-server.crt",
			"--tls-private-key-file=/var/lib/kube-controller-manager-server/kube-controller-manager-server.key",
		)
	}

	command = append(command,
		"--tls-cipher-suites=TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,TLS_RSA_WITH_AES_128_CBC_SHA,TLS_RSA_WITH_AES_256_CBC_SHA,TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA",
		"--use-service-account-credentials=true",
		"--v=2",
	)
	return command
}

func getHorizontalPodAutoscalerConfig(config *gardencorev1beta1.HorizontalPodAutoscalerConfig) *gardencorev1beta1.HorizontalPodAutoscalerConfig {
	defaultHPATolerance := gardencorev1beta1.DefaultHPATolerance
	horizontalPodAutoscalerConfig := gardencorev1beta1.HorizontalPodAutoscalerConfig{
		CPUInitializationPeriod: &metav1.Duration{Duration: 5 * time.Minute},
		DownscaleDelay:          &metav1.Duration{Duration: 15 * time.Minute},
		DownscaleStabilization:  &metav1.Duration{Duration: 5 * time.Minute},
		InitialReadinessDelay:   &metav1.Duration{Duration: 30 * time.Second},
		SyncPeriod:              &metav1.Duration{Duration: 30 * time.Second},
		Tolerance:               &defaultHPATolerance,
		UpscaleDelay:            &metav1.Duration{Duration: 1 * time.Minute},
	}

	if config != nil {
		if config.CPUInitializationPeriod != nil {
			horizontalPodAutoscalerConfig.CPUInitializationPeriod = config.CPUInitializationPeriod
		}
		if config.DownscaleDelay != nil {
			horizontalPodAutoscalerConfig.DownscaleDelay = config.DownscaleDelay
		}
		if config.DownscaleStabilization != nil {
			horizontalPodAutoscalerConfig.DownscaleStabilization = config.DownscaleStabilization
		}
		if config.InitialReadinessDelay != nil {
			horizontalPodAutoscalerConfig.InitialReadinessDelay = config.InitialReadinessDelay
		}
		if config.SyncPeriod != nil {
			horizontalPodAutoscalerConfig.SyncPeriod = config.SyncPeriod
		}
		if config.Tolerance != nil {
			horizontalPodAutoscalerConfig.Tolerance = config.Tolerance
		}
		if config.UpscaleDelay != nil {
			horizontalPodAutoscalerConfig.UpscaleDelay = config.UpscaleDelay
		}
	}
	return &horizontalPodAutoscalerConfig
}
