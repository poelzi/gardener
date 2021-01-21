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

package event

import (
	"context"
	"fmt"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/logger"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var nowFunc = time.Now

func (c *Controller) enqueueEvent(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.eventQueue.Add(key)
}

func (c *Controller) reconcileEvent(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	gardenClient, err := c.clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get garden client: %w", err)
	}

	log := logger.NewFieldLogger(logger.Logger, "event", req.Namespace+"/"+req.Name)

	event := &corev1.Event{}
	if err := gardenClient.Client().Get(ctx, req.NamespacedName, event); err != nil {
		if apierrors.IsNotFound(err) {
			log.Debug("Skipping because Event has been deleted")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	if isShootEvent(event) {
		return reconcile.Result{}, nil
	}

	deleteAt := event.LastTimestamp.Add(c.cfg.TTLNonShootEvents.Duration)
	timeUntilDeletion := deleteAt.Sub(nowFunc())
	if timeUntilDeletion > 0 {
		return reconcile.Result{RequeueAfter: timeUntilDeletion}, nil
	}

	err = gardenClient.Client().Delete(ctx, event)
	return reconcile.Result{}, err
}

func isShootEvent(event *corev1.Event) bool {
	if gv, err := schema.ParseGroupVersion(event.InvolvedObject.APIVersion); err != nil || gv.Group != gardencorev1beta1.GroupName {
		return false
	}
	if event.InvolvedObject.Kind == "Shoot" {
		return true
	}
	return false
}
