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

package utils_test

import (
	. "github.com/gardener/gardener/pkg/utils"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/types"
	"k8s.io/utils/pointer"
)

var _ = Describe("utils", func() {
	Describe("#MergeStringMaps", func() {
		It("should return nil", func() {
			result := MergeStringMaps(nil, nil)

			Expect(result).To(BeNil())
		})

		It("should return an empty map", func() {
			emptyMap := map[string]string{}

			result := MergeStringMaps(emptyMap, nil)

			Expect(result).To(Equal(emptyMap))
		})

		It("should return a merged map", func() {
			var (
				oldMap = map[string]string{
					"a": "1",
					"b": "2",
				}
				newMap = map[string]string{
					"b": "20",
					"c": "3",
				}
			)

			result := MergeStringMaps(oldMap, newMap)

			Expect(result).To(Equal(map[string]string{
				"a": "1",
				"b": "20",
				"c": "3",
			}))
		})
	})

	DescribeTable("#IsTrue",
		func(value *bool, matcher GomegaMatcher) {
			Expect(IsTrue(value)).To(matcher)
		},
		Entry("nil", nil, BeFalse()),
		Entry("false", pointer.BoolPtr(false), BeFalse()),
		Entry("true", pointer.BoolPtr(true), BeTrue()),
	)

	DescribeTable("#IDForKeyWithOptionalValue",
		func(key string, value *string, expectation string) {
			Expect(IDForKeyWithOptionalValue(key, value)).To(Equal(expectation))
		},
		Entry("only key", "foo", nil, "foo"),
		Entry("key and value", "foo", pointer.StringPtr("bar"), "foo=bar"),
	)

	Describe("#Indent", func() {
		var spaces = 2

		It("should not indent a single-line string", func() {
			Expect(Indent("foo", spaces)).To(Equal("foo"))
		})

		It("should properly indent a multi-line string", func() {
			Expect(Indent(`foo
bar
baz`, spaces)).To(Equal(`foo
  bar
  baz`))
		})
	})

	Describe("#ShallowCopyMapStringInterface", func() {
		It("should create a shallow copy of the map", func() {
			v := map[string]interface{}{"foo": nil, "bar": map[string]interface{}{"baz": nil}}

			c := ShallowCopyMapStringInterface(v)

			Expect(c).To(Equal(v))

			c["foo"] = 1
			Expect(v["foo"]).To(BeNil())

			c["bar"].(map[string]interface{})["baz"] = "bang"
			Expect(v["bar"].(map[string]interface{})["baz"]).To(Equal("bang"))
		})
	})
})
