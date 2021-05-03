// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package cloudprofile

import (
	"context"
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/logger"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (c *Controller) cloudProfileAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.cloudProfileQueue.Add(key)
}

func (c *Controller) cloudProfileUpdate(_, newObj interface{}) {
	c.cloudProfileAdd(newObj)
}

func (c *Controller) cloudProfileDelete(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.cloudProfileQueue.Add(key)
}

// NewCloudProfileReconciler creates a new instance of a reconciler which reconciles CloudProfiles.
func NewCloudProfileReconciler(l logrus.FieldLogger, gardenClient client.Client, recorder record.EventRecorder) reconcile.Reconciler {
	return &cloudProfileReconciler{
		logger:       l,
		gardenClient: gardenClient,
		recorder:     recorder,
	}
}

type cloudProfileReconciler struct {
	logger       logrus.FieldLogger
	gardenClient client.Client
	recorder     record.EventRecorder
}

func (r *cloudProfileReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	cloudProfile := &gardencorev1beta1.CloudProfile{}
	if err := r.gardenClient.Get(ctx, request.NamespacedName, cloudProfile); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Infof("Object %q is gone, stop reconciling: %v", request.Name, err)
			return reconcile.Result{}, nil
		}
		r.logger.Infof("Unable to retrieve object %q from store: %v", request.Name, err)
		return reconcile.Result{}, err
	}

	cloudProfileLogger := logger.NewFieldLogger(logger.Logger, "cloudprofile", cloudProfile.Name)

	// The deletionTimestamp labels the CloudProfile as intended to get deleted. Before deletion, it has to be ensured that
	// no Shoots and Seed are assigned to the CloudProfile anymore. If this is the case then the controller will remove
	// the finalizers from the CloudProfile so that it can be garbage collected.
	if cloudProfile.DeletionTimestamp != nil {
		if !sets.NewString(cloudProfile.Finalizers...).Has(gardencorev1beta1.GardenerName) {
			return reconcile.Result{}, nil
		}

		associatedShoots, err := controllerutils.DetermineShootsAssociatedTo(ctx, r.gardenClient, cloudProfile)
		if err != nil {
			cloudProfileLogger.Error(err.Error())
			return reconcile.Result{}, err
		}

		if len(associatedShoots) == 0 {
			cloudProfileLogger.Infof("No Shoots are referencing the CloudProfile. Deletion accepted.")

			if err := controllerutils.PatchRemoveFinalizers(ctx, r.gardenClient, cloudProfile, gardencorev1beta1.GardenerName); client.IgnoreNotFound(err) != nil {
				logger.Logger.Errorf("could not remove finalizer from CloudProfile: %s", err.Error())
				return reconcile.Result{}, err
			}
			return reconcile.Result{}, nil
		}

		message := fmt.Sprintf("Can't delete CloudProfile, because the following Shoots are still referencing it: %+v", associatedShoots)
		cloudProfileLogger.Info(message)
		r.recorder.Event(cloudProfile, corev1.EventTypeNormal, v1beta1constants.EventResourceReferenced, message)

		return reconcile.Result{}, fmt.Errorf("CloudProfile %q still has references", cloudProfile.Name)
	}

	if err := controllerutils.PatchAddFinalizers(ctx, r.gardenClient, cloudProfile, gardencorev1beta1.GardenerName); err != nil {
		logger.Logger.Errorf("could not add finalizer to CloudProfile: %s", err.Error())
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}
