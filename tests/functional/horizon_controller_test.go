package functional_test

import (
	"fmt"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	"github.com/openstack-k8s-operators/horizon-operator/pkg/horizon"
	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	. "github.com/openstack-k8s-operators/lib-common/modules/common/test/helpers"

	memcachedv1 "github.com/openstack-k8s-operators/infra-operator/apis/memcached/v1beta1"
)

var _ = Describe("Horizon controller", func() {

	var horizonName types.NamespacedName
	var deploymentName types.NamespacedName
	var memcachedSpec memcachedv1.MemcachedSpec

	BeforeEach(func() {
		horizonName = types.NamespacedName{
			Name:      "horizon",
			Namespace: namespace,
		}
		deploymentName = types.NamespacedName{
			Name:      "horizon",
			Namespace: horizonName.Namespace,
		}
		memcachedSpec = memcachedv1.MemcachedSpec{
			Replicas: ptr.To[int32](3),
		}

		// lib-common uses OPERATOR_TEMPLATES env var to locate the "templates"
		// directory of the operator. We need to set them othervise lib-common
		// will fail to generate the ConfigMap as it does not find common.sh
		err := os.Setenv("OPERATOR_TEMPLATES", "../../templates")
		Expect(err).NotTo(HaveOccurred())
	})

	When("A Horizon instance is created", func() {
		BeforeEach(func() {
			DeferCleanup(th.DeleteInstance, CreateHorizon(horizonName, GetDefaultHorizonSpec()))
		})

		It("should have the Spec and Status fields initialized", func() {
			horizon := GetHorizon(horizonName)
			Expect(horizon.Spec.Secret).Should(Equal("test-osp-secret"))
			Expect(*(horizon.Spec.Replicas)).Should(Equal(int32(1)))
		})

		It("should have a finalizer", func() {
			// the reconciler loop adds the finalizer so we have to wait for
			// it to run
			Eventually(func() []string {
				return GetHorizon(horizonName).Finalizers
			}, timeout, interval).Should(ContainElement("Horizon"))
		})

		It("should have Unknown Conditions initialized", func() {
			th.ExpectCondition(
				horizonName,
				ConditionGetterFunc(HorizonConditionGetter),
				condition.ReadyCondition,
				corev1.ConditionFalse,
			)
			th.ExpectCondition(
				horizonName,
				ConditionGetterFunc(HorizonConditionGetter),
				condition.InputReadyCondition,
				corev1.ConditionFalse,
			)

			for _, cond := range []condition.Type{
				condition.MemcachedReadyCondition,
				condition.ServiceConfigReadyCondition,
				condition.ExposeServiceReadyCondition,
				condition.DeploymentReadyCondition,
				condition.TLSInputReadyCondition,
			} {
				th.ExpectCondition(
					horizonName,
					ConditionGetterFunc(HorizonConditionGetter),
					cond,
					corev1.ConditionUnknown,
				)
			}
		})
	})

	When("the proper secret is provided", func() {
		BeforeEach(func() {
			DeferCleanup(th.DeleteInstance, CreateHorizon(horizonName, GetDefaultHorizonSpec()))
			DeferCleanup(
				k8sClient.Delete, ctx, CreateHorizonSecret(namespace, SecretName))
		})
		It("should have input ready", func() {
			th.ExpectCondition(
				horizonName,
				ConditionGetterFunc(HorizonConditionGetter),
				condition.ReadyCondition,
				corev1.ConditionFalse,
			)
			th.ExpectCondition(
				horizonName,
				ConditionGetterFunc(HorizonConditionGetter),
				condition.InputReadyCondition,
				corev1.ConditionTrue,
			)
			th.ExpectCondition(
				horizonName,
				ConditionGetterFunc(HorizonConditionGetter),
				condition.MemcachedReadyCondition,
				corev1.ConditionFalse,
			)
			th.ExpectCondition(
				horizonName,
				ConditionGetterFunc(HorizonConditionGetter),
				condition.ExposeServiceReadyCondition,
				corev1.ConditionUnknown,
			)
		})
	})

	When("Memcached instance is available", func() {
		BeforeEach(func() {
			DeferCleanup(th.DeleteInstance, CreateHorizon(horizonName, GetDefaultHorizonSpec()))
			DeferCleanup(
				k8sClient.Delete, ctx, CreateHorizonSecret(namespace, SecretName))
			DeferCleanup(infra.DeleteMemcached, infra.CreateMemcached(namespace, "memcached", memcachedSpec))
			infra.SimulateMemcachedReady(types.NamespacedName{
				Name:      "memcached",
				Namespace: namespace,
			})
		})
		It("should have memcached ready", func() {
			th.ExpectCondition(
				horizonName,
				ConditionGetterFunc(HorizonConditionGetter),
				condition.ReadyCondition,
				corev1.ConditionFalse,
			)
			th.ExpectCondition(
				horizonName,
				ConditionGetterFunc(HorizonConditionGetter),
				condition.MemcachedReadyCondition,
				corev1.ConditionTrue,
			)
			th.ExpectCondition(
				horizonName,
				ConditionGetterFunc(HorizonConditionGetter),
				condition.ServiceConfigReadyCondition,
				corev1.ConditionFalse,
			)
			th.ExpectCondition(
				horizonName,
				ConditionGetterFunc(HorizonConditionGetter),
				condition.ExposeServiceReadyCondition,
				corev1.ConditionUnknown,
			)
		})
	})

	When("keystoneAPI instance is available", func() {
		BeforeEach(func() {
			DeferCleanup(th.DeleteInstance, CreateHorizon(horizonName, GetDefaultHorizonSpec()))
			DeferCleanup(
				k8sClient.Delete, ctx, CreateHorizonSecret(namespace, SecretName))
			DeferCleanup(infra.DeleteMemcached, infra.CreateMemcached(namespace, "memcached", memcachedSpec))
			infra.SimulateMemcachedReady(types.NamespacedName{
				Name:      "memcached",
				Namespace: namespace,
			})
			keystoneAPI := keystone.CreateKeystoneAPI(namespace)
			DeferCleanup(keystone.DeleteKeystoneAPI, keystoneAPI)
		})

		It("should have service config ready and expose service ready", func() {
			th.ExpectCondition(
				horizonName,
				ConditionGetterFunc(HorizonConditionGetter),
				condition.ReadyCondition,
				corev1.ConditionFalse,
			)
			th.ExpectCondition(
				horizonName,
				ConditionGetterFunc(HorizonConditionGetter),
				condition.ServiceConfigReadyCondition,
				corev1.ConditionTrue,
			)
			th.ExpectCondition(
				horizonName,
				ConditionGetterFunc(HorizonConditionGetter),
				condition.ExposeServiceReadyCondition,
				corev1.ConditionTrue,
			)
			th.ExpectCondition(
				horizonName,
				ConditionGetterFunc(HorizonConditionGetter),
				condition.DeploymentReadyCondition,
				corev1.ConditionFalse,
			)
		})
		It("should create a ConfigMap for local_settings", func() {
			cm := th.GetConfigMap(types.NamespacedName{
				Namespace: horizonName.Namespace,
				Name:      horizonName.Name + "-config-data",
			})
			Expect(cm.Data["local_settings.py"]).Should(
				ContainSubstring("OPENSTACK_KEYSTONE_URL = \"http://keystone-internal.openstack.svc:5000/v3\""))
			Expect(cm.Data["local_settings.py"]).Should(
				ContainSubstring("'LOCATION': [ 'memcached-0.memcached:11211', 'memcached-1.memcached:11211', 'memcached-2.memcached:11211' ]"))
		})
	})

	When("deployment is ready", func() {
		BeforeEach(func() {
			DeferCleanup(th.DeleteInstance, CreateHorizon(horizonName, GetDefaultHorizonSpec()))
			DeferCleanup(
				k8sClient.Delete, ctx, CreateHorizonSecret(namespace, SecretName))
			DeferCleanup(infra.DeleteMemcached, infra.CreateMemcached(namespace, "memcached", memcachedSpec))
			infra.SimulateMemcachedReady(types.NamespacedName{
				Name:      "memcached",
				Namespace: namespace,
			})
			keystoneAPI := keystone.CreateKeystoneAPI(namespace)
			DeferCleanup(keystone.DeleteKeystoneAPI, keystoneAPI)
			th.SimulateDeploymentReplicaReady(deploymentName)
		})

		It("should have deployment ready", func() {
			th.ExpectCondition(
				horizonName,
				ConditionGetterFunc(HorizonConditionGetter),
				condition.ReadyCondition,
				corev1.ConditionTrue,
			)
			th.ExpectCondition(
				horizonName,
				ConditionGetterFunc(HorizonConditionGetter),
				condition.DeploymentReadyCondition,
				corev1.ConditionTrue,
			)
		})
		It("should have ReadyCount set", func() {
			Eventually(func() int32 {
				return GetHorizon(horizonName).Status.ReadyCount
			}, timeout, interval).Should(Equal(int32(1)))
		})
	})

	When("TLS is enabled", func() {
		BeforeEach(func() {
			DeferCleanup(th.DeleteInstance, CreateHorizon(horizonName, GetTLSHorizonSpec()))
			DeferCleanup(
				k8sClient.Delete, ctx, CreateHorizonSecret(namespace, SecretName))
			DeferCleanup(infra.DeleteMemcached, infra.CreateMemcached(namespace, "memcached", memcachedSpec))
			infra.SimulateMemcachedReady(types.NamespacedName{
				Name:      "memcached",
				Namespace: namespace,
			})
			keystoneAPI := keystone.CreateKeystoneAPI(namespace)
			DeferCleanup(keystone.DeleteKeystoneAPI, keystoneAPI)
		})

		It("reports that the CA secret is missing", func() {
			th.ExpectConditionWithDetails(
				horizonName,
				ConditionGetterFunc(HorizonConditionGetter),
				condition.TLSInputReadyCondition,
				corev1.ConditionFalse,
				condition.ErrorReason,
				fmt.Sprintf("TLSInput error occured in TLS sources Secret %s/combined-ca-bundle not found", namespace),
			)
			th.ExpectCondition(
				horizonName,
				ConditionGetterFunc(HorizonConditionGetter),
				condition.ReadyCondition,
				corev1.ConditionFalse,
			)
		})

		It("reports that the cert secret is missing", func() {
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCABundleSecret(types.NamespacedName{
				Name:      CABundleSecretName,
				Namespace: namespace,
			}))
			th.ExpectConditionWithDetails(
				horizonName,
				ConditionGetterFunc(HorizonConditionGetter),
				condition.TLSInputReadyCondition,
				corev1.ConditionFalse,
				condition.ErrorReason,
				fmt.Sprintf("TLSInput error occured in TLS sources Secret %s/horizon-tls-certs not found", namespace),
			)
			th.ExpectCondition(
				horizonName,
				ConditionGetterFunc(HorizonConditionGetter),
				condition.ReadyCondition,
				corev1.ConditionFalse,
			)
		})

		It("creates a Deployment for horizon with TLS certs attached", func() {
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCABundleSecret(types.NamespacedName{
				Name:      CABundleSecretName,
				Namespace: namespace,
			}))
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(types.NamespacedName{
				Name:      InternalCertSecretName,
				Namespace: namespace,
			}))

			th.SimulateDeploymentReplicaReady(deploymentName)

			th.ExpectCondition(
				horizonName,
				ConditionGetterFunc(HorizonConditionGetter),
				condition.ServiceConfigReadyCondition,
				corev1.ConditionTrue,
			)

			d := th.GetDeployment(deploymentName)

			// check TLS volumes
			th.AssertVolumeExists(CABundleSecretName, d.Spec.Template.Spec.Volumes)
			th.AssertVolumeExists(InternalCertSecretName, d.Spec.Template.Spec.Volumes)

			svcC := d.Spec.Template.Spec.Containers[0]

			// check TLS volume mounts
			th.AssertVolumeMountExists(CABundleSecretName, "tls-ca-bundle.pem", svcC.VolumeMounts)
			th.AssertVolumeMountExists(InternalCertSecretName, "tls.key", svcC.VolumeMounts)
			th.AssertVolumeMountExists(InternalCertSecretName, "tls.crt", svcC.VolumeMounts)

			// check port and scheme for the container/probes
			Expect(svcC.Ports[0].ContainerPort).To(Equal(horizon.HorizonPortTLS))
			Expect(svcC.Ports[0].Name).To(Equal(horizon.HorizonPortName))
			Expect(svcC.StartupProbe.HTTPGet.Scheme).To(Equal(corev1.URISchemeHTTPS))
			Expect(svcC.StartupProbe.HTTPGet.Port.StrVal).To(Equal(horizon.HorizonPortName))
			Expect(svcC.ReadinessProbe.HTTPGet.Scheme).To(Equal(corev1.URISchemeHTTPS))
			Expect(svcC.ReadinessProbe.HTTPGet.Port.StrVal).To(Equal(horizon.HorizonPortName))
			Expect(svcC.LivenessProbe.HTTPGet.Scheme).To(Equal(corev1.URISchemeHTTPS))
			Expect(svcC.LivenessProbe.HTTPGet.Port.StrVal).To(Equal(horizon.HorizonPortName))
		})

		It("reconfigures the horizon pods when CA changes", func() {
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCABundleSecret(types.NamespacedName{
				Name:      CABundleSecretName,
				Namespace: namespace,
			}))
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(types.NamespacedName{
				Name:      InternalCertSecretName,
				Namespace: namespace,
			}))

			th.SimulateDeploymentReplicaReady(deploymentName)

			originalHash := GetEnvVarValue(
				th.GetDeployment(deploymentName).Spec.Template.Spec.Containers[0].Env,
				"CONFIG_HASH",
				"",
			)
			Expect(originalHash).NotTo(BeEmpty())

			originalSecret := th.GetSecret(types.NamespacedName{
				Name:      horizon.ServiceName,
				Namespace: namespace,
			})
			Expect(originalSecret.Data).To(HaveKey("horizon-secret"))

			// Change the content of the CA secret
			th.UpdateSecret(types.NamespacedName{
				Name:      CABundleSecretName,
				Namespace: namespace,
			},
				"tls-ca-bundle.pem",
				[]byte("DifferentCAData"),
			)

			// Assert that the deployment is updated
			Eventually(func(g Gomega) {
				newHash := GetEnvVarValue(
					th.GetDeployment(deploymentName).Spec.Template.Spec.Containers[0].Env,
					"CONFIG_HASH",
					"",
				)
				g.Expect(newHash).NotTo(BeEmpty())
				g.Expect(newHash).NotTo(Equal(originalHash))
				newSecret := th.GetSecret(types.NamespacedName{
					Name:      horizon.ServiceName,
					Namespace: namespace,
				})
				g.Expect(newSecret.Data).To(HaveKey("horizon-secret"))
				g.Expect(newSecret.Data["horizon-secret"]).NotTo(Equal(originalSecret.Data["horizon-secret"]))
			}, timeout, interval).Should(Succeed())
		})
	})
})
