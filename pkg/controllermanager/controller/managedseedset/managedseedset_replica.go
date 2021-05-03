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

package managedseedset

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	seedmanagementhelper "github.com/gardener/gardener/pkg/apis/seedmanagement/helper"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	seedmanagementv1alpha1constants "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1/constants"
	operationshoot "github.com/gardener/gardener/pkg/operation/shoot"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ReplicaStatus represents a creation / update / deletion status of a ManagedSeedSet replica.
// During replica creation / update, the status changes like this:
// x => ShootReconciling => ShootReconciled => ManagedSeedPreparing => ManagedSeedRegistered
// During replica deletion, the status changes like this:
// ManagedSeedRegistered => ManagedSeedDeleting => ShootReconciled => ShootDeleting => x
// If shoot reconciliation or deletion fails, the status can become also ShootReconcileFailed or ShootDeleteFailed.
// Upon retry, it will become ShootReconciling or ShootDeleting until it either succeeds or fails again.
// Note that we are only interested in the shoot reconciliation / deletion status if the managed seed doesn't exist
// (either not created yet or already deleted), or if it's on a lower revision than the shoot (during updates).
type ReplicaStatus int

// Replica status constants
const (
	StatusUnknown               ReplicaStatus = iota // 0
	StatusShootReconcileFailed                       // 1
	StatusShootDeleteFailed                          // 2
	StatusShootReconciling                           // 3
	StatusShootDeleting                              // 4
	StatusShootReconciled                            // 5
	StatusManagedSeedPreparing                       // 6
	StatusManagedSeedDeleting                        // 7
	StatusManagedSeedRegistered                      // 8
)

// String returns a representation of this replica status as a string.
func (rs ReplicaStatus) String() string {
	switch rs {
	case StatusShootReconcileFailed:
		return "ShootReconcileFailed"
	case StatusShootDeleteFailed:
		return "ShootDeleteFailed"
	case StatusShootReconciling:
		return "ShootReconciling"
	case StatusShootDeleting:
		return "ShootDeleting"
	case StatusShootReconciled:
		return "ShootReconciled"
	case StatusManagedSeedPreparing:
		return "ManagedSeedPreparing"
	case StatusManagedSeedDeleting:
		return "ManagedSeedDeleting"
	case StatusManagedSeedRegistered:
		return "ManagedSeedRegistered"
	default:
		return "Unknown"
	}
}

// Replica represents a ManagedSeedSet replica.
type Replica interface {
	// GetName returns this replica's name.
	GetName() string
	// GetFullName returns this replica's full name.
	GetFullName() string
	// GetOrdinal returns this replica's ordinal. If the replica has no ordinal, -1 is returned.
	GetOrdinal() int
	// GetStatus returns this replica's status. If the replica's managed seed doesn't exist,
	// it returns one of the StatusShoot* statuses, depending on the shoot state.
	// Otherwise, it returns one of the ManagedSeed* statuses, depending on the managed seed state.
	GetStatus() ReplicaStatus
	// IsSeedReady returns true if this replica's seed is ready, false otherwise.
	IsSeedReady() bool
	// GetShootHealthStatus returns this replica's shoot health status (healthy, progressing, or unhealthy).
	GetShootHealthStatus() operationshoot.Status
	// IsDeletable returns true if this replica can be deleted, false otherwise. A replica can be deleted if it has no
	// scheduled shoots and is not protected by the "protect-from-deletion" annotation.
	IsDeletable() bool
	// CreateShoot initializes this replica's shoot and then creates it using the given context and client.
	CreateShoot(ctx context.Context, c client.Client, ordinal int) error
	// CreateManagedSeed initializes this replica's managed seed, and then creates it using the given context and client.
	CreateManagedSeed(ctx context.Context, c client.Client) error
	// DeleteShoot deletes this replica's shoot using the given context and client.
	DeleteShoot(ctx context.Context, c client.Client) error
	// DeleteManagedSeed deletes this replica's managed seed using the given context and client.
	DeleteManagedSeed(ctx context.Context, c client.Client) error
	// RetryShoot retries this replica's shoot using the given context and client.
	RetryShoot(ctx context.Context, c client.Client) error
}

// ReplicaFactory provides a method for creating new replicas.
type ReplicaFactory interface {
	// NewReplica creates and returns a new replica with the given parameters.
	NewReplica(*seedmanagementv1alpha1.ManagedSeedSet, *gardencorev1beta1.Shoot, *seedmanagementv1alpha1.ManagedSeed, *gardencorev1beta1.Seed, bool) Replica
}

// ReplicaFactoryFunc is a function that implements ReplicaFactory.
type ReplicaFactoryFunc func(*seedmanagementv1alpha1.ManagedSeedSet, *gardencorev1beta1.Shoot, *seedmanagementv1alpha1.ManagedSeed, *gardencorev1beta1.Seed, bool) Replica

// NewReplica creates and returns a new Replica with the given parameters.
func (f ReplicaFactoryFunc) NewReplica(
	set *seedmanagementv1alpha1.ManagedSeedSet,
	shoot *gardencorev1beta1.Shoot,
	managedSeed *seedmanagementv1alpha1.ManagedSeed,
	seed *gardencorev1beta1.Seed,
	hasScheduledShoots bool,
) Replica {
	return f(set, shoot, managedSeed, seed, hasScheduledShoots)
}

// replica is a concrete implementation of Replica. It has a shoot, a managed seed, the seed registered by it, and
// all shoots scheduled on the seed.
type replica struct {
	set                *seedmanagementv1alpha1.ManagedSeedSet
	shoot              *gardencorev1beta1.Shoot
	managedSeed        *seedmanagementv1alpha1.ManagedSeed
	seed               *gardencorev1beta1.Seed
	hasScheduledShoots bool
}

// NewReplica creates and returns a new Replica with the given parameters.
func NewReplica(
	set *seedmanagementv1alpha1.ManagedSeedSet,
	shoot *gardencorev1beta1.Shoot,
	managedSeed *seedmanagementv1alpha1.ManagedSeed,
	seed *gardencorev1beta1.Seed,
	hasScheduledShoots bool,
) Replica {
	return &replica{
		set:                set,
		shoot:              shoot,
		managedSeed:        managedSeed,
		seed:               seed,
		hasScheduledShoots: hasScheduledShoots,
	}
}

// GetName returns this replica's name. This is the name of the shoot, managed seed, and seed of this replica.
func (r *replica) GetName() string {
	if r.shoot == nil {
		return ""
	}
	return r.shoot.Name
}

// GetFullName returns this replica's full name. This is the namespace/name of the shoot and managed seed of this replica.
func (r *replica) GetFullName() string {
	if r.shoot == nil {
		return ""
	}
	return kutil.ObjectName(r.shoot)
}

// GetOrdinal returns this replica's ordinal. If the replica has no ordinal, -1 is returned.
func (r *replica) GetOrdinal() int {
	if r.shoot == nil {
		return -1
	}
	return getOrdinal(r.shoot.Name)
}

// GetStatus returns this replica's status. If the replica's managed seed doesn't exit,
// it returns one of the StatusShoot* statuses, depending on the shoot state.
// Otherwise, it returns one of the ManagedSeed* statuses, depending on the managed seed state.
func (r *replica) GetStatus() ReplicaStatus {
	switch {
	case r.shoot != nil && r.managedSeed == nil:
		switch {
		case shootReconcileSucceeded(r.shoot):
			return StatusShootReconciled
		case shootReconcileFailed(r.shoot):
			return StatusShootReconcileFailed
		case shootDeleteFailed(r.shoot):
			return StatusShootDeleteFailed
		case r.shoot.DeletionTimestamp == nil:
			return StatusShootReconciling
		default:
			return StatusShootDeleting
		}
	case r.shoot != nil && r.managedSeed != nil:
		switch {
		case managedSeedRegistered(r.managedSeed):
			return StatusManagedSeedRegistered
		case r.managedSeed.DeletionTimestamp == nil:
			return StatusManagedSeedPreparing
		default:
			return StatusManagedSeedDeleting
		}
	default:
		return StatusUnknown
	}
}

// IsSeedReady returns true if this replica's seed is ready, false otherwise.
func (r *replica) IsSeedReady() bool {
	return r.seed != nil && seedReady(r.seed)
}

// GetShootHealthStatus returns this replica's shoot health status (healthy, progressing, or unhealthy).
func (r *replica) GetShootHealthStatus() operationshoot.Status {
	if r.shoot == nil {
		return operationshoot.StatusUnhealthy
	}
	return shootHealthStatus(r.shoot)
}

// IsDeletable returns true if this replica can be deleted, false otherwise. A replica can be deleted if it has no
// scheduled shoots and is not protected by the "protect-from-deletion" annotation.
func (r *replica) IsDeletable() bool {
	shootProtected := r.shoot != nil && kutil.HasMetaDataAnnotation(r.shoot, seedmanagementv1alpha1constants.AnnotationProtectFromDeletion, "true")
	managedSeedProtected := r.managedSeed != nil && kutil.HasMetaDataAnnotation(r.managedSeed, seedmanagementv1alpha1constants.AnnotationProtectFromDeletion, "true")
	return !r.hasScheduledShoots && !shootProtected && !managedSeedProtected
}

// CreateShoot initializes this replica's shoot and then creates it using the given context and client.
func (r *replica) CreateShoot(ctx context.Context, c client.Client, ordinal int) error {
	if r.shoot == nil {
		r.shoot = newShoot(r.set, ordinal)
		return kutil.IgnoreAlreadyExists(c.Create(ctx, r.shoot))
	}
	return nil
}

// CreateManagedSeed initializes this replica's managed seed, and then creates it using the given context and client.
func (r *replica) CreateManagedSeed(ctx context.Context, c client.Client) error {
	if r.managedSeed == nil {
		var err error
		if r.managedSeed, err = newManagedSeed(r.set, r.GetOrdinal()); err != nil {
			return err
		}
		return kutil.IgnoreAlreadyExists(c.Create(ctx, r.managedSeed))
	}
	return nil
}

// DeleteShoot deletes this replica's shoot using the given context and client.
func (r *replica) DeleteShoot(ctx context.Context, c client.Client) error {
	if r.shoot != nil {
		if err := kutil.SetAnnotationAndUpdate(ctx, c, r.shoot, gutil.ConfirmationDeletion, "true"); err != nil {
			return err
		}
		return client.IgnoreNotFound(c.Delete(ctx, r.shoot))
	}
	return nil
}

// DeleteManagedSeed deletes this replica's managed seed using the given context and client.
func (r *replica) DeleteManagedSeed(ctx context.Context, c client.Client) error {
	if r.managedSeed != nil {
		return client.IgnoreNotFound(c.Delete(ctx, r.managedSeed))
	}
	return nil
}

// RetryShoot retries this replica's shoot using the given context and client.
func (r *replica) RetryShoot(ctx context.Context, c client.Client) error {
	if r.shoot == nil {
		return nil
	}
	if err := kutil.SetAnnotationAndUpdate(ctx, c, r.shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationRetry); err != nil {
		return err
	}
	return nil
}

func shootReconcileSucceeded(shoot *gardencorev1beta1.Shoot) bool {
	lastOp := shoot.Status.LastOperation
	return shoot.Generation == shoot.Status.ObservedGeneration && shoot.DeletionTimestamp == nil && lastOp != nil &&
		(lastOp.Type == gardencorev1beta1.LastOperationTypeCreate || lastOp.Type == gardencorev1beta1.LastOperationTypeReconcile) &&
		lastOp.State == gardencorev1beta1.LastOperationStateSucceeded
}

func shootReconcileFailed(shoot *gardencorev1beta1.Shoot) bool {
	lastOp := shoot.Status.LastOperation
	return shoot.Generation == shoot.Status.ObservedGeneration && shoot.DeletionTimestamp == nil && lastOp != nil &&
		(lastOp.Type == gardencorev1beta1.LastOperationTypeCreate || lastOp.Type == gardencorev1beta1.LastOperationTypeReconcile) &&
		lastOp.State == gardencorev1beta1.LastOperationStateFailed
}

func shootDeleteFailed(shoot *gardencorev1beta1.Shoot) bool {
	lastOp := shoot.Status.LastOperation
	return shoot.Generation == shoot.Status.ObservedGeneration && shoot.DeletionTimestamp != nil && lastOp != nil &&
		lastOp.Type == gardencorev1beta1.LastOperationTypeDelete &&
		lastOp.State == gardencorev1beta1.LastOperationStateFailed
}

func managedSeedRegistered(managedSeed *seedmanagementv1alpha1.ManagedSeed) bool {
	conditionSeedRegistered := gardencorev1beta1helper.GetCondition(managedSeed.Status.Conditions, seedmanagementv1alpha1.ManagedSeedSeedRegistered)
	return managedSeed.Generation == managedSeed.Status.ObservedGeneration && managedSeed.DeletionTimestamp == nil &&
		conditionSeedRegistered != nil && conditionSeedRegistered.Status == gardencorev1beta1.ConditionTrue
}

func seedReady(seed *gardencorev1beta1.Seed) bool {
	conditionGardenletReady := gardencorev1beta1helper.GetCondition(seed.Status.Conditions, gardencorev1beta1.SeedGardenletReady)
	conditionBootstrapped := gardencorev1beta1helper.GetCondition(seed.Status.Conditions, gardencorev1beta1.SeedBootstrapped)
	conditionBackupBucketsReady := gardencorev1beta1helper.GetCondition(seed.Status.Conditions, gardencorev1beta1.SeedBackupBucketsReady)
	return seed.Generation == seed.Status.ObservedGeneration && seed.DeletionTimestamp == nil &&
		conditionGardenletReady != nil && conditionGardenletReady.Status == gardencorev1beta1.ConditionTrue &&
		conditionBootstrapped != nil && conditionBootstrapped.Status == gardencorev1beta1.ConditionTrue &&
		(conditionBackupBucketsReady == nil || conditionBackupBucketsReady.Status == gardencorev1beta1.ConditionTrue)
}

func shootHealthStatus(shoot *gardencorev1beta1.Shoot) operationshoot.Status {
	if value, ok := shoot.Labels[v1beta1constants.ShootStatus]; ok {
		return operationshoot.Status(value)
	}
	return operationshoot.StatusProgressing
}

// newShoot creates a new shoot object for the given set and ordinal.
func newShoot(set *seedmanagementv1alpha1.ManagedSeedSet, ordinal int) *gardencorev1beta1.Shoot {
	name := getName(set, ordinal)

	// Initialize shoot
	shoot := &gardencorev1beta1.Shoot{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   set.Namespace,
			Labels:      set.Spec.ShootTemplate.Labels,
			Annotations: set.Spec.ShootTemplate.Annotations,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(set, seedmanagementv1alpha1.SchemeGroupVersion.WithKind("ManagedSeedSet")),
			},
		},
		Spec: set.Spec.ShootTemplate.Spec,
	}

	// Replace placeholders in shoot spec with the actual replica name
	replacePlaceholdersInShootSpec(&shoot.Spec, name)

	return shoot
}

// newManagedSeed creates a new managed seed object for the given set and ordinal.
func newManagedSeed(set *seedmanagementv1alpha1.ManagedSeedSet, ordinal int) (*seedmanagementv1alpha1.ManagedSeed, error) {
	name := getName(set, ordinal)

	// Initialize managed seed
	managedSeed := &seedmanagementv1alpha1.ManagedSeed{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   set.Namespace,
			Labels:      set.Spec.Template.Labels,
			Annotations: set.Spec.Template.Annotations,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(set, seedmanagementv1alpha1.SchemeGroupVersion.WithKind("ManagedSeedSet")),
			},
		},
		Spec: seedmanagementv1alpha1.ManagedSeedSpec{
			Shoot: &seedmanagementv1alpha1.Shoot{
				Name: name,
			},
			SeedTemplate: set.Spec.Template.Spec.SeedTemplate,
			Gardenlet:    set.Spec.Template.Spec.Gardenlet,
		},
	}

	// Replace placeholders in seed spec with the actual replica name
	switch {
	case managedSeed.Spec.SeedTemplate != nil:
		replacePlaceholdersInSeedSpec(&managedSeed.Spec.SeedTemplate.Spec, name)
	case managedSeed.Spec.Gardenlet != nil:
		// Decode gardenlet configuration
		gardenletConfig, err := seedmanagementhelper.DecodeGardenletConfiguration(&managedSeed.Spec.Gardenlet.Config, false)
		if err != nil {
			return nil, err
		}
		replacePlaceholdersInSeedSpec(&gardenletConfig.SeedConfig.Spec, name)
	}

	return managedSeed, nil
}

const placeholder = "replica-name"

func replacePlaceholdersInShootSpec(spec *gardencorev1beta1.ShootSpec, name string) {
	if spec.DNS != nil && spec.DNS.Domain != nil {
		spec.DNS.Domain = pointer.StringPtr(strings.Replace(*spec.DNS.Domain, placeholder, name, -1))
	}
}

func replacePlaceholdersInSeedSpec(spec *gardencorev1beta1.SeedSpec, name string) {
	switch {
	case spec.DNS.IngressDomain != nil:
		spec.DNS.IngressDomain = pointer.StringPtr(strings.Replace(*spec.DNS.IngressDomain, placeholder, name, -1))
	case spec.Ingress != nil:
		spec.Ingress.Domain = strings.Replace(spec.Ingress.Domain, placeholder, name, -1)
	}
}

// getName returns the replica object name for the given set and ordinal.
func getName(set *seedmanagementv1alpha1.ManagedSeedSet, ordinal int) string {
	return fmt.Sprintf("%s-%d", set.Name, ordinal)
}

// getFullName returns the replica's full object name (namespace/name) for the given set and ordinal.
func getFullName(set *seedmanagementv1alpha1.ManagedSeedSet, ordinal int) string {
	return fmt.Sprintf("%s/%s", set.Namespace, getName(set, ordinal))
}

// ordinalRegex is a regular expression that extracts the ordinal from the name of a replica object.
var ordinalRegex = regexp.MustCompile(".*-([0-9]+)$")

// getOrdinal gets the ordinal from the given replica object name.
// If the object was not created by a ManagedSeedSet, its ordinal is considered to be -1.
func getOrdinal(name string) int {
	subMatches := ordinalRegex.FindStringSubmatch(name)
	if len(subMatches) < 2 {
		return -1
	}
	ordinal, err := strconv.ParseInt(subMatches[1], 10, 32)
	if err != nil {
		return -1
	}
	return int(ordinal)
}
