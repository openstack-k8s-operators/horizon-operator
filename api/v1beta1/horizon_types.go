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
	"strings"

	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/service"
	topologyv1 "github.com/openstack-k8s-operators/infra-operator/apis/topology/v1beta1"
	"github.com/openstack-k8s-operators/lib-common/modules/common/tls"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	"github.com/openstack-k8s-operators/lib-common/modules/storage"
	"k8s.io/apimachinery/pkg/util/validation/field"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// ContainerImage - default fall-back container image for Horizon if associated env var not provided
	ContainerImage = "quay.io/podified-antelope-centos9/openstack-horizon:current-podified"
	// HorizonCustomThemeMountPath -
	HorizonCustomThemeMountPath = "/etc/openstack-dashboard/theme"
	// HorizonCustomThemeSetting -
	HorizonCustomThemeSetting = "/etc/openstack-dashboard/local_settings.d"
	// HorizonThemeExtraVolType -
	HorizonThemeExtraVolType = "theme"
)

// HorizonSpec defines the desired state of Horizon
type HorizonSpec struct {
	// +kubebuilder:validation:Required
	// horizon Container Image URL
	ContainerImage string `json:"containerImage"`

	HorizonSpecCore `json:",inline"`
}

// HorizonSpecCore -
type HorizonSpecCore struct {
	// +kubebuilder:validation:Optional
	// NodeSelector to target subset of worker nodes running this service
	NodeSelector *map[string]string `json:"nodeSelector,omitempty"`

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
	// Override, provides the ability to override the generated manifest of several child resources.
	Override HorizionOverrideSpec `json:"override,omitempty"`

	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// TLS - Parameters related to the TLS
	TLS tls.SimpleService `json:"tls,omitempty"`

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
	// +kubebuilder:default="# add your customization here"
	// CustomServiceConfig - customize the service config using this parameter to change service defaults,
	// or overwrite rendered information using raw OpenStack config format. The content gets added to
	// to /etc/openstack-dashboard/local_settings.d directory as 9999_custom_settings.py file.
	CustomServiceConfig string `json:"customServiceConfig"`

	// +kubebuilder:validation:Required
	// +kubebuilder:default=memcached
	// Memcached instance name.
	MemcachedInstance string `json:"memcachedInstance"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	// PreserveJobs - do not delete jobs after they finished e.g. to check logs
	PreserveJobs bool `json:"preserveJobs"`

	// ExtraMounts containing conf files
	// +kubebuilder:default={}
	ExtraMounts []HorizonExtraVolMounts `json:"extraMounts,omitempty"`

	// +kubebuilder:validation:Optional
	// NetworkAttachments is a list of NetworkAttachment resource names to expose the services to the given network
	NetworkAttachments []string `json:"networkAttachments,omitempty"`

	// +kubebuilder:validation:Optional
	// TopologyRef to apply the Topology defined by the associated CR referenced
	// by name
	TopologyRef *topologyv1.TopoRef `json:"topologyRef,omitempty"`
}

// HorizionOverrideSpec to override the generated manifest of several child resources.
type HorizionOverrideSpec struct {
	// Override configuration for the Service created to serve traffic to the cluster.
	Service *service.RoutedOverrideSpec `json:"service,omitempty"`
}

// HorizonStatus defines the observed state of Horizon
type HorizonStatus struct {
	// Map of hashes to track e.g. job status
	Hash map[string]string `json:"hash,omitempty"`

	// Endpoint url to access OpenStack Dashboard
	Endpoint string `json:"endpoint,omitempty"`

	// Conditions
	Conditions condition.Conditions `json:"conditions,omitempty" option:"true"`

	// ReadyCount of Horizon instances
	ReadyCount int32 `json:"readyCount,omitempty"`

	// ObservedGeneration - the most recent generation observed for this
	// service. If the observed generation is less than the spec generation,
	// then the controller has not processed the latest changes injected by
	// the opentack-operator in the top-level CR (e.g. the ContainerImage)
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// NetworkAttachments status of the deployment pods
	NetworkAttachments map[string][]string `json:"networkAttachments,omitempty"`

	// LastAppliedTopology - the last applied Topology
	LastAppliedTopology *topologyv1.TopoRef `json:"lastAppliedTopology,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="NetworkAttachments",type="string",JSONPath=".status.networkAttachments",description="NetworkAttachments"
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
		ContainerImageURL: util.GetEnvVar("RELATED_IMAGE_HORIZON_IMAGE_URL_DEFAULT", ContainerImage),
	}

	SetupHorizonDefaults(horizonDefaults)
}

// HorizonExtraVolMounts exposes additional parameters processed by the horizon-operator
// and defines the common VolMounts structure provided by the main storage module
type HorizonExtraVolMounts struct {
	// +kubebuilder:validation:Optional
	Name string `json:"name,omitempty"`
	// +kubebuilder:validation:Optional
	Region string `json:"region,omitempty"`
	// +kubebuilder:validation:Required
	VolMounts []storage.VolMounts `json:"extraVol"`
}

// Propagate is a function used to filter VolMounts according to the specified
// PropagationType array
func (c *HorizonExtraVolMounts) Propagate(svc []storage.PropagationType) []storage.VolMounts {
	var vl []storage.VolMounts
	for _, gv := range c.VolMounts {
		var extraMountType string = fmt.Sprintf("%s", string(gv.ExtraVolType))
		if strings.Contains(strings.ToLower(extraMountType), HorizonThemeExtraVolType) {
			// Ignore an invalid path that does not match with
			// HorizonCustomThemeMountPath
			if ok := c.ValidateThemeExtraMountPath(gv.Mounts); ok {
				vl = append(vl, gv.Propagate(svc)...)
			}
		} else {
			vl = append(vl, gv.Propagate(svc)...)
		}
	}
	return vl
}

// ValidateThemeExtraMountPath -
func (c *HorizonExtraVolMounts) ValidateThemeExtraMountPath(volMount []corev1.VolumeMount) bool {
	for _, m := range volMount {
		// if at least one entry is not valid, ignore the extraMount
		if !strings.Contains(m.MountPath, HorizonCustomThemeMountPath) &&
			!strings.Contains(m.MountPath, HorizonCustomThemeSetting) {
				return false
		}
	}
	return true
}

// ValidateTopology -
func (instance *HorizonSpecCore) ValidateTopology(
	basePath *field.Path,
	namespace string,
) field.ErrorList {
	var allErrs field.ErrorList
	allErrs = append(allErrs, topologyv1.ValidateTopologyRef(
		instance.TopologyRef,
		*basePath.Child("topologyRef"), namespace)...)
	return allErrs
}
