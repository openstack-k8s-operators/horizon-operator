package functional_test

import (
	"fmt"
	"os"

	. "github.com/onsi/ginkgo/v2" //revive:disable:dot-imports
	. "github.com/onsi/gomega"    //revive:disable:dot-imports

	//revive:disable-next-line:dot-imports
	. "github.com/openstack-k8s-operators/lib-common/modules/common/test/helpers"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	"github.com/openstack-k8s-operators/horizon-operator/internal/horizon"
	memcachedv1 "github.com/openstack-k8s-operators/infra-operator/apis/memcached/v1beta1"
	topologyv1 "github.com/openstack-k8s-operators/infra-operator/apis/topology/v1beta1"
	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
)

var _ = Describe("Horizon controller", func() {

	var horizonName types.NamespacedName
	var deploymentName types.NamespacedName
	var memcachedSpec memcachedv1.MemcachedSpec
	var horizonTopologies []types.NamespacedName

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
			MemcachedSpecCore: memcachedv1.MemcachedSpecCore{
				Replicas: ptr.To[int32](3),
			},
		}
		horizonTopologies = []types.NamespacedName{
			{
				Namespace: namespace,
				Name:      fmt.Sprintf("%s-topology", horizonName.Name),
			},
			{
				Namespace: namespace,
				Name:      fmt.Sprintf("%s-topology-alt", horizonName.Name),
			},
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
			}, timeout, interval).Should(ContainElement("openstack.org/horizon"))
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
				condition.CreateServiceReadyCondition,
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
				condition.CreateServiceReadyCondition,
				corev1.ConditionUnknown,
			)
		})
	})

	When("No secret is provided", func() {
		BeforeEach(func() {
			horizonSpec := GetDefaultHorizonSpec()
			horizonSpec["secret"] = ""

			DeferCleanup(th.DeleteInstance, CreateHorizon(horizonName, horizonSpec))
		})

		It("Should set the inputReady condition to false", func() {

			var missingDependenciesReason condition.Reason = "missing dependencies"
			var missingDependenciesMessage = "missing openstack secret"

			th.ExpectConditionWithDetails(
				horizonName,
				ConditionGetterFunc(HorizonConditionGetter),
				condition.InputReadyCondition,
				corev1.ConditionFalse,
				missingDependenciesReason,
				missingDependenciesMessage,
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
				condition.CreateServiceReadyCondition,
				corev1.ConditionUnknown,
			)
		})
	})

	When("keystoneAPI instance is available", func() {
		var keystoneAPIName types.NamespacedName

		BeforeEach(func() {
			DeferCleanup(th.DeleteInstance, CreateHorizon(horizonName, GetDefaultHorizonSpec()))
			DeferCleanup(
				k8sClient.Delete, ctx, CreateHorizonSecret(namespace, SecretName))
			DeferCleanup(infra.DeleteMemcached, infra.CreateMemcached(namespace, "memcached", memcachedSpec))
			infra.SimulateMemcachedReady(types.NamespacedName{
				Name:      "memcached",
				Namespace: namespace,
			})
			keystoneAPIName = keystone.CreateKeystoneAPI(namespace)
			DeferCleanup(keystone.DeleteKeystoneAPI, keystoneAPIName)
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
				condition.CreateServiceReadyCondition,
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
				ContainSubstring(
					fmt.Sprintf(
						"'LOCATION': [ 'memcached-0.memcached.%s.svc:11211','memcached-1.memcached.%s.svc:11211','memcached-2.memcached.%s.svc:11211' ]",
						horizonName.Namespace, horizonName.Namespace, horizonName.Namespace,
					)))
		})

		It("updates the KeystoneAuthURL if keystone internal endpoint changes", func() {
			newInternalEndpoint := "https://keystone-internal"

			keystone.UpdateKeystoneAPIEndpoint(keystoneAPIName, "internal", newInternalEndpoint)
			logger.Info("Reconfigured")

			Eventually(func(g Gomega) {
				cm := th.GetConfigMap(types.NamespacedName{
					Namespace: horizonName.Namespace,
					Name:      horizonName.Name + "-config-data",
				})
				g.Expect(cm).ShouldNot(BeNil())

				conf := string(cm.Data["local_settings.py"])
				g.Expect(string(conf)).Should(
					ContainSubstring("OPENSTACK_KEYSTONE_URL = \"%s/v3\"", newInternalEndpoint))
			}, timeout, interval).Should(Succeed())
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
		It("should set default environment in deployment", func() {
			// Assert that the watcher deployment is created
			deployment := th.GetDeployment(deploymentName)
			Expect(deployment.Spec.Template.Spec.Containers[1].Env).
				To(ContainElement(corev1.EnvVar{Name: "ENABLE_WATCHER", Value: "no", ValueFrom: nil}))
			Expect(deployment.Spec.Template.Spec.Containers[1].Env).
				To(ContainElement(corev1.EnvVar{Name: "ENABLE_OCTAVIA", Value: "yes", ValueFrom: nil}))
		})
		It("Should have liveness, readiness and startup Probes defined", func() {
			deployment := th.GetDeployment(deploymentName)
			Expect(deployment.Spec.Template.Spec.Containers[1].LivenessProbe.ProbeHandler.HTTPGet.Path).To(Equal("/dashboard/auth/login/?next=/dashboard/"))
			Expect(deployment.Spec.Template.Spec.Containers[1].StartupProbe.ProbeHandler.HTTPGet.Path).To(Equal("/dashboard/auth/login/?next=/dashboard/"))
			Expect(deployment.Spec.Template.Spec.Containers[1].ReadinessProbe.ProbeHandler.HTTPGet.Path).To(Equal("/dashboard/auth/login/?next=/dashboard/"))
		})
	})

	When("Deployment rollout is progressing", func() {
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
			th.SimulateDeploymentProgressing(deploymentName)
		})

		It("shows the deployment progressing in DeploymentReadyCondition", func() {
			th.ExpectConditionWithDetails(
				horizonName,
				ConditionGetterFunc(HorizonConditionGetter),
				condition.DeploymentReadyCondition,
				corev1.ConditionFalse,
				condition.RequestedReason,
				condition.DeploymentReadyRunningMessage,
			)
			th.ExpectCondition(
				horizonName,
				ConditionGetterFunc(HorizonConditionGetter),
				condition.ReadyCondition,
				corev1.ConditionFalse,
			)
		})

		It("still shows the deployment progressing in DeploymentReadyCondition when rollout hits ProgressDeadlineExceeded", func() {
			th.SimulateDeploymentProgressDeadlineExceeded(deploymentName)
			th.ExpectConditionWithDetails(
				horizonName,
				ConditionGetterFunc(HorizonConditionGetter),
				condition.DeploymentReadyCondition,
				corev1.ConditionFalse,
				condition.RequestedReason,
				condition.DeploymentReadyRunningMessage,
			)
			th.ExpectCondition(
				horizonName,
				ConditionGetterFunc(HorizonConditionGetter),
				condition.ReadyCondition,
				corev1.ConditionFalse,
			)
		})

		It("reaches Ready when deployment rollout finished", func() {
			th.ExpectConditionWithDetails(
				horizonName,
				ConditionGetterFunc(HorizonConditionGetter),
				condition.DeploymentReadyCondition,
				corev1.ConditionFalse,
				condition.RequestedReason,
				condition.DeploymentReadyRunningMessage,
			)
			th.ExpectCondition(
				horizonName,
				ConditionGetterFunc(HorizonConditionGetter),
				condition.ReadyCondition,
				corev1.ConditionFalse,
			)

			th.SimulateDeploymentReplicaReady(deploymentName)
			th.ExpectCondition(
				horizonName,
				ConditionGetterFunc(HorizonConditionGetter),
				condition.DeploymentReadyCondition,
				corev1.ConditionTrue,
			)

			th.ExpectCondition(
				horizonName,
				ConditionGetterFunc(HorizonConditionGetter),
				condition.ReadyCondition,
				corev1.ConditionTrue,
			)
		})
	})

	When("watcher keystone service exists", func() {
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
			watcherServiceSpec := map[string]any{
				"enabled":            true,
				"passwordSelector":   "WatcherPassword",
				"secret":             "osp-secret",
				"serviceDescription": "Watcher Service",
				"serviceName":        "watcher",
				"serviceType":        "infra-optim",
				"serviceUser":        "watcher",
			}
			watcherServiceraw := map[string]any{
				"apiVersion": "keystone.openstack.org/v1beta1",
				"kind":       "KeystoneService",
				"metadata": map[string]any{
					"name":      "watcher",
					"namespace": namespace,
				},
				"spec": watcherServiceSpec,
			}
			th.CreateUnstructured(watcherServiceraw)
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
		It("should set ENABLE_WATCHER to yes in deployment environment", func() {
			// Assert that the watcher deployment is created
			deployment := th.GetDeployment(deploymentName)
			Expect(deployment.Spec.Template.Spec.Containers[1].Env).
				To(ContainElement(corev1.EnvVar{Name: "ENABLE_WATCHER", Value: "yes", ValueFrom: nil}))
			Expect(deployment.Spec.Template.Spec.Containers[1].Env).
				To(ContainElement(corev1.EnvVar{Name: "ENABLE_OCTAVIA", Value: "yes", ValueFrom: nil}))
		})
	})

	When("Topology is referenced", func() {
		var topologyRef, topologyRefAlt *topologyv1.TopoRef
		BeforeEach(func() {
			// Define the two topology references used in this test
			topologyRef = &topologyv1.TopoRef{
				Name:      horizonTopologies[0].Name,
				Namespace: horizonTopologies[0].Namespace,
			}
			topologyRefAlt = &topologyv1.TopoRef{
				Name:      horizonTopologies[1].Name,
				Namespace: horizonTopologies[1].Namespace,
			}

			// Create Test Topologies
			for _, t := range horizonTopologies {
				// Build the topology Spec
				topologySpec, _ := GetSampleTopologySpec(t.Name)
				infra.CreateTopology(t, topologySpec)
			}
			spec := GetDefaultHorizonSpec()
			spec["topologyRef"] = map[string]any{
				"name": topologyRef.Name,
			}
			DeferCleanup(th.DeleteInstance, CreateHorizon(horizonName, spec))
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

		It("check topology has been applied", func() {
			Eventually(func(g Gomega) {
				tp := infra.GetTopology(types.NamespacedName{
					Name:      topologyRef.Name,
					Namespace: topologyRef.Namespace,
				})
				finalizers := tp.GetFinalizers()
				g.Expect(finalizers).To(HaveLen(1))
				horizon := GetHorizon(horizonName)
				g.Expect(horizon.Status.LastAppliedTopology).ToNot(BeNil())
				g.Expect(horizon.Status.LastAppliedTopology).To(Equal(topologyRef))
				g.Expect(finalizers).To(ContainElement(
					fmt.Sprintf("openstack.org/horizon-%s", horizonName.Name)))
			}, timeout, interval).Should(Succeed())
		})
		It("sets topology in resource specs", func() {
			Eventually(func(g Gomega) {
				_, topologySpecObj := GetSampleTopologySpec(topologyRef.Name)
				g.Expect(th.GetDeployment(deploymentName).Spec.Template.Spec.TopologySpreadConstraints).ToNot(BeNil())
				g.Expect(th.GetDeployment(deploymentName).Spec.Template.Spec.TopologySpreadConstraints).To(Equal(topologySpecObj))
				g.Expect(th.GetDeployment(deploymentName).Spec.Template.Spec.Affinity).To(BeNil())
			}, timeout, interval).Should(Succeed())
		})
		It("updates topology when the reference changes", func() {
			Eventually(func(g Gomega) {
				horizon := GetHorizon(horizonName)
				horizon.Spec.TopologyRef.Name = topologyRefAlt.Name
				g.Expect(k8sClient.Update(ctx, horizon)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				tp := infra.GetTopology(types.NamespacedName{
					Name:      topologyRefAlt.Name,
					Namespace: topologyRefAlt.Namespace,
				})
				finalizers := tp.GetFinalizers()
				g.Expect(finalizers).To(HaveLen(1))
				horizon := GetHorizon(horizonName)
				g.Expect(horizon.Status.LastAppliedTopology).ToNot(BeNil())
				g.Expect(horizon.Status.LastAppliedTopology).To(Equal(topologyRefAlt))
				g.Expect(finalizers).To(ContainElement(
					fmt.Sprintf("openstack.org/horizon-%s", horizonName.Name)))
				// Verify the previous referenced topology has no finalizer
				tp = infra.GetTopology(types.NamespacedName{
					Name:      topologyRef.Name,
					Namespace: topologyRef.Namespace,
				})
				finalizers = tp.GetFinalizers()
				g.Expect(finalizers).To(BeEmpty())
			}, timeout, interval).Should(Succeed())
		})
		It("removes topologyRef from the spec", func() {
			Eventually(func(g Gomega) {
				horizon := GetHorizon(horizonName)
				// Remove the TopologyRef from the existing Horizon .Spec
				horizon.Spec.TopologyRef = nil
				g.Expect(k8sClient.Update(ctx, horizon)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				horizon := GetHorizon(horizonName)
				g.Expect(horizon.Status.LastAppliedTopology).Should(BeNil())
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(th.GetDeployment(deploymentName).Spec.Template.Spec.TopologySpreadConstraints).To(BeNil())
				g.Expect(th.GetDeployment(deploymentName).Spec.Template.Spec.Affinity).ToNot(BeNil())
			}, timeout, interval).Should(Succeed())

			// Verify the existing topologies have no finalizer anymore
			Eventually(func(g Gomega) {
				for _, topology := range horizonTopologies {
					tp := infra.GetTopology(types.NamespacedName{
						Name:      topology.Name,
						Namespace: topology.Namespace,
					})
					finalizers := tp.GetFinalizers()
					g.Expect(finalizers).To(BeEmpty())
				}
			}, timeout, interval).Should(Succeed())
		})
	})

	When("extraMounts are passed", func() {
		var horizonExtraMountsSecretName, horizonExtraMountsPath string
		BeforeEach(func() {
			spec := GetDefaultHorizonSpec()
			horizonExtraMountsPath = "/var/log/foo"
			horizonExtraMountsSecretName = "foo"
			spec["extraMounts"] = GetExtraMounts(horizonExtraMountsSecretName, horizonExtraMountsPath)

			DeferCleanup(th.DeleteInstance, CreateHorizon(horizonName, spec))
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

		It("Check extraMounts of the resulting Deployment", func() {
			// Get Horizon Deployment
			dp := th.GetDeployment(deploymentName)
			// Check the resulting deployment fields
			Expect(dp.Spec.Template.Spec.Volumes).To(HaveLen(5))
			Expect(dp.Spec.Template.Spec.Containers).To(HaveLen(2))
			// Get the horizon container
			container := dp.Spec.Template.Spec.Containers[1]
			// Fail if horizon doesn't have the right number of VolumeMounts
			// entries
			Expect(container.VolumeMounts).To(HaveLen(7))
			// Inspect VolumeMounts and make sure we have the Foo MountPath
			// provided through extraMounts
			th.AssertVolumeMountPathExists(horizonExtraMountsSecretName,
				horizonExtraMountsPath, "", container.VolumeMounts)
		})
	})

	When("nodeSelector is set", func() {
		BeforeEach(func() {
			spec := GetDefaultHorizonSpec()
			spec["nodeSelector"] = map[string]any{
				"foo": "bar",
			}
			DeferCleanup(th.DeleteInstance, CreateHorizon(horizonName, spec))
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

		It("sets nodeSelector in resource specs", func() {
			Eventually(func(g Gomega) {
				g.Expect(th.GetDeployment(deploymentName).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
			}, timeout, interval).Should(Succeed())
		})

		It("updates nodeSelector in resource specs when changed", func() {
			Eventually(func(g Gomega) {
				g.Expect(th.GetDeployment(deploymentName).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				horizon := GetHorizon(horizonName)
				newNodeSelector := map[string]string{
					"foo2": "bar2",
				}
				horizon.Spec.NodeSelector = &newNodeSelector
				g.Expect(k8sClient.Update(ctx, horizon)).Should(Succeed())
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				th.SimulateDeploymentReplicaReady(deploymentName)
				g.Expect(th.GetDeployment(deploymentName).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo2": "bar2"}))
			}, timeout, interval).Should(Succeed())
		})

		It("removes nodeSelector from resource specs when cleared", func() {
			Eventually(func(g Gomega) {
				g.Expect(th.GetDeployment(deploymentName).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				horizon := GetHorizon(horizonName)
				emptyNodeSelector := map[string]string{}
				horizon.Spec.NodeSelector = &emptyNodeSelector
				g.Expect(k8sClient.Update(ctx, horizon)).Should(Succeed())
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				th.SimulateDeploymentReplicaReady(deploymentName)
				g.Expect(th.GetDeployment(deploymentName).Spec.Template.Spec.NodeSelector).To(BeNil())
			}, timeout, interval).Should(Succeed())
		})

		It("removes nodeSelector from resource specs when nilled", func() {
			Eventually(func(g Gomega) {
				g.Expect(th.GetDeployment(deploymentName).Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				horizon := GetHorizon(horizonName)
				horizon.Spec.NodeSelector = nil
				g.Expect(k8sClient.Update(ctx, horizon)).Should(Succeed())
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				th.SimulateDeploymentReplicaReady(deploymentName)
				g.Expect(th.GetDeployment(deploymentName).Spec.Template.Spec.NodeSelector).To(BeNil())
			}, timeout, interval).Should(Succeed())
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
				fmt.Sprintf("TLSInput is missing: %s", CABundleSecretName),
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
				condition.RequestedReason,
				fmt.Sprintf("TLSInput is missing: secrets \"%s in namespace %s\" not found",
					InternalCertSecretName, namespace),
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

			th.ExpectCondition(
				horizonName,
				ConditionGetterFunc(HorizonConditionGetter),
				condition.TLSInputReadyCondition,
				corev1.ConditionTrue,
			)

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

			svcC := d.Spec.Template.Spec.Containers[1]

			// check TLS volume mounts
			th.AssertVolumeMountPathExists(CABundleSecretName, "", "tls-ca-bundle.pem", svcC.VolumeMounts)
			th.AssertVolumeMountPathExists(InternalCertSecretName, "", "tls.key", svcC.VolumeMounts)
			th.AssertVolumeMountPathExists(InternalCertSecretName, "", "tls.crt", svcC.VolumeMounts)

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

			th.ExpectCondition(
				horizonName,
				ConditionGetterFunc(HorizonConditionGetter),
				condition.TLSInputReadyCondition,
				corev1.ConditionTrue,
			)

			th.SimulateDeploymentReplicaReady(deploymentName)

			originalHash := GetEnvVarValue(
				th.GetDeployment(deploymentName).Spec.Template.Spec.Containers[1].Env,
				"CONFIG_HASH",
				"",
			)
			Expect(originalHash).NotTo(BeEmpty())

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
					th.GetDeployment(deploymentName).Spec.Template.Spec.Containers[1].Env,
					"CONFIG_HASH",
					"",
				)
				g.Expect(newHash).NotTo(BeEmpty())
				g.Expect(newHash).NotTo(Equal(originalHash))
			}, timeout, interval).Should(Succeed())
		})
	})
	When("Horizon CR instance is built with NAD", func() {
		var nad map[string][]string
		BeforeEach(func() {
			nadDef := th.CreateNetworkAttachmentDefinition(types.NamespacedName{
				Namespace: namespace,
				Name:      "storage",
			})
			DeferCleanup(th.DeleteInstance, nadDef)
			rawSpec := map[string]any{
				"secret":             SecretName,
				"networkAttachments": []string{"storage"},
				"memcachedInstance":  "memcached",
			}
			DeferCleanup(th.DeleteInstance, CreateHorizon(horizonName, rawSpec))
			DeferCleanup(
				k8sClient.Delete, ctx, CreateHorizonSecret(namespace, SecretName))
			DeferCleanup(infra.DeleteMemcached, infra.CreateMemcached(namespace, "memcached", memcachedSpec))
			infra.SimulateMemcachedReady(types.NamespacedName{
				Name:      "memcached",
				Namespace: namespace,
			})
			keystoneAPI := keystone.CreateKeystoneAPI(namespace)
			DeferCleanup(keystone.DeleteKeystoneAPI, keystoneAPI)
			nad = map[string][]string{deploymentName.Namespace + "/storage": {"172.18.0.1"}}
			th.SimulateDeploymentReadyWithPods(
				horizonName,
				nad,
			)

		})
		It("Check the resulting endpoints of the generated sub-CRs", func() {
			Eventually(func(g Gomega) {
				horizon := GetHorizon(horizonName)
				g.Expect(horizon.Spec.NetworkAttachments).To(Equal([]string{"storage"}))
				g.Expect(nad).To(Equal(horizon.Status.NetworkAttachments))
			}, timeout, interval).Should(Succeed())
		})
	})
})
