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

package kernelconfig_test

import (
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components/kernelconfig"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"
)

var _ = Describe("Component", func() {
	Describe("#Config", func() {
		var component components.Component

		BeforeEach(func() {
			component = New()
		})

		It("should return the expected units and files", func() {
			units, files, err := component.Config(components.Context{})

			Expect(err).NotTo(HaveOccurred())
			Expect(units).To(ConsistOf(
				extensionsv1alpha1.Unit{
					Name:    "systemd-sysctl.service",
					Command: pointer.StringPtr("restart"),
					Enable:  pointer.BoolPtr(true),
				},
			))
			Expect(files).To(ConsistOf(
				extensionsv1alpha1.File{
					Path:        "/etc/sysctl.d/99-k8s-general.conf",
					Permissions: pointer.Int32Ptr(0644),
					Content: extensionsv1alpha1.FileContent{
						Inline: &extensionsv1alpha1.FileContentInline{
							Data: data,
						},
					},
				},
			))
		})
	})
})

const data = `# A higher vm.max_map_count is great for elasticsearch, mongo, or other mmap users
# See https://github.com/kubernetes/kops/issues/1340
vm.max_map_count = 135217728
# See https://github.com/kubernetes/kubernetes/pull/38001
kernel.softlockup_panic = 1
kernel.softlockup_all_cpu_backtrace = 1
# See https://github.com/kubernetes/kube-deploy/issues/261
# Increase the number of connections
net.core.somaxconn = 32768
# Increase number of incoming connections backlog
net.core.netdev_max_backlog = 5000
# Maximum Socket Receive Buffer
net.core.rmem_max = 16777216
# Default Socket Send Buffer
net.core.wmem_max = 16777216
# Increase the maximum total buffer-space allocatable
net.ipv4.tcp_wmem = 4096 12582912 16777216
net.ipv4.tcp_rmem = 4096 12582912 16777216
# Mitigate broken TCP connections
# https://github.com/kubernetes/kubernetes/issues/41916#issuecomment-312428731
net.ipv4.tcp_retries2 = 8
# Increase the number of outstanding syn requests allowed
net.ipv4.tcp_max_syn_backlog = 8096
# For persistent HTTP connections
net.ipv4.tcp_slow_start_after_idle = 0
# Increase the tcp-time-wait buckets pool size to prevent simple DOS attacks
net.ipv4.tcp_tw_reuse = 1
# Allowed local port range.
net.ipv4.ip_local_port_range = 32768 65535
# Max number of packets that can be queued on interface input
# If kernel is receiving packets faster than can be processed
# this queue increases
net.core.netdev_max_backlog = 16384
# Increase size of file handles and inode cache
fs.file-max = 20000000
# Max number of inotify instances and watches for a user
# Since dockerd runs as a single user, the default instances value of 128 per user is too low
# e.g. uses of inotify: nginx ingress controller, kubectl logs -f
fs.inotify.max_user_instances = 8192
fs.inotify.max_user_watches = 524288
# HANA requirement
# See https://www.sap.com/developer/tutorials/hxe-ua-install-using-docker.html
fs.aio-max-nr = 262144
vm.memory_failure_early_kill = 1
# A common problem on Linux systems is running out of space in the conntrack table,
# which can cause poor iptables performance.
# This can happen if you run a lot of workloads on a given host,
# or if your workloads create a lot of TCP connections or bidirectional UDP streams.
net.netfilter.nf_conntrack_max = 1048576
`
