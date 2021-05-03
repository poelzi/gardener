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

package kubelet

import (
	"time"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	oscutils "github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/utils"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"
	"k8s.io/utils/pointer"
)

var _ = Describe("ConfigCodec", func() {
	var (
		kubeletConfig = &kubeletconfigv1beta1.KubeletConfiguration{
			TypeMeta: metav1.TypeMeta{
				Kind:       "KubeletConfiguration",
				APIVersion: "kubelet.config.k8s.io/v1beta1",
			},
			SyncFrequency:      metav1.Duration{Duration: 1 * time.Minute},
			FileCheckFrequency: metav1.Duration{Duration: 20 * time.Second},
			HTTPCheckFrequency: metav1.Duration{Duration: 20 * time.Second},
			RotateCertificates: true,
			Authentication: kubeletconfigv1beta1.KubeletAuthentication{
				X509: kubeletconfigv1beta1.KubeletX509Authentication{
					ClientCAFile: "/var/lib/kubelet/ca.crt",
				},
				Webhook: kubeletconfigv1beta1.KubeletWebhookAuthentication{
					Enabled:  pointer.BoolPtr(true),
					CacheTTL: metav1.Duration{Duration: 2 * time.Minute},
				},
				Anonymous: kubeletconfigv1beta1.KubeletAnonymousAuthentication{
					Enabled: pointer.BoolPtr(false),
				},
			},
			Authorization: kubeletconfigv1beta1.KubeletAuthorization{
				Mode: "Webhook",
				Webhook: kubeletconfigv1beta1.KubeletWebhookAuthorization{
					CacheAuthorizedTTL:   metav1.Duration{Duration: 5 * time.Minute},
					CacheUnauthorizedTTL: metav1.Duration{Duration: 30 * time.Second},
				},
			},
			RegistryPullQPS:         pointer.Int32Ptr(5),
			RegistryBurst:           10,
			EventRecordQPS:          pointer.Int32Ptr(50),
			EventBurst:              50,
			EnableDebuggingHandlers: pointer.BoolPtr(true),
			ClusterDomain:           "cluster.local",
			ClusterDNS: []string{
				"100.64.0.10",
			},
			NodeStatusUpdateFrequency:   metav1.Duration{Duration: 10 * time.Second},
			ImageMinimumGCAge:           metav1.Duration{Duration: 2 * time.Minute},
			ImageGCHighThresholdPercent: pointer.Int32Ptr(50),
			ImageGCLowThresholdPercent:  pointer.Int32Ptr(40),
			VolumeStatsAggPeriod:        metav1.Duration{Duration: 1 * time.Minute},
			CgroupRoot:                  "/",
			CgroupsPerQOS:               pointer.BoolPtr(true),
			CgroupDriver:                "cgroupfs",
			CPUManagerPolicy:            "none",
			CPUManagerReconcilePeriod:   metav1.Duration{Duration: 10 * time.Second},
			RuntimeRequestTimeout:       metav1.Duration{Duration: 2 * time.Minute},
			HairpinMode:                 "promiscuous-bridge",
			MaxPods:                     110,
			ResolverConfig:              "/etc/resolv.conf",
			CPUCFSQuota:                 pointer.BoolPtr(true),
			MaxOpenFiles:                1000000,
			KubeAPIQPS:                  pointer.Int32Ptr(50),
			KubeAPIBurst:                50,
			SerializeImagePulls:         pointer.BoolPtr(true),
			EvictionHard: map[string]string{
				"imagefs.available":  "5%",
				"imagefs.inodesFree": "5%",
				"memory.available":   "100Mi",
				"nodefs.available":   "5%",
				"nodefs.inodesFree":  "5%",
			},
			EvictionSoft: map[string]string{
				"imagefs.available":  "10%",
				"imagefs.inodesFree": "10%",
				"memory.available":   "200Mi",
				"nodefs.available":   "10%",
				"nodefs.inodesFree":  "10%",
			},
			EvictionSoftGracePeriod: map[string]string{
				"nodefs.available":   "1m30s",
				"nodefs.inodesFree":  "1m30s",
				"imagefs.available":  "1m30s",
				"imagefs.inodesFree": "1m30s",
				"memory.available":   "1m30s",
			},
			EvictionPressureTransitionPeriod: metav1.Duration{Duration: 4 * time.Minute},
			EvictionMaxPodGracePeriod:        90,
			EnableControllerAttachDetach:     pointer.BoolPtr(true),
			FailSwapOn:                       pointer.BoolPtr(true),
			KubeReserved: map[string]string{
				"cpu":    "80m",
				"memory": "1Gi",
			},
			EnforceNodeAllocatable: []string{
				"pods",
			},
		}

		data = `apiVersion: kubelet.config.k8s.io/v1beta1
authentication:
  anonymous:
    enabled: false
  webhook:
    cacheTTL: 2m0s
    enabled: true
  x509:
    clientCAFile: /var/lib/kubelet/ca.crt
authorization:
  mode: Webhook
  webhook:
    cacheAuthorizedTTL: 5m0s
    cacheUnauthorizedTTL: 30s
cgroupDriver: cgroupfs
cgroupRoot: /
cgroupsPerQOS: true
clusterDNS:
- 100.64.0.10
clusterDomain: cluster.local
cpuCFSQuota: true
cpuManagerPolicy: none
cpuManagerReconcilePeriod: 10s
enableControllerAttachDetach: true
enableDebuggingHandlers: true
enforceNodeAllocatable:
- pods
eventBurst: 50
eventRecordQPS: 50
evictionHard:
  imagefs.available: 5%
  imagefs.inodesFree: 5%
  memory.available: 100Mi
  nodefs.available: 5%
  nodefs.inodesFree: 5%
evictionMaxPodGracePeriod: 90
evictionPressureTransitionPeriod: 4m0s
evictionSoft:
  imagefs.available: 10%
  imagefs.inodesFree: 10%
  memory.available: 200Mi
  nodefs.available: 10%
  nodefs.inodesFree: 10%
evictionSoftGracePeriod:
  imagefs.available: 1m30s
  imagefs.inodesFree: 1m30s
  memory.available: 1m30s
  nodefs.available: 1m30s
  nodefs.inodesFree: 1m30s
failSwapOn: true
fileCheckFrequency: 20s
hairpinMode: promiscuous-bridge
httpCheckFrequency: 20s
imageGCHighThresholdPercent: 50
imageGCLowThresholdPercent: 40
imageMinimumGCAge: 2m0s
kind: KubeletConfiguration
kubeAPIBurst: 50
kubeAPIQPS: 50
kubeReserved:
  cpu: 80m
  memory: 1Gi
logging: {}
maxOpenFiles: 1000000
maxPods: 110
nodeStatusReportFrequency: 0s
nodeStatusUpdateFrequency: 10s
registryBurst: 10
registryPullQPS: 5
resolvConf: /etc/resolv.conf
rotateCertificates: true
runtimeRequestTimeout: 2m0s
serializeImagePulls: true
shutdownGracePeriod: 0s
shutdownGracePeriodCriticalPods: 0s
streamingConnectionIdleTimeout: 0s
syncFrequency: 1m0s
volumeStatsAggPeriod: 1m0s
`

		fileContent = &extensionsv1alpha1.FileContentInline{
			Data: data,
		}
	)

	Describe("#Encode", func() {
		It("should encode the given KubeletConfiguration into a FileContentInline appropriately", func() {
			// Create codec
			c := NewConfigCodec(oscutils.NewFileContentInlineCodec())

			// Call Encode and check result
			fci, err := c.Encode(kubeletConfig, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(fci).To(Equal(fileContent))
		})
	})

	Describe("#Decode", func() {
		It("should decode a KubeletConfiguration from the given FileContentInline appropriately", func() {
			// Create codec
			c := NewConfigCodec(oscutils.NewFileContentInlineCodec())

			// Call Decode and check result
			kc, err := c.Decode(fileContent)
			Expect(err).NotTo(HaveOccurred())
			Expect(kc).To(Equal(kubeletConfig))
		})
	})
})
