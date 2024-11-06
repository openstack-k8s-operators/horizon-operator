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
	horizonv1 "github.com/openstack-k8s-operators/horizon-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/lib-common/modules/storage"
	corev1 "k8s.io/api/core/v1"
)

func getVolumes(
	name string,
	extraVol []horizonv1.HorizonExtraVolMounts,
	svc []storage.PropagationType,
) []corev1.Volume {
	//	var scriptsVolumeDefaultMode int32 = 0755
	var config0640AccessMode int32 = 0640
	var config0600AccessMode int32 = 0600
	res := []corev1.Volume{
		{
			Name: "config-data",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					DefaultMode: &config0640AccessMode,
					LocalObjectReference: corev1.LocalObjectReference{
						Name: name + "-config-data",
					},
				},
			},
		},
		{
			Name: "horizon-secret-key",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  ServiceName,
					DefaultMode: &config0600AccessMode,
				},
			},
		},
	}
	for _, exv := range extraVol {
		for _, vol := range exv.Propagate(svc) {
			for _, v := range vol.Volumes {
				volumeSource, _ := v.ToCoreVolumeSource()
				convertedVolume := corev1.Volume{
					Name:         v.Name,
					VolumeSource: *volumeSource,
				}
				res = append(res, convertedVolume)
			}
		}
	}
	return res
}

// getVolumeMounts - general VolumeMounts
func getVolumeMounts(
	extraVol []horizonv1.HorizonExtraVolMounts,
	svc []storage.PropagationType,
) []corev1.VolumeMount {
	vm := []corev1.VolumeMount{
		{
			Name:      "config-data",
			MountPath: "/var/lib/config-data/default/",
			ReadOnly:  false,
		},
		{
			Name:      "config-data",
			MountPath: "/var/lib/kolla/config_files/config.json",
			SubPath:   "horizon.json",
			ReadOnly:  true,
		},
		{
			MountPath: "/run/openstack-dashboard/.secrets",
			ReadOnly:  true,
			Name:      "horizon-secret-key",
		},
	}
	for _, exv := range extraVol {
		for _, vol := range exv.Propagate(svc) {
			vm = append(vm, vol.Mounts...)
		}
	}
	return vm
}
