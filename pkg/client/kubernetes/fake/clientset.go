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

package fake

import (
	"context"

	"github.com/gardener/gardener/pkg/chartrenderer"
	gardencoreclientset "github.com/gardener/gardener/pkg/client/core/clientset/versioned"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenseedmanagementclientset "github.com/gardener/gardener/pkg/client/seedmanagement/clientset/versioned"

	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/version"
	kubernetesclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	apiregistrationclientset "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ kubernetes.Interface = &ClientSet{}

// ClientSet contains information to provide a fake implementation for tests.
type ClientSet struct {
	CheckForwardPodPortFn

	applier              kubernetes.Applier
	chartRenderer        chartrenderer.Interface
	chartApplier         kubernetes.ChartApplier
	restConfig           *rest.Config
	client               client.Client
	apiReader            client.Reader
	directClient         client.Client
	cache                cache.Cache
	kubernetes           kubernetesclientset.Interface
	gardenCore           gardencoreclientset.Interface
	gardenSeedManagement gardenseedmanagementclientset.Interface
	apiextension         apiextensionsclientset.Interface
	apiregistration      apiregistrationclientset.Interface
	restClient           rest.Interface
	version              string
}

// NewClientSet returns a new empty fake ClientSet.
func NewClientSet() *ClientSet {
	return &ClientSet{}
}

// Applier returns the applier of this ClientSet.
func (c *ClientSet) Applier() kubernetes.Applier {
	return c.applier
}

// ChartRenderer returns a ChartRenderer populated with the cluster's Capabilities.
func (c *ClientSet) ChartRenderer() chartrenderer.Interface {
	return c.chartRenderer
}

// ChartApplier returns a ChartApplier using the ClientSet's ChartRenderer and Applier.
func (c *ClientSet) ChartApplier() kubernetes.ChartApplier {
	return c.chartApplier
}

// RESTConfig will return the restConfig attribute of the Client object.
func (c *ClientSet) RESTConfig() *rest.Config {
	return c.restConfig
}

// Client returns the controller-runtime client of this ClientSet.
func (c *ClientSet) Client() client.Client {
	return c.client
}

// APIReader returns a client.Reader that directly reads from the API server.
func (c *ClientSet) APIReader() client.Reader {
	return c.apiReader
}

// DirectClient returns a controller-runtime client, which can be used to talk to the API server directly
// (without using a cache).
// Deprecated: used APIReader instead, if the controller can't tolerate stale reads.
func (c *ClientSet) DirectClient() client.Client {
	return c.directClient
}

// Cache returns the clientset's controller-runtime cache. It can be used to get Informers for arbitrary objects.
func (c *ClientSet) Cache() cache.Cache {
	return c.cache
}

// Kubernetes will return the kubernetes attribute of the Client object.
func (c *ClientSet) Kubernetes() kubernetesclientset.Interface {
	return c.kubernetes
}

// GardenCore will return the gardenCore attribute of the Client object.
func (c *ClientSet) GardenCore() gardencoreclientset.Interface {
	return c.gardenCore
}

// GardenSeedManagement will return the gardenSeedManagement attribute of the Client object.
func (c *ClientSet) GardenSeedManagement() gardenseedmanagementclientset.Interface {
	return c.gardenSeedManagement
}

// APIExtension will return the apiextension ClientSet attribute of the Client object.
func (c *ClientSet) APIExtension() apiextensionsclientset.Interface {
	return c.apiextension
}

// APIRegistration will return the apiregistration attribute of the Client object.
func (c *ClientSet) APIRegistration() apiregistrationclientset.Interface {
	return c.apiregistration
}

// RESTClient will return the restClient attribute of the Client object.
func (c *ClientSet) RESTClient() rest.Interface {
	return c.restClient
}

// Version returns the GitVersion of the Kubernetes client stored on the object.
func (c *ClientSet) Version() string {
	return c.version
}

// DiscoverVersion tries to retrieve the server version using the kubernetes discovery client.
func (c *ClientSet) DiscoverVersion() (*version.Info, error) {
	serverVersion, err := c.Kubernetes().Discovery().ServerVersion()
	if err != nil {
		return nil, err
	}

	c.version = serverVersion.GitVersion
	return serverVersion, nil
}

// Start does nothing as the fake ClientSet does not support it.
func (c *ClientSet) Start(context.Context) {
}

// WaitForCacheSync does nothing and return trues.
func (c *ClientSet) WaitForCacheSync(context.Context) bool {
	return true
}

// ForwardPodPort does nothing as the fake ClientSet does not support it.
func (c *ClientSet) ForwardPodPort(string, string, int, int) (chan struct{}, error) {
	return nil, nil
}

// CheckForwardPodPortFn is a type alias for a function checking port forwarding for pods.
type CheckForwardPodPortFn func(string, string, int, int) error

// CheckForwardPodPort does nothing as the fake ClientSet does not support it.
func (f CheckForwardPodPortFn) CheckForwardPodPort(namespace, name string, src, dst int) error {
	return f(namespace, name, src, dst)
}
