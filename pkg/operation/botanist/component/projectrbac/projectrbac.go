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

package projectrbac

import (
	"context"
	"fmt"
	"strings"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/utils/flow"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	NamePrefixSpecificProjectAdmin      = "gardener.cloud:system:project:"
	NamePrefixSpecificProjectMember     = "gardener.cloud:system:project-member:"
	NamePrefixSpecificProjectUAM        = "gardener.cloud:system:project-uam:"
	NamePrefixSpecificProjectViewer     = "gardener.cloud:system:project-viewer:"
	NamePrefixSpecificProjectExtensions = "gardener.cloud:extension:project:"

	NameProjectMember = "gardener.cloud:system:project-member"
	NameProjectViewer = "gardener.cloud:system:project-viewer"
)

type Interface interface {
	component.Deployer
	DeleteStaleExtensionRolesResources(context.Context) error
}

// New creates a new instance of DeployWaiter for the RBAC resources required to interact with Projects.
func New(client client.Client, project *gardencorev1beta1.Project) (Interface, error) {
	if project.Spec.Namespace == nil {
		return nil, fmt.Errorf("cannot create Interface for a project with `.spec.namespace=nil`")
	}

	return &projectRBAC{
		client:  client,
		project: project,
	}, nil
}

type projectRBAC struct {
	client  client.Client
	project *gardencorev1beta1.Project
}

func (p *projectRBAC) Deploy(ctx context.Context) error {
	var (
		admins  []rbacv1.Subject
		members []rbacv1.Subject
		uams    []rbacv1.Subject
		viewers []rbacv1.Subject

		extensionRolesNameToSubjects = map[string][]rbacv1.Subject{}
		extensionRolesNames          = sets.NewString()
	)

	if p.project.Spec.Owner != nil {
		admins = []rbacv1.Subject{*p.project.Spec.Owner}
	}

	for _, member := range p.project.Spec.Members {
		for _, role := range append([]string{member.Role}, member.Roles...) {
			if role == gardencorev1beta1.ProjectMemberAdmin || role == gardencorev1beta1.ProjectMemberOwner {
				members = append(members, member.Subject)
			}
			if role == gardencorev1beta1.ProjectMemberUserAccessManager {
				uams = append(uams, member.Subject)
			}
			if role == gardencorev1beta1.ProjectMemberViewer {
				viewers = append(viewers, member.Subject)
			}

			if strings.HasPrefix(role, gardencorev1beta1.ProjectMemberExtensionPrefix) {
				extensionRoleName := getExtensionRoleNameFromRole(role)
				extensionRolesNameToSubjects[extensionRoleName] = append(extensionRolesNameToSubjects[extensionRoleName], member.Subject)
				extensionRolesNames.Insert(extensionRoleName)
			}
		}
	}

	fns := []flow.TaskFn{
		// project admin resources
		func(ctx context.Context) error {
			return p.reconcileResources(
				ctx,
				NamePrefixSpecificProjectAdmin+p.project.Name,
				true,
				nil,
				nil,
				admins,
				nil,
				[]rbacv1.PolicyRule{
					{
						APIGroups:     []string{""},
						Resources:     []string{"namespaces"},
						ResourceNames: []string{*p.project.Spec.Namespace},
						Verbs:         []string{"get"},
					},
					{
						APIGroups:     []string{gardencorev1beta1.SchemeGroupVersion.Group},
						Resources:     []string{"projects"},
						ResourceNames: []string{p.project.Name},
						Verbs:         []string{"get", "patch", "manage-members", "update", "delete"},
					},
				},
			)
		},

		// project uam resources
		func(ctx context.Context) error {
			return p.reconcileResources(
				ctx,
				NamePrefixSpecificProjectUAM+p.project.Name,
				true,
				nil,
				nil,
				uams,
				nil,
				[]rbacv1.PolicyRule{
					{
						APIGroups:     []string{gardencorev1beta1.SchemeGroupVersion.Group},
						Resources:     []string{"projects"},
						ResourceNames: []string{p.project.Name},
						Verbs:         []string{"get", "manage-members", "patch", "update"},
					},
				},
			)
		},

		// project members resources
		func(ctx context.Context) error {
			return p.reconcileResources(
				ctx,
				NamePrefixSpecificProjectMember+p.project.Name,
				true,
				pointer.StringPtr(NameProjectMember),
				nil,
				members,
				nil,
				[]rbacv1.PolicyRule{
					{
						APIGroups:     []string{""},
						Resources:     []string{"namespaces"},
						ResourceNames: []string{*p.project.Spec.Namespace},
						Verbs:         []string{"get"},
					},
					{
						APIGroups:     []string{gardencorev1beta1.SchemeGroupVersion.Group},
						Resources:     []string{"projects"},
						ResourceNames: []string{p.project.Name},
						Verbs:         []string{"get", "patch", "update", "delete"},
					},
				},
			)
		},

		// project viewer resources
		func(ctx context.Context) error {
			return p.reconcileResources(
				ctx,
				NamePrefixSpecificProjectViewer+p.project.Name,
				true,
				pointer.StringPtr(NameProjectViewer),
				nil,
				viewers,
				nil,
				[]rbacv1.PolicyRule{
					{
						APIGroups:     []string{""},
						Resources:     []string{"namespaces"},
						ResourceNames: []string{*p.project.Spec.Namespace},
						Verbs:         []string{"get"},
					},
					{
						APIGroups:     []string{gardencorev1beta1.SchemeGroupVersion.Group},
						Resources:     []string{"projects"},
						ResourceNames: []string{p.project.Name},
						Verbs:         []string{"get"},
					},
				},
			)
		},
	}

	// project extension roles resources
	for _, roleName := range extensionRolesNames.List() {
		var (
			name            = fmt.Sprintf("%s%s:%s", NamePrefixSpecificProjectExtensions, p.project.Name, roleName)
			subjects        = extensionRolesNameToSubjects[roleName]
			aggregationRule = &rbacv1.AggregationRule{
				ClusterRoleSelectors: []metav1.LabelSelector{
					{MatchLabels: map[string]string{"rbac.gardener.cloud/aggregate-to-extension-role": roleName}},
				},
			}
		)

		fns = append(fns, func(ctx context.Context) error {
			return p.reconcileResources(
				ctx,
				name,
				false,
				&name,
				p.getExtensionRolesResourceLabels(),
				subjects,
				aggregationRule,
				nil,
			)
		})
	}

	return flow.Parallel(fns...)(ctx)
}

func (p *projectRBAC) reconcileResources(
	ctx context.Context,
	clusterRoleName string,
	withClusterRoleBinding bool,
	roleBindingName *string,
	labels map[string]string,
	subjects []rbacv1.Subject,
	aggregationRule *rbacv1.AggregationRule,
	rules []rbacv1.PolicyRule,
) error {
	subjectsUnique := removeDuplicateSubjects(subjects)

	ownerRef := metav1.NewControllerRef(&p.project.ObjectMeta, gardencorev1beta1.SchemeGroupVersion.WithKind("Project"))
	ownerRef.BlockOwnerDeletion = pointer.BoolPtr(false)

	clusterRole := emptyClusterRole(clusterRoleName)
	if _, err := controllerutil.CreateOrUpdate(ctx, p.client, clusterRole, func() error {
		clusterRole.OwnerReferences = []metav1.OwnerReference{*ownerRef}
		clusterRole.Labels = labels
		clusterRole.AggregationRule = aggregationRule
		clusterRole.Rules = rules
		return nil
	}); err != nil {
		return err
	}

	if withClusterRoleBinding {
		clusterRoleBinding := emptyClusterRoleBinding(clusterRoleName)
		if _, err := controllerutil.CreateOrUpdate(ctx, p.client, clusterRoleBinding, func() error {
			clusterRoleBinding.OwnerReferences = []metav1.OwnerReference{*ownerRef}
			clusterRoleBinding.Labels = labels
			clusterRoleBinding.RoleRef = rbacv1.RoleRef{
				APIGroup: rbacv1.SchemeGroupVersion.Group,
				Kind:     "ClusterRole",
				Name:     clusterRoleBinding.Name,
			}
			clusterRoleBinding.Subjects = subjectsUnique
			return nil
		}); err != nil {
			return err
		}
	}

	if roleBindingName != nil {
		roleBinding := emptyRoleBinding(*roleBindingName, *p.project.Spec.Namespace)
		if _, err := controllerutil.CreateOrUpdate(ctx, p.client, roleBinding, func() error {
			roleBinding.OwnerReferences = []metav1.OwnerReference{*ownerRef}
			roleBinding.Labels = labels
			roleBinding.RoleRef = rbacv1.RoleRef{
				APIGroup: rbacv1.SchemeGroupVersion.Group,
				Kind:     "ClusterRole",
				Name:     roleBinding.Name,
			}
			roleBinding.Subjects = subjectsUnique
			return nil
		}); err != nil {
			return err
		}
	}

	return nil
}

func (p *projectRBAC) Destroy(ctx context.Context) error {
	if err := p.deleteExtensionRolesResources(ctx, sets.NewString()); err != nil {
		return err
	}

	return kutil.DeleteObjects(ctx, p.client,
		emptyClusterRole(NamePrefixSpecificProjectAdmin+p.project.Name),
		emptyClusterRoleBinding(NamePrefixSpecificProjectAdmin+p.project.Name),

		emptyClusterRole(NamePrefixSpecificProjectUAM+p.project.Name),
		emptyClusterRoleBinding(NamePrefixSpecificProjectUAM+p.project.Name),

		emptyClusterRole(NamePrefixSpecificProjectMember+p.project.Name),
		emptyClusterRoleBinding(NamePrefixSpecificProjectMember+p.project.Name),
		emptyRoleBinding(NameProjectMember, *p.project.Spec.Namespace),

		emptyClusterRole(NamePrefixSpecificProjectViewer+p.project.Name),
		emptyClusterRoleBinding(NamePrefixSpecificProjectViewer+p.project.Name),
		emptyRoleBinding(NameProjectViewer, *p.project.Spec.Namespace),
	)
}

func (p *projectRBAC) DeleteStaleExtensionRolesResources(ctx context.Context) error {
	wantedExtensionRolesNames := sets.NewString()

	for _, member := range p.project.Spec.Members {
		for _, role := range append([]string{member.Role}, member.Roles...) {

			if strings.HasPrefix(role, gardencorev1beta1.ProjectMemberExtensionPrefix) {
				extensionRoleName := getExtensionRoleNameFromRole(role)
				wantedExtensionRolesNames.Insert(extensionRoleName)
			}
		}
	}

	return p.deleteExtensionRolesResources(ctx, wantedExtensionRolesNames)
}

func (p *projectRBAC) deleteExtensionRolesResources(ctx context.Context, wantedExtensionRolesNames sets.String) error {
	for _, list := range []client.ObjectList{
		&rbacv1.RoleBindingList{},
		&rbacv1.ClusterRoleList{},
	} {
		if err := p.client.List(ctx, list, client.InNamespace(*p.project.Spec.Namespace), client.MatchingLabels(p.getExtensionRolesResourceLabels())); err != nil {
			return err
		}

		if err := meta.EachListItem(list, func(obj runtime.Object) error {
			o := obj.(client.Object)
			if wantedExtensionRolesNames.Has(getExtensionRoleNameFromRBAC(o.GetName(), p.project.Name)) {
				return nil
			}

			return client.IgnoreNotFound(p.client.Delete(ctx, o))
		}); err != nil {
			return err
		}
	}

	return nil
}

func (p *projectRBAC) getExtensionRolesResourceLabels() map[string]string {
	return map[string]string{
		v1beta1constants.GardenRole:  v1beta1constants.LabelExtensionProjectRole,
		v1beta1constants.ProjectName: p.project.Name,
	}
}

func getExtensionRoleNameFromRBAC(resourceName, projectName string) string {
	return strings.TrimPrefix(resourceName, NamePrefixSpecificProjectExtensions+projectName+":")
}

func getExtensionRoleNameFromRole(role string) string {
	return strings.TrimPrefix(role, gardencorev1beta1.ProjectMemberExtensionPrefix)
}

func emptyClusterRole(name string) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: name}}
}

func emptyClusterRoleBinding(name string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: name}}
}

func emptyRoleBinding(name, namespace string) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
}

func removeDuplicateSubjects(subjects []rbacv1.Subject) []rbacv1.Subject {
	var (
		key = func(subject rbacv1.Subject) string {
			return fmt.Sprintf("%s_%s_%s_%s", subject.APIGroup, subject.Kind, subject.Namespace, subject.Name)
		}
		processed = sets.NewString()
		out       []rbacv1.Subject
	)

	for _, subject := range subjects {
		if k := key(subject); !processed.Has(k) {
			out = append(out, subject)
			processed.Insert(k)
		}
	}

	return out
}
