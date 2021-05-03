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

package terraformer_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"

	. "github.com/gardener/gardener/extensions/pkg/terraformer"
)

var _ = Describe("terraformer", func() {
	var (
		c   client.Client
		ctx context.Context
		log logr.Logger
	)

	BeforeEach(func() {
		ctx = context.Background()
		log = logzap.New(logzap.WriteTo(GinkgoWriter))
	})

	Describe("#IsStateEmpty", func() {
		var (
			terraformer        Terraformer
			state              *corev1.ConfigMap
			config             *corev1.ConfigMap
			variable           *corev1.Secret
			emptyInfraState    = ""
			nonEmptyInfraState = "Some non-empty infra state"
		)
		const (
			purpose                = "purpose"
			image                  = "image"
			stateName              = name + "." + purpose + StateSuffix
			configName             = name + "." + purpose + ConfigSuffix
			variableName           = name + "." + purpose + VariablesSuffix
			expectingEmptyState    = true
			expectingNonEmptyState = false
		)

		BeforeEach(func() {
			state = &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: stateName}}
			config = &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: configName}}
			variable = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: variableName}}
		})

		DescribeTable(
			"Should respect terraformer finalizer as non-empty state",
			func(stateFinalizers, configFinalizers, variableFinalizers []string, infraState *string, expectation bool) {
				state.ObjectMeta.Finalizers = stateFinalizers
				config.ObjectMeta.Finalizers = configFinalizers
				variable.ObjectMeta.Finalizers = variableFinalizers

				if infraState != nil {
					state.Data = map[string]string{
						StateKey: *infraState,
					}
				}

				c = fakeclient.NewClientBuilder().WithScheme(scheme.Scheme).WithRuntimeObjects(config, state, variable).Build()
				terraformer = New(log, c, nil, purpose, namespace, name, image)
				Expect(terraformer.IsStateEmpty(ctx)).To(Equal(expectation))

			},
			Entry("No finalizer without state", []string{}, []string{}, []string{}, nil, expectingEmptyState),
			Entry("No finalizer with empty state", []string{}, []string{}, []string{}, &emptyInfraState, expectingEmptyState),
			Entry("Other finalizer with nil state", []string{"gardener.cloud"}, []string{}, []string{}, nil, expectingEmptyState),
			Entry("Finalizer only on state configmap with nil state", []string{TerraformerFinalizer}, []string{}, []string{}, nil, expectingNonEmptyState),
			Entry("Finalizer only on config configmap with nil state", []string{}, []string{TerraformerFinalizer}, []string{}, nil, expectingNonEmptyState),
			Entry("Finalizer only on variables secrets with nil state", []string{}, []string{}, []string{TerraformerFinalizer}, nil, expectingNonEmptyState),
			Entry("Finalizer on all resources with non-empty state", []string{TerraformerFinalizer}, []string{TerraformerFinalizer}, []string{TerraformerFinalizer}, &nonEmptyInfraState, expectingNonEmptyState),
			Entry("No finalizers with non-empty state", []string{}, []string{}, []string{}, &nonEmptyInfraState, expectingNonEmptyState),
			Entry("Finalizer with non-empty state", []string{TerraformerFinalizer}, []string{}, []string{}, &nonEmptyInfraState, expectingNonEmptyState),
		)

		DescribeTable(
			"Should ignore already gone resources",
			func(stateFinalizers []string, infraState *string, expectation bool) {
				state.ObjectMeta.Finalizers = stateFinalizers

				if infraState != nil {
					state.Data = map[string]string{
						StateKey: *infraState,
					}
				}

				c = fakeclient.NewClientBuilder().WithScheme(scheme.Scheme).WithRuntimeObjects(state).Build()
				terraformer = New(log, c, nil, purpose, namespace, name, image)
				Expect(terraformer.IsStateEmpty(ctx)).To(Equal(expectation))

			},
			Entry("No state with finalizer", []string{TerraformerFinalizer}, nil, expectingNonEmptyState),
			Entry("No state without finalizer", []string{}, nil, expectingEmptyState),
			Entry("No state with other finalizer", []string{"gardener.cloud"}, nil, expectingEmptyState),
			Entry("Empty state with finalizer", []string{TerraformerFinalizer}, &emptyInfraState, expectingNonEmptyState),
			Entry("Empty state without finalizer", []string{}, &emptyInfraState, expectingEmptyState),
			Entry("Empty state with other finalizer", []string{"gardener.cloud"}, &emptyInfraState, expectingEmptyState),
			Entry("Non-empty state with finalizer", []string{TerraformerFinalizer}, &nonEmptyInfraState, expectingNonEmptyState),
			Entry("Non-empty state without finalizer", []string{}, &nonEmptyInfraState, expectingNonEmptyState),
			Entry("Non-empty state with other finalizer", []string{"gardener.cloud"}, &nonEmptyInfraState, expectingNonEmptyState),
		)

		It("Should detect empty state if no resource exist", func() {
			c = fakeclient.NewClientBuilder().Build()
			terraformer = New(log, c, nil, purpose, namespace, name, image)
			Expect(terraformer.IsStateEmpty(ctx)).To(BeTrue())
		})
	})
})
