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
	"fmt"

	horizonv1 "github.com/openstack-k8s-operators/horizon-operator/api/v1beta1"
	memcachedv1 "github.com/openstack-k8s-operators/infra-operator/apis/memcached/v1beta1"
	topologyv1 "github.com/openstack-k8s-operators/infra-operator/apis/topology/v1beta1"
	common "github.com/openstack-k8s-operators/lib-common/modules/common"
	"github.com/openstack-k8s-operators/lib-common/modules/common/affinity"
	env "github.com/openstack-k8s-operators/lib-common/modules/common/env"
	"github.com/openstack-k8s-operators/lib-common/modules/common/pod"
	"github.com/openstack-k8s-operators/lib-common/modules/common/tls"
	"github.com/openstack-k8s-operators/lib-common/modules/users"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

const (
	// ThemeSetupCommand unpacks any *.tar.gz found under the theme
	// ExtraMount into the themes EmptyDir, after seeding it with the
	// image's own baked-in stock themes. theme_setup itself is unchanged --
	// it already takes source/target dir args (it was never actually an
	// image-baked kolla script despite its old "kolla_theme_setup" name --
	// it's always lived in this repo's own templates/horizon/bin/, shipped
	// via the "scripts" ConfigMap).
	//
	// Plain "cp -r", not "cp -a": the trailing-dot form ("src/. dest/")
	// also tries to apply the source directory's own attributes to the
	// pre-existing destination directory (the EmptyDir mount root, owned
	// by root). Under "-a" (--preserve=all) that includes an attempted
	// utime()/chown() on a directory this non-root, capability-dropped
	// container doesn't own, which fails with EPERM. Attribute
	// preservation isn't needed here anyway -- this only seeds a
	// throwaway runtime copy of static theme assets.
	ThemeSetupCommand = "cp -r " + ThemesFinalPath + "/. " + ThemesSeedPath +
		"/ && /usr/local/bin/theme_setup /etc/openstack-dashboard/theme " + ThemesSeedPath
	// SecretSetupCommand re-materializes the horizon secret key into a
	// pod-owned EmptyDir with real 0600 permissions. Kubernetes always
	// mounts Secret-backed files as root-owned, and forces at least
	// group-read (0640) onto them so the pod's FSGroup can read them at
	// all -- horizon's own secret_key.read_from_file() rejects that as
	// "insecure permissions", requiring exactly 0600. Since a file this
	// init container creates itself is owned by its own UID (not root),
	// plain owner-only 0600 works without needing group access.
	//
	// Deliberately kept as its own, separate init container rather than
	// merged into dashboard-setup: writes through
	// getSecretSeedVolumeMount() (a whole-directory mount, no SubPath) so
	// there's no missing-path ambiguity for kubelet to resolve, unlike
	// GetHorizonSecretVolumeMount() (see its doc comment) -- a SubPath
	// mount of a not-yet-existing path gets auto-created as a directory,
	// permanently, for that container's lifetime, confirmed on a real
	// cluster ("cat > <path>" failing "Is a directory", and every earlier
	// "insecure permissions" failure was the same root cause under a
	// different symptom). Merging this into dashboard-setup was tried
	// first (a single trivial command, seemingly no reason not to) but
	// doesn't work: dashboard-setup's own SubPath mount of the final path
	// is established by kubelet at ITS OWN container start, before any
	// command in it runs, so writing the file from within the same
	// container can never happen soon enough. The file must already exist
	// before the first container referencing it via SubPath even starts --
	// requiring a genuinely separate, earlier-running container. Must run
	// before both dashboard-setup and the main container.
	SecretSetupCommand = "cat /var/lib/openstack/secret-src/horizon-secret > " +
		SecretSeedPath + "/horizon-secret && chmod 0600 " + SecretSeedPath + "/horizon-secret"
	// DashboardSetupCommand runs dashboard_setup
	// (templates/horizon/bin/dashboard_setup, an operator-owned, trimmed
	// adaptation of tcib's kolla_extend_start script -- brought in-repo, like
	// theme_setup already was, so this operator doesn't depend on tcib
	// continuing to ship kolla_extend_start in the image once kolla_start
	// itself is gone everywhere). It does two things this migration must not
	// silently drop: enables/disables per-service dashboard panels
	// (Designate/Heat/Ironic/Manila/Octavia/Watcher/CloudKitty) based on the
	// ENABLE_* env vars set in getEnvVars(), and regenerates Horizon's
	// compiled/minified static assets ("manage.py collectstatic"+"compress")
	// whenever a panel actually changes -- which, since dashboard-enabled
	// always starts empty, is every pod start for every always-on service.
	// Relies on secret-setup (see SecretSetupCommand) having already run:
	// Django settings load calls secret_key.read_from_file() the moment
	// manage.py starts, against GetHorizonSecretVolumeMount()'s SubPath
	// mount of the same real final path.
	DashboardSetupCommand    = "/usr/local/bin/container-scripts/dashboard_setup"
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
	memcached *memcachedv1.Memcached,
) (*appsv1.Deployment, error) {

	containerPort := corev1.ContainerPort{
		Name:          horizonContainerPortName,
		Protocol:      corev1.ProtocolTCP,
		ContainerPort: HorizonPort,
	}

	livenessProbe := formatProbes()
	readinessProbe := formatProbes()
	startupProbe := formatStartupProbe()

	envVars := getEnvVars(configHash, enabledServices)

	// create Volumes and VolumeMounts
	volumes := append(getVolumes(instance.Name, instance.Spec.ExtraMounts, HorizonPropagation),
		GetLogVolume(), GetRunHttpdVolume(), GetVarLogHttpdVolume(), GetThemesVolume(),
		GetDashboardEnabledVolume(), GetDashboardStaticVolume())
	volumeMounts := append(getVolumeMounts(instance.Spec.ExtraMounts, HorizonPropagation),
		GetLogVolumeMount(), GetRunHttpdVolumeMount(), GetVarLogHttpdVolumeMount(), GetThemesVolumeMount(),
		GetDashboardEnabledVolumeMount(), GetDashboardStaticVolumeMount())
	themeSetupVolumeMounts := getThemeSetupVolumeMounts(instance.Spec.ExtraMounts, HorizonPropagation)

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

	// add MTLS cert if defined
	if memcached.GetMemcachedMTLSSecret() != "" {
		certMountPath := memcachedv1.CertPathDst
		keyMountPath := memcachedv1.KeyPathDst
		volumes = append(volumes, memcached.CreateMTLSVolume())
		volumeMounts = append(volumeMounts, memcached.CreateMTLSVolumeMounts(&certMountPath, &keyMountPath)...)
	}

	// dashboard-setup needs everything the main container has (config,
	// TLS/mTLS certs, dashboard-enabled/dashboard-static, and the
	// final-path themes mount so collectstatic sees exactly what httpd will
	// serve) plus the scripts dir (to exec dashboard_setup), its own
	// settings-hash scratch mount -- so it's built from a copy of the
	// now-fully-assembled volumeMounts rather than reconstructed from
	// scratch. Its inherited horizon-secret mount (GetHorizonSecretVolumeMount(),
	// read-only) is kept as-is here, not swapped for a writable one:
	// dashboard-setup only ever reads .horizon-secret (manage.py loading
	// Django settings), it never writes it -- secret-setup, a separate,
	// earlier-running init container, owns writing it. See
	// GetHorizonSecretVolumeMount()'s doc comment for why that write can't
	// happen from within dashboard-setup itself.
	dashboardSetupVolumeMounts := append(append([]corev1.VolumeMount{}, volumeMounts...),
		getScriptsDirVolumeMount(), GetSettingsHashVolumeMount())
	volumes = append(volumes, GetSettingsHashVolume())

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
					ServiceAccountName:           instance.RbacResourceName(),
					AutomountServiceAccountToken: ptr.To(false),
					// horizon's kolla-based httpd workers always ran as
					// apache, never under a dedicated "horizon" system
					// user (confirmed via `ps -ef` on a running kolla
					// pod) -- so apache (ApacheUID) is used here as the
					// primary user identity, with HorizonGID as the run
					// and fs group, to preserve read/write compatibility
					// with any pre-existing ExtraMount content (NFS,
					// pre-provisioned PVCs) still owned by apache from
					// before this migration.
					SecurityContext: pod.RestrictivePodSecurityContext(users.ApacheUID, users.HorizonGID),
					InitContainers: []corev1.Container{
						{
							Name: "theme-setup",
							Command: []string{
								"/bin/bash",
							},
							Args:            []string{"-c", ThemeSetupCommand},
							Image:           instance.Spec.ContainerImage,
							SecurityContext: pod.RestrictiveSecurityContext(users.ApacheUID, users.HorizonGID),
							VolumeMounts:    themeSetupVolumeMounts,
						},
						{
							// must run before dashboard-setup and the main
							// container: both reference .horizon-secret via
							// a SubPath mount, which requires the file to
							// already exist as a regular file by the time
							// they start -- see GetHorizonSecretVolumeMount()
							Name: "secret-setup",
							Command: []string{
								"/bin/bash",
							},
							Args:            []string{"-c", SecretSetupCommand},
							Image:           instance.Spec.ContainerImage,
							SecurityContext: pod.RestrictiveSecurityContext(users.ApacheUID, users.HorizonGID),
							VolumeMounts:    []corev1.VolumeMount{getSecretSrcVolumeMount(), getSecretSeedVolumeMount()},
						},
						{
							// must run after theme-setup (collectstatic
							// needs to see the final, fully-seeded theme
							// state, not the original image directory) and
							// after secret-setup (Django settings load
							// reads .horizon-secret before manage.py can
							// run anything)
							Name: "dashboard-setup",
							Command: []string{
								"/bin/bash",
							},
							Args:            []string{"-c", DashboardSetupCommand},
							Image:           instance.Spec.ContainerImage,
							SecurityContext: pod.RestrictiveSecurityContext(users.ApacheUID, users.HorizonGID),
							Env:             env.MergeEnvs([]corev1.EnvVar{}, envVars),
							VolumeMounts:    dashboardSetupVolumeMounts,
						},
					},
					Containers: []corev1.Container{
						// the first container in a pod is the default selected
						// by oc log so define the log stream container first.
						{
							Name: instance.Name + "-log",
							Command: []string{
								"/bin/bash",
							},
							Args:            []string{"-c", "tail -n+1 -F " + LogFile},
							Image:           instance.Spec.ContainerImage,
							SecurityContext: pod.RestrictiveSecurityContext(users.ApacheUID, users.HorizonGID),
							Env:             env.MergeEnvs([]corev1.EnvVar{}, envVars),
							VolumeMounts:    []corev1.VolumeMount{GetLogVolumeMount()},
							Resources:       instance.Spec.Resources,
						},
						{
							Name: ServiceName,
							Command: []string{
								"/usr/sbin/httpd",
							},
							Args:            []string{"-DFOREGROUND"},
							Image:           instance.Spec.ContainerImage,
							SecurityContext: pod.RestrictiveSecurityContext(users.ApacheUID, users.HorizonGID),
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

	envVars["ENABLE_DESIGNATE"] = env.SetValue("yes")
	envVars["ENABLE_HEAT"] = env.SetValue("yes")
	envVars["ENABLE_IRONIC"] = env.SetValue("yes")
	envVars["ENABLE_MANILA"] = env.SetValue("yes")
	envVars["ENABLE_OCTAVIA"] = env.SetValue("yes")
	envVars["ENABLE_CLOUDKITTY"] = env.SetValue(enabledServices["cloudkitty"])
	envVars["ENABLE_WATCHER"] = env.SetValue(enabledServices["watcher"])
	envVars["CONFIG_HASH"] = env.SetValue(configHash)

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

func formatStartupProbe() *corev1.Probe {

	return &corev1.Probe{
		TimeoutSeconds:   5,
		PeriodSeconds:    10,
		FailureThreshold: 12,
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

	svc, err = instance.Spec.TLS.ToService()
	if err != nil {
		return err
	}

	// add CA cert if defined
	if instance.Spec.TLS.CaBundleSecretName != "" {
		t.volumes = append(t.volumes, instance.Spec.TLS.CreateVolume())
		t.volumeMounts = append(t.volumeMounts, instance.Spec.TLS.CreateVolumeMounts(nil)...)
	}

	certMount := fmt.Sprintf("/etc/pki/tls/certs/%s.crt", ServiceName)
	keyMount := fmt.Sprintf("/etc/pki/tls/private/%s.key", ServiceName)
	svc.CertMount = &certMount
	svc.KeyMount = &keyMount

	t.containerPort.ContainerPort = HorizonPortTLS
	t.livenessProbe.HTTPGet.Scheme = corev1.URISchemeHTTPS
	t.readinessProbe.HTTPGet.Scheme = corev1.URISchemeHTTPS
	t.startupProbe.HTTPGet.Scheme = corev1.URISchemeHTTPS
	t.volumes = append(t.volumes, svc.CreateVolume(ServiceName))
	t.volumeMounts = append(t.volumeMounts, svc.CreateVolumeMounts(ServiceName)...)

	return nil
}
