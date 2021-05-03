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

package containerruntime

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
)

// reconciler reconciles ContainerRuntime resources of Gardener's
// `extensions.gardener.cloud` API group.
type reconciler struct {
	logger   logr.Logger
	actuator Actuator

	client        client.Client
	reader        client.Reader
	statusUpdater extensionscontroller.StatusUpdater
}

// NewReconciler creates a new reconcile.Reconciler that reconciles
// ContainerRuntime resources of Gardener's `extensions.gardener.cloud` API group.
func NewReconciler(actuator Actuator) reconcile.Reconciler {
	logger := log.Log.WithName(ControllerName)

	return extensionscontroller.OperationAnnotationWrapper(
		func() client.Object { return &extensionsv1alpha1.ContainerRuntime{} },
		&reconciler{
			logger:        logger,
			actuator:      actuator,
			statusUpdater: extensionscontroller.NewStatusUpdater(logger),
		},
	)
}

// InjectFunc enables dependency injection into the actuator.
func (r *reconciler) InjectFunc(f inject.Func) error {
	return f(r.actuator)
}

// InjectClient injects the controller runtime client into the reconciler.
func (r *reconciler) InjectClient(client client.Client) error {
	r.client = client
	r.statusUpdater.InjectClient(client)
	return nil
}

func (r *reconciler) InjectAPIReader(reader client.Reader) error {
	r.reader = reader
	return nil
}

// Reconcile is the reconciler function that gets executed in case there are new events for `ContainerRuntime` resources.
func (r *reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	cr := &extensionsv1alpha1.ContainerRuntime{}
	if err := r.client.Get(ctx, request.NamespacedName, cr); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	cluster, err := extensionscontroller.GetCluster(ctx, r.client, cr.Namespace)
	if err != nil {
		return reconcile.Result{}, err
	}
	if extensionscontroller.IsFailed(cluster) {
		r.logger.Info("Stop reconciling ContainerRuntime of failed Shoot.", "namespace", request.Namespace, "name", cr.Name)
		return reconcile.Result{}, nil
	}

	operationType := gardencorev1beta1helper.ComputeOperationType(cr.ObjectMeta, cr.Status.LastOperation)

	switch {
	case extensionscontroller.IsMigrated(cr):
		return reconcile.Result{}, nil
	case operationType == gardencorev1beta1.LastOperationTypeMigrate:
		return r.migrate(ctx, cr, cluster)
	case cr.DeletionTimestamp != nil:
		return r.delete(ctx, cr, cluster)
	case cr.Annotations[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationRestore:
		return r.restore(ctx, cr, cluster)
	default:
		return r.reconcile(ctx, cr, cluster, operationType)
	}
}

func (r *reconciler) reconcile(ctx context.Context, cr *extensionsv1alpha1.ContainerRuntime, cluster *extensionscontroller.Cluster, operationType gardencorev1beta1.LastOperationType) (reconcile.Result, error) {
	if err := controllerutils.EnsureFinalizer(ctx, r.reader, r.client, cr, FinalizerName); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.statusUpdater.Processing(ctx, cr, operationType, "Reconciling the container runtime"); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.actuator.Reconcile(ctx, cr, cluster); err != nil {
		_ = r.statusUpdater.Error(ctx, cr, extensionscontroller.ReconcileErrCauseOrErr(err), operationType, "Error reconciling container runtime")
		return extensionscontroller.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, cr, operationType, "Successfully reconciled container runtime"); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) restore(ctx context.Context, cr *extensionsv1alpha1.ContainerRuntime, cluster *extensionscontroller.Cluster) (reconcile.Result, error) {
	if err := controllerutils.EnsureFinalizer(ctx, r.reader, r.client, cr, FinalizerName); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.statusUpdater.Processing(ctx, cr, gardencorev1beta1.LastOperationTypeRestore, "Restoring the container runtime"); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.actuator.Restore(ctx, cr, cluster); err != nil {
		_ = r.statusUpdater.Error(ctx, cr, extensionscontroller.ReconcileErrCauseOrErr(err), gardencorev1beta1.LastOperationTypeRestore, "Error restoring container runtime")
		return extensionscontroller.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, cr, gardencorev1beta1.LastOperationTypeRestore, "Successfully restored container runtime"); err != nil {
		return reconcile.Result{}, err
	}

	if err := extensionscontroller.RemoveAnnotation(ctx, r.client, cr, v1beta1constants.GardenerOperation); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing annotation from ContainerRuntime: %+v", err)
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) delete(ctx context.Context, cr *extensionsv1alpha1.ContainerRuntime, cluster *extensionscontroller.Cluster) (reconcile.Result, error) {
	if !controllerutil.ContainsFinalizer(cr, FinalizerName) {
		r.logger.Info("Deleting container runtime causes a no-op as there is no finalizer.", "containerruntime", cr.Name)
		return reconcile.Result{}, nil
	}

	if err := r.statusUpdater.Processing(ctx, cr, gardencorev1beta1.LastOperationTypeDelete, "Deleting the container runtime"); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.actuator.Delete(ctx, cr, cluster); err != nil {
		_ = r.statusUpdater.Error(ctx, cr, extensionscontroller.ReconcileErrCauseOrErr(err), gardencorev1beta1.LastOperationTypeDelete, "Error deleting container runtime")
		return extensionscontroller.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, cr, gardencorev1beta1.LastOperationTypeDelete, "Successfully deleted container runtime"); err != nil {
		return reconcile.Result{}, err
	}

	r.logger.Info("Removing finalizer.", "containerruntime", cr.Name)
	if err := controllerutils.RemoveFinalizer(ctx, r.reader, r.client, cr, FinalizerName); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing finalizer from the container runtime resource: %+v", err)
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) migrate(ctx context.Context, cr *extensionsv1alpha1.ContainerRuntime, cluster *extensionscontroller.Cluster) (reconcile.Result, error) {
	if err := r.statusUpdater.Processing(ctx, cr, gardencorev1beta1.LastOperationTypeMigrate, "Migrating the container runtime"); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.actuator.Migrate(ctx, cr, cluster); err != nil {
		_ = r.statusUpdater.Error(ctx, cr, extensionscontroller.ReconcileErrCauseOrErr(err), gardencorev1beta1.LastOperationTypeMigrate, "Error migrating container runtime")
		return extensionscontroller.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, cr, gardencorev1beta1.LastOperationTypeMigrate, "Successfully migrated container runtime"); err != nil {
		return reconcile.Result{}, err
	}

	r.logger.Info("Removing all finalizers.", "containerruntime", cr.Name)
	if err := extensionscontroller.DeleteAllFinalizers(ctx, r.client, cr); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing finalizers from the container runtime resource: %+v", err)
	}

	if err := extensionscontroller.RemoveAnnotation(ctx, r.client, cr, v1beta1constants.GardenerOperation); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing annotation from ContainerRuntime: %+v", err)
	}

	return reconcile.Result{}, nil
}
