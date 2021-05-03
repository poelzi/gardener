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

package graph

import (
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"
)

func (g *graph) setupSeedWatch(informer cache.Informer) {
	informer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			seed, ok := obj.(*gardencorev1beta1.Seed)
			if !ok {
				return
			}
			g.handleSeedCreateOrUpdate(seed)
		},

		UpdateFunc: func(oldObj, newObj interface{}) {
			oldSeed, ok := oldObj.(*gardencorev1beta1.Seed)
			if !ok {
				return
			}

			newSeed, ok := newObj.(*gardencorev1beta1.Seed)
			if !ok {
				return
			}

			if !apiequality.Semantic.DeepEqual(oldSeed.Spec.SecretRef, newSeed.Spec.SecretRef) ||
				!gardencorev1beta1helper.SeedBackupSecretRefEqual(oldSeed.Spec.Backup, newSeed.Spec.Backup) {
				g.handleSeedCreateOrUpdate(newSeed)
			}
		},

		DeleteFunc: func(obj interface{}) {
			if tombstone, ok := obj.(toolscache.DeletedFinalStateUnknown); ok {
				obj = tombstone.Obj
			}
			seed, ok := obj.(*gardencorev1beta1.Seed)
			if !ok {
				return
			}
			g.handleSeedDelete(seed)
		},
	})
}

func (g *graph) handleSeedCreateOrUpdate(seed *gardencorev1beta1.Seed) {
	start := time.Now()
	defer func() {
		metricUpdateDuration.WithLabelValues("Seed", "CreateOrUpdate").Observe(time.Since(start).Seconds())
	}()
	g.lock.Lock()
	defer g.lock.Unlock()

	g.deleteAllIncomingEdges(VertexTypeSecret, VertexTypeSeed, "", seed.Name)
	g.deleteAllIncomingEdges(VertexTypeNamespace, VertexTypeSeed, "", seed.Name)

	seedVertex := g.getOrCreateVertex(VertexTypeSeed, "", seed.Name)
	namespaceVertex := g.getOrCreateVertex(VertexTypeNamespace, "", gardenerutils.ComputeGardenNamespace(seed.Name))
	g.addEdge(namespaceVertex, seedVertex)

	if seed.Spec.SecretRef != nil {
		secretVertex := g.getOrCreateVertex(VertexTypeSecret, seed.Spec.SecretRef.Namespace, seed.Spec.SecretRef.Name)
		g.addEdge(secretVertex, seedVertex)
	}

	if seed.Spec.Backup != nil {
		secretVertex := g.getOrCreateVertex(VertexTypeSecret, seed.Spec.Backup.SecretRef.Namespace, seed.Spec.Backup.SecretRef.Name)
		g.addEdge(secretVertex, seedVertex)
	}
}

func (g *graph) handleSeedDelete(seed *gardencorev1beta1.Seed) {
	start := time.Now()
	defer func() {
		metricUpdateDuration.WithLabelValues("Seed", "Delete").Observe(time.Since(start).Seconds())
	}()
	g.lock.Lock()
	defer g.lock.Unlock()

	g.deleteVertex(VertexTypeSeed, "", seed.Name)
}
