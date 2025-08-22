package horizon

import (
	"testing"

	horizonv1 "github.com/openstack-k8s-operators/horizon-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/lib-common/modules/common/tls"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

type testCase struct {
	name            string
	instance        *horizonv1.Horizon
	expectedVolumes []corev1.Volume
	expectedMounts  []corev1.VolumeMount
	expectedError   bool
	errorContains   string
}

func TestFormatTLSOptions(t *testing.T) {

	var tlsSecretName = "generic-tls-secret" // #nosec G101
	var defaultMode int32 = 256
	testCases := []testCase{
		{
			name: "Valid TLS Configuration",
			instance: &horizonv1.Horizon{
				Spec: horizonv1.HorizonSpec{
					HorizonSpecCore: horizonv1.HorizonSpecCore{
						TLS: tls.SimpleService{
							GenericService: tls.GenericService{
								SecretName: &tlsSecretName,
							},
						},
					},
				},
			},
			expectedVolumes: []corev1.Volume{
				{
					Name: "horizon-tls-certs",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName:  "generic-tls-secret",
							DefaultMode: &defaultMode,
						},
					},
				},
				{Name: "combined-ca-bundle",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName:  "combined-ca-bundle",
							DefaultMode: &defaultMode,
						},
					},
				},
				{
					Name: "config-data",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							DefaultMode: &defaultMode,
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "horizon-config-data",
							},
						},
					},
				},
				{
					Name: "horizon-secret-key",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName:  ServiceName,
							DefaultMode: &defaultMode,
						},
					},
				},
			},

			expectedMounts: []corev1.VolumeMount{
				{
					Name:             "horizon-tls-certs",
					MountPath:        "/var/lib/config-data/tls/certs/horizon.crt",
					ReadOnly:         true,
					SubPath:          "tls.crt",
					MountPropagation: nil,
					SubPathExpr:      "",
				},
				{
					Name:             "combined-ca-bundle",
					ReadOnly:         true,
					SubPath:          "tls-ca-bundle.pem",
					MountPath:        "/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem",
					MountPropagation: nil,
					SubPathExpr:      "",
				},
				{
					Name:             "horizon-tls-certs",
					ReadOnly:         true,
					SubPath:          "tls.key",
					MountPath:        "/var/lib/config-data/tls/private/horizon.key",
					MountPropagation: nil,
					SubPathExpr:      "",
				},
			},
			expectedError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			options := &TLSRequiredOptions{
				containerPort:  &corev1.ContainerPort{},
				livenessProbe:  &corev1.Probe{ProbeHandler: corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{}}},
				readinessProbe: &corev1.Probe{ProbeHandler: corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{}}},
				startupProbe:   &corev1.Probe{ProbeHandler: corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{}}},
			}

			err := options.formatTLSOptions(tc.instance)

			if tc.expectedError {
				assert.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
			} else {
				assert.NoError(t, err)
				for _, elem := range options.volumes {
					assert.Contains(t, tc.expectedVolumes, elem)
				}
				for _, elem := range options.volumeMounts {
					assert.Contains(t, tc.expectedMounts, elem)
				}

				assert.Equal(t, corev1.URISchemeHTTPS, options.livenessProbe.HTTPGet.Scheme)
				assert.Equal(t, corev1.URISchemeHTTPS, options.readinessProbe.HTTPGet.Scheme)
				assert.Equal(t, corev1.URISchemeHTTPS, options.startupProbe.HTTPGet.Scheme)
				assert.Equal(t, int32(HorizonPortTLS), options.containerPort.ContainerPort)

			}
		})
	}
}
