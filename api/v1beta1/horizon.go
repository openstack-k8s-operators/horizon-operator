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

package v1beta1

import (
	"fmt"
	"reflect"
	"context"

	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	appsv1 "k8s.io/api/apps/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"

)

// HorizonEndpointChangedPredicate - primary purpose is to return true if
// the Horizon Status.Endpoints has changed (e.g. it has been set)
// In addition also returns true if it gets deleted (it helps to react to
// a CR deletion event). This predicate is used to watch the Horizon endpoint
// from other services that need to set this value in their configuration
var HorizonEndpointChangedPredicate = predicate.Funcs{
	UpdateFunc: func(e event.UpdateEvent) bool {
		if e.ObjectOld == nil || e.ObjectNew == nil {
			return false
		}
		oldPod, okOld := e.ObjectOld.(*Horizon)
		newPod, okNew := e.ObjectNew.(*Horizon)

		if !okOld || !okNew {
			return false
		}

		// Compare the Endpoint Status fields of the old and new .Status.Endpoint
		epIsDifferent := !reflect.DeepEqual(oldPod.Status.Endpoint, newPod.Status.Endpoint)
		return epIsDifferent
	},
	DeleteFunc: func(_ event.DeleteEvent) bool {
		// By default, we might want to react to deletions
		return true
	},
}

// GetHorizon - Get Horizon CR in the namespace passed as input. It lists the
// Items deployed in the current namespace and return the Horizon object if
// it exists, else an error
func GetHorizon(
	ctx context.Context,
	h *helper.Helper,
	namespace string,
) (*Horizon, error) {
	horizonList := &HorizonList{}

	listOpts := []client.ListOption{
		client.InNamespace(namespace),
	}

	err := h.GetClient().List(ctx, horizonList, listOpts...)
	if err != nil {
		return nil, err
	}

	if len(horizonList.Items) > 1 {
		return nil, fmt.Errorf("More then one Horizon object found in namespace %s", namespace)
	}

	if len(horizonList.Items) == 0 {
		return nil, k8s_errors.NewNotFound(
			appsv1.Resource("Horizon"),
			fmt.Sprintf("No Horizon object found in namespace %s", namespace),
		)
	}
	return &horizonList.Items[0], nil
}
