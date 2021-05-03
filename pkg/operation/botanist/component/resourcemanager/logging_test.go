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

package resourcemanager_test

import (
	"github.com/gardener/gardener/pkg/operation/botanist/component/resourcemanager"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Logging", func() {
	Describe("#CentralLoggingConfiguration", func() {
		It("should return the expected logging parser and filter", func() {
			loggingConfig, err := resourcemanager.CentralLoggingConfiguration()

			Expect(err).NotTo(HaveOccurred())
			Expect(loggingConfig.Parsers).To(Equal(`[PARSER]
    Name        gardenerResourceManagerParser
    Format      json
    Time_Key    ts
    Time_Format %Y-%m-%dT%H:%M:%S.%L
`))

			Expect(loggingConfig.Filters).To(Equal(`[FILTER]
    Name                parser
    Match               kubernetes.*gardener-resource-manager*gardener-resource-manager*
    Key_Name            log
    Parser              gardenerResourceManagerParser
    Reserve_Data        True

[FILTER]
    Name                modify
    Match               kubernetes.*gardener-resource-manager*gardener-resource-manager*
    Rename              level  severity
    Rename              msg    log
    Rename              logger source
`))
		})
	})
})
