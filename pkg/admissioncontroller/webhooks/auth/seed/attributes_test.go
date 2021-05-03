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

package seed_test

import (
	. "github.com/gardener/gardener/pkg/admissioncontroller/webhooks/auth/seed"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	authorizationv1 "k8s.io/api/authorization/v1"
	userpkg "k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
)

var _ = Describe("Attributes", func() {
	var (
		userName             = "foo"
		userID               = "uid"
		userGroups           = []string{"foo", "bar"}
		userExtraStringSlice = map[string][]string{"baz": {"foo"}}
		userExtra            = map[string]authorizationv1.ExtraValue{"baz": {"foo"}}
		user                 = &userpkg.DefaultInfo{
			Name:   userName,
			UID:    userID,
			Groups: userGroups,
			Extra:  userExtraStringSlice,
		}
		expectUserToBeCorrect = func(user userpkg.Info) {
			Expect(user.GetName()).To(Equal(userName))
			Expect(user.GetUID()).To(Equal(userID))
			Expect(user.GetGroups()).To(Equal(userGroups))
			Expect(user.GetExtra()).To(Equal(userExtraStringSlice))
		}

		verb        = "verb"
		version     = "apiversion"
		group       = "group"
		namespace   = "namespace"
		name        = "name"
		resource    = "resource"
		subresource = "subresource"
		path        = "/path"

		resourceAttributes    authorizationv1.ResourceAttributes
		nonResourceAttributes authorizationv1.NonResourceAttributes
		sarSpec               authorizationv1.SubjectAccessReviewSpec

		expectedResourceAttributesRecord    authorizer.AttributesRecord
		expectedNonResourceAttributesRecord authorizer.AttributesRecord
	)

	BeforeEach(func() {
		resourceAttributes = authorizationv1.ResourceAttributes{
			Verb:        verb,
			Namespace:   namespace,
			Group:       group,
			Version:     version,
			Resource:    resource,
			Subresource: subresource,
			Name:        name,
		}
		nonResourceAttributes = authorizationv1.NonResourceAttributes{
			Verb: verb,
			Path: path,
		}
		sarSpec = authorizationv1.SubjectAccessReviewSpec{
			User:   userName,
			Groups: userGroups,
			UID:    userID,
			Extra:  userExtra,
		}

		expectedResourceAttributesRecord = authorizer.AttributesRecord{
			User:            user,
			Verb:            verb,
			Namespace:       namespace,
			APIGroup:        group,
			APIVersion:      version,
			Resource:        resource,
			Subresource:     subresource,
			Name:            name,
			ResourceRequest: true,
		}
		expectedNonResourceAttributesRecord = authorizer.AttributesRecord{
			User:            user,
			Verb:            verb,
			Path:            path,
			ResourceRequest: false,
		}
	})

	Describe("#ResourceAttributesFrom", func() {
		It("should return the expected attributes record", func() {
			result := ResourceAttributesFrom(user, resourceAttributes)

			Expect(result).To(Equal(expectedResourceAttributesRecord))
			expectUserToBeCorrect(result.User)
		})
	})

	Describe("#NonResourceAttributesFrom", func() {
		It("should return the expected attributes record", func() {
			result := NonResourceAttributesFrom(user, nonResourceAttributes)

			Expect(result).To(Equal(expectedNonResourceAttributesRecord))
			expectUserToBeCorrect(result.User)
		})
	})

	Describe("#AuthorizationAttributesFrom", func() {
		It("should return the expected attributes record (neither)", func() {
			result := AuthorizationAttributesFrom(sarSpec)

			Expect(result).To(Equal(authorizer.AttributesRecord{}))
		})

		It("should return the expected attributes record (resource)", func() {
			sarSpec.ResourceAttributes = &resourceAttributes

			result := AuthorizationAttributesFrom(sarSpec)

			Expect(result).To(Equal(expectedResourceAttributesRecord))
			expectUserToBeCorrect(result.User)
		})

		It("should return the expected attributes record (non-resource)", func() {
			sarSpec.NonResourceAttributes = &nonResourceAttributes

			result := AuthorizationAttributesFrom(sarSpec)

			Expect(result).To(Equal(expectedNonResourceAttributesRecord))
			expectUserToBeCorrect(result.User)
		})
	})
})
