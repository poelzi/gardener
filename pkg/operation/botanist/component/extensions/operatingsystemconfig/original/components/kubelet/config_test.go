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

package kubelet_test

import (
	"time"

	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components/kubelet"
	"github.com/gardener/gardener/pkg/utils"

	"github.com/Masterminds/semver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"
	"k8s.io/utils/pointer"
)

var _ = Describe("Config", func() {
	var (
		clusterDNSAddress = "foo"
		clusterDomain     = "bar"
		params            = components.ConfigurableKubeletConfigParameters{
			CpuCFSQuota:                      pointer.BoolPtr(false),
			CpuManagerPolicy:                 pointer.StringPtr("policy"),
			EvictionHard:                     map[string]string{"memory.available": "123"},
			EvictionMinimumReclaim:           map[string]string{"imagefs.available": "123"},
			EvictionSoft:                     map[string]string{"imagefs.inodesFree": "123"},
			EvictionSoftGracePeriod:          map[string]string{"nodefs.available": "123"},
			EvictionPressureTransitionPeriod: &metav1.Duration{Duration: 42 * time.Minute},
			EvictionMaxPodGracePeriod:        pointer.Int32Ptr(120),
			FailSwapOn:                       pointer.BoolPtr(false),
			FeatureGates:                     map[string]bool{"Foo": false},
			KubeReserved:                     map[string]string{"cpu": "123"},
			MaxPods:                          pointer.Int32Ptr(24),
			PodPidsLimit:                     pointer.Int64Ptr(101),
			SystemReserved:                   map[string]string{"memory": "321"},
		}

		kubeletConfigWithDefaults = &kubeletconfigv1beta1.KubeletConfiguration{
			Authentication: kubeletconfigv1beta1.KubeletAuthentication{
				Anonymous: kubeletconfigv1beta1.KubeletAnonymousAuthentication{
					Enabled: pointer.BoolPtr(false),
				},
				X509: kubeletconfigv1beta1.KubeletX509Authentication{
					ClientCAFile: "/var/lib/kubelet/ca.crt",
				},
				Webhook: kubeletconfigv1beta1.KubeletWebhookAuthentication{
					Enabled:  pointer.BoolPtr(true),
					CacheTTL: metav1.Duration{Duration: 2 * time.Minute},
				},
			},
			Authorization: kubeletconfigv1beta1.KubeletAuthorization{
				Mode: kubeletconfigv1beta1.KubeletAuthorizationModeWebhook,
				Webhook: kubeletconfigv1beta1.KubeletWebhookAuthorization{
					CacheAuthorizedTTL:   metav1.Duration{Duration: 5 * time.Minute},
					CacheUnauthorizedTTL: metav1.Duration{Duration: 30 * time.Second},
				},
			},
			CgroupDriver:                 "cgroupfs",
			CgroupRoot:                   "/",
			CgroupsPerQOS:                pointer.BoolPtr(true),
			ClusterDNS:                   []string{clusterDNSAddress},
			ClusterDomain:                clusterDomain,
			CPUCFSQuota:                  pointer.BoolPtr(true),
			CPUManagerPolicy:             "none",
			CPUManagerReconcilePeriod:    metav1.Duration{Duration: 10 * time.Second},
			EnableControllerAttachDetach: pointer.BoolPtr(true),
			EnableDebuggingHandlers:      pointer.BoolPtr(true),
			EnableServer:                 pointer.BoolPtr(true),
			EnforceNodeAllocatable:       []string{"pods"},
			EventBurst:                   50,
			EventRecordQPS:               pointer.Int32Ptr(50),
			EvictionHard: map[string]string{
				"memory.available":   "100Mi",
				"imagefs.available":  "5%",
				"imagefs.inodesFree": "5%",
				"nodefs.available":   "5%",
				"nodefs.inodesFree":  "5%",
			},
			EvictionMinimumReclaim: map[string]string{
				"memory.available":   "0Mi",
				"imagefs.available":  "0Mi",
				"imagefs.inodesFree": "0Mi",
				"nodefs.available":   "0Mi",
				"nodefs.inodesFree":  "0Mi",
			},
			EvictionSoft: map[string]string{
				"memory.available":   "200Mi",
				"imagefs.available":  "10%",
				"imagefs.inodesFree": "10%",
				"nodefs.available":   "10%",
				"nodefs.inodesFree":  "10%",
			},
			EvictionSoftGracePeriod: map[string]string{
				"memory.available":   "1m30s",
				"imagefs.available":  "1m30s",
				"imagefs.inodesFree": "1m30s",
				"nodefs.available":   "1m30s",
				"nodefs.inodesFree":  "1m30s",
			},
			EvictionPressureTransitionPeriod: metav1.Duration{Duration: 4 * time.Minute},
			EvictionMaxPodGracePeriod:        90,
			FailSwapOn:                       pointer.BoolPtr(true),
			FileCheckFrequency:               metav1.Duration{Duration: 20 * time.Second},
			HairpinMode:                      kubeletconfigv1beta1.PromiscuousBridge,
			HTTPCheckFrequency:               metav1.Duration{Duration: 20 * time.Second},
			ImageGCHighThresholdPercent:      pointer.Int32Ptr(50),
			ImageGCLowThresholdPercent:       pointer.Int32Ptr(40),
			ImageMinimumGCAge:                metav1.Duration{Duration: 2 * time.Minute},
			KubeAPIBurst:                     50,
			KubeAPIQPS:                       pointer.Int32Ptr(50),
			KubeReserved: map[string]string{
				"cpu":    "80m",
				"memory": "1Gi",
			},
			MaxOpenFiles:              1000000,
			MaxPods:                   110,
			NodeStatusUpdateFrequency: metav1.Duration{Duration: 10 * time.Second},
			PodsPerCore:               0,
			ReadOnlyPort:              0,
			RegistryBurst:             10,
			RegistryPullQPS:           pointer.Int32Ptr(5),
			ResolverConfig:            "/etc/resolv.conf",
			RuntimeRequestTimeout:     metav1.Duration{Duration: 2 * time.Minute},
			SerializeImagePulls:       pointer.BoolPtr(true),
			SyncFrequency:             metav1.Duration{Duration: time.Minute},
			VolumeStatsAggPeriod:      metav1.Duration{Duration: time.Minute},
		}

		kubeletConfigWithParams = &kubeletconfigv1beta1.KubeletConfiguration{
			Authentication: kubeletconfigv1beta1.KubeletAuthentication{
				Anonymous: kubeletconfigv1beta1.KubeletAnonymousAuthentication{
					Enabled: pointer.BoolPtr(false),
				},
				X509: kubeletconfigv1beta1.KubeletX509Authentication{
					ClientCAFile: "/var/lib/kubelet/ca.crt",
				},
				Webhook: kubeletconfigv1beta1.KubeletWebhookAuthentication{
					Enabled:  pointer.BoolPtr(true),
					CacheTTL: metav1.Duration{Duration: 2 * time.Minute},
				},
			},
			Authorization: kubeletconfigv1beta1.KubeletAuthorization{
				Mode: kubeletconfigv1beta1.KubeletAuthorizationModeWebhook,
				Webhook: kubeletconfigv1beta1.KubeletWebhookAuthorization{
					CacheAuthorizedTTL:   metav1.Duration{Duration: 5 * time.Minute},
					CacheUnauthorizedTTL: metav1.Duration{Duration: 30 * time.Second},
				},
			},
			CgroupDriver:                 "cgroupfs",
			CgroupRoot:                   "/",
			CgroupsPerQOS:                pointer.BoolPtr(true),
			ClusterDomain:                clusterDomain,
			ClusterDNS:                   []string{clusterDNSAddress},
			CPUCFSQuota:                  params.CpuCFSQuota,
			CPUManagerPolicy:             *params.CpuManagerPolicy,
			CPUManagerReconcilePeriod:    metav1.Duration{Duration: 10 * time.Second},
			EnableControllerAttachDetach: pointer.BoolPtr(true),
			EnableDebuggingHandlers:      pointer.BoolPtr(true),
			EnableServer:                 pointer.BoolPtr(true),
			EnforceNodeAllocatable:       []string{"pods"},
			EventBurst:                   50,
			EventRecordQPS:               pointer.Int32Ptr(50),
			EvictionHard: utils.MergeStringMaps(params.EvictionHard, map[string]string{
				"imagefs.available":  "5%",
				"imagefs.inodesFree": "5%",
				"nodefs.available":   "5%",
				"nodefs.inodesFree":  "5%",
			}),
			EvictionMinimumReclaim: utils.MergeStringMaps(params.EvictionMinimumReclaim, map[string]string{
				"memory.available":   "0Mi",
				"imagefs.inodesFree": "0Mi",
				"nodefs.available":   "0Mi",
				"nodefs.inodesFree":  "0Mi",
			}),
			EvictionSoft: utils.MergeStringMaps(params.EvictionSoft, map[string]string{
				"memory.available":  "200Mi",
				"imagefs.available": "10%",
				"nodefs.available":  "10%",
				"nodefs.inodesFree": "10%",
			}),
			EvictionSoftGracePeriod: utils.MergeStringMaps(params.EvictionSoftGracePeriod, map[string]string{
				"memory.available":   "1m30s",
				"imagefs.available":  "1m30s",
				"imagefs.inodesFree": "1m30s",
				"nodefs.inodesFree":  "1m30s",
			}),
			EvictionPressureTransitionPeriod: *params.EvictionPressureTransitionPeriod,
			EvictionMaxPodGracePeriod:        *params.EvictionMaxPodGracePeriod,
			FailSwapOn:                       params.FailSwapOn,
			FeatureGates:                     params.FeatureGates,
			FileCheckFrequency:               metav1.Duration{Duration: 20 * time.Second},
			HairpinMode:                      kubeletconfigv1beta1.PromiscuousBridge,
			HTTPCheckFrequency:               metav1.Duration{Duration: 20 * time.Second},
			ImageGCHighThresholdPercent:      pointer.Int32Ptr(50),
			ImageGCLowThresholdPercent:       pointer.Int32Ptr(40),
			ImageMinimumGCAge:                metav1.Duration{Duration: 2 * time.Minute},
			KubeAPIBurst:                     50,
			KubeAPIQPS:                       pointer.Int32Ptr(50),
			KubeReserved:                     utils.MergeStringMaps(params.KubeReserved, map[string]string{"memory": "1Gi"}),
			MaxOpenFiles:                     1000000,
			MaxPods:                          *params.MaxPods,
			NodeStatusUpdateFrequency:        metav1.Duration{Duration: 10 * time.Second},
			PodsPerCore:                      0,
			PodPidsLimit:                     params.PodPidsLimit,
			ReadOnlyPort:                     0,
			RegistryBurst:                    10,
			RegistryPullQPS:                  pointer.Int32Ptr(5),
			ResolverConfig:                   "/etc/resolv.conf",
			RuntimeRequestTimeout:            metav1.Duration{Duration: 2 * time.Minute},
			SerializeImagePulls:              pointer.BoolPtr(true),
			SyncFrequency:                    metav1.Duration{Duration: time.Minute},
			SystemReserved:                   params.SystemReserved,
			VolumeStatsAggPeriod:             metav1.Duration{Duration: time.Minute},
		}
	)

	DescribeTable("#Config",
		func(kubernetesVersion string, clusterDNSAddress, clusterDomain string, params components.ConfigurableKubeletConfigParameters, expectedConfig *kubeletconfigv1beta1.KubeletConfiguration, mutateExpectConfigFn func(*kubeletconfigv1beta1.KubeletConfiguration)) {
			expectation := expectedConfig.DeepCopy()
			if mutateExpectConfigFn != nil {
				mutateExpectConfigFn(expectation)
			}

			Expect(kubelet.Config(semver.MustParse(kubernetesVersion), clusterDNSAddress, clusterDomain, params)).To(Equal(expectation))
		},

		Entry(
			"kubernetes 1.15 w/o defaults",
			"1.15.1",
			clusterDNSAddress,
			clusterDomain,
			components.ConfigurableKubeletConfigParameters{},
			kubeletConfigWithDefaults,
			func(cfg *kubeletconfigv1beta1.KubeletConfiguration) { cfg.RotateCertificates = true },
		),
		Entry(
			"kubernetes 1.15 w/ defaults",
			"1.15.1",
			clusterDNSAddress,
			clusterDomain,
			params,
			kubeletConfigWithParams,
			func(cfg *kubeletconfigv1beta1.KubeletConfiguration) { cfg.RotateCertificates = true },
		),

		Entry(
			"kubernetes 1.16 w/o defaults",
			"1.16.1",
			clusterDNSAddress,
			clusterDomain,
			components.ConfigurableKubeletConfigParameters{},
			kubeletConfigWithDefaults,
			func(cfg *kubeletconfigv1beta1.KubeletConfiguration) { cfg.RotateCertificates = true },
		),
		Entry(
			"kubernetes 1.16 w/ defaults",
			"1.16.1",
			clusterDNSAddress,
			clusterDomain,
			params,
			kubeletConfigWithParams,
			func(cfg *kubeletconfigv1beta1.KubeletConfiguration) { cfg.RotateCertificates = true },
		),

		Entry(
			"kubernetes 1.17 w/o defaults",
			"1.17.1",
			clusterDNSAddress,
			clusterDomain,
			components.ConfigurableKubeletConfigParameters{},
			kubeletConfigWithDefaults,
			func(cfg *kubeletconfigv1beta1.KubeletConfiguration) { cfg.RotateCertificates = true },
		),
		Entry(
			"kubernetes 1.17 w/ defaults",
			"1.17.1",
			clusterDNSAddress,
			clusterDomain,
			params,
			kubeletConfigWithParams,
			func(cfg *kubeletconfigv1beta1.KubeletConfiguration) { cfg.RotateCertificates = true },
		),

		Entry(
			"kubernetes 1.18 w/o defaults",
			"1.18.1",
			clusterDNSAddress,
			clusterDomain,
			components.ConfigurableKubeletConfigParameters{},
			kubeletConfigWithDefaults,
			func(cfg *kubeletconfigv1beta1.KubeletConfiguration) { cfg.RotateCertificates = true },
		),
		Entry(
			"kubernetes 1.18 w/ defaults",
			"1.18.1",
			clusterDNSAddress,
			clusterDomain,
			params,
			kubeletConfigWithParams,
			func(cfg *kubeletconfigv1beta1.KubeletConfiguration) { cfg.RotateCertificates = true },
		),

		Entry(
			"kubernetes 1.19 w/o defaults",
			"1.19.1",
			clusterDNSAddress,
			clusterDomain,
			components.ConfigurableKubeletConfigParameters{},
			kubeletConfigWithDefaults,
			func(cfg *kubeletconfigv1beta1.KubeletConfiguration) {
				cfg.RotateCertificates = true
				cfg.VolumePluginDir = "/var/lib/kubelet/volumeplugins"
			},
		),
		Entry(
			"kubernetes 1.19 w/ defaults",
			"1.19.1",
			clusterDNSAddress,
			clusterDomain,
			params,
			kubeletConfigWithParams,
			func(cfg *kubeletconfigv1beta1.KubeletConfiguration) {
				cfg.RotateCertificates = true
				cfg.VolumePluginDir = "/var/lib/kubelet/volumeplugins"
			},
		),

		Entry(
			"kubernetes 1.20 w/o defaults",
			"1.20.1",
			clusterDNSAddress,
			clusterDomain,
			components.ConfigurableKubeletConfigParameters{},
			kubeletConfigWithDefaults,
			func(cfg *kubeletconfigv1beta1.KubeletConfiguration) {
				cfg.RotateCertificates = true
				cfg.VolumePluginDir = "/var/lib/kubelet/volumeplugins"
			},
		),
		Entry(
			"kubernetes 1.20 w/ defaults",
			"1.20.1",
			clusterDNSAddress,
			clusterDomain,
			params,
			kubeletConfigWithParams,
			func(cfg *kubeletconfigv1beta1.KubeletConfiguration) {
				cfg.RotateCertificates = true
				cfg.VolumePluginDir = "/var/lib/kubelet/volumeplugins"
			},
		),
	)
})
