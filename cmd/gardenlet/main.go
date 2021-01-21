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

package main

import (
	"os/exec"

	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"github.com/gardener/gardener/cmd/gardenlet/app"
	"github.com/gardener/gardener/cmd/utils"
	"github.com/gardener/gardener/pkg/gardenlet/features"
)

func main() {
	utils.DeduplicateWarnings()
	features.RegisterFeatureGates()

	if err := exec.Command("which", "openvpn").Run(); err != nil {
		panic("openvpn is not installed or not executable. cannot start gardenlet.")
	}

	ctx := signals.SetupSignalHandler()
	if err := app.NewCommandStartGardenlet().ExecuteContext(ctx); err != nil {
		panic(err)
	}
}
