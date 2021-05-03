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

package containerd_test

import (
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components/containerd"
	"github.com/gardener/gardener/pkg/utils"

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
					Name:    "containerd-monitor.service",
					Command: pointer.StringPtr("start"),
					Enable:  pointer.BoolPtr(true),
					Content: pointer.StringPtr(`[Unit]
Description=Containerd-monitor daemon
After=containerd.service
[Install]
WantedBy=multi-user.target
[Service]
Restart=always
EnvironmentFile=/etc/environment
ExecStart=/opt/bin/health-monitor-containerd`),
				},
				extensionsv1alpha1.Unit{
					Name:   "containerd-logrotate.service",
					Enable: pointer.BoolPtr(true),
					Content: pointer.StringPtr(`[Unit]
Description=Rotate and Compress System Logs
[Service]
ExecStart=/usr/sbin/logrotate /etc/systemd/containerd.conf
[Install]
WantedBy=multi-user.target`),
				},
				extensionsv1alpha1.Unit{
					Name:    "containerd-logrotate.timer",
					Command: pointer.StringPtr("start"),
					Enable:  pointer.BoolPtr(true),
					Content: pointer.StringPtr(`[Unit]
Description=Log Rotation at each 10 minutes
[Timer]
OnCalendar=*:0/10
AccuracySec=1min
Persistent=true
[Install]
WantedBy=multi-user.target`),
				},
			))
			Expect(files).To(ConsistOf(
				extensionsv1alpha1.File{
					Path:        "/opt/bin/health-monitor-containerd",
					Permissions: pointer.Int32Ptr(0755),
					Content: extensionsv1alpha1.FileContent{
						Inline: &extensionsv1alpha1.FileContentInline{
							Encoding: "b64",
							Data:     utils.EncodeBase64([]byte(healthMonitorScript)),
						},
					},
				},
				extensionsv1alpha1.File{
					Path:        "/etc/systemd/containerd.conf",
					Permissions: pointer.Int32Ptr(0644),
					Content: extensionsv1alpha1.FileContent{
						Inline: &extensionsv1alpha1.FileContentInline{
							Data: logRotateData,
						},
					},
				},
			))
		})
	})
})

const (
	healthMonitorScript = `#!/bin/bash
set -o nounset
set -o pipefail

function containerd_monitoring {
  echo "ContainerD monitor has started !"
  while [ 1 ]; do
    if ! timeout 60 ctr c list > /dev/null; then
      echo "ContainerD daemon failed!"
      pkill containerd
      sleep 30
    else
      sleep $SLEEP_SECONDS
    fi
  done
}

SLEEP_SECONDS=10
echo "Start health monitoring for containerd"
containerd_monitoring
`

	logRotateData = `/var/log/pods/*/*/*.log {
    rotate 14
    copytruncate
    missingok
    notifempty
    compress
    maxsize 100M
    daily
    dateext
    dateformat -%Y%m%d-%s
    create 0644 root root
}
`
)
