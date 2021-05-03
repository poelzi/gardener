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

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"
)

func (g *graph) setupBackupEntryWatch(informer cache.Informer) {
	informer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			backupEntry, ok := obj.(*gardencorev1beta1.BackupEntry)
			if !ok {
				return
			}
			g.handleBackupEntryCreateOrUpdate(backupEntry)
		},

		UpdateFunc: func(oldObj, newObj interface{}) {
			oldBackupEntry, ok := oldObj.(*gardencorev1beta1.BackupEntry)
			if !ok {
				return
			}

			newBackupEntry, ok := newObj.(*gardencorev1beta1.BackupEntry)
			if !ok {
				return
			}

			if !apiequality.Semantic.DeepEqual(oldBackupEntry.Spec.SeedName, newBackupEntry.Spec.SeedName) ||
				oldBackupEntry.Spec.BucketName != newBackupEntry.Spec.BucketName {
				g.handleBackupEntryCreateOrUpdate(newBackupEntry)
			}
		},

		DeleteFunc: func(obj interface{}) {
			if tombstone, ok := obj.(toolscache.DeletedFinalStateUnknown); ok {
				obj = tombstone.Obj
			}
			backupEntry, ok := obj.(*gardencorev1beta1.BackupEntry)
			if !ok {
				return
			}
			g.handleBackupEntryDelete(backupEntry)
		},
	})
}

func (g *graph) handleBackupEntryCreateOrUpdate(backupEntry *gardencorev1beta1.BackupEntry) {
	start := time.Now()
	defer func() {
		metricUpdateDuration.WithLabelValues("BackupEntry", "CreateOrUpdate").Observe(time.Since(start).Seconds())
	}()
	g.lock.Lock()
	defer g.lock.Unlock()

	g.deleteVertex(VertexTypeBackupEntry, backupEntry.Namespace, backupEntry.Name)

	var (
		backupEntryVertex  = g.getOrCreateVertex(VertexTypeBackupEntry, backupEntry.Namespace, backupEntry.Name)
		backupBucketVertex = g.getOrCreateVertex(VertexTypeBackupBucket, "", backupEntry.Spec.BucketName)
	)

	g.addEdge(backupEntryVertex, backupBucketVertex)

	if backupEntry.Spec.SeedName != nil {
		seedVertex := g.getOrCreateVertex(VertexTypeSeed, "", *backupEntry.Spec.SeedName)
		g.addEdge(backupEntryVertex, seedVertex)
	}
}

func (g *graph) handleBackupEntryDelete(backupEntry *gardencorev1beta1.BackupEntry) {
	start := time.Now()
	defer func() {
		metricUpdateDuration.WithLabelValues("BackupEntry", "Delete").Observe(time.Since(start).Seconds())
	}()
	g.lock.Lock()
	defer g.lock.Unlock()

	g.deleteVertex(VertexTypeBackupEntry, backupEntry.Namespace, backupEntry.Name)
}
