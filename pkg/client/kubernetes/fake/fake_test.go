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

package fake_test

import (
	"context"
	"errors"

	"github.com/gardener/gardener/pkg/chartrenderer"
	gardencorefake "github.com/gardener/gardener/pkg/client/core/clientset/versioned/fake"
	"github.com/gardener/gardener/pkg/client/kubernetes/fake"
	mockkubernetes "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	"github.com/gardener/gardener/pkg/client/kubernetes/test"
	gardenseedmanagementfake "github.com/gardener/gardener/pkg/client/seedmanagement/clientset/versioned/fake"
	mockdiscovery "github.com/gardener/gardener/pkg/mock/client-go/discovery"
	mockcache "github.com/gardener/gardener/pkg/mock/controller-runtime/cache"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	apiextensionsfake "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	kubernetesfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	apiregistrationfake "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset/fake"
)

var _ = Describe("Fake ClientSet", func() {
	var (
		builder *fake.ClientSetBuilder
		ctrl    *gomock.Controller
	)

	BeforeEach(func() {
		builder = fake.NewClientSetBuilder()
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("should correctly set applier attribute", func() {
		applier := mockkubernetes.NewMockApplier(ctrl)
		cs := builder.WithApplier(applier).Build()

		Expect(cs.Applier()).To(BeIdenticalTo(applier))
	})

	It("should correctly set chartRenderer attribute", func() {
		chartRenderer := chartrenderer.NewWithServerVersion(&version.Info{Major: "1", Minor: "18"})
		cs := builder.WithChartRenderer(chartRenderer).Build()

		Expect(cs.ChartRenderer()).To(BeIdenticalTo(chartRenderer))
	})

	It("should correctly set chartApplier attribute", func() {
		chartApplier := mockkubernetes.NewMockChartApplier(ctrl)
		cs := builder.WithChartApplier(chartApplier).Build()

		Expect(cs.ChartApplier()).To(BeIdenticalTo(chartApplier))
	})

	It("should correctly set restConfig attribute", func() {
		restConfig := &rest.Config{}
		cs := builder.WithRESTConfig(restConfig).Build()

		Expect(cs.RESTConfig()).To(BeIdenticalTo(restConfig))
	})

	It("should correctly set client attribute", func() {
		client := mockclient.NewMockClient(ctrl)
		cs := builder.WithClient(client).Build()

		Expect(cs.Client()).To(BeIdenticalTo(client))
	})

	It("should correctly set apiReader attribute", func() {
		apiReader := mockclient.NewMockReader(ctrl)
		cs := builder.WithAPIReader(apiReader).Build()

		Expect(cs.APIReader()).To(BeIdenticalTo(apiReader))
	})

	It("should correctly set directClient attribute", func() {
		directClient := mockclient.NewMockClient(ctrl)
		cs := builder.WithDirectClient(directClient).Build()

		Expect(cs.DirectClient()).To(BeIdenticalTo(directClient))
	})

	It("should correctly set cache attribute", func() {
		cache := mockcache.NewMockCache(ctrl)
		cs := builder.WithCache(cache).Build()

		Expect(cs.Cache()).To(BeIdenticalTo(cache))
	})

	It("should correctly set kubernetes attribute", func() {
		kubernetes := kubernetesfake.NewSimpleClientset()
		cs := builder.WithKubernetes(kubernetes).Build()

		Expect(cs.Kubernetes()).To(BeIdenticalTo(kubernetes))
	})

	It("should correctly set gardenCore attribute", func() {
		gardenCore := gardencorefake.NewSimpleClientset()
		cs := builder.WithGardenCore(gardenCore).Build()

		Expect(cs.GardenCore()).To(BeIdenticalTo(gardenCore))
	})

	It("should correctly set gardenSeedManagement attribute", func() {
		gardenSeedManagement := gardenseedmanagementfake.NewSimpleClientset()
		cs := builder.WithGardenSeedManagement(gardenSeedManagement).Build()

		Expect(cs.GardenSeedManagement()).To(BeIdenticalTo(gardenSeedManagement))
	})

	It("should correctly set apiextension attribute", func() {
		apiextension := apiextensionsfake.NewSimpleClientset()
		cs := builder.WithAPIExtension(apiextension).Build()

		Expect(cs.APIExtension()).To(BeIdenticalTo(apiextension))
	})

	It("should correctly set apiregistration attribute", func() {
		apiregistration := apiregistrationfake.NewSimpleClientset()
		cs := builder.WithAPIRegistration(apiregistration).Build()

		Expect(cs.APIRegistration()).To(BeIdenticalTo(apiregistration))
	})

	It("should correctly set restClient attribute", func() {
		disc, err := discovery.NewDiscoveryClientForConfig(&rest.Config{})
		Expect(err).NotTo(HaveOccurred())
		restClient := disc.RESTClient()
		cs := builder.WithRESTClient(restClient).Build()

		Expect(cs.RESTClient()).To(BeIdenticalTo(restClient))
	})

	It("should correctly set version attribute", func() {
		version := "1.18.0"
		cs := builder.WithVersion(version).Build()

		Expect(cs.Version()).To(Equal(version))
	})

	Context("#DiscoverVersion", func() {
		It("should correctly refresh server version", func() {
			oldVersion, newVersion := "1.18.1", "1.18.2"
			cs := builder.
				WithVersion(oldVersion).
				WithKubernetes(test.NewClientSetWithFakedServerVersion(nil, &version.Info{GitVersion: newVersion})).
				Build()

			Expect(cs.Version()).To(Equal(oldVersion))
			_, err := cs.DiscoverVersion()
			Expect(err).NotTo(HaveOccurred())
			Expect(cs.Version()).To(Equal(newVersion))
		})

		It("should fail if discovery fails", func() {
			discovery := mockdiscovery.NewMockDiscoveryInterface(ctrl)
			discovery.EXPECT().ServerVersion().Return(nil, errors.New("fake"))

			cs := builder.
				WithKubernetes(test.NewClientSetWithDiscovery(nil, discovery)).
				Build()

			_, err := cs.DiscoverVersion()
			Expect(err).To(MatchError("fake"))
		})
	})

	It("should do nothing on Start", func() {
		cs := fake.NewClientSet()

		cs.Start(context.Background())
	})

	It("should do nothing on WaitForCacheSync", func() {
		cs := fake.NewClientSet()

		Expect(cs.WaitForCacheSync(context.Background())).To(BeTrue())
	})

	It("should do nothing on ForwardPodPort", func() {
		cs := fake.NewClientSet()

		ch, err := cs.ForwardPodPort("", "", 0, 0)
		Expect(ch).To(BeNil())
		Expect(err).NotTo(HaveOccurred())
	})

	It("should do nothing on CheckForwardPodPort", func() {
		cs := builder.Build()

		Expect(cs.CheckForwardPodPort("", "", 0, 0)).To(Succeed())
	})

})
