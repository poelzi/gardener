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

package projectrbac_test

import (
	"context"
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/projectrbac"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("ProjectRBAC", func() {
	var (
		ctrl *gomock.Controller
		c    *mockclient.MockClient

		project     *gardencorev1beta1.Project
		projectRBAC Interface
		err         error

		ctx         = context.TODO()
		fakeErr     = fmt.Errorf("fake error")
		projectName = "foobar"
		namespace   = "garden-" + projectName

		extensionRolePrefix = "gardener.cloud:extension:project:" + projectName + ":"
		extensionRole1      = "foo"
		extensionRole2      = "bar"

		member1 = rbacv1.Subject{Kind: rbacv1.UserKind, Name: "member1"}
		member2 = rbacv1.Subject{Kind: rbacv1.UserKind, Name: "member2"}
		member3 = rbacv1.Subject{Kind: rbacv1.UserKind, Name: "member3"}

		clusterRoleProjectAdmin        *rbacv1.ClusterRole
		clusterRoleBindingProjectAdmin *rbacv1.ClusterRoleBinding

		clusterRoleProjectUAM        *rbacv1.ClusterRole
		clusterRoleBindingProjectUAM *rbacv1.ClusterRoleBinding

		clusterRoleProjectMember        *rbacv1.ClusterRole
		clusterRoleBindingProjectMember *rbacv1.ClusterRoleBinding
		roleBindingProjectMember        *rbacv1.RoleBinding

		clusterRoleProjectViewer        *rbacv1.ClusterRole
		clusterRoleBindingProjectViewer *rbacv1.ClusterRoleBinding
		roleBindingProjectViewer        *rbacv1.RoleBinding

		clusterRoleProjectExtensionRole1 *rbacv1.ClusterRole
		roleBindingProjectExtensionRole1 *rbacv1.RoleBinding

		extensionRoleListOptions = []client.ListOption{
			client.InNamespace(namespace),
			client.MatchingLabels{
				v1beta1constants.GardenRole:  v1beta1constants.LabelExtensionProjectRole,
				v1beta1constants.ProjectName: projectName,
			},
		}

		emptyExtensionRoleBinding1 = rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      extensionRolePrefix + extensionRole1,
				Namespace: namespace,
			},
		}
		emptyExtensionRoleBinding2 = rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      extensionRolePrefix + extensionRole2,
				Namespace: namespace,
			},
		}
		emptyExtensionClusterRole1 = rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: extensionRolePrefix + extensionRole1,
			},
		}
		emptyExtensionClusterRole2 = rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: extensionRolePrefix + extensionRole2,
			},
		}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)

		project = &gardencorev1beta1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name: projectName,
			},
			Spec: gardencorev1beta1.ProjectSpec{
				Namespace: &namespace,
			},
		}
		projectRBAC, err = New(c, project)
		Expect(err).NotTo(HaveOccurred())

		clusterRoleProjectAdmin = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:system:project:" + projectName,
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "core.gardener.cloud/v1beta1",
					Kind:               "Project",
					Name:               projectName,
					Controller:         pointer.BoolPtr(true),
					BlockOwnerDeletion: pointer.BoolPtr(false),
				}},
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups:     []string{""},
					Resources:     []string{"namespaces"},
					ResourceNames: []string{namespace},
					Verbs:         []string{"get"},
				},
				{
					APIGroups:     []string{gardencorev1beta1.SchemeGroupVersion.Group},
					Resources:     []string{"projects"},
					ResourceNames: []string{projectName},
					Verbs:         []string{"get", "patch", "manage-members", "update", "delete"},
				},
			},
		}
		clusterRoleBindingProjectAdmin = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:system:project:" + projectName,
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "core.gardener.cloud/v1beta1",
					Kind:               "Project",
					Name:               projectName,
					Controller:         pointer.BoolPtr(true),
					BlockOwnerDeletion: pointer.BoolPtr(false),
				}},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "gardener.cloud:system:project:" + projectName,
			},
		}

		clusterRoleProjectUAM = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:system:project-uam:" + projectName,
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "core.gardener.cloud/v1beta1",
					Kind:               "Project",
					Name:               projectName,
					Controller:         pointer.BoolPtr(true),
					BlockOwnerDeletion: pointer.BoolPtr(false),
				}},
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups:     []string{gardencorev1beta1.SchemeGroupVersion.Group},
					Resources:     []string{"projects"},
					ResourceNames: []string{projectName},
					Verbs:         []string{"get", "manage-members", "patch", "update"},
				},
			},
		}
		clusterRoleBindingProjectUAM = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:system:project-uam:" + projectName,
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "core.gardener.cloud/v1beta1",
					Kind:               "Project",
					Name:               projectName,
					Controller:         pointer.BoolPtr(true),
					BlockOwnerDeletion: pointer.BoolPtr(false),
				}},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "gardener.cloud:system:project-uam:" + projectName,
			},
		}

		clusterRoleProjectMember = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:system:project-member:" + projectName,
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "core.gardener.cloud/v1beta1",
					Kind:               "Project",
					Name:               projectName,
					Controller:         pointer.BoolPtr(true),
					BlockOwnerDeletion: pointer.BoolPtr(false),
				}},
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups:     []string{""},
					Resources:     []string{"namespaces"},
					ResourceNames: []string{namespace},
					Verbs:         []string{"get"},
				},
				{
					APIGroups:     []string{gardencorev1beta1.SchemeGroupVersion.Group},
					Resources:     []string{"projects"},
					ResourceNames: []string{projectName},
					Verbs:         []string{"get", "patch", "update", "delete"},
				},
			},
		}
		clusterRoleBindingProjectMember = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:system:project-member:" + projectName,
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "core.gardener.cloud/v1beta1",
					Kind:               "Project",
					Name:               projectName,
					Controller:         pointer.BoolPtr(true),
					BlockOwnerDeletion: pointer.BoolPtr(false),
				}},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "gardener.cloud:system:project-member:" + projectName,
			},
		}
		roleBindingProjectMember = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener.cloud:system:project-member",
				Namespace: namespace,
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "core.gardener.cloud/v1beta1",
					Kind:               "Project",
					Name:               projectName,
					Controller:         pointer.BoolPtr(true),
					BlockOwnerDeletion: pointer.BoolPtr(false),
				}},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "gardener.cloud:system:project-member",
			},
		}

		clusterRoleProjectViewer = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:system:project-viewer:" + projectName,
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "core.gardener.cloud/v1beta1",
					Kind:               "Project",
					Name:               projectName,
					Controller:         pointer.BoolPtr(true),
					BlockOwnerDeletion: pointer.BoolPtr(false),
				}},
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups:     []string{""},
					Resources:     []string{"namespaces"},
					ResourceNames: []string{namespace},
					Verbs:         []string{"get"},
				},
				{
					APIGroups:     []string{gardencorev1beta1.SchemeGroupVersion.Group},
					Resources:     []string{"projects"},
					ResourceNames: []string{projectName},
					Verbs:         []string{"get"},
				},
			},
		}
		clusterRoleBindingProjectViewer = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:system:project-viewer:" + projectName,
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "core.gardener.cloud/v1beta1",
					Kind:               "Project",
					Name:               projectName,
					Controller:         pointer.BoolPtr(true),
					BlockOwnerDeletion: pointer.BoolPtr(false),
				}},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "gardener.cloud:system:project-viewer:" + projectName,
			},
		}
		roleBindingProjectViewer = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener.cloud:system:project-viewer",
				Namespace: namespace,
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "core.gardener.cloud/v1beta1",
					Kind:               "Project",
					Name:               projectName,
					Controller:         pointer.BoolPtr(true),
					BlockOwnerDeletion: pointer.BoolPtr(false),
				}},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "gardener.cloud:system:project-viewer",
			},
		}

		clusterRoleProjectExtensionRole1 = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: extensionRolePrefix + extensionRole1,
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "core.gardener.cloud/v1beta1",
					Kind:               "Project",
					Name:               projectName,
					Controller:         pointer.BoolPtr(true),
					BlockOwnerDeletion: pointer.BoolPtr(false),
				}},
				Labels: map[string]string{
					"gardener.cloud/role":         "extension-project-role",
					"project.gardener.cloud/name": projectName,
				},
			},
			AggregationRule: &rbacv1.AggregationRule{
				ClusterRoleSelectors: []metav1.LabelSelector{
					{MatchLabels: map[string]string{"rbac.gardener.cloud/aggregate-to-extension-role": extensionRole1}},
				},
			},
		}
		roleBindingProjectExtensionRole1 = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      extensionRolePrefix + extensionRole1,
				Namespace: namespace,
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "core.gardener.cloud/v1beta1",
					Kind:               "Project",
					Name:               projectName,
					Controller:         pointer.BoolPtr(true),
					BlockOwnerDeletion: pointer.BoolPtr(false),
				}},
				Labels: map[string]string{
					"gardener.cloud/role":         "extension-project-role",
					"project.gardener.cloud/name": projectName,
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     extensionRolePrefix + extensionRole1,
			},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Deploy", func() {
		It("should successfully reconcile all project RBAC resources", func() {
			project.Spec.Owner = &member3
			project.Spec.Members = []gardencorev1beta1.ProjectMember{
				{
					Subject: member1,
					Role:    "extension:" + extensionRole1,
					Roles:   []string{"viewer"},
				},
				{
					Subject: member2,
					Roles:   []string{"admin", "uam"},
				},
				{
					Subject: member3,
					Roles:   []string{"owner", "viewer", "admin"},
				},
			}

			clusterRoleBindingProjectAdmin.Subjects = []rbacv1.Subject{member3}
			clusterRoleBindingProjectUAM.Subjects = []rbacv1.Subject{member2}
			clusterRoleBindingProjectMember.Subjects = []rbacv1.Subject{member2, member3}
			roleBindingProjectMember.Subjects = []rbacv1.Subject{member2, member3}
			clusterRoleBindingProjectViewer.Subjects = []rbacv1.Subject{member1, member3}
			roleBindingProjectViewer.Subjects = []rbacv1.Subject{member1, member3}
			roleBindingProjectExtensionRole1.Subjects = []rbacv1.Subject{member1}

			// project admin
			c.EXPECT().Get(ctx, kutil.Key(clusterRoleProjectAdmin.Name), gomock.AssignableToTypeOf(&rbacv1.ClusterRole{}))
			c.EXPECT().Update(ctx, clusterRoleProjectAdmin)
			c.EXPECT().Get(ctx, kutil.Key(clusterRoleBindingProjectAdmin.Name), gomock.AssignableToTypeOf(&rbacv1.ClusterRoleBinding{}))
			c.EXPECT().Update(ctx, clusterRoleBindingProjectAdmin)

			// project uam
			c.EXPECT().Get(ctx, kutil.Key(clusterRoleProjectUAM.Name), gomock.AssignableToTypeOf(&rbacv1.ClusterRole{}))
			c.EXPECT().Update(ctx, clusterRoleProjectUAM)
			c.EXPECT().Get(ctx, kutil.Key(clusterRoleBindingProjectUAM.Name), gomock.AssignableToTypeOf(&rbacv1.ClusterRoleBinding{}))
			c.EXPECT().Update(ctx, clusterRoleBindingProjectUAM)

			// project member
			c.EXPECT().Get(ctx, kutil.Key(clusterRoleProjectMember.Name), gomock.AssignableToTypeOf(&rbacv1.ClusterRole{}))
			c.EXPECT().Update(ctx, clusterRoleProjectMember)
			c.EXPECT().Get(ctx, kutil.Key(clusterRoleBindingProjectMember.Name), gomock.AssignableToTypeOf(&rbacv1.ClusterRoleBinding{}))
			c.EXPECT().Update(ctx, clusterRoleBindingProjectMember)
			c.EXPECT().Get(ctx, kutil.Key(roleBindingProjectMember.Namespace, roleBindingProjectMember.Name), gomock.AssignableToTypeOf(&rbacv1.RoleBinding{}))
			c.EXPECT().Update(ctx, roleBindingProjectMember)

			// project viewer
			c.EXPECT().Get(ctx, kutil.Key(clusterRoleProjectViewer.Name), gomock.AssignableToTypeOf(&rbacv1.ClusterRole{}))
			c.EXPECT().Update(ctx, clusterRoleProjectViewer)
			c.EXPECT().Get(ctx, kutil.Key(clusterRoleBindingProjectViewer.Name), gomock.AssignableToTypeOf(&rbacv1.ClusterRoleBinding{}))
			c.EXPECT().Update(ctx, clusterRoleBindingProjectViewer)
			c.EXPECT().Get(ctx, kutil.Key(roleBindingProjectViewer.Namespace, roleBindingProjectViewer.Name), gomock.AssignableToTypeOf(&rbacv1.RoleBinding{}))
			c.EXPECT().Update(ctx, roleBindingProjectViewer)

			// project extension roles
			c.EXPECT().Get(ctx, kutil.Key(clusterRoleProjectExtensionRole1.Name), gomock.AssignableToTypeOf(&rbacv1.ClusterRole{}))
			c.EXPECT().Update(ctx, clusterRoleProjectExtensionRole1)
			c.EXPECT().Get(ctx, kutil.Key(roleBindingProjectExtensionRole1.Namespace, roleBindingProjectExtensionRole1.Name), gomock.AssignableToTypeOf(&rbacv1.RoleBinding{}))
			c.EXPECT().Update(ctx, roleBindingProjectExtensionRole1)

			Expect(projectRBAC.Deploy(ctx)).To(Succeed())
		})
	})

	Describe("#Destroy", func() {
		It("should successfully delete all project RBAC resources", func() {
			project.Spec.Members = []gardencorev1beta1.ProjectMember{{Role: "extension:" + extensionRole1}}

			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&rbacv1.RoleBindingList{}), extensionRoleListOptions).DoAndReturn(func(_ context.Context, list *rbacv1.RoleBindingList, _ ...client.ListOption) error {
				list.Items = []rbacv1.RoleBinding{emptyExtensionRoleBinding1, emptyExtensionRoleBinding2}
				return nil
			})
			c.EXPECT().Delete(ctx, &emptyExtensionRoleBinding1)
			c.EXPECT().Delete(ctx, &emptyExtensionRoleBinding2)

			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRoleList{}), extensionRoleListOptions).DoAndReturn(func(_ context.Context, list *rbacv1.ClusterRoleList, _ ...client.ListOption) error {
				list.Items = []rbacv1.ClusterRole{emptyExtensionClusterRole1, emptyExtensionClusterRole2}
				return nil
			})
			c.EXPECT().Delete(ctx, &emptyExtensionClusterRole1)
			c.EXPECT().Delete(ctx, &emptyExtensionClusterRole2)

			c.EXPECT().Delete(ctx, &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:system:project:" + projectName}})
			c.EXPECT().Delete(ctx, &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:system:project:" + projectName}})

			c.EXPECT().Delete(ctx, &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:system:project-uam:" + projectName}})
			c.EXPECT().Delete(ctx, &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:system:project-uam:" + projectName}})

			c.EXPECT().Delete(ctx, &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:system:project-member:" + projectName}})
			c.EXPECT().Delete(ctx, &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:system:project-member:" + projectName}})
			c.EXPECT().Delete(ctx, &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:system:project-member", Namespace: namespace}})

			c.EXPECT().Delete(ctx, &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:system:project-viewer:" + projectName}})
			c.EXPECT().Delete(ctx, &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:system:project-viewer:" + projectName}})
			c.EXPECT().Delete(ctx, &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:system:project-viewer", Namespace: namespace}})

			Expect(projectRBAC.Destroy(ctx)).To(Succeed())
		})
	})

	Describe("#DeleteStaleExtensionRolesResources", func() {
		It("should do nothing because neither rolebindings nor clusterroles exist", func() {
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&rbacv1.RoleBindingList{}), extensionRoleListOptions)
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRoleList{}), extensionRoleListOptions)

			Expect(projectRBAC.DeleteStaleExtensionRolesResources(ctx)).To(Succeed())
		})

		It("should return an error because listing the rolebindings failed", func() {
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&rbacv1.RoleBindingList{}), extensionRoleListOptions).Return(fakeErr)

			Expect(projectRBAC.DeleteStaleExtensionRolesResources(ctx)).To(MatchError(fakeErr))
		})

		It("should return an error because listing the clusterroles failed", func() {
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&rbacv1.RoleBindingList{}), extensionRoleListOptions)
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRoleList{}), extensionRoleListOptions).Return(fakeErr)

			Expect(projectRBAC.DeleteStaleExtensionRolesResources(ctx)).To(MatchError(fakeErr))
		})

		It("should return an error because deleting a stale rolebinding failed", func() {
			gomock.InOrder(
				c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&rbacv1.RoleBindingList{}), extensionRoleListOptions).DoAndReturn(func(_ context.Context, list *rbacv1.RoleBindingList, _ ...client.ListOption) error {
					list.Items = []rbacv1.RoleBinding{emptyExtensionRoleBinding1, emptyExtensionRoleBinding2}
					return nil
				}),
				c.EXPECT().Delete(ctx, &emptyExtensionRoleBinding1).Return(fakeErr),
			)

			Expect(projectRBAC.DeleteStaleExtensionRolesResources(ctx)).To(MatchError(fakeErr))
		})

		It("should return an error because deleting a stale clusterrole failed", func() {
			gomock.InOrder(
				c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&rbacv1.RoleBindingList{}), extensionRoleListOptions).DoAndReturn(func(_ context.Context, list *rbacv1.RoleBindingList, _ ...client.ListOption) error {
					list.Items = []rbacv1.RoleBinding{emptyExtensionRoleBinding1, emptyExtensionRoleBinding2}
					return nil
				}),
				c.EXPECT().Delete(ctx, &emptyExtensionRoleBinding1),
				c.EXPECT().Delete(ctx, &emptyExtensionRoleBinding2),

				c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRoleList{}), extensionRoleListOptions).DoAndReturn(func(_ context.Context, list *rbacv1.ClusterRoleList, _ ...client.ListOption) error {
					list.Items = []rbacv1.ClusterRole{emptyExtensionClusterRole1, emptyExtensionClusterRole2}
					return nil
				}),
				c.EXPECT().Delete(ctx, &emptyExtensionClusterRole1).Return(fakeErr),
			)

			Expect(projectRBAC.DeleteStaleExtensionRolesResources(ctx)).To(MatchError(fakeErr))
		})

		It("should succeed deleting the stale rolebindings and clusterroles", func() {
			project.Spec.Members = []gardencorev1beta1.ProjectMember{{Role: "extension:" + extensionRole1}}

			gomock.InOrder(
				c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&rbacv1.RoleBindingList{}), extensionRoleListOptions).DoAndReturn(func(_ context.Context, list *rbacv1.RoleBindingList, _ ...client.ListOption) error {
					list.Items = []rbacv1.RoleBinding{emptyExtensionRoleBinding1, emptyExtensionRoleBinding2}
					return nil
				}),
				c.EXPECT().Delete(ctx, &emptyExtensionRoleBinding2),

				c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRoleList{}), extensionRoleListOptions).DoAndReturn(func(_ context.Context, list *rbacv1.ClusterRoleList, _ ...client.ListOption) error {
					list.Items = []rbacv1.ClusterRole{emptyExtensionClusterRole1, emptyExtensionClusterRole2}
					return nil
				}),
				c.EXPECT().Delete(ctx, &emptyExtensionClusterRole2),
			)

			Expect(projectRBAC.DeleteStaleExtensionRolesResources(ctx)).To(Succeed())
		})
	})
})
