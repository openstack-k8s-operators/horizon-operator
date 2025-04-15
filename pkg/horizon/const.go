/*

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

package horizon

import (
	"github.com/openstack-k8s-operators/lib-common/modules/storage"
)

const (
	// ServiceName -
	ServiceName = "horizon"

	// DatabaseName -
	DatabaseName = "horizon"

	// HorizonSvcPort - It maps the Svc (80) to the Container Port (8080)
	HorizonSvcPort int32 = 80

	// HorizonPort - used by httpd inside the Horizon container
	HorizonPort int32 = 8080

	// HorizonSvcPortTLS - It maps the Svc (443) to the Container Port (8443)
	HorizonSvcPortTLS int32 = 443

	// HorizonPortTLS -
	HorizonPortTLS int32 = 8443

	// HorizonPortName -
	HorizonPortName = "horizon"

	// HorizonExtraVolTypeUndefined can be used to label an extraMount which is
	// not associated to anything in particular
	HorizonExtraVolTypeUndefined storage.ExtraVolType = "Undefined"

	// Horizon is the global ServiceType that refers to all the components deployed
	// by the horizon-operator
	Horizon storage.PropagationType = "Horizon"

	// ApacheUID - apache uid inside the horizon container
	ApacheUID int64 = 48

	// KollaUID - assigned as additional group, is required by the bootstrap process
	// https://github.com/openstack/kolla/blob/master/kolla/common/users.py
	KollaUID int64 = 42400
)

// HorizonPropagation is the  definition of the Horizon propagation service
var HorizonPropagation = []storage.PropagationType{Horizon}
