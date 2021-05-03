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

package v18_test

import (
	"testing"

	v18 "github.com/gardener/gardener/pkg/operation/botanist/component/gardenerkubescheduler/v18"
	"github.com/gardener/gardener/third_party/kube-scheduler/v18/v1alpha2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"
)

func TestConfigurator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Botanist Component GardenerKubeScheduler v18 Suite")
}

var _ = Describe("NewConfigurator", func() {
	It("should not return nil", func() {
		c, err := v18.NewConfigurator("baz", "test", &v1alpha2.KubeSchedulerConfiguration{})

		Expect(err).NotTo(HaveOccurred())
		Expect(c).NotTo(BeNil())
	})
})

var _ = Describe("Config", func() {
	var output, sha256 string

	JustBeforeEach(func() {
		c, err := v18.NewConfigurator("baz", "test", &v1alpha2.KubeSchedulerConfiguration{
			Profiles: []v1alpha2.KubeSchedulerProfile{
				{
					SchedulerName: pointer.StringPtr("test"),
				},
			},
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(c).NotTo(BeNil())

		output, sha256, err = c.Config()
		Expect(err).NotTo(HaveOccurred())
	})

	It("returns correct config", func() {
		Expect(output).To(Equal(`apiVersion: kubescheduler.config.k8s.io/v1alpha2
bindTimeoutSeconds: null
clientConnection:
  acceptContentTypes: ""
  burst: 0
  contentType: ""
  kubeconfig: ""
  qps: 0
extenders: null
kind: KubeSchedulerConfiguration
leaderElection:
  leaderElect: true
  leaseDuration: 15s
  renewDeadline: 10s
  resourceLock: leases
  resourceName: baz
  resourceNamespace: test
  retryPeriod: 2s
podInitialBackoffSeconds: null
podMaxBackoffSeconds: null
profiles:
- schedulerName: test
`))
	})

	It("returns correct hash", func() {
		Expect(sha256).To(Equal("963561865568f3d43baf03ff2b57a9a8b74e314dd2e6a63acc65ed3552268488"))
	})
})
