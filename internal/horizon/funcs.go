package horizon

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

// HttpdSecurityContext -
func HttpdSecurityContext() *corev1.SecurityContext {
	return &corev1.SecurityContext{
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{
				"MKNOD",
			},
		},
		RunAsUser:                ptr.To(ApacheUID),
		RunAsGroup:               ptr.To(KollaUID),
		AllowPrivilegeEscalation: ptr.To(true),
	}
}
