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

package logrotate_test

import (
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components/logrotate"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"
)

var _ = Describe("Logrotate", func() {
	Describe("#Config", func() {
		It("should return the expected units and files", func() {
			var (
				prefix       = "foo"
				pathConfig   = "/bar/baz"
				pathLogFiles = "/var/log/foo/*.log"
			)

			units, files := logrotate.Config(pathConfig, pathLogFiles, prefix)

			Expect(units).To(ConsistOf(
				extensionsv1alpha1.Unit{
					Name:   prefix + "-logrotate.service",
					Enable: pointer.BoolPtr(true),
					Content: pointer.StringPtr(`[Unit]
Description=Rotate and Compress System Logs
[Service]
ExecStart=/usr/sbin/logrotate ` + pathConfig + `
[Install]
WantedBy=multi-user.target`),
				},
				extensionsv1alpha1.Unit{
					Name:    prefix + "-logrotate.timer",
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
					Path:        pathConfig,
					Permissions: pointer.Int32Ptr(0644),
					Content: extensionsv1alpha1.FileContent{
						Inline: &extensionsv1alpha1.FileContentInline{
							Data: pathLogFiles + ` {
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
`,
						},
					},
				},
			))
		})
	})
})
