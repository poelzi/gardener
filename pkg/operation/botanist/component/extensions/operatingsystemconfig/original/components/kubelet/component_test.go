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

package kubelet_test

import (
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components/kubelet"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/imagevector"

	"github.com/Masterminds/semver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"
)

var _ = Describe("Component", func() {
	var (
		component components.Component
		ctx       components.Context

		kubeletCACertificate       = "certificate"
		kubeletCACertificateBase64 = utils.EncodeBase64([]byte(kubeletCACertificate))
	)

	BeforeEach(func() {
		component = New()
		ctx = components.Context{}
	})

	DescribeTable("#Config",
		func(kubernetesVersion string, criName extensionsv1alpha1.CRIName, execStartPreFn func(string) string, kubeletFlags, kubeletConfig string) {
			ctx.CRIName = criName
			ctx.KubernetesVersion = semver.MustParse(kubernetesVersion)
			ctx.KubeletCACertificate = kubeletCACertificate
			ctx.Images = map[string]*imagevector.Image{
				"hyperkube": {
					Name:       "pause-container",
					Repository: hyperkubeImageRepo,
					Tag:        pointer.StringPtr(hyperkubeImageTag),
				},
				"pause-container": {
					Name:       "pause-container",
					Repository: pauseContainerImageRepo,
					Tag:        pointer.StringPtr(pauseContainerImageTag),
				},
			}

			units, files, err := component.Config(ctx)

			Expect(err).NotTo(HaveOccurred())
			Expect(units).To(ConsistOf(
				extensionsv1alpha1.Unit{
					Name:    "kubelet.service",
					Command: pointer.StringPtr("start"),
					Enable:  pointer.BoolPtr(true),
					Content: pointer.StringPtr(`[Unit]
Description=kubelet daemon
Documentation=https://kubernetes.io/docs/admin/kubelet
` + unitConfigAfterCRI(criName) + `
[Install]
WantedBy=multi-user.target
[Service]
Restart=always
RestartSec=5
EnvironmentFile=/etc/environment
EnvironmentFile=-/var/lib/kubelet/extra_args
ExecStartPre=` + execStartPreFn("kubelet") + `
ExecStart=/opt/bin/kubelet \` + kubeletFlags),
				},
				extensionsv1alpha1.Unit{
					Name:    "kubelet-monitor.service",
					Command: pointer.StringPtr("start"),
					Enable:  pointer.BoolPtr(true),
					Content: pointer.StringPtr(`[Unit]
Description=Kubelet-monitor daemon
After=kubelet.service
[Install]
WantedBy=multi-user.target
[Service]
Restart=always
EnvironmentFile=/etc/environment
ExecStartPre=` + execStartPreFn("kubectl") + `
ExecStart=/opt/bin/health-monitor-kubelet`),
				},
			))
			Expect(files).To(ConsistOf(
				extensionsv1alpha1.File{
					Path:        "/var/lib/kubelet/ca.crt",
					Permissions: pointer.Int32Ptr(0644),
					Content: extensionsv1alpha1.FileContent{
						Inline: &extensionsv1alpha1.FileContentInline{
							Encoding: "b64",
							Data:     kubeletCACertificateBase64,
						},
					},
				},
				extensionsv1alpha1.File{
					Path:        "/var/lib/kubelet/config/kubelet",
					Permissions: pointer.Int32Ptr(0644),
					Content: extensionsv1alpha1.FileContent{
						Inline: &extensionsv1alpha1.FileContentInline{
							Encoding: "b64",
							Data:     utils.EncodeBase64([]byte(kubeletConfig)),
						},
					},
				},
				extensionsv1alpha1.File{
					Path:        "/opt/bin/health-monitor-kubelet",
					Permissions: pointer.Int32Ptr(0755),
					Content: extensionsv1alpha1.FileContent{
						Inline: &extensionsv1alpha1.FileContentInline{
							Encoding: "b64",
							Data:     utils.EncodeBase64([]byte(healthMonitorScript)),
						},
					},
				},
			))
		},

		Entry(
			"kubernetes 1.15, w/ docker",
			"1.15.1",
			extensionsv1alpha1.CRINameDocker,
			execStartPreLess117,
			kubeletFlagsDocker(extensionsv1alpha1.CRINameDocker, true),
			kubeletConfig(true, false),
		),
		Entry(
			"kubernetes 1.15, w/ containerd",
			"1.15.1",
			extensionsv1alpha1.CRINameContainerD,
			execStartPreLess117,
			kubeletFlagsDocker(extensionsv1alpha1.CRINameContainerD, true),
			kubeletConfig(true, false),
		),

		Entry(
			"kubernetes 1.16, w/ docker",
			"1.16.1",
			extensionsv1alpha1.CRINameDocker,
			execStartPreLess117,
			kubeletFlagsDocker(extensionsv1alpha1.CRINameDocker, true),
			kubeletConfig(true, false),
		),
		Entry(
			"kubernetes 1.16, w/ containerd",
			"1.16.1",
			extensionsv1alpha1.CRINameContainerD,
			execStartPreLess117,
			kubeletFlagsDocker(extensionsv1alpha1.CRINameContainerD, true),
			kubeletConfig(true, false),
		),

		Entry(
			"kubernetes 1.17, w/ docker",
			"1.17.1",
			extensionsv1alpha1.CRINameDocker,
			execStartPreLess119,
			kubeletFlagsDocker(extensionsv1alpha1.CRINameDocker, true),
			kubeletConfig(true, false),
		),
		Entry(
			"kubernetes 1.17, w/ containerd",
			"1.17.1",
			extensionsv1alpha1.CRINameContainerD,
			execStartPreLess119,
			kubeletFlagsDocker(extensionsv1alpha1.CRINameContainerD, true),
			kubeletConfig(true, false),
		),

		Entry(
			"kubernetes 1.18, w/ docker",
			"1.18.1",
			extensionsv1alpha1.CRINameDocker,
			execStartPreLess119,
			kubeletFlagsDocker(extensionsv1alpha1.CRINameDocker, true),
			kubeletConfig(true, false),
		),
		Entry(
			"kubernetes 1.18, w/ containerd",
			"1.18.1",
			extensionsv1alpha1.CRINameContainerD,
			execStartPreLess119,
			kubeletFlagsDocker(extensionsv1alpha1.CRINameContainerD, true),
			kubeletConfig(true, false),
		),

		Entry(
			"kubernetes 1.19, w/ docker",
			"1.19.1",
			extensionsv1alpha1.CRINameDocker,
			execStartPreGreaterEqual119,
			kubeletFlagsDocker(extensionsv1alpha1.CRINameDocker, false),
			kubeletConfig(true, true),
		),
		Entry(
			"kubernetes 1.19, w/ containerd",
			"1.19.1",
			extensionsv1alpha1.CRINameContainerD,
			execStartPreGreaterEqual119,
			kubeletFlagsDocker(extensionsv1alpha1.CRINameContainerD, false),
			kubeletConfig(true, true),
		),

		Entry(
			"kubernetes 1.20, w/ docker",
			"1.20.1",
			extensionsv1alpha1.CRINameDocker,
			execStartPreGreaterEqual119,
			kubeletFlagsDocker(extensionsv1alpha1.CRINameDocker, false),
			kubeletConfig(true, true),
		),
		Entry(
			"kubernetes 1.20, w/ containerd",
			"1.20.1",
			extensionsv1alpha1.CRINameContainerD,
			execStartPreGreaterEqual119,
			kubeletFlagsDocker(extensionsv1alpha1.CRINameContainerD, false),
			kubeletConfig(true, true),
		),
	)
})

const (
	healthMonitorScript = `#!/bin/bash
set -o nounset
set -o pipefail

function kubelet_monitoring {
  echo "Wait for 2 minutes for kubelet to be functional"
  sleep 120
  local -r max_seconds=10
  local output=""

  function kubectl {
    /opt/bin/kubectl --kubeconfig /var/lib/kubelet/kubeconfig-real "$@"
  }

  function restart_kubelet {
    pkill -f "kubelet"
  }

  function patch_internal_ip {
    echo "Updating Node object $2 with InternalIP $3."
    curl \
      -XPATCH \
      -H "Content-Type: application/strategic-merge-patch+json" \
      -H "Accept: application/json" \
      "$1/api/v1/nodes/$2/status" \
      --data "{\"status\":{\"addresses\":[{\"address\": \"$3\", \"type\":\"InternalIP\"}]}}" \
      --cacert <(base64 -d <<< $(kubectl config view -o jsonpath={.clusters[0].cluster.certificate-authority-data} --raw)) \
      --key /var/lib/kubelet/pki/kubelet-client-current.pem \
      --cert /var/lib/kubelet/pki/kubelet-client-current.pem \
    > /dev/null 2>&1
  }

  timeframe=600
  toggle_threshold=5
  count_kubelet_alternating_between_ready_and_not_ready_within_timeframe=0
  time_kubelet_not_ready_first_occurrence=0
  last_kubelet_ready_state="True"

  while [ 1 ]; do
    # Check whether the kubelet's /healthz endpoint reports unhealthiness
    if ! output=$(curl -m $max_seconds -f -s -S http://127.0.0.1:10248/healthz 2>&1); then
      echo $output
      echo "Kubelet is unhealthy!"
      restart_kubelet
      sleep 60
      continue
    fi

    node_object="$(kubectl get nodes -l kubernetes.io/hostname=$(hostname) -o json)"
    node_status="$(echo $node_object | jq -r '.items[0].status')"
    if [[ -z "$node_status" ]] || [[ "$node_status" == "null" ]]; then
      echo "Node object for this hostname not found in the system, waiting."
      sleep 20
      count_kubelet_alternating_between_ready_and_not_ready_within_timeframe=0
      time_kubelet_not_ready_first_occurrence=0
      continue
    fi

    # Check whether the kubelet does report an InternalIP node address
    node_ip_internal="$(echo $node_status | jq -r '.addresses[] | select(.type=="InternalIP") | .address')"
    node_ip_external="$(echo $node_status | jq -r '.addresses[] | select(.type=="ExternalIP") | .address')"
    if [[ -z "$node_ip_internal" ]] && [[ -z "$node_ip_external" ]]; then
      echo "Kubelet has not reported an InternalIP nor an ExternalIP node address yet.";
      if ! [[ -z ${K8S_NODE_IP_INTERNAL_LAST_SEEN+x} ]]; then
        echo "Check if last seen InternalIP "$K8S_NODE_IP_INTERNAL_LAST_SEEN" can be used";
        if ip address show | grep $K8S_NODE_IP_INTERNAL_LAST_SEEN > /dev/null; then
          echo "Last seen InternalIP "$K8S_NODE_IP_INTERNAL_LAST_SEEN" is still up-to-date";
          server="$(kubectl config view -o jsonpath={.clusters[0].cluster.server})"
          node_name="$(echo $node_object | jq -r '.items[0].metadata.name')"
          if patch_internal_ip $server $node_name $K8S_NODE_IP_INTERNAL_LAST_SEEN; then
            echo "Successfully updated Node object."
            continue
          else
            echo "An error occurred while updating the Node object."
          fi
        fi
      fi
      echo "Updating Node object is not possible. Restarting Kubelet.";
      restart_kubelet
      sleep 20
      continue
    elif ! [[ -z "$node_ip_internal" ]]; then
      export K8S_NODE_IP_INTERNAL_LAST_SEEN="$node_ip_internal"
    fi

    # Check whether kubelet ready status toggles between true and false and reboot VM if happened too often.
    if status="$(echo $node_status | jq -r '.conditions[] | select(.type=="Ready") | .status')"; then
      if [[ "$status" != "True" ]]; then
        if [[ $time_kubelet_not_ready_first_occurrence == 0 ]]; then
          time_kubelet_not_ready_first_occurrence=$(date +%s)
          echo "Start tracking kubelet ready status toggles."
        fi
      else
        if [[ $time_kubelet_not_ready_first_occurrence != 0 ]]; then
          if [[ "$last_kubelet_ready_state" != "$status" ]]; then
            count_kubelet_alternating_between_ready_and_not_ready_within_timeframe=$((count_kubelet_alternating_between_ready_and_not_ready_within_timeframe+1))
            echo "count_kubelet_alternating_between_ready_and_not_ready_within_timeframe=$count_kubelet_alternating_between_ready_and_not_ready_within_timeframe"
            if [[ $count_kubelet_alternating_between_ready_and_not_ready_within_timeframe -ge $toggle_threshold ]]; then
              sudo reboot
            fi
          fi
        fi
      fi

      if [[ $time_kubelet_not_ready_first_occurrence != 0 && $(($(date +%s)-$time_kubelet_not_ready_first_occurrence)) -ge $timeframe ]]; then
        count_kubelet_alternating_between_ready_and_not_ready_within_timeframe=0
        time_kubelet_not_ready_first_occurrence=0
        echo "Resetting kubelet ready status toggle tracking."
      fi

      last_kubelet_ready_state="$status"
    fi

    sleep $SLEEP_SECONDS
  done
}

SLEEP_SECONDS=10
echo "Start health monitoring for kubelet"
kubelet_monitoring
`

	pauseContainerImageRepo = "foo.io"
	pauseContainerImageTag  = "v1.2.3"
	hyperkubeImageRepo      = "hyperkube.io"
	hyperkubeImageTag       = "v4.5.6"
)

func execStartPreLess117(binary string) string {
	return `/usr/bin/docker run --rm -v /opt/bin:/opt/bin:rw ` + hyperkubeImageRepo + `:` + hyperkubeImageTag + ` /bin/sh -c "cp /usr/local/bin/` + binary + ` /opt/bin"`
}

func execStartPreLess119(binary string) string {
	return `/usr/bin/docker run --rm -v /opt/bin:/opt/bin:rw --entrypoint /bin/sh ` + hyperkubeImageRepo + `:` + hyperkubeImageTag + ` -c "cp /usr/local/bin/` + binary + ` /opt/bin"`
}

func execStartPreGreaterEqual119(binary string) string {
	return `/usr/bin/env sh -c "ID=\"$(/usr/bin/docker run --rm -d -v /opt/bin:/opt/bin:rw ` + hyperkubeImageRepo + `:` + hyperkubeImageTag + `)\"; /usr/bin/docker cp \"$ID\":/` + binary + ` /opt/bin; /usr/bin/docker stop \"$ID\"; chmod +x /opt/bin/` + binary + `"`
}

func unitConfigAfterCRI(criName extensionsv1alpha1.CRIName) string {
	if criName == extensionsv1alpha1.CRINameContainerD {
		return `After=containerd.service`
	}
	return `After=docker.service
Wants=docker.socket rpc-statd.service`
}

func kubeletFlagsDocker(criName extensionsv1alpha1.CRIName, volumePluginDir bool) string {
	var out string

	out += `
    --bootstrap-kubeconfig=/var/lib/kubelet/kubeconfig-bootstrap \
    --config=/var/lib/kubelet/config/kubelet \
    --cni-bin-dir=/opt/cni/bin/ \
    --cni-conf-dir=/etc/cni/net.d/ \
    --image-pull-progress-deadline=1m0s \`

	out += `
    --kubeconfig=/var/lib/kubelet/kubeconfig-real \
    --network-plugin=cni \`

	if volumePluginDir {
		out += `
    --volume-plugin-dir=/var/lib/kubelet/volumeplugins \`
	}

	if criName == extensionsv1alpha1.CRINameContainerD {
		out += `
    --container-runtime=remote \
    --container-runtime-endpoint=unix:///run/containerd/containerd.sock \`
	} else {
		out += `
    --pod-infra-container-image=foo.io:v1.2.3 \`
	}

	out += `
    --v=2 $KUBELET_EXTRA_ARGS`

	return out
}

func kubeletConfig(
	rotateCertificates bool,
	volumePluginDir bool,
) string {
	out := `apiVersion: kubelet.config.k8s.io/v1beta1
authentication:
  anonymous:
    enabled: false
  webhook:
    cacheTTL: 2m0s
    enabled: true
  x509:
    clientCAFile: /var/lib/kubelet/ca.crt
authorization:
  mode: Webhook
  webhook:
    cacheAuthorizedTTL: 5m0s
    cacheUnauthorizedTTL: 30s
cgroupDriver: cgroupfs
cgroupRoot: /
cgroupsPerQOS: true
clusterDNS:
- ""
cpuCFSQuota: true
cpuManagerPolicy: none
cpuManagerReconcilePeriod: 10s
enableControllerAttachDetach: true
enableDebuggingHandlers: true
enableServer: true
enforceNodeAllocatable:
- pods
eventBurst: 50
eventRecordQPS: 50
evictionHard:
  imagefs.available: 5%
  imagefs.inodesFree: 5%
  memory.available: 100Mi
  nodefs.available: 5%
  nodefs.inodesFree: 5%
evictionMaxPodGracePeriod: 90
evictionMinimumReclaim:
  imagefs.available: 0Mi
  imagefs.inodesFree: 0Mi
  memory.available: 0Mi
  nodefs.available: 0Mi
  nodefs.inodesFree: 0Mi
evictionPressureTransitionPeriod: 4m0s
evictionSoft:
  imagefs.available: 10%
  imagefs.inodesFree: 10%
  memory.available: 200Mi
  nodefs.available: 10%
  nodefs.inodesFree: 10%
evictionSoftGracePeriod:
  imagefs.available: 1m30s
  imagefs.inodesFree: 1m30s
  memory.available: 1m30s
  nodefs.available: 1m30s
  nodefs.inodesFree: 1m30s
failSwapOn: true
fileCheckFrequency: 20s
hairpinMode: promiscuous-bridge
httpCheckFrequency: 20s
imageGCHighThresholdPercent: 50
imageGCLowThresholdPercent: 40
imageMinimumGCAge: 2m0s
kind: KubeletConfiguration
kubeAPIBurst: 50
kubeAPIQPS: 50
kubeReserved:
  cpu: 80m
  memory: 1Gi
logging: {}
maxOpenFiles: 1000000
maxPods: 110
nodeStatusReportFrequency: 0s
nodeStatusUpdateFrequency: 10s
registryBurst: 10
registryPullQPS: 5
resolvConf: /etc/resolv.conf`

	if rotateCertificates {
		out += `
rotateCertificates: true`
	}

	out += `
runtimeRequestTimeout: 2m0s
serializeImagePulls: true
shutdownGracePeriod: 0s
shutdownGracePeriodCriticalPods: 0s
streamingConnectionIdleTimeout: 0s
syncFrequency: 1m0s`

	if volumePluginDir {
		out += `
volumePluginDir: /var/lib/kubelet/volumeplugins`
	}

	out += `
volumeStatsAggPeriod: 1m0s
`

	return out
}
