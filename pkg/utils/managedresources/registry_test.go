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

package managedresources_test

import (
	. "github.com/gardener/gardener/pkg/utils/managedresources"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
)

var _ = Describe("Registry", func() {
	var (
		scheme       = runtime.NewScheme()
		serial       = json.NewSerializerWithOptions(json.DefaultMetaFactory, scheme, scheme, json.SerializerOptions{Yaml: true, Pretty: false, Strict: false})
		codecFactory = serializer.NewCodecFactory(scheme)

		registry *Registry

		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "secret:name",
				Namespace: "foo",
			},
		}
		secretFilename   = "secret__" + secret.Namespace + "__secret_name.yaml"
		secretSerialized = []byte(`apiVersion: v1
kind: Secret
metadata:
  creationTimestamp: null
  name: ` + secret.Name + `
  namespace: ` + secret.Namespace + `
`)

		roleBinding = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "rolebinding.name",
				Namespace: "bar",
			},
		}
		roleBindingFilename   = "rolebinding__" + roleBinding.Namespace + "__rolebinding.name.yaml"
		roleBindingSerialized = []byte(`apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  creationTimestamp: null
  name: ` + roleBinding.Name + `
  namespace: ` + roleBinding.Namespace + `
roleRef:
  apiGroup: ""
  kind: ""
  name: ""
`)
	)

	BeforeEach(func() {
		Expect(kubernetesscheme.AddToScheme(scheme)).To(Succeed())

		registry = NewRegistry(scheme, codecFactory, serial)
	})

	Describe("#Add", func() {
		It("should successfully add the object", func() {
			Expect(registry.Add(&corev1.Secret{})).To(Succeed())
		})

		It("should do nothing because the object is nil", func() {
			Expect(registry.Add(nil)).To(Succeed())
		})

		It("should return an error due to duplicates in registry", func() {
			Expect(registry.Add(&corev1.Secret{})).To(Succeed())
			Expect(registry.Add(&corev1.Secret{})).To(MatchError(ContainSubstring("duplicate filename in registry")))
		})

		It("should return an error due to failed serialization", func() {
			registry = NewRegistry(runtime.NewScheme(), codecFactory, serial)

			err := registry.Add(&corev1.Secret{})
			Expect(err).To(HaveOccurred())
			Expect(runtime.IsNotRegisteredError(err)).To(BeTrue())
		})
	})

	Describe("#SerializedObjects", func() {
		It("should return the serialized object map", func() {
			Expect(registry.Add(secret)).To(Succeed())
			Expect(registry.Add(roleBinding)).To(Succeed())

			Expect(registry.SerializedObjects()).To(Equal(map[string][]byte{
				secretFilename:      secretSerialized,
				roleBindingFilename: roleBindingSerialized,
			}))
		})
	})

	Describe("#AddAllAndSerialize", func() {
		It("should add all objects and return the serialized object map", func() {
			objectMap, err := registry.AddAllAndSerialize(secret, roleBinding)
			Expect(err).NotTo(HaveOccurred())
			Expect(objectMap).To(Equal(map[string][]byte{
				secretFilename:      secretSerialized,
				roleBindingFilename: roleBindingSerialized,
			}))
		})
	})

	Describe("#RegisteredObjects", func() {
		It("should return the registered objects", func() {
			Expect(registry.Add(secret)).To(Succeed())
			Expect(registry.Add(roleBinding)).To(Succeed())

			Expect(registry.RegisteredObjects()).To(Equal(map[string]client.Object{
				secretFilename:      secret,
				roleBindingFilename: roleBinding,
			}))
		})
	})

	Describe("#String", func() {
		It("should return the string representation of the registry", func() {
			Expect(registry.Add(secret)).To(Succeed())
			Expect(registry.Add(roleBinding)).To(Succeed())

			result := registry.String()
			Expect(result).To(ContainSubstring(`* ` + secretFilename + `:
` + string(secretSerialized)))
			Expect(result).To(ContainSubstring(`* ` + roleBindingFilename + `:
` + string(roleBindingSerialized)))
		})
	})
})
