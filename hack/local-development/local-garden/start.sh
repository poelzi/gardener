#!/usr/bin/env bash
#
# Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o errexit
set -o nounset
set -o pipefail

LOCAL_GARDEN_LABEL=${1:-local-garden}
KUBECONFIGPATH="$(dirname $0)/kubeconfigs/default-admin.conf"

mkdir -p dev

echo "# Remove old containers and create the docker user network"
$(dirname $0)/cleanup $LOCAL_GARDEN_LABEL
docker network create gardener-dev --label $LOCAL_GARDEN_LABEL

echo "# Start the nodeless kubernetes environment"
$(dirname $0)/run-kube-etcd $LOCAL_GARDEN_LABEL
$(dirname $0)/run-kube-apiserver $LOCAL_GARDEN_LABEL
$(dirname $0)/run-kube-controller-manager $LOCAL_GARDEN_LABEL

echo "# This etcd will be used to storge gardener resources (e.g., seeds, shoots)"
$(dirname $0)/run-gardener-etcd $LOCAL_GARDEN_LABEL

for i in 1..10; do
  if $(KUBECONFIG=$KUBECONFIGPATH kubectl cluster-info > /dev/null 2>&1); then
    break
  fi
  echo "# Waiting until Kube-Apiserver is available"
done

echo "# Applying proxy RBAC for the extension controller"
echo "# After this step, you can start using the cluster at KUBECONFIG=hack/local-development/local-garden/kubeconfigs/default-admin.conf"
$(dirname $0)/apply-rbac-garden-ns

echo "# Now you can start using the cluster at with \`export KUBECONFIG=hack/local-development/local-garden/kubeconfigs/default-admin.conf\`"
echo "# Then you need to run \`make dev-setup\` to setup config and certificates files for gardener's components and to register the gardener-apiserver."
echo "# Finally, run \`make start-apiserver,start-controller-manager,start-scheduler,start-gardenlet\` to start the gardener components as usual."