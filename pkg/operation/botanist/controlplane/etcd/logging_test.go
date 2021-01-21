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

package etcd_test

import (
	. "github.com/gardener/gardener/pkg/operation/botanist/controlplane/etcd"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Logging", func() {
	Describe("#CentralLoggingConfiguration", func() {
		It("should return the expected logging parser and filter", func() {
			loggingConfig, err := CentralLoggingConfiguration()

			Expect(err).NotTo(HaveOccurred())
			Expect(loggingConfig.Parsers).To(Equal(`[PARSER]
    Name        etcdParser
    Format      regex
    Regex       ^(?<time>\d{4}-\d{2}-\d{2}\s+[^ ]*)\s+(?<severity>\w+)\s+\|\s+(?<source>[^ :]*):\s+(?<log>.*)
    Time_Key    time
    Time_Format %Y-%m-%d %H:%M:%S.%L

[PARSER]
    Name        backupRestoreParser
    Format      regex
    Regex       ^time="(?<time>\d{4}-\d{2}-\d{2}T[^"]*)"\s+level=(?<severity>\w+)\smsg="(?<log>.*)"
    Time_Key    time
    Time_Format %Y-%m-%dT%H:%M:%S%z
`))

			Expect(loggingConfig.Filters).To(Equal(`[FILTER]
    Name                parser
    Match               kubernetes.*etcd*etcd*
    Key_Name            log
    Parser              etcdParser
    Reserve_Data        True

[FILTER]
    Name                parser
    Match               kubernetes.*etcd*backup-restore*
    Key_Name            log
    Parser              backupRestoreParser
    Reserve_Data        True
`))
			Expect(loggingConfig.PodPrefix).To(BeEmpty())
			Expect(loggingConfig.UserExposed).To(BeFalse())
		})
	})
})
