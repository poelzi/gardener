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

package logging

import (
	"context"
	"fmt"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/test/framework"
	"github.com/gardener/gardener/test/framework/resources/templates"

	"github.com/onsi/ginkgo"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
)

const (
	deltaLogsCount            = 1
	deltaLogsDuration         = "180s"
	logsCount                 = 2000
	logsDuration              = "90s"
	numberOfSimulatedClusters = 100

	initializationTimeout          = 5 * time.Minute
	getLogsFromLokiTimeout         = 15 * time.Minute
	loggerDeploymentCleanupTimeout = 5 * time.Minute

	fluentBitName                 = "fluent-bit"
	lokiName                      = "loki"
	garden                        = "garden"
	loggerDeploymentName          = "logger"
	logger                        = "logger-.*"
	fluentBitConfigMapName        = "fluent-bit-config"
	fluentBitClusterRoleName      = "fluent-bit-read"
	simulatesShootNamespacePrefix = "shoot--logging--test-"
	lokiConfigMapName             = "loki-config"
)

var _ = ginkgo.Describe("Seed logging testing", func() {

	f := framework.NewShootFramework(nil)
	gardenNamespace := &corev1.Namespace{}
	fluentBit := &appsv1.DaemonSet{}
	fluentBitConfMap := &corev1.ConfigMap{}
	fluentBitService := &corev1.Service{}
	fluentBitClusterRole := &rbacv1.ClusterRole{}
	fluentBitClusterRoleBinding := &rbacv1.ClusterRoleBinding{}
	fluentBitServiceAccount := &corev1.ServiceAccount{}
	fluentBitPriorityClass := &schedulingv1.PriorityClass{}
	clusterCRD := &apiextensionsv1.CustomResourceDefinition{}
	lokiSts := &appsv1.StatefulSet{}
	lokiServiceAccount := &corev1.ServiceAccount{}
	lokiService := &corev1.Service{}
	lokiConfMap := &corev1.ConfigMap{}

	framework.CBeforeEach(func(ctx context.Context) {
		checkRequiredResources(ctx, f.SeedClient)
		framework.ExpectNoError(f.SeedClient.Client().Get(ctx, types.NamespacedName{Namespace: v1beta1constants.GardenNamespace, Name: fluentBitName}, fluentBit))
		framework.ExpectNoError(f.SeedClient.Client().Get(ctx, types.NamespacedName{Namespace: v1beta1constants.GardenNamespace, Name: fluentBitConfigMapName}, fluentBitConfMap))
		framework.ExpectNoError(f.SeedClient.Client().Get(ctx, types.NamespacedName{Namespace: v1beta1constants.GardenNamespace, Name: fluentBitName}, fluentBitService))
		framework.ExpectNoError(f.SeedClient.Client().Get(ctx, types.NamespacedName{Namespace: v1beta1constants.GardenNamespace, Name: fluentBitClusterRoleName}, fluentBitClusterRole))
		framework.ExpectNoError(f.SeedClient.Client().Get(ctx, types.NamespacedName{Namespace: v1beta1constants.GardenNamespace, Name: fluentBitClusterRoleName}, fluentBitClusterRoleBinding))
		framework.ExpectNoError(f.SeedClient.Client().Get(ctx, types.NamespacedName{Namespace: v1beta1constants.GardenNamespace, Name: fluentBitName}, fluentBitServiceAccount))
		framework.ExpectNoError(f.SeedClient.Client().Get(ctx, types.NamespacedName{Namespace: v1beta1constants.GardenNamespace, Name: fluentBitName}, fluentBitPriorityClass))
		framework.ExpectNoError(f.SeedClient.Client().Get(ctx, types.NamespacedName{Namespace: "", Name: "clusters.extensions.gardener.cloud"}, clusterCRD))
		framework.ExpectNoError(f.SeedClient.Client().Get(ctx, types.NamespacedName{Namespace: v1beta1constants.GardenNamespace, Name: lokiName}, lokiSts))
		framework.ExpectNoError(f.SeedClient.Client().Get(ctx, types.NamespacedName{Namespace: v1beta1constants.GardenNamespace, Name: lokiName}, lokiServiceAccount))
		framework.ExpectNoError(f.SeedClient.Client().Get(ctx, types.NamespacedName{Namespace: v1beta1constants.GardenNamespace, Name: lokiName}, lokiService))
		framework.ExpectNoError(f.SeedClient.Client().Get(ctx, types.NamespacedName{Namespace: v1beta1constants.GardenNamespace, Name: lokiConfigMapName}, lokiConfMap))
	}, initializationTimeout)

	f.Beta().Serial().CIt("should get container logs from loki for all namespaces", func(ctx context.Context) {
		ginkgo.By("Deploy the garden Namespace")
		gardenNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: v1beta1constants.GardenNamespace,
			},
		}
		framework.ExpectNoError(create(ctx, f.ShootClient.Client(), gardenNamespace))

		ginkgo.By("Deploy the Loki StatefulSet")
		framework.ExpectNoError(create(ctx, f.ShootClient.Client(), lokiServiceAccount))
		framework.ExpectNoError(create(ctx, f.ShootClient.Client(), lokiConfMap))
		lokiService.Spec.ClusterIP = ""
		framework.ExpectNoError(create(ctx, f.ShootClient.Client(), lokiService))
		// Remove the Loki PVC as it is no needed for the test
		lokiSts.Spec.VolumeClaimTemplates = nil
		// Instead use an empty dir volume
		lokiDataVolumeSize := resource.MustParse("500Mi")
		lokiDataVolume := corev1.Volume{
			Name: "loki",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					SizeLimit: &lokiDataVolumeSize,
				},
			},
		}
		lokiSts.Spec.Template.Spec.Volumes = append(lokiSts.Spec.Template.Spec.Volumes, lokiDataVolume)
		for index, container := range lokiSts.Spec.Template.Spec.Containers {
			if container.Name == lokiName {
				r := corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("800m"),
						corev1.ResourceMemory: resource.MustParse("1.5Gi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("900m"),
						corev1.ResourceMemory: resource.MustParse("2.5Gi"),
					},
				}
				lokiSts.Spec.Template.Spec.Containers[index].Resources = r
			}
		}
		framework.ExpectNoError(create(ctx, f.ShootClient.Client(), lokiSts))

		ginkgo.By("Wait until Loki StatefulSet is ready")
		framework.ExpectNoError(f.WaitUntilStatefulSetIsRunning(ctx, lokiName, v1beta1constants.GardenNamespace, f.ShootClient))

		ginkgo.By("Deploy the cluster CRD")
		clusterCRD.Spec.PreserveUnknownFields = false
		for version := range clusterCRD.Spec.Versions {
			clusterCRD.Spec.Versions[version].Schema.OpenAPIV3Schema.XPreserveUnknownFields = pointer.BoolPtr(true)
		}
		framework.ExpectNoError(create(ctx, f.ShootClient.Client(), clusterCRD))

		ginkgo.By("Deploy the fluent-bit RBAC")
		framework.ExpectNoError(create(ctx, f.ShootClient.Client(), fluentBitServiceAccount))
		framework.ExpectNoError(create(ctx, f.ShootClient.Client(), fluentBitPriorityClass))
		framework.ExpectNoError(create(ctx, f.ShootClient.Client(), fluentBitClusterRole))
		framework.ExpectNoError(create(ctx, f.ShootClient.Client(), fluentBitClusterRoleBinding))
		framework.ExpectNoError(f.RenderAndDeployTemplate(ctx, f.ShootClient, "fluent-bit-psp-clusterrolebinding.yaml", nil))

		ginkgo.By("Deploy the fluent-bit DaemonSet")
		framework.ExpectNoError(create(ctx, f.ShootClient.Client(), fluentBitConfMap))
		framework.ExpectNoError(create(ctx, f.ShootClient.Client(), fluentBit))

		ginkgo.By("Wait until fluent-bit DaemonSet is ready")
		framework.ExpectNoError(f.WaitUntilDaemonSetIsRunning(ctx, f.ShootClient.Client(), fluentBitName, v1beta1constants.GardenNamespace))

		ginkgo.By("Deploy the simulated cluster and shoot controlplane namespaces")
		for i := 0; i < numberOfSimulatedClusters; i++ {
			shootNamespace := getShootNamesapce(i)
			ginkgo.By(fmt.Sprintf("Deploy namespace %s", shootNamespace.Name))
			framework.ExpectNoError(create(ctx, f.ShootClient.Client(), shootNamespace))

			cluster := getCluster(i)
			ginkgo.By(fmt.Sprintf("Deploy cluster %s", cluster.Name))
			framework.ExpectNoError(create(ctx, f.ShootClient.DirectClient(), cluster))

			ginkgo.By(fmt.Sprintf("Deploy the loki service in namespace %s", shootNamespace.Name))
			lokiShootService := getLokiShootService(i)
			framework.ExpectNoError(create(ctx, f.ShootClient.Client(), lokiShootService))

			ginkgo.By(fmt.Sprintf("Deploy the logger application in namespace %s", shootNamespace.Name))
			loggerParams := map[string]interface{}{
				"LoggerName":          loggerDeploymentName,
				"HelmDeployNamespace": shootNamespace.Name,
				"AppLabel":            loggerDeploymentName,
				"DeltaLogsCount":      deltaLogsCount,
				"DeltaLogsDuration":   deltaLogsDuration,
				"LogsCount":           logsCount,
				"LogsDuration":        logsDuration,
			}
			framework.ExpectNoError(f.RenderAndDeployTemplate(ctx, f.ShootClient, templates.LoggerAppName, loggerParams))
		}

		loggerLabels := labels.SelectorFromSet(map[string]string{
			"app": "logger",
		})
		for i := 0; i < numberOfSimulatedClusters; i++ {
			shootNamespace := fmt.Sprintf("%s%v", simulatesShootNamespacePrefix, i)
			ginkgo.By(fmt.Sprintf("Wait until logger application is ready in namespace %s", shootNamespace))
			framework.ExpectNoError(f.WaitUntilDeploymentsWithLabelsIsReady(ctx, loggerLabels, shootNamespace, f.ShootClient))
		}

		ginkgo.By("Verify loki received logger application logs for all namespaces")
		framework.ExpectNoError(WaitUntilLokiReceivesLogs(ctx, 30*time.Second, f, "", v1beta1constants.GardenNamespace, "pod_name", logger, logsCount*numberOfSimulatedClusters, numberOfSimulatedClusters, f.ShootClient))

	}, getLogsFromLokiTimeout, framework.WithCAfterTest(func(ctx context.Context) {
		ginkgo.By("Cleaning up logger app resources")
		for i := 0; i < numberOfSimulatedClusters; i++ {
			shootNamespace := getShootNamesapce(i)
			loggerDeploymentToDelete := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: shootNamespace.Name,
					Name:      "logger",
				},
			}
			framework.ExpectNoError(kutil.DeleteObject(ctx, f.ShootClient.Client(), loggerDeploymentToDelete))

			cluster := getCluster(i)
			framework.ExpectNoError(kutil.DeleteObject(ctx, f.ShootClient.Client(), cluster))

			lokiShootService := getLokiShootService(i)
			framework.ExpectNoError(kutil.DeleteObject(ctx, f.ShootClient.Client(), lokiShootService))

			framework.ExpectNoError(kutil.DeleteObject(ctx, f.ShootClient.Client(), shootNamespace))
		}

		ginkgo.By("Cleaning up garden namespace")
		objectsToDelete := []client.Object{
			fluentBit,
			fluentBitConfMap,
			fluentBitService,
			fluentBitClusterRole,
			fluentBitClusterRoleBinding,
			fluentBitServiceAccount,
			fluentBitPriorityClass,
			gardenNamespace,
		}
		for _, object := range objectsToDelete {
			framework.ExpectNoError(kutil.DeleteObject(ctx, f.ShootClient.Client(), object))
		}
	}, loggerDeploymentCleanupTimeout))
})
