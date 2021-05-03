// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apacht.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tolerationrestriction

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/gardener/gardener/pkg/apis/core"
	corevalidation "github.com/gardener/gardener/pkg/apis/core/validation"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	coreinformers "github.com/gardener/gardener/pkg/client/core/informers/internalversion"
	corelisters "github.com/gardener/gardener/pkg/client/core/listers/core/internalversion"
	"github.com/gardener/gardener/pkg/utils"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/plugin/pkg/shoot/tolerationrestriction/apis/shoottolerationrestriction"
	"github.com/gardener/gardener/plugin/pkg/shoot/tolerationrestriction/apis/shoottolerationrestriction/validation"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/admission"
)

const (
	// PluginName is the name of this admission plugin.
	PluginName = "ShootTolerationRestriction"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, func(cfg io.Reader) (admission.Interface, error) {
		config, err := LoadConfiguration(cfg)
		if err != nil {
			return nil, err
		}

		if err := validation.ValidateConfiguration(config); err != nil {
			return nil, fmt.Errorf("invalid config: %+v", err)
		}

		return New(config)
	})
}

// TolerationRestriction contains listers and admission handler.
type TolerationRestriction struct {
	*admission.Handler

	projectLister corelisters.ProjectLister
	readyFunc     admission.ReadyFunc

	defaults  []core.Toleration
	allowlist []core.Toleration
}

var (
	_ = admissioninitializer.WantsInternalCoreInformerFactory(&TolerationRestriction{})

	readyFuncs []admission.ReadyFunc
)

// New creates a new TolerationRestriction admission plugin.
func New(config *shoottolerationrestriction.Configuration) (*TolerationRestriction, error) {
	return &TolerationRestriction{
		Handler:   admission.NewHandler(admission.Create, admission.Update),
		defaults:  config.Defaults,
		allowlist: config.Whitelist,
	}, nil
}

// AssignReadyFunc assigns the ready function to the admission handler.
func (t *TolerationRestriction) AssignReadyFunc(f admission.ReadyFunc) {
	t.readyFunc = f
	t.SetReadyFunc(f)
}

// SetInternalCoreInformerFactory sets the internal garden core informer factory.
func (t *TolerationRestriction) SetInternalCoreInformerFactory(f coreinformers.SharedInformerFactory) {
	projectInformer := f.Core().InternalVersion().Projects()
	t.projectLister = projectInformer.Lister()

	readyFuncs = append(readyFuncs, projectInformer.Informer().HasSynced)
}

func (t *TolerationRestriction) waitUntilReady(attrs admission.Attributes) error {
	// Wait until the caches have been synced
	if t.readyFunc == nil {
		t.AssignReadyFunc(func() bool {
			for _, readyFunc := range readyFuncs {
				if !readyFunc() {
					return false
				}
			}
			return true
		})
	}

	if !t.WaitForReady() {
		return admission.NewForbidden(attrs, errors.New("not yet ready to handle request"))
	}

	return nil
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (t *TolerationRestriction) ValidateInitialization() error {
	if t.projectLister == nil {
		return errors.New("missing Project lister")
	}
	return nil
}

var _ admission.ValidationInterface = &TolerationRestriction{}

// Admit defaults shoot tolerations with both global and project defaults.
func (t *TolerationRestriction) Admit(ctx context.Context, a admission.Attributes, o admission.ObjectInterfaces) error {
	if err := t.waitUntilReady(a); err != nil {
		return fmt.Errorf("err while waiting for ready %v", err)
	}

	if a.GetKind().GroupKind() != core.Kind("Shoot") {
		return nil
	}

	if a.GetOperation() != admission.Create {
		return nil
	}

	shoot, ok := a.GetObject().(*core.Shoot)
	if !ok {
		return apierrors.NewBadRequest("could not convert resource into Shoot object")
	}

	if err := t.admitShoot(shoot); err != nil {
		return admission.NewForbidden(a, err)
	}

	return nil
}

func (t *TolerationRestriction) admitShoot(shoot *core.Shoot) error {
	project, err := gutil.ProjectForNamespaceFromInternalLister(t.projectLister, shoot.Namespace)
	if err != nil {
		return apierrors.NewBadRequest(fmt.Sprintf("could not find referenced project: %+v", err.Error()))
	}

	defaults := t.defaults
	if project.Spec.Tolerations != nil {
		defaults = append(defaults, project.Spec.Tolerations.Defaults...)
	}

	existingKeys := sets.NewString()
	for _, toleration := range shoot.Spec.Tolerations {
		existingKeys.Insert(toleration.Key)
	}

	// do not change shoot tolerations if they specify a key already
	for _, toleration := range defaults {
		if !existingKeys.Has(toleration.Key) {
			shoot.Spec.Tolerations = append(shoot.Spec.Tolerations, toleration)
		}
	}

	return nil
}

// Validate makes admissions decisions based on the allowed project tolerations or globally allowed tolerations.
func (t *TolerationRestriction) Validate(ctx context.Context, a admission.Attributes, o admission.ObjectInterfaces) error {
	if err := t.waitUntilReady(a); err != nil {
		return fmt.Errorf("err while waiting for ready %v", err)
	}

	if a.GetKind().GroupKind() != core.Kind("Shoot") {
		return nil
	}

	shoot, ok := a.GetObject().(*core.Shoot)
	if !ok {
		return apierrors.NewBadRequest("could not convert resource into Shoot object")
	}

	var oldShoot *core.Shoot
	if a.GetOperation() == admission.Update && a.GetOldObject() != nil {
		oldShoot, ok = a.GetOldObject().(*core.Shoot)
		if !ok {
			return apierrors.NewBadRequest("could not convert old resource into Shoot object")
		}
	}

	if err := t.validateShoot(shoot, oldShoot); err != nil {
		return admission.NewForbidden(a, err)
	}

	return nil
}

func (t *TolerationRestriction) validateShoot(shoot, oldShoot *core.Shoot) error {
	tolerationsToValidate := shoot.Spec.Tolerations
	if oldShoot != nil {
		tolerationsToValidate = getNewOrChangedTolerations(shoot, oldShoot)
	}

	project, err := gutil.ProjectForNamespaceFromInternalLister(t.projectLister, shoot.Namespace)
	if err != nil {
		return apierrors.NewBadRequest(fmt.Sprintf("could not find referenced project: %+v", err.Error()))
	}

	allowlist := t.allowlist
	if project.Spec.Tolerations != nil {
		allowlist = append(allowlist, project.Spec.Tolerations.Whitelist...)
	}

	if errList := corevalidation.ValidateTolerationsAgainstAllowlist(tolerationsToValidate, allowlist, field.NewPath("spec", "tolerations")); len(errList) > 0 {
		return fmt.Errorf("error while validating tolerations against allowlist: %+v", errList)
	}
	return nil
}

func getNewOrChangedTolerations(shoot, oldShoot *core.Shoot) []core.Toleration {
	var (
		oldTolerations          = sets.NewString()
		newOrChangedTolerations []core.Toleration
	)

	for _, toleration := range oldShoot.Spec.Tolerations {
		oldTolerations.Insert(utils.IDForKeyWithOptionalValue(toleration.Key, toleration.Value))
	}

	for _, toleration := range shoot.Spec.Tolerations {
		if !oldTolerations.Has(utils.IDForKeyWithOptionalValue(toleration.Key, toleration.Value)) {
			newOrChangedTolerations = append(newOrChangedTolerations, toleration)
		}
	}

	return newOrChangedTolerations
}
