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

package seed

import (
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/logger"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func filterGardenSecret(obj interface{}) bool {
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return false
	}
	if secret.Namespace != v1beta1constants.GardenNamespace {
		return false
	}
	return gardenRoleSelector.Matches(labels.Set(secret.Labels))
}

func (c *Controller) enqueueSeeds() {
	seeds, err := c.seedLister.List(labels.Everything())
	if err != nil {
		logger.Logger.Errorf("Could not enqueue seeds: %v", err)
	}
	for _, seed := range seeds {
		c.seedQueue.Add(client.ObjectKeyFromObject(seed).String())
	}
}

func (c *Controller) gardenSecretAdd(_ interface{}) {
	c.enqueueSeeds()
}

func (c *Controller) gardenSecretUpdate(oldObj, newObj interface{}) {
	oldSecret := oldObj.(*corev1.Secret)
	newSecret := newObj.(*corev1.Secret)

	if !apiequality.Semantic.DeepEqual(oldSecret, newSecret) {
		c.enqueueSeeds()
	}
}

func (c *Controller) gardenSecretDelete(_ interface{}) {
	c.enqueueSeeds()
}
