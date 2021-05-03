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

// Deprecated: this is the deprecated gardener testframework.
// Use gardener/test/framework instead
package maintenance

import (
	"context"
	"fmt"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/utils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/sirupsen/logrus"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CleanupCloudProfile tries to update the CloudProfile with retries to make sure the machine image version & kubernetes version introduced during the integration test is being removed
func CleanupCloudProfile(ctx context.Context, gardenClient client.Client, cloudProfileName string, testMachineImage gardencorev1beta1.ShootMachineImage, testKubernetesVersions []gardencorev1beta1.ExpirableVersion) error {
	var (
		attempt int
	)

	// then delete the test versions
	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() (err error) {
		existingCloudProfile := &gardencorev1beta1.CloudProfile{}
		if err = gardenClient.Get(ctx, client.ObjectKey{Name: cloudProfileName}, existingCloudProfile); err != nil {
			return err
		}

		// clean machine image
		removedCloudProfileImages := []gardencorev1beta1.MachineImage{}
		for _, image := range existingCloudProfile.Spec.MachineImages {
			versionExists, index := helper.ShootMachineImageVersionExists(image, testMachineImage)
			if versionExists {
				image.Versions = append(image.Versions[:index], image.Versions[index+1:]...)
			}
			removedCloudProfileImages = append(removedCloudProfileImages, image)
		}
		existingCloudProfile.Spec.MachineImages = removedCloudProfileImages

		// clean kubernetes CloudProfile Version
		removedKubernetesVersions := []gardencorev1beta1.ExpirableVersion{}
		for _, cloudprofileVersion := range existingCloudProfile.Spec.Kubernetes.Versions {
			versionShouldBeRemoved := false
			for _, versionToBeRemoved := range testKubernetesVersions {
				if cloudprofileVersion.Version == versionToBeRemoved.Version {
					versionShouldBeRemoved = true
					break
				}
			}
			if !versionShouldBeRemoved {
				removedKubernetesVersions = append(removedKubernetesVersions, cloudprofileVersion)
			}
		}
		existingCloudProfile.Spec.Kubernetes.Versions = removedKubernetesVersions

		// update Cloud Profile to remove the test machine image
		if err = gardenClient.Update(ctx, existingCloudProfile); err != nil {
			logger.Logger.Errorf("attempt %d failed to update CloudProfile %s due to %v", attempt, existingCloudProfile.Name, err)
			return err
		}

		return nil
	}); err != nil {
		return err
	}
	return nil
}

// WaitForExpectedMachineImageMaintenance polls a shoot until the given deadline is exceeded. Checks if the shoot's machine image  equals the targetImage and if an image update is required.
func WaitForExpectedMachineImageMaintenance(ctx context.Context, logger *logrus.Logger, gardenClient client.Client, s *gardencorev1beta1.Shoot, targetMachineImage gardencorev1beta1.ShootMachineImage, imageUpdateRequired bool, deadline time.Time) error {
	return wait.PollImmediateUntil(2*time.Second, func() (bool, error) {
		shoot := &gardencorev1beta1.Shoot{}
		err := gardenClient.Get(ctx, client.ObjectKey{Namespace: s.Namespace, Name: s.Name}, shoot)
		if err != nil {
			return false, err
		}

		// If one worker of the shoot got an machine image update, we assume the maintenance to having been successful
		// in the integration test we only use one worker pool
		nameVersions := make(map[string]string)
		for _, worker := range shoot.Spec.Provider.Workers {
			if worker.Machine.Image.Version != nil {
				nameVersions[worker.Machine.Image.Name] = *worker.Machine.Image.Version
			}
			if worker.Machine.Image != nil && apiequality.Semantic.DeepEqual(*worker.Machine.Image, targetMachineImage) && imageUpdateRequired {
				logger.Infof("shoot maintained properly - received machine image update")
				return true, nil
			}
		}

		now := time.Now()
		nowIsAfterDeadline := now.After(deadline)
		if nowIsAfterDeadline && imageUpdateRequired {
			return false, fmt.Errorf("shoot did not get the expected machine image maintenance. Deadline exceeded. ")
		} else if nowIsAfterDeadline && !imageUpdateRequired {
			logger.Infof("shoot maintained properly - did not receive an machineImage update")
			return true, nil
		}
		logger.Infof("shoot %s has workers with machine images (name:version) '%v'. Target image: %s-%s. ImageUpdateRequired: %t. Deadline is in %v", shoot.Name, nameVersions, targetMachineImage.Name, *targetMachineImage.Version, imageUpdateRequired, deadline.Sub(now))
		return false, nil
	}, ctx.Done())
}

// WaitForExpectedKubernetesVersionMaintenance polls a shoot until the given deadline is exceeded. Checks if the shoot's kubernetes version equals the targetVersion and if an kubernetes version update is required.
func WaitForExpectedKubernetesVersionMaintenance(ctx context.Context, logger *logrus.Logger, gardenClient client.Client, s *gardencorev1beta1.Shoot, targetVersion string, kubernetesVersionUpdateRequired bool, deadline time.Time) error {
	return wait.PollImmediateUntil(2*time.Second, func() (bool, error) {
		shoot := &gardencorev1beta1.Shoot{}
		err := gardenClient.Get(ctx, client.ObjectKey{Namespace: s.Namespace, Name: s.Name}, shoot)
		if err != nil {
			return false, err
		}

		if shoot.Spec.Kubernetes.Version == targetVersion && kubernetesVersionUpdateRequired {
			logger.Infof("shoot maintained properly - received kubernetes version update")
			return true, nil
		}

		now := time.Now()
		nowIsAfterDeadline := now.After(deadline)
		if nowIsAfterDeadline && kubernetesVersionUpdateRequired {
			return false, fmt.Errorf("shoot did not get the expected kubernetes version maintenance. Deadline exceeded. ")
		} else if nowIsAfterDeadline && !kubernetesVersionUpdateRequired {
			logger.Infof("shoot maintained properly - did not receive an kubernetes version update")
			return true, nil
		}
		logger.Infof("shoot %s has kubernetes version %s. Target version: %s. Kubernetes Version Update Required: %t. Deadline is in %v", shoot.Name, shoot.Spec.Kubernetes.Version, targetVersion, kubernetesVersionUpdateRequired, deadline.Sub(now))
		return false, nil
	}, ctx.Done())
}

// TryUpdateShootForMachineImageMaintenance tries to update the maintenance section of the shoot spec regarding the machine image
func TryUpdateShootForMachineImageMaintenance(ctx context.Context, gardenClient client.Client, shootToUpdate *gardencorev1beta1.Shoot) error {
	shoot := &gardencorev1beta1.Shoot{ObjectMeta: shootToUpdate.ObjectMeta}
	return kutil.TryUpdate(ctx, retry.DefaultBackoff, gardenClient, shoot, func() error {
		if shootToUpdate.Spec.Maintenance.AutoUpdate != nil {
			shoot.Spec.Maintenance.AutoUpdate.MachineImageVersion = shootToUpdate.Spec.Maintenance.AutoUpdate.MachineImageVersion
		}
		shoot.Spec.Provider.Workers = shootToUpdate.Spec.Provider.Workers
		return nil
	})
}

// StartShootMaintenance adds the maintenance annotation on the Shoot to start the Shoot Maintenance
func StartShootMaintenance(ctx context.Context, gardenClient client.Client, shootToUpdate *gardencorev1beta1.Shoot) error {
	shoot := &gardencorev1beta1.Shoot{ObjectMeta: shootToUpdate.ObjectMeta}
	return kutil.TryUpdate(ctx, retry.DefaultBackoff, gardenClient, shoot, func() error {
		shoot.Annotations[v1beta1constants.GardenerOperation] = v1beta1constants.ShootOperationMaintain
		return nil
	})
}

// TryUpdateShootForKubernetesMaintenance tries to update the maintenance section of the shoot spec regarding the Kubernetes version
func TryUpdateShootForKubernetesMaintenance(ctx context.Context, gardenClient client.Client, shootToUpdate *gardencorev1beta1.Shoot) error {
	shoot := &gardencorev1beta1.Shoot{ObjectMeta: shootToUpdate.ObjectMeta}

	return kutil.TryUpdate(ctx, retry.DefaultBackoff, gardenClient, shoot, func() error {
		shoot.Spec.Kubernetes.Version = shootToUpdate.Spec.Kubernetes.Version
		shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion = shootToUpdate.Spec.Maintenance.AutoUpdate.KubernetesVersion
		shoot.Annotations = utils.MergeStringMaps(shoot.Annotations, shootToUpdate.Annotations)
		return nil
	})
}

// TryUpdateCloudProfileForMachineImageMaintenance tries to update the images of the Cloud Profile
func TryUpdateCloudProfileForMachineImageMaintenance(ctx context.Context, gardenClient client.Client, shoot *gardencorev1beta1.Shoot, testMachineImage gardencorev1beta1.ShootMachineImage, expirationDate *metav1.Time, classification *gardencorev1beta1.VersionClassification) error {
	cloudProfile := &gardencorev1beta1.CloudProfile{
		ObjectMeta: metav1.ObjectMeta{
			Name: shoot.Spec.CloudProfileName,
		},
	}

	return kutil.TryUpdate(ctx, retry.DefaultBackoff, gardenClient, cloudProfile, func() error {
		// update Cloud Profile with expirationDate for integration test machine image
		for i, image := range cloudProfile.Spec.MachineImages {
			versionExists, index := helper.ShootMachineImageVersionExists(image, testMachineImage)
			if versionExists {
				cloudProfile.Spec.MachineImages[i].Versions[index].ExpirationDate = expirationDate
				cloudProfile.Spec.MachineImages[i].Versions[index].Classification = classification
			}
		}
		return nil
	})
}

// TryUpdateCloudProfileForKubernetesVersionMaintenance tries to update a specific kubernetes version of the Cloud Profile
func TryUpdateCloudProfileForKubernetesVersionMaintenance(ctx context.Context, gardenClient client.Client, shoot *gardencorev1beta1.Shoot, targetVersion string, expirationDate *metav1.Time, classification *gardencorev1beta1.VersionClassification) error {
	cloudProfile := &gardencorev1beta1.CloudProfile{
		ObjectMeta: metav1.ObjectMeta{
			Name: shoot.Spec.CloudProfileName,
		},
	}

	return kutil.TryUpdate(ctx, retry.DefaultBackoff, gardenClient, cloudProfile, func() error {
		// update kubernetes version in cloud profile with an expiration date
		for i, version := range cloudProfile.Spec.Kubernetes.Versions {
			if version.Version == targetVersion {
				cloudProfile.Spec.Kubernetes.Versions[i].Classification = classification
				cloudProfile.Spec.Kubernetes.Versions[i].ExpirationDate = expirationDate
			}
		}
		return nil
	})
}
