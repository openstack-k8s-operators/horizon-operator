/*
Copyright 2023.

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

	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// ContainerImage - default fall-back container image for Horizon if associated env var not provided
	ContainerImage = "quay.io/podified-antelope-centos9/openstack-horizon:current-podified"
)

// HorizonSpec defines the desired state of Horizon
type HorizonSpec struct {
	// +kubebuilder:validation:Required
	// horizon Container Image URL
	ContainerImage string `json:"containerImage"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=1
	// +kubebuilder:validation:Maximum=32
	// +kubebuilder:validation:Minimum=0
	// Replicas of horizon API to run
	Replicas *int32 `json:"replicas"`

	// +kubebuilder:validation:Required
	// Secret containing OpenStack password information for Horizon Secret Key
	Secret string `json:"secret"`

	// +kubebuilder:validation:Optional
	// NodeSelector to target subset of worker nodes running this service
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// +kubebuilder:validation:Optional
	// Debug - enable debug for different deploy stages. If an init container is used, it runs and the
	// actual action pod gets started with sleep infinity
	Debug HorizonDebug `json:"debug,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	// PreserveJobs - do not delete jobs after they finished e.g. to check logs
	PreserveJobs bool `json:"preserveJobs"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default="# add your customization here"
	// CustomServiceConfig - customize the service config using this parameter to change service defaults,
	// or overwrite rendered information using raw OpenStack config format. The content gets added to
	// to /etc/openstack-dashboard/local_settings.d directory as 9999_custom_settings.py file.
	CustomServiceConfig string `json:"customServiceConfig"`

	// +kubebuilder:validation:Optional
	// ConfigOverwrite - interface to overwrite default config files like e.g. logging.conf or policy.json.
	// But can also be used to add additional files. Those get added to the service config dir in /etc/<service> .
	// TODO: -> implement
	DefaultConfigOverwrite map[string]string `json:"defaultConfigOverwrite,omitempty"`

	// +kubebuilder:validation:Optional
	// Resources - Compute Resources required by this service (Limits/Requests).
	// https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// +kubebuilder:validation:Optional
	// HorizonRoute holds all of the necessary options for configuring the Horizon Route object.
	// This can be used to configure TLS
	//TODO(bshephar) Implement everything about this. It's just a placeholder at the moment.
	Route HorizonRoute `json:"route,omitempty"`

	// +kubebuilder:validation:Required
	// +kubebuilder:default=memcached
	// Memcached instance name.
	MemcachedInstance string `json:"memcachedInstance"`
}

// HorizonDebug can be used to enable debug in the Horizon service
type HorizonDebug struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	// Service enable debug
	Service bool `json:"service"`
}

// HorizonRoute is used to define all of the information for the OpenShift route
// todo(bshephar) implement
type HorizonRoute struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=horizon
	RouteName string `json:"routeName"`

	//TODO(bshephar) We need to implement TLS handling here to secure the route
	// +kubebuilder:validation:Optional
	RouteTLSEnabled string `json:"routeTLSEnabled,omitempty"`

	//TODO(bshephar) We need to implement TLS handling here to secure the route
	// +kubebuilder:validation:Optional
	RouteTLSCA string `json:"routeTLSCA,omitempty"`

	//TODO(bshephar) We need to implement TLS handling here to secure the route
	// +kubebuilder:validation:Optional
	RouteTLSKey string `json:"routeTLSKey,omitempty"`

	//TODO(bshephar) We need to implement TLS handling here to secure the route
	// +kubebuilder:validation:Optional
	RouteLocation string `json:"routeLocation,omitempty"`
}

// HorizonStatus defines the observed state of Horizon
type HorizonStatus struct {
	// ReadyCount of Horizon instances
	ReadyCount int32 `json:"readyCount,omitempty"`

	// Map of hashes to track e.g. job status
	Hash map[string]string `json:"hash,omitempty"`

	// Endpoint url to access OpenStack Dashboard
	Endpoint string `json:"endpoint,omitempty"`

	// Conditions
	Conditions condition.Conditions `json:"conditions,omitempty" option:"true"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[0].status",description="Status"
//+kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[0].message",description="Message"

// Horizon is the Schema for the horizons API
type Horizon struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HorizonSpec   `json:"spec,omitempty"`
	Status HorizonStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// HorizonList contains a list of Horizon
type HorizonList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Horizon `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Horizon{}, &HorizonList{})
}

// GetEndpoint - Returns the OpenStack Dashboard URL
func (instance Horizon) GetEndpoint() (string, error) {
	url := instance.Status.Endpoint
	if url == "" {
		return "", fmt.Errorf("dashboard url not found")
	}
	return url, nil
}

// IsReady - returns true if Horizon is reconciled successfully
func (instance Horizon) IsReady() bool {
	return instance.Status.Conditions.IsTrue(condition.ReadyCondition)
}

// RbacConditionsSet - set the conditions for the rbac object
func (instance Horizon) RbacConditionsSet(c *condition.Condition) {
	instance.Status.Conditions.Set(c)
}

// RbacNamespace - return the namespace
func (instance Horizon) RbacNamespace() string {
	return instance.Namespace
}

// RbacResourceName - return the name to be used for rbac objects (serviceaccount, role, rolebinding)
func (instance Horizon) RbacResourceName() string {
	return "horizon-" + instance.Name
}

// SetupDefaults - initializes any CRD field defaults based on environment variables (the defaulting mechanism itself is implemented via webhooks)
func SetupDefaults() {
	// Acquire environmental defaults and initialize Horizon defaults with them
	horizonDefaults := HorizonDefaults{
		ContainerImageURL: util.GetEnvVar("HORIZON_IMAGE_URL_DEFAULT", ContainerImage),
	}

	SetupHorizonDefaults(horizonDefaults)
}
