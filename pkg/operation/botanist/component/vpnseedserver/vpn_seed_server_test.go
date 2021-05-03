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

package vpnseedserver_test

import (
	"context"
	"fmt"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/vpnseedserver"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	"github.com/gogo/protobuf/types"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"istio.io/api/networking/v1beta1"
	networkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	autoscalingv1beta2 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1beta2"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("VpnSeedServer", func() {
	var (
		ctrl          *gomock.Controller
		c             *mockclient.MockClient
		vpnSeedServer VPNSeedServer

		ctx                     = context.TODO()
		fakeErr                 = fmt.Errorf("fake error")
		namespace               = "shoot--foo--bar"
		vpnImage                = "eu.gcr.io/gardener-project/gardener/vpn-seed-server:v1.2.3"
		envoyImage              = "envoyproxy/envoy:v4.5.6"
		kubeAPIServerHost       = "foo.bar"
		serviceNetwork          = "10.0.0.0/24"
		podNetwork              = "10.0.1.0/24"
		nodeNetwork             = "10.0.2.0/24"
		replicas          int32 = 1
		vpaUpdateMode           = autoscalingv1beta2.UpdateModeAuto

		secretNameTLSAuth     = VpnSeedServerTLSAuth
		secretChecksumTLSAuth = "1234"
		secretDataTLSAuth     = map[string][]byte{"vpn.tlsauth": []byte("baz")}
		secretNameServer      = DeploymentName
		secretChecksumServer  = "5678"
		secretDataServer      = map[string][]byte{"ca.crt": []byte("baz"), "tls.crt": []byte("baz"), "tls.key": []byte("baz")}
		secretNameDH          = "vpn-seed-server-dh"
		secretChecksumDH      = "9012"
		secretDataDH          = map[string][]byte{"dh2048.pem": []byte("baz")}
		secrets               = Secrets{
			TLSAuth:          component.Secret{Name: secretNameTLSAuth, Checksum: secretChecksumTLSAuth, Data: secretDataTLSAuth},
			Server:           component.Secret{Name: secretNameServer, Checksum: secretChecksumServer, Data: secretDataServer},
			DiffieHellmanKey: component.Secret{Name: secretNameDH, Checksum: secretChecksumDH, Data: secretDataDH},
		}

		configMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vpn-seed-server-envoy-config",
				Namespace: namespace,
			},
			Data: map[string]string{
				"envoy.yaml": `static_resources:
  listeners:
  - name: listener_0
    address:
      socket_address:
        protocol: TCP
        address: 0.0.0.0
        port_value: 9443
    listener_filters:
    - name: "envoy.filters.listener.tls_inspector"
      typed_config: {}
    filter_chains:
    - transport_socket:
        name: envoy.transport_sockets.tls
        typed_config:
          "@type": type.googleapis.com/envoy.extensions.transport_sockets.tls.v3.DownstreamTlsContext
          common_tls_context:
            tls_certificates:
            - certificate_chain: { filename: "/etc/tls/tls.crt" }
              private_key: { filename: "/etc/tls/tls.key" }
            validation_context:
              trusted_ca:
                filename: /etc/tls/ca.crt
      filters:
      - name: envoy.filters.network.http_connection_manager
        typed_config:
          "@type": type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager
          stat_prefix: ingress_http
          route_config:
            name: local_route
            virtual_hosts:
            - name: local_service
              domains:
              - "*"
              routes:
              - match:
                  connect_matcher: {}
                route:
                  cluster: dynamic_forward_proxy_cluster
                  upgrade_configs:
                  - upgrade_type: CONNECT
                    connect_config: {}
          http_filters:
          - name: envoy.filters.http.dynamic_forward_proxy
            typed_config:
              "@type": type.googleapis.com/envoy.extensions.filters.http.dynamic_forward_proxy.v3.FilterConfig
              dns_cache_config:
                name: dynamic_forward_proxy_cache_config
                dns_lookup_family: V4_ONLY
          - name: envoy.filters.http.router
          http_protocol_options:
            accept_http_10: true
          upgrade_configs:
          - upgrade_type: CONNECT
  clusters:
  - name: dynamic_forward_proxy_cluster
    connect_timeout: 1s
    lb_policy: CLUSTER_PROVIDED
    cluster_type:
      name: envoy.clusters.dynamic_forward_proxy
      typed_config:
        "@type": type.googleapis.com/envoy.extensions.clusters.dynamic_forward_proxy.v3.ClusterConfig
        dns_cache_config:
          name: dynamic_forward_proxy_cache_config
          dns_lookup_family: V4_ONLY`,
			},
		}

		deployment = func(nodeNetwork *string) *appsv1.Deployment {
			maxSurge := intstr.FromInt(100)
			maxUnavailable := intstr.FromInt(0)
			deploy := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      DeploymentName,
					Namespace: namespace,
					Labels: map[string]string{
						v1beta1constants.GardenRole:                      v1beta1constants.GardenRoleControlPlane,
						v1beta1constants.LabelApp:                        DeploymentName,
						"networking.gardener.cloud/from-shoot-apiserver": "allowed",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas:             pointer.Int32Ptr(replicas),
					RevisionHistoryLimit: pointer.Int32Ptr(1),
					Selector: &metav1.LabelSelector{MatchLabels: map[string]string{
						v1beta1constants.LabelApp: DeploymentName,
					}},
					Strategy: appsv1.DeploymentStrategy{
						RollingUpdate: &appsv1.RollingUpdateDeployment{
							MaxUnavailable: &maxUnavailable,
							MaxSurge:       &maxSurge,
						},
						Type: appsv1.RollingUpdateDeploymentStrategyType,
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								v1beta1constants.GardenRole:                          v1beta1constants.GardenRoleControlPlane,
								v1beta1constants.LabelApp:                            DeploymentName,
								v1beta1constants.LabelNetworkPolicyToShootNetworks:   v1beta1constants.LabelNetworkPolicyAllowed,
								v1beta1constants.LabelNetworkPolicyToDNS:             v1beta1constants.LabelNetworkPolicyAllowed,
								v1beta1constants.LabelNetworkPolicyToPrivateNetworks: v1beta1constants.LabelNetworkPolicyAllowed,
								v1beta1constants.LabelNetworkPolicyFromPrometheus:    v1beta1constants.LabelNetworkPolicyAllowed,
							},
							Annotations: map[string]string{
								"checksum/secret-" + secretNameTLSAuth: secrets.TLSAuth.Checksum,
								"checksum/secret-" + secretNameDH:      secrets.DiffieHellmanKey.Checksum,
							},
						},
						Spec: corev1.PodSpec{
							AutomountServiceAccountToken: pointer.BoolPtr(false),
							PriorityClassName:            v1beta1constants.PriorityClassNameShootControlPlane,
							DNSPolicy:                    corev1.DNSDefault, // make sure to not use the coredns for DNS resolution.
							Containers: []corev1.Container{
								{
									Name:            DeploymentName,
									Image:           vpnImage,
									ImagePullPolicy: corev1.PullIfNotPresent,
									Ports: []corev1.ContainerPort{
										{
											Name:          "tcp-tunnel",
											ContainerPort: 1194,
											Protocol:      corev1.ProtocolTCP,
										},
									},
									SecurityContext: &corev1.SecurityContext{
										Capabilities: &corev1.Capabilities{
											Add: []corev1.Capability{
												"NET_ADMIN",
											},
										},
										Privileged: pointer.BoolPtr(true),
									},
									Env: []corev1.EnvVar{
										{
											Name:  "SERVICE_NETWORK",
											Value: serviceNetwork,
										},
										{
											Name:  "POD_NETWORK",
											Value: podNetwork,
										},
									},
									Resources: corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceCPU:    resource.MustParse("100m"),
											corev1.ResourceMemory: resource.MustParse("100Mi"),
										},
										Limits: corev1.ResourceList{
											corev1.ResourceCPU:    resource.MustParse("1"),
											corev1.ResourceMemory: resource.MustParse("1Gi"),
										},
									},
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      DeploymentName,
											MountPath: "/srv/secrets/vpn-server",
										},
										{
											Name:      VpnSeedServerTLSAuth,
											MountPath: "/srv/secrets/tlsauth",
										},
										{
											Name:      "vpn-seed-server-dh",
											MountPath: "/srv/secrets/dh",
										},
									},
								},
								{
									Name:            "envoy-proxy",
									Image:           envoyImage,
									ImagePullPolicy: corev1.PullIfNotPresent,
									SecurityContext: &corev1.SecurityContext{
										Capabilities: &corev1.Capabilities{
											Add: []corev1.Capability{
												"NET_BIND_SERVICE",
											},
										},
									},
									Command: []string{
										"envoy",
										"--concurrency",
										"2",
										"-c",
										"/etc/envoy/envoy.yaml",
									},
									Resources: corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceCPU:    resource.MustParse("20m"),
											corev1.ResourceMemory: resource.MustParse("20Mi"),
										},
										Limits: corev1.ResourceList{
											corev1.ResourceCPU:    resource.MustParse("200m"),
											corev1.ResourceMemory: resource.MustParse("300Mi"),
										},
									},
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "envoy-config",
											MountPath: "/etc/envoy",
										},
										{
											Name:      DeploymentName,
											MountPath: "/etc/tls",
										},
									},
								},
							},
							TerminationGracePeriodSeconds: pointer.Int64Ptr(30),
							Volumes: []corev1.Volume{
								{
									Name: DeploymentName,
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName: DeploymentName,
										},
									},
								},
								{
									Name: VpnSeedServerTLSAuth,
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName: VpnSeedServerTLSAuth,
										},
									},
								},
								{
									Name: "vpn-seed-server-dh",
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName: "vpn-seed-server-dh",
										},
									},
								},
								{
									Name: "envoy-config",
									VolumeSource: corev1.VolumeSource{
										ConfigMap: &corev1.ConfigMapVolumeSource{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "vpn-seed-server-envoy-config",
											},
										},
									},
								},
							},
						},
					},
				},
			}
			if nodeNetwork != nil {
				deploy.Spec.Template.Spec.Containers[0].Env = append(deploy.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{Name: "NODE_NETWORK", Value: *nodeNetwork})
			}
			return deploy
		}

		destinationRule = &networkingv1beta1.DestinationRule{
			ObjectMeta: metav1.ObjectMeta{Name: DeploymentName, Namespace: namespace},
			Spec: v1beta1.DestinationRule{
				ExportTo: []string{"*"},
				Host:     fmt.Sprintf("%s.%s.svc.cluster.local", DeploymentName, namespace),
				TrafficPolicy: &v1beta1.TrafficPolicy{
					ConnectionPool: &v1beta1.ConnectionPoolSettings{
						Tcp: &v1beta1.ConnectionPoolSettings_TCPSettings{
							MaxConnections: 5000,
							TcpKeepalive: &v1beta1.ConnectionPoolSettings_TCPSettings_TcpKeepalive{
								Interval: &types.Duration{
									Seconds: 75,
								},
								Time: &types.Duration{
									Seconds: 7200,
								},
							},
						},
					},
					Tls: &v1beta1.ClientTLSSettings{
						Mode: v1beta1.ClientTLSSettings_DISABLE,
					},
				},
			},
		}

		gateway = &networkingv1beta1.Gateway{
			ObjectMeta: metav1.ObjectMeta{Name: DeploymentName, Namespace: namespace},
			Spec: v1beta1.Gateway{
				Selector: map[string]string{
					"istio": "ingressgateway",
				},
				Servers: []*v1beta1.Server{
					{
						Hosts: []string{kubeAPIServerHost},
						Port: &v1beta1.Port{
							Name:     "tls-tunnel",
							Number:   8132,
							Protocol: "HTTP",
						},
					},
				},
			},
		}

		networkPolicy = &networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "allow-to-vpn-seed-server",
				Namespace: namespace,
				Annotations: map[string]string{
					"gardener.cloud/description": "Allows only Ingress/Egress between the kube-apiserver of the same control plane and the corresponding vpn-seed-server and Ingress from the istio ingress gateway to the vpn-seed-server.",
				},
			},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
						v1beta1constants.LabelApp:   DeploymentName,
					},
				},
				Ingress: []networkingv1.NetworkPolicyIngressRule{
					{
						From: []networkingv1.NetworkPolicyPeer{
							{
								PodSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
										v1beta1constants.LabelApp:   v1beta1constants.LabelKubernetes,
										v1beta1constants.LabelRole:  v1beta1constants.LabelAPIServer,
									},
								},
							},
						},
					},
					{
						From: []networkingv1.NetworkPolicyPeer{
							{
								// we don't want to modify existing labels on the istio namespace
								NamespaceSelector: &metav1.LabelSelector{},
								PodSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										v1beta1constants.LabelApp: "istio-ingressgateway",
									},
								},
							},
						},
					},
				},
				Egress: []networkingv1.NetworkPolicyEgressRule{
					{
						To: []networkingv1.NetworkPolicyPeer{
							{
								PodSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
										v1beta1constants.LabelApp:   v1beta1constants.LabelKubernetes,
										v1beta1constants.LabelRole:  v1beta1constants.LabelAPIServer,
									},
								},
							},
						},
					},
				},
				PolicyTypes: []networkingv1.PolicyType{
					networkingv1.PolicyTypeIngress,
					networkingv1.PolicyTypeEgress,
				},
			},
		}

		secretDH = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "vpn-seed-server-dh", Namespace: namespace},
			Type:       corev1.SecretTypeOpaque,
			Data:       secretDataDH,
		}

		secretTLSAuth = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: VpnSeedServerTLSAuth, Namespace: namespace},
			Type:       corev1.SecretTypeOpaque,
			Data:       secretDataTLSAuth,
		}

		secretServer = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: DeploymentName, Namespace: namespace},
			Type:       corev1.SecretTypeTLS,
			Data:       secretDataServer,
		}

		service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ServiceName,
				Namespace: namespace,
				Annotations: map[string]string{
					"networking.istio.io/exportTo": "*",
				},
			},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeClusterIP,
				Ports: []corev1.ServicePort{
					{
						Name:       DeploymentName,
						Port:       1194,
						TargetPort: intstr.FromInt(1194),
					},
					{
						Name:       "http-proxy",
						Port:       9443,
						TargetPort: intstr.FromInt(9443),
					},
				},
				Selector: map[string]string{
					v1beta1constants.LabelApp: DeploymentName,
				},
			},
		}

		virtualService = &networkingv1beta1.VirtualService{
			ObjectMeta: metav1.ObjectMeta{Name: DeploymentName, Namespace: namespace},
			Spec: v1beta1.VirtualService{
				ExportTo: []string{"*"},
				Hosts:    []string{kubeAPIServerHost},
				Gateways: []string{DeploymentName},
				Http: []*v1beta1.HTTPRoute{
					{
						Route: []*v1beta1.HTTPRouteDestination{
							{
								Destination: &v1beta1.Destination{
									Port: &v1beta1.PortSelector{
										Number: 1194,
									},
									Host: DeploymentName,
								},
							},
						},
					},
				},
			},
		}

		vpa = &autoscalingv1beta2.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{Name: DeploymentName + "-vpa", Namespace: namespace},
			Spec: autoscalingv1beta2.VerticalPodAutoscalerSpec{
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: appsv1.SchemeGroupVersion.String(),
					Kind:       "Deployment",
					Name:       DeploymentName,
				},
				UpdatePolicy: &autoscalingv1beta2.PodUpdatePolicy{
					UpdateMode: &vpaUpdateMode,
				},
				ResourcePolicy: &autoscalingv1beta2.PodResourcePolicy{
					ContainerPolicies: []autoscalingv1beta2.ContainerResourcePolicy{
						{
							ContainerName: DeploymentName,
							MinAllowed: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("100Mi"),
							},
						},
						{
							ContainerName: "envoy-proxy",
							MinAllowed: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("20m"),
								corev1.ResourceMemory: resource.MustParse("20Mi"),
							},
						},
					},
				},
			},
		}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)

		vpnSeedServer = New(c, namespace, envoyImage, vpnImage, &kubeAPIServerHost, serviceNetwork, podNetwork, nil, replicas)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Deploy", func() {
		Context("missing secret information", func() {
			It("should return an error because the TLSAuth secret information is not provided", func() {
				Expect(vpnSeedServer.Deploy(ctx)).To(MatchError(ContainSubstring("missing TLSAuth secret information")))
			})

			It("should return an error because the DH secret information is not provided", func() {
				vpnSeedServer.SetSecrets(Secrets{TLSAuth: component.Secret{Name: secretNameTLSAuth, Checksum: secretChecksumTLSAuth}})
				Expect(vpnSeedServer.Deploy(ctx)).To(MatchError(ContainSubstring("missing DH secret information")))
			})

			It("should return an error because the Server secret information is not provided", func() {
				vpnSeedServer.SetSecrets(Secrets{
					TLSAuth:          component.Secret{Name: secretNameTLSAuth, Checksum: secretChecksumTLSAuth},
					DiffieHellmanKey: component.Secret{Name: secretNameDH, Checksum: secretChecksumDH},
				})
				Expect(vpnSeedServer.Deploy(ctx)).To(MatchError(ContainSubstring("missing server secret information")))
			})
		})

		Context("secret information available", func() {
			BeforeEach(func() {
				vpnSeedServer.SetSecrets(secrets)
			})

			It("should fail because the server secret cannot be created", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Return(fakeErr),
				)

				Expect(vpnSeedServer.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the tlsAuth secret cannot be created", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName+"-tlsauth"), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Return(fakeErr),
				)

				Expect(vpnSeedServer.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the dh secret cannot be created", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName+"-tlsauth"), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName+"-dh"), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Return(fakeErr),
				)

				Expect(vpnSeedServer.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the networkpolicy cannot be created", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName+"-tlsauth"), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName+"-dh"), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, "allow-to-vpn-seed-server"), gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{})).Return(fakeErr),
				)

				Expect(vpnSeedServer.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the deployment cannot be created", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName+"-tlsauth"), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName+"-dh"), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, "allow-to-vpn-seed-server"), gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{})).Return(fakeErr),
				)

				Expect(vpnSeedServer.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the configMap cannot be created", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName+"-tlsauth"), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName+"-dh"), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, "allow-to-vpn-seed-server"), gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName+"-envoy-config"), gomock.AssignableToTypeOf(&corev1.ConfigMap{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.ConfigMap{})).Return(fakeErr),
				)

				Expect(vpnSeedServer.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the gateway cannot be created", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName+"-tlsauth"), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName+"-dh"), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, "allow-to-vpn-seed-server"), gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName+"-envoy-config"), gomock.AssignableToTypeOf(&corev1.ConfigMap{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.ConfigMap{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&networkingv1beta1.Gateway{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&networkingv1beta1.Gateway{})).Return(fakeErr),
				)

				Expect(vpnSeedServer.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the destinationRule cannot be created", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName+"-tlsauth"), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName+"-dh"), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, "allow-to-vpn-seed-server"), gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName+"-envoy-config"), gomock.AssignableToTypeOf(&corev1.ConfigMap{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.ConfigMap{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&networkingv1beta1.Gateway{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&networkingv1beta1.Gateway{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&networkingv1beta1.DestinationRule{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&networkingv1beta1.DestinationRule{})).Return(fakeErr),
				)

				Expect(vpnSeedServer.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the virtualService cannot be created", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName+"-tlsauth"), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName+"-dh"), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, "allow-to-vpn-seed-server"), gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName+"-envoy-config"), gomock.AssignableToTypeOf(&corev1.ConfigMap{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.ConfigMap{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&networkingv1beta1.Gateway{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&networkingv1beta1.Gateway{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&networkingv1beta1.DestinationRule{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&networkingv1beta1.DestinationRule{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&networkingv1beta1.VirtualService{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&networkingv1beta1.VirtualService{})).Return(fakeErr),
				)

				Expect(vpnSeedServer.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the service cannot be created", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName+"-tlsauth"), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName+"-dh"), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, "allow-to-vpn-seed-server"), gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName+"-envoy-config"), gomock.AssignableToTypeOf(&corev1.ConfigMap{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.ConfigMap{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&networkingv1beta1.Gateway{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&networkingv1beta1.Gateway{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&networkingv1beta1.DestinationRule{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&networkingv1beta1.DestinationRule{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&networkingv1beta1.VirtualService{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&networkingv1beta1.VirtualService{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, ServiceName), gomock.AssignableToTypeOf(&corev1.Service{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Service{})).Return(fakeErr),
				)

				Expect(vpnSeedServer.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the vpa cannot be created", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName+"-tlsauth"), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName+"-dh"), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, "allow-to-vpn-seed-server"), gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName+"-envoy-config"), gomock.AssignableToTypeOf(&corev1.ConfigMap{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.ConfigMap{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&networkingv1beta1.Gateway{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&networkingv1beta1.Gateway{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&networkingv1beta1.DestinationRule{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&networkingv1beta1.DestinationRule{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&networkingv1beta1.VirtualService{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&networkingv1beta1.VirtualService{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, ServiceName), gomock.AssignableToTypeOf(&corev1.Service{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Service{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName+"-vpa"), gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{})).Return(fakeErr),
				)

				Expect(vpnSeedServer.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should successfully deploy all resources (w/o node network)", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Do(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) {
						Expect(obj).To(DeepEqual(secretServer))
					}),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName+"-tlsauth"), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Do(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) {
						Expect(obj).To(DeepEqual(secretTLSAuth))
					}),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName+"-dh"), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Do(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) {
						Expect(obj).To(DeepEqual(secretDH))
					}),
					c.EXPECT().Get(ctx, kutil.Key(namespace, "allow-to-vpn-seed-server"), gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{})).Do(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) {
						Expect(obj).To(DeepEqual(networkPolicy))
					}),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{})).Do(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) {
						Expect(obj).To(DeepEqual(deployment(nil)))
					}),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName+"-envoy-config"), gomock.AssignableToTypeOf(&corev1.ConfigMap{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.ConfigMap{})).Do(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) {
						Expect(obj).To(DeepEqual(configMap))
					}),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&networkingv1beta1.Gateway{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&networkingv1beta1.Gateway{})).Do(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) {
						Expect(obj).To(DeepEqual(gateway))
					}),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&networkingv1beta1.DestinationRule{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&networkingv1beta1.DestinationRule{})).Do(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) {
						Expect(obj).To(DeepEqual(destinationRule))
					}),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&networkingv1beta1.VirtualService{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&networkingv1beta1.VirtualService{})).Do(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) {
						Expect(obj).To(DeepEqual(virtualService))
					}),
					c.EXPECT().Get(ctx, kutil.Key(namespace, ServiceName), gomock.AssignableToTypeOf(&corev1.Service{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Service{})).Do(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) {
						Expect(obj).To(DeepEqual(service))
					}),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName+"-vpa"), gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{})).Do(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) {
						Expect(obj).To(DeepEqual(vpa))
					}),
				)
				Expect(vpnSeedServer.Deploy(ctx)).To(Succeed())
			})

			It("should successfully deploy all resources (w/ node network)", func() {
				vpnSeedServer = New(c, namespace, envoyImage, vpnImage, &kubeAPIServerHost, serviceNetwork, podNetwork, &nodeNetwork, replicas)
				vpnSeedServer.SetSecrets(secrets)

				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Do(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) {
						Expect(obj).To(DeepEqual(secretServer))
					}),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName+"-tlsauth"), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Do(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) {
						Expect(obj).To(DeepEqual(secretTLSAuth))
					}),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName+"-dh"), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Do(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) {
						Expect(obj).To(DeepEqual(secretDH))
					}),
					c.EXPECT().Get(ctx, kutil.Key(namespace, "allow-to-vpn-seed-server"), gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{})).Do(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) {
						Expect(obj).To(DeepEqual(networkPolicy))
					}),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{})).Do(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) {
						Expect(obj).To(DeepEqual(deployment(&nodeNetwork)))
					}),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName+"-envoy-config"), gomock.AssignableToTypeOf(&corev1.ConfigMap{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.ConfigMap{})).Do(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) {
						Expect(obj).To(DeepEqual(configMap))
					}),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&networkingv1beta1.Gateway{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&networkingv1beta1.Gateway{})).Do(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) {
						Expect(obj).To(DeepEqual(gateway))
					}),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&networkingv1beta1.DestinationRule{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&networkingv1beta1.DestinationRule{})).Do(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) {
						Expect(obj).To(DeepEqual(destinationRule))
					}),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&networkingv1beta1.VirtualService{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&networkingv1beta1.VirtualService{})).Do(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) {
						Expect(obj).To(DeepEqual(virtualService))
					}),
					c.EXPECT().Get(ctx, kutil.Key(namespace, ServiceName), gomock.AssignableToTypeOf(&corev1.Service{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Service{})).Do(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) {
						Expect(obj).To(DeepEqual(service))
					}),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName+"-vpa"), gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{})).Do(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) {
						Expect(obj).To(DeepEqual(vpa))
					}),
				)

				Expect(vpnSeedServer.Deploy(ctx)).To(Succeed())
			})
		})
	})

	Describe("#Destroy", func() {

		It("should fail because the server secret cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}).Return(fakeErr),
			)

			Expect(vpnSeedServer.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the tlsAuth secret cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-tlsauth"}}).Return(fakeErr),
			)

			Expect(vpnSeedServer.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the dh secret cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-tlsauth"}}),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-dh"}}).Return(fakeErr),
			)

			Expect(vpnSeedServer.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the networkpolicy cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-tlsauth"}}),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-dh"}}),
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "allow-to-vpn-seed-server"}}).Return(fakeErr),
			)

			Expect(vpnSeedServer.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the deployment cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-tlsauth"}}),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-dh"}}),
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "allow-to-vpn-seed-server"}}),
				c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}).Return(fakeErr),
			)

			Expect(vpnSeedServer.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the configMap cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-tlsauth"}}),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-dh"}}),
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "allow-to-vpn-seed-server"}}),
				c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-envoy-config"}}).Return(fakeErr),
			)

			Expect(vpnSeedServer.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the gateway cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-tlsauth"}}),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-dh"}}),
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "allow-to-vpn-seed-server"}}),
				c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-envoy-config"}}),
				c.EXPECT().Delete(ctx, &networkingv1beta1.Gateway{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}).Return(fakeErr),
			)

			Expect(vpnSeedServer.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the destinationRule cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-tlsauth"}}),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-dh"}}),
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "allow-to-vpn-seed-server"}}),
				c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-envoy-config"}}),
				c.EXPECT().Delete(ctx, &networkingv1beta1.Gateway{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &networkingv1beta1.DestinationRule{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}).Return(fakeErr),
			)

			Expect(vpnSeedServer.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the virtualService cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-tlsauth"}}),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-dh"}}),
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "allow-to-vpn-seed-server"}}),
				c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-envoy-config"}}),
				c.EXPECT().Delete(ctx, &networkingv1beta1.Gateway{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &networkingv1beta1.DestinationRule{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &networkingv1beta1.VirtualService{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}).Return(fakeErr),
			)

			Expect(vpnSeedServer.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the service cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-tlsauth"}}),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-dh"}}),
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "allow-to-vpn-seed-server"}}),
				c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-envoy-config"}}),
				c.EXPECT().Delete(ctx, &networkingv1beta1.Gateway{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &networkingv1beta1.DestinationRule{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &networkingv1beta1.VirtualService{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: ServiceName}}).Return(fakeErr),
			)

			Expect(vpnSeedServer.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the vpa cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-tlsauth"}}),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-dh"}}),
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "allow-to-vpn-seed-server"}}),
				c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-envoy-config"}}),
				c.EXPECT().Delete(ctx, &networkingv1beta1.Gateway{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &networkingv1beta1.DestinationRule{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &networkingv1beta1.VirtualService{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: ServiceName}}),
				c.EXPECT().Delete(ctx, &autoscalingv1beta2.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-vpa"}}).Return(fakeErr),
			)

			Expect(vpnSeedServer.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should successfully destroy all resources", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-tlsauth"}}),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-dh"}}),
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "allow-to-vpn-seed-server"}}),
				c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-envoy-config"}}),
				c.EXPECT().Delete(ctx, &networkingv1beta1.Gateway{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &networkingv1beta1.DestinationRule{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &networkingv1beta1.VirtualService{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: ServiceName}}),
				c.EXPECT().Delete(ctx, &autoscalingv1beta2.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-vpa"}}),
			)
			Expect(vpnSeedServer.Destroy(ctx)).To(Succeed())
		})
	})

	Describe("#Wait", func() {
		It("should return nil as it's not implemented as of now", func() {
			Expect(vpnSeedServer.Wait(ctx)).To(Succeed())
		})
	})

	Describe("#WaitCleanup", func() {
		It("should return nil as it's not implemented as of now", func() {
			Expect(vpnSeedServer.WaitCleanup(ctx)).To(Succeed())
		})
	})
})
