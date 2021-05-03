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
	gacmetrics "github.com/gardener/gardener/pkg/admissioncontroller/metrics"

	"github.com/prometheus/client_golang/prometheus"
)

const seedAuthorizerSubsystem = "seed_authorizer"

var (
	metricUpdateDuration = gacmetrics.Factory.NewHistogramVec(
		prometheus.HistogramOpts{
			Subsystem: seedAuthorizerSubsystem,
			Namespace: gacmetrics.Namespace,
			Name:      "graph_update_duration_seconds",
			Help:      "Histogram of duration of resource dependency graph updates in seed authorizer.",
			// Start with 0.1ms with the last bucket being [~200ms, Inf)
			Buckets: prometheus.ExponentialBuckets(0.0001, 2, 12),
		},
		[]string{
			"kind",
			"operation",
		},
	)

	metricPathCheckDuration = gacmetrics.Factory.NewHistogramVec(
		prometheus.HistogramOpts{
			Subsystem: seedAuthorizerSubsystem,
			Namespace: gacmetrics.Namespace,
			Name:      "graph_path_check_duration_seconds",
			Help:      "Histogram of duration of checks whether a path exists in the resource dependency graph in seed authorizer.",
			// Start with 0.1ms with the last bucket being [~200ms, Inf)
			Buckets: prometheus.ExponentialBuckets(0.0001, 2, 12),
		},
		[]string{
			"fromKind",
			"toKind",
		},
	)
)
