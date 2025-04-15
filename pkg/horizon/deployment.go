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
	topologyv1 "github.com/openstack-k8s-operators/infra-operator/apis/topology/v1beta1"
	common "github.com/openstack-k8s-operators/lib-common/modules/common"
	"github.com/openstack-k8s-operators/lib-common/modules/common/affinity"
	env "github.com/openstack-k8s-operators/lib-common/modules/common/env"
	"github.com/openstack-k8s-operators/lib-common/modules/common/tls"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	// ServiceCommand is the command used to run Kolla and launch the initial Apache process
	ServiceCommand           = "/usr/local/bin/kolla_theme_setup && /usr/local/bin/kolla_start"
	horizonDashboardURL      = "/dashboard/auth/login/?next=/dashboard/"
	horizonContainerPortName = "horizon"
)

// TLSRequiredOptions -
type TLSRequiredOptions struct {
	containerPort  *corev1.ContainerPort
	livenessProbe  *corev1.Probe
	readinessProbe *corev1.Probe
	startupProbe   *corev1.Probe
	volumes        []corev1.Volume
	volumeMounts   []corev1.VolumeMount
}

// Deployment creates the k8s deployment structure required to run Horizon
func Deployment(
	instance *horizonv1.Horizon,
	configHash string,
	labels map[string]string,
	annotations map[string]string,
	enabledServices map[string]string,
	topology *topologyv1.Topology,
) (*appsv1.Deployment, error) {

	args := []string{"-c", ServiceCommand}

	containerPort := corev1.ContainerPort{
		Name:          horizonContainerPortName,
		Protocol:      corev1.ProtocolTCP,
		ContainerPort: HorizonPort,
	}

	livenessProbe := formatProbes()
	readinessProbe := formatProbes()
	startupProbe := formatProbes()

	envVars := getEnvVars(configHash, enabledServices)

	// create Volumes and VolumeMounts
	volumes := getVolumes(instance.Name, instance.Spec.ExtraMounts, HorizonPropagation)
	volumeMounts := getVolumeMounts(instance.Spec.ExtraMounts, HorizonPropagation)

	if instance.Spec.TLS.Enabled() {
		tlsRequiredOptions := TLSRequiredOptions{
			&containerPort,
			livenessProbe,
			readinessProbe,
			startupProbe,
			volumes,
			volumeMounts,
		}

		err := tlsRequiredOptions.formatTLSOptions(instance)
		if err != nil {
			return nil, err
		}
		volumes, volumeMounts = tlsRequiredOptions.volumes, tlsRequiredOptions.volumeMounts
		livenessProbe = tlsRequiredOptions.livenessProbe
		readinessProbe = tlsRequiredOptions.readinessProbe
		startupProbe = tlsRequiredOptions.startupProbe
		containerPort = *tlsRequiredOptions.containerPort
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ServiceName,
			Namespace: instance.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Replicas: instance.Spec.Replicas,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: annotations,
					Labels:      labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: instance.RbacResourceName(),
					Containers: []corev1.Container{
						{
							Name: ServiceName,
							Command: []string{
								"/bin/bash"},
							Args:            args,
							Image:           instance.Spec.ContainerImage,
							SecurityContext: HttpdSecurityContext(),
							Env:             env.MergeEnvs([]corev1.EnvVar{}, envVars),
							VolumeMounts:    volumeMounts,
							Resources:       instance.Spec.Resources,
							ReadinessProbe:  readinessProbe,
							LivenessProbe:   livenessProbe,
							StartupProbe:    startupProbe,
							Ports:           []corev1.ContainerPort{containerPort},
						},
					},
					Volumes: volumes,
				},
			},
		},
	}

	if instance.Spec.NodeSelector != nil {
		deployment.Spec.Template.Spec.NodeSelector = *instance.Spec.NodeSelector
	}

	if topology != nil {
		topology.ApplyTo(&deployment.Spec.Template)
	} else {
		// If possible two pods of the same service should not
		// run on the same worker node. If this is not possible
		// the get still created on the same worker node.
		deployment.Spec.Template.Spec.Affinity = affinity.DistributePods(
			common.AppSelector,
			[]string{
				ServiceName,
			},
			corev1.LabelHostname,
		)
	}

	return deployment, nil
}

func getEnvVars(configHash string, enabledServices map[string]string) map[string]env.Setter {

	envVars := map[string]env.Setter{}

	envVars["KOLLA_CONFIG_STRATEGY"] = env.SetValue("COPY_ALWAYS")
	envVars["ENABLE_DESIGNATE"] = env.SetValue("yes")
	envVars["ENABLE_HEAT"] = env.SetValue("yes")
	envVars["ENABLE_IRONIC"] = env.SetValue("yes")
	envVars["ENABLE_MANILA"] = env.SetValue("yes")
	envVars["ENABLE_OCTAVIA"] = env.SetValue("yes")
	envVars["ENABLE_WATCHER"] = env.SetValue(enabledServices["watcher"])
	envVars["CONFIG_HASH"] = env.SetValue(configHash)
	envVars["UNPACK_THEME"] = env.SetValue("true")

	return envVars
}

func formatProbes() *corev1.Probe {

	return &corev1.Probe{
		TimeoutSeconds:      5,
		PeriodSeconds:       10,
		InitialDelaySeconds: 10,
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: horizonDashboardURL,
				Port: intstr.FromString(horizonContainerPortName),
			},
		},
	}
}

func (t *TLSRequiredOptions) formatTLSOptions(instance *horizonv1.Horizon) error {

	var err error
	var svc *tls.Service

	svc, err = instance.Spec.TLS.GenericService.ToService()
	if err != nil {
		return err
	}

	// add CA cert if defined
	if instance.Spec.TLS.CaBundleSecretName != "" {
		t.volumes = append(t.volumes, instance.Spec.TLS.CreateVolume())
		t.volumeMounts = append(t.volumeMounts, instance.Spec.TLS.CreateVolumeMounts(nil)...)
	}

	t.containerPort.ContainerPort = HorizonPortTLS
	t.livenessProbe.HTTPGet.Scheme = corev1.URISchemeHTTPS
	t.readinessProbe.HTTPGet.Scheme = corev1.URISchemeHTTPS
	t.startupProbe.HTTPGet.Scheme = corev1.URISchemeHTTPS
	t.volumes = append(t.volumes, svc.CreateVolume(ServiceName))
	t.volumeMounts = append(t.volumeMounts, svc.CreateVolumeMounts(ServiceName)...)

	return nil
}
