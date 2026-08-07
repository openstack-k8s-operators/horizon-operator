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
	"github.com/openstack-k8s-operators/lib-common/modules/common/volume"
	"github.com/openstack-k8s-operators/lib-common/modules/storage"
	corev1 "k8s.io/api/core/v1"
)

func getVolumes(
	name string,
	extraVol []horizonv1.HorizonExtraVolMounts,
	svc []storage.PropagationType,
) []corev1.Volume {
	var config0440AccessMode int32 = 0440
	var config0600AccessMode int32 = 0600
	res := []corev1.Volume{
		{
			Name: "config-data",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					DefaultMode: &config0440AccessMode,
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
		// seeded by dashboard-setup, read by the main container at its
		// final path
		GetHorizonSecretVolume(),
		// needed by the theme-setup init container only, but kept in the
		// shared pod-level volume list alongside everything else
		getScriptVolume(),
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

// getVolumeMounts - general VolumeMounts for the main httpd container. The
// merge/staging pattern kolla used is gone -- each file is mounted directly
// at its final destination via SubPath from the same "config-data"
// ConfigMap.
func getVolumeMounts(
	extraVol []horizonv1.HorizonExtraVolMounts,
	svc []storage.PropagationType,
) []corev1.VolumeMount {
	vm := []corev1.VolumeMount{
		{
			Name:      "config-data",
			MountPath: "/etc/httpd/conf/httpd.conf",
			SubPath:   "httpd.conf",
			ReadOnly:  true,
		},
		{
			Name:      "config-data",
			MountPath: "/etc/httpd/conf.d/ssl.conf",
			SubPath:   "ssl.conf",
			ReadOnly:  true,
		},
		{
			Name:      "config-data",
			MountPath: "/etc/openstack-dashboard/local_settings",
			SubPath:   "local_settings.py",
			ReadOnly:  true,
		},
		{
			Name:      "config-data",
			MountPath: "/etc/openstack-dashboard/local_settings.d/9999_custom_settings.py",
			SubPath:   "9999_custom_settings.py",
			ReadOnly:  true,
		},
		GetHorizonSecretVolumeMount(),
	}
	for _, exv := range extraVol {
		for _, vol := range exv.Propagate(svc) {
			vm = append(vm, vol.Mounts...)
		}
	}
	return vm
}

// getThemeSetupVolumeMounts - VolumeMounts for the theme-setup init
// container: the scripts (theme_setup, unchanged), every ExtraMount
// (needed so a theme-type source -- Secret/ConfigMap/NFS/etc. -- is
// readable here too), and the "themes" EmptyDir mounted at a temporary seed
// path rather than the final destination, so this container still sees the
// image's own baked-in theme directory, un-shadowed by any mount.
func getThemeSetupVolumeMounts(
	extraVol []horizonv1.HorizonExtraVolMounts,
	svc []storage.PropagationType,
) []corev1.VolumeMount {
	vm := getScriptVolumeMount()
	vm = append(vm, corev1.VolumeMount{
		Name:      "themes",
		MountPath: ThemesSeedPath,
	})
	for _, exv := range extraVol {
		for _, vol := range exv.Propagate(svc) {
			vm = append(vm, vol.Mounts...)
		}
	}
	return vm
}

// GetHorizonSecretVolume - EmptyDir the secret-setup init container seeds
// with a 0600, pod-owned copy of the horizon secret key. Kubernetes Secret
// volumes are always root-owned, and FSGroup-based group access forces at
// least group-read (0640) onto the mounted file -- horizon's own
// secret_key.read_from_file() rejects that outright ("should be 0600"). A
// freshly created file in this EmptyDir is owned by whichever container
// created it (the init container's own UID), so owner-only 0600 works
// without needing group access at all.
func GetHorizonSecretVolume() corev1.Volume {
	return volume.WritableDirVolume("horizon-secret")
}

// GetHorizonSecretVolumeMount - the main container's (and dashboard-setup's)
// read-only mount of the seeded secret at its real, final path, via SubPath.
//
// This MUST NOT be the first mount of this path in the pod. A SubPath mount
// is a bind mount of one specific path inside the volume; if that path
// doesn't exist yet when a container referencing it starts, kubelet
// auto-creates it -- as a DIRECTORY, not a file -- to satisfy the bind
// mount, permanently, for that container's lifetime (confirmed on a real
// cluster: `cat > path` failed with "Is a directory", and every earlier
// "insecure permissions" failure was actually the same root cause -- a
// kubelet-created directory's mode is never 0600). The fix isn't anything
// this mount itself can do: `secret-setup` (see SecretSetupCommand) writes
// the real file into this same EmptyDir first, through a *whole-directory*
// mount (getSecretSeedVolumeMount(), no SubPath, so there's no missing-path
// ambiguity to auto-create anything for) -- as long as that container
// finishes before any container referencing this SubPath mount starts,
// kubelet finds the target already exists as a regular file and binds to
// it directly. `secret-setup` must therefore run before both `dashboard-setup`
// and the main container.
func GetHorizonSecretVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      "horizon-secret",
		MountPath: "/etc/openstack-dashboard/.horizon-secret",
		SubPath:   "horizon-secret",
		ReadOnly:  true,
	}
}

// getSecretSrcVolumeMount - secret-setup's read-only mount of the original
// Secret-backed key (whatever mode Kubernetes actually applies).
func getSecretSrcVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      "horizon-secret-key",
		MountPath: "/var/lib/openstack/secret-src/horizon-secret",
		SubPath:   "horizon-secret",
		ReadOnly:  true,
	}
}

// SecretSeedPath is secret-setup's whole-directory (no SubPath) mount point
// of the "horizon-secret" EmptyDir -- deliberately not the final path, and
// deliberately not via SubPath, so writing the key file here never risks
// the SubPath-auto-creates-a-directory problem documented on
// GetHorizonSecretVolumeMount(): mounting the whole, already-existing
// EmptyDir root has no missing-path ambiguity for kubelet to resolve.
const SecretSeedPath = "/var/lib/openstack/secret-seed" //nolint:gosec

// getSecretSeedVolumeMount - see SecretSeedPath.
func getSecretSeedVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      "horizon-secret",
		MountPath: SecretSeedPath,
	}
}

// getScriptsDirVolumeMount - the generic scripts ConfigMap directory mount,
// shared by any init container that needs to exec a script from it.
func getScriptsDirVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      "scripts",
		MountPath: "/usr/local/bin/container-scripts",
		ReadOnly:  true,
	}
}

// getScriptVolumeMount - theme-setup's mounts: the generic scripts
// directory plus a dedicated theme_setup alias.
func getScriptVolumeMount() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		getScriptsDirVolumeMount(),
		{
			Name:      "scripts",
			MountPath: "/usr/local/bin/theme_setup",
			ReadOnly:  true,
			SubPath:   "theme_setup",
		},
	}
}

// getScriptVolume -
func getScriptVolume() corev1.Volume {
	var scriptsVolumeDefaultMode int32 = 0755
	return corev1.Volume{
		Name: "scripts",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				DefaultMode: &scriptsVolumeDefaultMode,
				LocalObjectReference: corev1.LocalObjectReference{
					Name: ServiceName + "-scripts",
				},
			},
		},
	}
}

// GetLogVolumeMount - Horizon API LogVolumeMount
func GetLogVolumeMount() corev1.VolumeMount {
	return volume.WritableDirVolumeMount(logVolume, "/var/log/horizon")
}

// GetLogVolume - Horizon API LogVolume
func GetLogVolume() corev1.Volume {
	return volume.WritableDirVolume(logVolume)
}

// ThemesSeedPath is where the theme-setup init container seeds the
// "themes" EmptyDir -- deliberately not the final destination, so the
// init container still sees the image's own baked-in theme directory
// un-shadowed by the EmptyDir mount.
const ThemesSeedPath = "/var/lib/openstack/themes-seed"

// ThemesFinalPath is where the main httpd container mounts the same
// "themes" EmptyDir, replacing the image's baked-in (read-only, from this
// container's perspective) theme directory with the seeded copy assembled
// by the init container.
const ThemesFinalPath = "/usr/share/openstack-dashboard/openstack_dashboard/themes"

// GetThemesVolume - EmptyDir shared between the theme-setup init container
// and the main httpd container.
func GetThemesVolume() corev1.Volume {
	return volume.WritableDirVolume("themes")
}

// GetThemesVolumeMount - the main container's mount of the "themes"
// EmptyDir at its real, final path.
func GetThemesVolumeMount() corev1.VolumeMount {
	return volume.WritableDirVolumeMount("themes", ThemesFinalPath)
}

// GetRunHttpdVolume - EmptyDir for httpd's PID file directory, needed once
// httpd runs as a non-root, FSGroup-only user (kolla used to chown
// /run/httpd at startup).
func GetRunHttpdVolume() corev1.Volume {
	return volume.WritableDirVolume(volume.RunHttpdVolumeName)
}

// GetRunHttpdVolumeMount -
func GetRunHttpdVolumeMount() corev1.VolumeMount {
	return volume.WritableDirVolumeMount(volume.RunHttpdVolumeName, volume.RunHttpdMountPath)
}

// GetVarLogHttpdVolume - EmptyDir for httpd's own logs, defense-in-depth
// for any RPM-shipped conf.d file that references a relative "logs/*" path.
func GetVarLogHttpdVolume() corev1.Volume {
	return volume.WritableDirVolume(volume.VarLogHttpdVolumeName)
}

// GetVarLogHttpdVolumeMount -
func GetVarLogHttpdVolumeMount() corev1.VolumeMount {
	return volume.WritableDirVolumeMount(volume.VarLogHttpdVolumeName, volume.VarLogHttpdMountPath)
}

// SitePackagesPath is this image's Python site-packages directory, needed to
// locate the per-service dashboard panel-enable destination directory.
// Kubernetes VolumeMount paths must be fixed strings, but kolla_extend_start
// itself detects this dynamically (`python3 --version`) -- if this image's
// Python version ever changes, this constant needs updating to match, or the
// dashboard-setup init container will fail loudly (permission denied writing
// to the real, unshadowed site-packages path) rather than silently
// misbehave. Confirmed via a real WSGI traceback: Python 3.9.
const SitePackagesPath = "/usr/lib/python3.9/site-packages"

// DashboardEnabledSeedPath is where the dashboard-setup init container seeds
// the "dashboard-enabled" EmptyDir -- deliberately not the final
// destination, so the init container still sees the image's own baked-in
// directory (if any) un-shadowed by the EmptyDir mount, mirroring the same
// pattern used for the theme directory.
const DashboardEnabledSeedPath = "/var/lib/openstack/dashboard-enabled-seed"

// DashboardEnabledFinalPath is where kolla_extend_start's config_dashboard
// function copies/removes per-service dashboard panel files
// (openstack_dashboard/local/enabled/*.py), and where the main httpd
// container needs to see the result for those panels to actually appear.
const DashboardEnabledFinalPath = SitePackagesPath + "/openstack_dashboard/local/enabled"

// GetDashboardEnabledVolume - EmptyDir shared between the dashboard-setup
// init container and the main httpd container.
func GetDashboardEnabledVolume() corev1.Volume {
	return volume.WritableDirVolume("dashboard-enabled")
}

// GetDashboardEnabledVolumeMount - the main container's mount of the
// "dashboard-enabled" EmptyDir at its real, final path.
func GetDashboardEnabledVolumeMount() corev1.VolumeMount {
	return volume.WritableDirVolumeMount("dashboard-enabled", DashboardEnabledFinalPath)
}

// DashboardStaticPath is Horizon's STATIC_ROOT, matching httpd.conf's own
// "Alias /dashboard/static" target. Unlike the theme directory or the
// dashboard-enabled directory, this EmptyDir is never seeded from the
// image's original content -- "manage.py collectstatic --clear" always
// deletes and rebuilds it from scratch from each installed app's own
// (untouched, read-only) static sources, so seeding it first would just be
// immediately-discarded work.
const DashboardStaticPath = "/usr/share/openstack-dashboard/static"

// GetDashboardStaticVolume - EmptyDir shared between the dashboard-setup
// init container (which populates it via collectstatic/compress) and the
// main httpd container (which serves it).
func GetDashboardStaticVolume() corev1.Volume {
	return volume.WritableDirVolume("dashboard-static")
}

// GetDashboardStaticVolumeMount - the main container's mount of the
// "dashboard-static" EmptyDir at its real, final path.
func GetDashboardStaticVolumeMount() corev1.VolumeMount {
	return volume.WritableDirVolumeMount("dashboard-static", DashboardStaticPath)
}

// GetSettingsHashVolume - EmptyDir for dashboard_setup's own settings-hash
// scratch file. Only ever mounted in the dashboard-setup init container --
// the main httpd container has no use for it. Not seeded: the hash file's
// only job here is to exist somewhere writable so the script doesn't abort
// via "set -o errexit" trying to write it: since dashboard-enabled always
// starts empty, config_dashboard's copies always fire for every always-on
// service, which always forces static regeneration regardless of what this
// hash file says.
func GetSettingsHashVolume() corev1.Volume {
	return volume.WritableDirVolume("settings-hash")
}

// GetSettingsHashVolumeMount -
func GetSettingsHashVolumeMount() corev1.VolumeMount {
	return volume.WritableDirVolumeMount("settings-hash", "/var/lib/openstack/settings-hash")
}
