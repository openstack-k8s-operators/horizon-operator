/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package v1beta1 implements webhook handlers for Horizon v1beta1 API resources.
package v1beta1

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	horizonv1beta1 "github.com/openstack-k8s-operators/horizon-operator/api/v1beta1"
)

var (
	// ErrInvalidObjectType is returned when an unexpected object type is provided
	ErrInvalidObjectType = errors.New("invalid object type")
)

// nolint:unused
// log is for logging in this package.
var horizonlog = logf.Log.WithName("horizon-resource")

// SetupHorizonWebhookWithManager registers the webhook for Horizon in the manager.
func SetupHorizonWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&horizonv1beta1.Horizon{}).
		WithValidator(&HorizonCustomValidator{}).
		WithDefaulter(&HorizonCustomDefaulter{}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-horizon-openstack-org-v1beta1-horizon,mutating=true,failurePolicy=fail,sideEffects=None,groups=horizon.openstack.org,resources=horizons,verbs=create;update,versions=v1beta1,name=mhorizon-v1beta1.kb.io,admissionReviewVersions=v1

// HorizonCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind Horizon when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type HorizonCustomDefaulter struct {
	// TODO(user): Add more fields as needed for defaulting
}

var _ webhook.CustomDefaulter = &HorizonCustomDefaulter{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind Horizon.
func (d *HorizonCustomDefaulter) Default(_ context.Context, obj runtime.Object) error {
	horizon, ok := obj.(*horizonv1beta1.Horizon)

	if !ok {
		return fmt.Errorf("expected an Horizon object but got %T: %w", obj, ErrInvalidObjectType)
	}
	horizonlog.Info("Defaulting for Horizon", "name", horizon.GetName())

	// Call the Default method on the Horizon type
	horizon.Default()

	return nil
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-horizon-openstack-org-v1beta1-horizon,mutating=false,failurePolicy=fail,sideEffects=None,groups=horizon.openstack.org,resources=horizons,verbs=create;update,versions=v1beta1,name=vhorizon-v1beta1.kb.io,admissionReviewVersions=v1

// HorizonCustomValidator struct is responsible for validating the Horizon resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type HorizonCustomValidator struct {
	// TODO(user): Add more fields as needed for validation
}

var _ webhook.CustomValidator = &HorizonCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Horizon.
func (v *HorizonCustomValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	horizon, ok := obj.(*horizonv1beta1.Horizon)
	if !ok {
		return nil, fmt.Errorf("expected a Horizon object but got %T: %w", obj, ErrInvalidObjectType)
	}
	horizonlog.Info("Validation for Horizon upon creation", "name", horizon.GetName())

	// Call the ValidateCreate method on the Horizon type
	return horizon.ValidateCreate()
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Horizon.
func (v *HorizonCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	horizon, ok := newObj.(*horizonv1beta1.Horizon)
	if !ok {
		return nil, fmt.Errorf("expected a Horizon object for the newObj but got %T: %w", newObj, ErrInvalidObjectType)
	}
	horizonlog.Info("Validation for Horizon upon update", "name", horizon.GetName())

	// Call the ValidateUpdate method on the Horizon type
	return horizon.ValidateUpdate(oldObj)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Horizon.
func (v *HorizonCustomValidator) ValidateDelete(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	horizon, ok := obj.(*horizonv1beta1.Horizon)
	if !ok {
		return nil, fmt.Errorf("expected a Horizon object but got %T: %w", obj, ErrInvalidObjectType)
	}
	horizonlog.Info("Validation for Horizon upon deletion", "name", horizon.GetName())

	// Call the ValidateDelete method on the Horizon type
	return horizon.ValidateDelete()
}
